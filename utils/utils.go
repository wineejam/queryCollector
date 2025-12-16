package utils

import (
	"database/sql"
	"fmt"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/gomodule/redigo/redis"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
	"io"
	"net/http"
	"queryCollector/engines"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func StartRedisJob(n9eUrl string, redisPool *redis.Pool, metric map[string]string, labelsStr string) {
	// redisPool
	connect := redisPool.Get() // 从连接池，取一个链接
	defer func(conn redis.Conn) {
		err := conn.Close()
		if err != nil {
			logrus.Fatalln("Redis 连接失败，error:%v", err)
			return
		}
	}(connect) // 函数运行结束 ，把连接放回连接池
	metricDesc, metricsName := metric["metricDesc"], metric["metricName"]
	redisType := metric["metricType"]
	redisKey := metric["redisKey"]

	msg := fmt.Sprintf("查询Redis[Key:%s][Type:%s]...", redisKey, redisType)
	// 查询Redis数据
	startTime := time.Now()
	result, err := engines.QueryRedis(connect, redisKey, redisType)
	if err != nil {
		msg += "失败。"
		logrus.Errorf("%s 获取redis数据失败，%s", msg, err.Error())
		return
	}
	endTime := time.Now()
	elapsedTime := endTime.Sub(startTime)
	msg += " 成功。"
	logrus.Infof("%s 耗时：%d 毫秒", msg, elapsedTime.Milliseconds())
	result2n9es := make([]map[string]string, 1)
	result2n9es = append(result2n9es, map[string]string{
		"metricsName": metricsName,
		"description": metricDesc,
		"labels":      labelsStr,
		"value":       fmt.Sprintf("%f", result),
	})
	post2N9e(result2n9es, n9eUrl, 3)
}

func StartDbJob(n9eUrl string, dbObj *sql.DB, isTag bool, metric map[string]string, labelsStr string) {
	// 检查是否为合规格式 test_metrics{label="app1",name="demo"} 100.00
	checkReg := regexp.MustCompile(`.*?{.*?}\s+\d+(\.\d+)?$`)
	if checkReg == nil {
		logrus.Errorln("regexp Compile err")
		return
	}
	metricDesc, metricsName := metric["metricDesc"], metric["metricName"]
	sqlContent := metric["sqlContent"]
	sqlContent = strings.Replace(sqlContent, ";", "", -1) // 去除;  oracle执行不能有;
	logrus.Debugf("sqlContent:%s\n", sqlContent)
	msg := fmt.Sprintf("查询Db[Name:%s][Description:%s]...", metricsName, metricDesc)
	startTime := time.Now()
	rows, err := engines.QueryDb(dbObj, sqlContent)
	if err != nil {
		logrus.Errorln("Query DB ERROR,", err)
		msg += "失败。"
		logrus.Errorf("%s Query DB失败，%s", msg, err.Error())
	}
	result2n9es := make([]map[string]string, 0)
	// 循环获取行
	for _, row := range rows {
		if isTag { // tag模式
			tmpResult := make(map[string]string)
			var metricsValue, metricsLabels, sqlDescription string
			for key, value := range row { // 循环获取列
				if strings.ToLower(key) == "count(*)" ||
					strings.ToLower(key) == "count(1)" ||
					strings.ToLower(key) == "total_number" {
					metricsValue = value
				} else {
					metricsLabels = metricsLabels + fmt.Sprintf("%s=\"%s\",", key, value)
				}
				sqlDescription = metricDesc
			}
			metricsLabels = strings.TrimRight(metricsLabels, ",")
			var output string
			if len(metricsLabels) > 0 {
				output = fmt.Sprintf("%s{description=\"%s\",%s} %s", metricsName, sqlDescription, metricsLabels, metricsValue)
			} else {
				output = fmt.Sprintf("%s{description=\"%s\"} %s", metricsName, sqlDescription, metricsValue)
			}
			fmt.Println("===OUTPUT:", output)
			// 符合正则规则的才输出
			if checkReg.MatchString(output) {
				tmpResult["description"] = sqlDescription
				tmpResult["metricsName"] = metricsName
				tmpResult["labels"] = labelsStr
				tmpResult["value"] = metricsValue
				tmpResult["metricsLabels"] = metricsLabels
				result2n9es = append(result2n9es, tmpResult)
			}

		} else {
			// 普通模式
			var sqlDescription string
			for key, value := range row { // 循环获取列
				sqlDescription = metricDesc + "-" + key
				output := fmt.Sprintf("%s{description=\"%s\"} %s", metricsName, sqlDescription, value)
				// 符合正则规则的才输出
				if checkReg.MatchString(output) {
					result2n9es = append(result2n9es, map[string]string{
						"description": sqlDescription,
						"metricsName": metricsName,
						"labels":      labelsStr,
						"value":       value,
					})
				}
			}
		}
	}
	endTime := time.Now()
	elapsedTime := endTime.Sub(startTime)
	logrus.Infof("%s成功，耗时：%d 毫秒", msg, elapsedTime.Milliseconds())
	post2N9e(result2n9es, n9eUrl, 3)
}

func StartEsJob(n9eUrl string, esObj *elasticsearch.Client, metric map[string]string, labelsStr string) {
	// 检查是否为合规格式 test_metrics{label="app1",name="demo"} 100.00
	checkReg := regexp.MustCompile(`.*?{.*?}\s+\d+(\.\d+)?$`)
	if checkReg == nil {
		logrus.Errorln("regexp Compile err")
		return
	}
	metricDesc, metricsName := metric["metricDesc"], metric["metricName"]
	sqlContent := metric["content"]
	indices := metric["indices"]
	if !strings.HasSuffix(indices, "*") { // 如果索引不以*结尾，则自动填充当天日期
		today := time.Now().Format("20060102")
		indices = fmt.Sprintf("%s%s", indices, today)
		sqlContent = strings.Replace(sqlContent, "indices_placeholder", indices, 1)
	} else {
		sqlContent = strings.Replace(sqlContent, "indices_placeholder", fmt.Sprintf("\\\"%s\\\"", indices), 1)
	}
	logrus.Debugf("sqlContent:%s", sqlContent)

	msg := fmt.Sprintf("查询ES[Name:%s][Description:%s]...", metricsName, metricDesc)
	startTime := time.Now()
	rows, err := engines.QueryES(esObj, sqlContent)
	if err != nil {
		logrus.Errorln("Query ES ERROR,", err)
		msg += "失败。"
	}
	result2n9es := make([]map[string]string, 0)
	fmt.Println("QUERY RESULT:", rows)
	var output string
	if len(rows) == 0 { // 查询结果为空
		logrus.Infof("查询结果为空")
		output = fmt.Sprintf("%s{description=\"%s\"} %s", metricsName, metricDesc, "0.00")
		fmt.Println("===OUTPUT:", output)
		result2n9es = append(result2n9es, map[string]string{
			"metricsName": metricsName, "description": metricDesc, "value": "0.00", "labels": labelsStr})
	} else {
		// 循环获取行
		for _, row := range rows {
			tmpResult := make(map[string]string)
			var metricsValue, metricsLabels, sqlDescription string
			for key, value := range row {
				key = strings.ToLower(key)
				// 将contents.前缀去掉
				key = strings.Replace(key, "contents.", "", -1)
				//fmt.Printf("key:%s,value:%s\n", key, value)
				if key == "total_number" ||
					strings.HasPrefix(key, "max") ||
					strings.HasPrefix(key, "min") ||
					strings.HasPrefix(key, "avg") ||
					strings.HasPrefix(key, "count") {
					metricsValue = value
				} else {
					metricsLabels = metricsLabels + fmt.Sprintf("%s=\"%s\",", key, value)
				}
				sqlDescription = metricDesc
			}
			metricsLabels = strings.TrimRight(metricsLabels, ",")
			//fmt.Println("metricsLabels:", metricsLabels)

			if len(metricsLabels) > 0 {
				output = fmt.Sprintf("%s{description=\"%s\",%s} %s", metricsName, sqlDescription, metricsLabels, metricsValue)
			} else {
				output = fmt.Sprintf("%s{description=\"%s\"} %s", metricsName, sqlDescription, metricsValue)
			}
			fmt.Println("===OUTPUT:", output)
			// 符合正则规则的才输出
			if checkReg.MatchString(output) {
				tmpResult["description"] = sqlDescription
				tmpResult["metricsName"] = metricsName
				tmpResult["labels"] = labelsStr
				tmpResult["value"] = metricsValue
				tmpResult["metricsLabels"] = metricsLabels
				result2n9es = append(result2n9es, tmpResult)
			}
		}
	}
	endTime := time.Now()
	elapsedTime := endTime.Sub(startTime)
	logrus.Infof("%s成功，耗时：%d 毫秒", msg, elapsedTime.Milliseconds())
	post2N9e(result2n9es, n9eUrl, 3)
}

func StartMongoJob(n9eUrl string, client *mongo.Client, metric map[string]string, labelsStr string) {
	// 检查是否为合规格式 test_metrics{label="app1",name="demo"} 100.00
	checkReg := regexp.MustCompile(`.*?{.*?}\s+\d+(\.\d+)?$`)
	if checkReg == nil {
		fmt.Println("regexp Compile err")
		return
	}
	databaseName, collection, queryJson, queryType := metric["databaseName"], metric["collection"], metric["queryJson"], metric["queryType"]
	metricDesc, metricsName := metric["metricDesc"], metric["metricName"]
	msg := fmt.Sprintf("查询MongoDB[Name:%s][Description:%s]...", metricsName, metricDesc)
	startTime := time.Now()
	results := engines.QueryMongo(client, databaseName, collection, queryJson)
	if results == nil {
		logrus.Errorf("%s失败", msg)
		return
	}
	metricContent := ""
	result2n9es := make([]map[string]string, 0)
	tmpResult := make(map[string]string)
	if queryType == "COUNT" {
		// 统计总数
		value := len(results)
		metricContent = fmt.Sprintf("%s{description=\"%s\"} %d", metricsName, metricDesc, value)
		if checkReg.MatchString(metricContent) {
			tmpResult["description"] = metricDesc
			tmpResult["metricsName"] = metricsName
			tmpResult["labels"] = labelsStr
			tmpResult["value"] = fmt.Sprintf("%d", value)
		}
		result2n9es = append(result2n9es, tmpResult)
	} else if queryType == "FIELD" {
		queryField := metric["queryField"]
		for _, result := range results {
			value := 0.0
			tmp := result[queryField]
			if valT, ok := tmp.(float64); ok { // 断言类型为float64
				value = valT
			}
			metricContent = fmt.Sprintf("%s{description=\"%s\"} %f", metricsName, metricDesc, value)
			if checkReg.MatchString(metricContent) {
				tmpResult["description"] = metricDesc
				tmpResult["metricsName"] = metricsName
				tmpResult["labels"] = labelsStr
				tmpResult["value"] = fmt.Sprintf("%f", value)
			}
			result2n9es = append(result2n9es, tmpResult)
		}
	} else {
		logrus.Errorln("Not Supported yet.")
	}
	endTime := time.Now()
	elapsedTime := endTime.Sub(startTime)
	logrus.Infof("%s成功，耗时：%d 毫秒", msg, elapsedTime.Milliseconds())
	post2N9e(result2n9es, n9eUrl, 3)
}

// 上报metrics数据到夜莺
func post2N9e(resultInfos []map[string]string, url string, timeout int) {
	for _, resultInfo := range resultInfos {
		var metricsContent string
		if len(resultInfo["metricsName"]) <= 0 {
			// 没有获取到 metricsName则直接跳过
			continue
		}
		if len(resultInfo["labels"]) > 0 {
			if len(resultInfo["metricsLabels"]) > 0 {
				metricsContent += fmt.Sprintf("%s{description=\"%s\",%s,%s} %s\n", resultInfo["metricsName"], resultInfo["description"], resultInfo["labels"], resultInfo["metricsLabels"], resultInfo["value"])
			} else {
				metricsContent += fmt.Sprintf("%s{description=\"%s\",%s} %s\n", resultInfo["metricsName"], resultInfo["description"], resultInfo["labels"], resultInfo["value"])
			}
		} else {
			if len(resultInfo["metricsLabels"]) > 0 {
				metricsContent += fmt.Sprintf("%s{description=\"%s\",%s} %s\n", resultInfo["metricsName"], resultInfo["description"], resultInfo["metricsLabels"], resultInfo["value"])
			} else {
				metricsContent += fmt.Sprintf("%s{description=\"%s\"} %s\n", resultInfo["metricsName"], resultInfo["description"], resultInfo["value"])
			}
		}

		metricsContLen := len([]rune(metricsContent))
		//logrus.Debugf(">metricsContent<: %v", metricsContent)
		if metricsContLen > 0 {
			ch := make(chan string)
			go post(ch, url, timeout, metricsContent)
			respResult := <-ch
			logrus.Infof("上报数据[%s]到n9e...%s", metricsContent, respResult)
		}
	}
}

// http POST请求
func post(ch chan<- string, url string, timeout int, body string) {
	client := &http.Client{
		Timeout: time.Duration(timeout) * time.Second,
	}
	payload := strings.NewReader(body)
	req, err := http.NewRequest("POST", url, payload)
	resp, err := client.Do(req)
	if err != nil {
		ch <- fmt.Sprintf("Send Http Post Request Error:%v", err)
		return
	}
	req.Header.Add("Content-Type", "text/plain")
	respCode := resp.StatusCode
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		ch <- fmt.Sprintf("Send Http Post Request Error:%v", err)
		return
	}
	if respCode == 200 {
		ch <- fmt.Sprintf("成功。[URL:%s]：Code:%s,respBody:%s", url, strconv.Itoa(respCode), string(respBody))
	} else {
		ch <- fmt.Sprintf("失败。[URL:%s]：Code:%s,respBody:%s", url, strconv.Itoa(respCode), string(respBody))
	}
}
