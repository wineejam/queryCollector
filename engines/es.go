package engines

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/sirupsen/logrus"
	"io"
	"regexp"
	"strings"
)

func InitConn(dsn string) (*elasticsearch.Client, error) {
	// 检查是否为合规格式 http://user:password@ip:port
	checkReg := regexp.MustCompile(`^https?://(.*?:.*?)@\d+\.\d+\.\d+\.\d+:\d+$`)
	if checkReg == nil {
		logrus.Errorf("regexp Compile err")
		return nil, errors.New("regexp Compile err")
	}
	EsHost, EsUser, EsPass := "", "", ""
	if checkReg.MatchString(dsn) {
		userPassReg := regexp.MustCompile(`://.*?:.*?@`)
		repl := "${1}"
		userPass := checkReg.ReplaceAllString(dsn, repl)
		EsHost = userPassReg.ReplaceAllString(dsn, "://")
		EsUser = strings.Split(userPass, ":")[0]
		EsPass = strings.Split(userPass, ":")[1]
	}
	if EsHost == "" {
		return nil, errors.New("ES host 为空，请检查配置。")
	}
	// 创建Elasticsearch客户端
	cfg := elasticsearch.Config{
		Addresses: []string{
			EsHost, // Elasticsearch地址
		},
		Username: EsUser, // 替换为你的用户名
		Password: EsPass, // 替换为你的密码
	}
	es, err := elasticsearch.NewClient(cfg)
	if err != nil {
		logrus.Errorf("连接ES失败，Error: %s", err)
		return nil, err
	}

	// 检查Elasticsearch是否正常运行
	res, err := es.Info()
	if err != nil {
		logrus.Errorf("获取ES版本信息失败，Error: %s", err)
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			logrus.Errorf("Close Error:%s", err)
		}
	}(res.Body)

	return es, nil
}

func QueryES(es *elasticsearch.Client, querySQL string) ([]map[string]string, error) { // 使用SQL查询
	query := fmt.Sprintf(`{"query": "%s"}`, strings.TrimSpace(querySQL))
	fmt.Printf("SQL原始查询语句，可直接在kibana中查询【%s】\n", query)
	//req := esapi.SQLQueryRequest{
	//	Body: strings.NewReader(query),
	//}
	//res, err := req.Do(context.Background(), es)
	res, err := es.SQL.Query(
		bytes.NewReader([]byte(query)),
		es.SQL.Query.WithFormat("json"),
	)
	if err != nil {
		logrus.Errorf("Error executing SQL query: %s", err)
		return nil, err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			logrus.Errorf("Close Error:%s", err)
		}
	}(res.Body)

	if res.IsError() {
		logrus.Errorf("Error in response: %s", res.String())
		return nil, errors.New(res.String())
	}

	// 解析结果
	var response map[string]interface{}
	if err := json.NewDecoder(res.Body).Decode(&response); err != nil {
		logrus.Errorf("Error parsing the response body: %s", err)
		return nil, err
	}

	// 提取并格式化结果
	results := parseSQLResponse(response)
	return results, nil
}

// 解析SQL查询响应并转换为[]map[string]string格式
func parseSQLResponse(response map[string]interface{}) []map[string]string {
	var results []map[string]string

	// 获取列名
	columns := response["columns"].([]interface{})
	columnNames := make([]string, len(columns))
	for i, col := range columns {
		columnNames[i] = col.(map[string]interface{})["name"].(string)
	}

	// 获取行数据
	rows := response["rows"].([]interface{})
	for _, row := range rows {
		rowData := row.([]interface{})
		rowMap := make(map[string]string)
		for i, colValue := range rowData {
			rowMap[columnNames[i]] = fmt.Sprintf("%v", colValue)
		}
		results = append(results, rowMap)
	}

	return results
}
