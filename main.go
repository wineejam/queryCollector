package main

import (
	"bufio"
	"dario.cat/mergo"
	"flag"
	"fmt"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
	"log"
	"os"
	"path/filepath"
	"queryCollector/conf"
	"queryCollector/engines"
	mylog "queryCollector/log"
	"queryCollector/utils"
	"regexp"
	"strings"
	"time"
)

var (
	// Version 项目版本信息
	version = ""
	// GoVersion Go版本信息
	goVersion = ""
	// BuildTime 构建时间
	buildTime = ""
	// GitCommit git提交commit id
	GitCommit = ""
)

// PrintVersion 输出版本信息
func printVersion() {
	fmt.Printf("Version: %s\n", version)
	fmt.Printf("Go Version: %s\n", goVersion)
	fmt.Printf("Build Time: %s\n", buildTime)
	fmt.Printf("Git Commit: %s\n", GitCommit)
	os.Exit(0)
}

func main() {
	defer func() {
		err := recover()
		if err != nil {
			panic(err)
		}
	}()

	// 命令行参数
	var configName = flag.String("config", "", "[必填]配置文件名称(Eg. config/config.toml)，相对路径/绝对路径都行")
	var logLevel = flag.String("loglevel", "info", "日志级别。取值：debug,info,warn,error，会覆盖配置文件中的设置")
	var logFormat = flag.String("logformat", "text", "日志输出格式是否为json类型。取值:json,text")
	var debug = flag.Bool("debug", false, "是否调试模式")
	var version = flag.Bool("version", false, "打印构建信息及版本号")

	flag.Parse() // 解析命令行参数

	if *version {
		printVersion()
	}

	// 设置 config为必选项
	if *configName == "" {
		logrus.Fatalln("请使用 -h 参数查看说明。")
	}
	if filepath.Ext(*configName) != ".toml" {
		logrus.Fatalln("配置文件扩展名只支持.toml")
	}

	logrus.SetReportCaller(true) // 调用者文件名与位置
	var logFormatter logrus.Formatter
	if *logFormat == "json" {
		logFormatter = &logrus.JSONFormatter{
			TimestampFormat: "2006-01-02 15:04:05",
			PrettyPrint:     true,
		}
	} else if *logFormat == "text" {
		logFormatter = &logrus.TextFormatter{
			DisableColors:   true,
			ForceQuote:      false,
			TimestampFormat: "2006-01-02 15:04:05",
		}
	} else {
		logrus.Fatalln("logFormat只支持 json|text 取值。")
	}
	logrus.SetFormatter(logFormatter) // 设置输出的日志格式

	// 读取配置文件
	var confObj conf.Config
	configs := confObj.ReadConfig(*configName)
	// 命令行参数日志级别覆盖配置文件中的配置
	var logLevelSetting = configs.LogSetting.Level
	if *logLevel != "" {
		logLevelSetting = *logLevel
	}

	if !configs.LogSetting.ConsoleLog {
		// 根据配置文件中的consoleLog来决定是否打印console日志
		src, err := os.OpenFile(os.DevNull, os.O_APPEND|os.O_WRONLY, os.ModeAppend)
		if err != nil {
			logrus.Fatalln("Open Src File err", err)
		}
		writer := bufio.NewWriter(src)
		logrus.SetOutput(writer)
	}

	// 获取当前可执行文件的绝对路径
	exePath, err := os.Executable()
	if err != nil {
		logrus.Fatalln("获取当前可执行文件的绝对路径失败。")
	}
	// 获取当前可执行文件所在的目录
	curDir := filepath.Dir(exePath)
	mylog.SetLogger(configs, curDir+"/logs", logLevelSetting, logFormatter)

	f, err := os.OpenFile(curDir+"/logs/cron-run.log", os.O_CREATE|os.O_APPEND|os.O_RDWR, os.ModePerm)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	cronLog := log.New(f, "cron-run: ", log.LstdFlags)
	cronJob := cron.New(cron.WithSeconds(), cron.WithChain(cron.DelayIfStillRunning(cron.DefaultLogger)), cron.WithLogger(
		cron.VerbosePrintfLogger(cronLog)))
	if *debug {
		logrus.Debugln("Configs:", configs)
	}
	// 2024.02.22 支持include子配置文件
	// 读取子配置文件
	var subConfObj conf.SubConfig
	for _, targetFileItem := range configs.TargetFileNames {
		var subConfigTargets conf.Targets
		targetFileName := targetFileItem
		subConfigContent := subConfObj.ReadSubConfig(targetFileName)
		subConfigTargets = subConfigContent.Targets
		// 追加合并
		if err = mergo.Merge(&configs.Targets, subConfigTargets, mergo.WithAppendSlice); err != nil {
			logrus.Fatal(err)
		}
	}
	// 遍历Redis指标并添加定时任务
	for _, redisItem := range configs.Targets.Redis {
		redisPool := engines.RedisNewPool(redisItem.Conn, redisItem.Pass, redisItem.Db)
		// 遍历所有指标项
		for _, redisMetric := range redisItem.Metrics {
			labels := redisMetric.Labels
			labelsStr := utils.JoinLabels(labels)
			if labelsStr != "" {
				logrus.Infof("LabelsStr:%s", labelsStr)
			}
			cronReg := redisMetric.Cron // 获取每个指标的定时任务表达式
			redisMetricInfo := map[string]string{
				"metricName": redisMetric.Name,
				"metricType": redisMetric.Type,
				"redisKey":   redisMetric.Key,
				"metricDesc": redisMetric.Description,
			}
			if *debug {
				utils.StartRedisJob(configs.PrometheusUrl, redisPool, redisMetricInfo, labelsStr)
			} else {
				msg := fmt.Sprintf("redisKey:%s,metricType:%s,metricDesc:%s", redisMetric.Key, redisMetric.Type, redisMetric.Description)
				logrus.Infof("添加Cron任务[%s](crontab:%s)完成", msg, cronReg)
				_, err = cronJob.AddFunc(cronReg, func() {
					utils.StartRedisJob(configs.PrometheusUrl, redisPool, redisMetricInfo, labelsStr)
				})
				if err != nil {
					logrus.Fatalf("运行%s cron任务失败，%v", msg, err)
				}
			}
		}
	}

	// 遍历Db指标并添加定时任务
	for _, dbItem := range configs.Targets.Rdbms {
		dbConnInfo := dbItem.Conn
		dbInfo := strings.Split(dbConnInfo, "@")[1]
		if len(dbConnInfo) <= 0 {
			logrus.Fatalln("获取dbConnInfo连接串失败，请检查config.toml")
		}
		dbType := dbItem.Type
		dbMaxConn := dbItem.MaxConn
		// 记录连接耗时
		startTime := time.Now()
		dbObj, err := engines.InitDB(dbType, dbConnInfo, dbMaxConn)
		endTime := time.Now()
		if err != nil {
			logrus.Errorf("连接数据库[%s](%s)失败~,Error:%v", dbInfo, dbType, err)
			continue
		}
		elapsedTime := endTime.Sub(startTime)
		logrus.Infof("连接数据库[%s]成功，耗时：%d 毫秒", dbInfo, elapsedTime.Milliseconds())
		for _, dbMetric := range dbItem.Metrics {
			labels := dbMetric.Labels
			metricName := dbMetric.Name
			isTag := dbMetric.IsTag
			logrus.Debugf("RDBMS,Name:%s,isTag:%v", metricName, isTag)
			labelsStr := utils.JoinLabels(labels)
			if labelsStr != "" {
				logrus.Infof("LabelsStr:%s", labelsStr)
			}
			cronReg := dbMetric.Cron // 获取每个指标的定时任务表达式
			dbMetricInfo := map[string]string{
				"metricName": metricName,
				"sqlContent": dbMetric.Content,
				"metricDesc": dbMetric.Description,
			}
			if *debug {
				utils.StartDbJob(configs.PrometheusUrl, dbObj, isTag, dbMetricInfo, labelsStr)
			} else {
				msg := fmt.Sprintf("Rdbms metricName:%s,metricDesc:%s", dbMetric.Name, dbMetric.Description)
				logrus.Infof("添加Cron任务[%s](crontab:%s)完成", msg, cronReg)
				_, err = cronJob.AddFunc(cronReg, func() {
					utils.StartDbJob(configs.PrometheusUrl, dbObj, isTag, dbMetricInfo, labelsStr)
				})
				if err != nil {
					logrus.Fatalf("运行%s cron任务失败，%v", msg, err)
				}
			}
		}
	}

	// MongoDB
	for _, mongoItem := range configs.Targets.MongoDB {
		mongoConn := mongoItem.Conn
		if len(mongoConn) <= 0 {
			logrus.Fatalln("获取MongoDB连接串失败，请检查config.toml")
		}
		tmp := strings.Split(mongoConn, "/")
		databaseName := tmp[len(tmp)-1]
		startTime := time.Now()
		client, err := engines.InitMongo(mongoConn)
		endTime := time.Now()
		if err != nil {
			logrus.Errorln("连接MongoDB失败，", err)
		}
		elapsedTime := endTime.Sub(startTime)
		logrus.Infof("连接MongoDB[%s]成功，耗时：%d 毫秒", strings.Split(mongoConn, "@")[1], elapsedTime.Milliseconds())
		for _, mongoMetric := range mongoItem.Metrics {
			labels := mongoMetric.Labels
			labelsStr := utils.JoinLabels(labels)
			if labelsStr != "" {
				logrus.Infof("LabelsStr:%s", labelsStr)
			}
			cronReg := mongoMetric.Cron // 获取每个指标的定时任务表达式
			mongoMetricInfo := map[string]string{
				"metricName":   mongoMetric.Name,
				"metricDesc":   mongoMetric.Description,
				"collection":   mongoMetric.Collection,
				"databaseName": databaseName,
				"queryJson":    mongoMetric.QueryJson,
				"queryType":    mongoMetric.QueryType,
				"queryField":   mongoMetric.QueryField,
			}
			if *debug {
				utils.StartMongoJob(configs.PrometheusUrl, client, mongoMetricInfo, labelsStr)
			} else {
				msg := fmt.Sprintf("MongoDB metricName:%s,metricDesc:%s", mongoMetric.Name, mongoMetric.Description)
				logrus.Infof("添加Cron任务[%s](crontab:%s)完成", msg, cronReg)
				_, err = cronJob.AddFunc(cronReg, func() {
					utils.StartMongoJob(configs.PrometheusUrl, client, mongoMetricInfo, labelsStr)
				})
				if err != nil {
					logrus.Fatalf("运行%s cron任务失败，%v", msg, err)
				}
			}
		}
	}

	// 遍历ES指标并添加定时任务
	for _, esItem := range configs.Targets.Es {
		esConnInfo := esItem.Conn
		userPassReg := regexp.MustCompile(`://.*?:.*?@`)
		EsHost := userPassReg.ReplaceAllString(esConnInfo, "://")
		if len(esConnInfo) <= 0 {
			logrus.Fatalln("获取esConnInfo连接串失败，请检查config.toml")
		}
		// 记录连接耗时
		startTime := time.Now()
		esClient, err := engines.InitConn(esConnInfo)
		endTime := time.Now()
		if err != nil {
			logrus.Errorf("连接ES[%s]失败~,Error:%v", EsHost, err)
			continue
		}
		elapsedTime := endTime.Sub(startTime)
		logrus.Infof("连接ES[%s]成功，耗时：%d 毫秒", EsHost, elapsedTime.Milliseconds())
		for _, esMetric := range esItem.Metrics {
			labels := esMetric.Labels
			index := esMetric.Indices
			metricName := esMetric.Name
			if index == "" {
				logrus.Fatalf("%s配置项未指定索引名，请检查config.toml", metricName)
			}
			labelsStr := utils.JoinLabels(labels)
			if labelsStr != "" {
				logrus.Infof("LabelsStr:%s", labelsStr)
			}
			cronReg := esMetric.Cron // 获取每个指标的定时任务表达式
			esMetricInfo := map[string]string{
				"metricName": metricName,
				"indices":    index,
				"content":    esMetric.Content,
				"metricDesc": esMetric.Description,
			}
			if *debug {
				utils.StartEsJob(configs.PrometheusUrl, esClient, esMetricInfo, labelsStr)
			} else {
				msg := fmt.Sprintf("ES metricName:%s,metricDesc:%s", esMetric.Name, esMetric.Description)
				logrus.Infof("添加Cron任务[%s](crontab:%s)完成", msg, cronReg)
				_, err = cronJob.AddFunc(cronReg, func() {
					utils.StartEsJob(configs.PrometheusUrl, esClient, esMetricInfo, labelsStr)
				})
				if err != nil {
					logrus.Fatalf("运行%s cron任务失败，%v", msg, err)
				}
			}
		}
	}
	if !*debug {
		// 启动任务
		cronJob.Start()
		// 不加下面语句也没问题，因为主进程结束，协程自然就被关闭
		defer cronJob.Stop() // 让函数或语句可以在当前函数执行完毕后（包括通过return正常结束或者panic导致的异常结束）执行
		// 因为Start()是启动了一个协程，所以需要使用 select{} 来阻塞，否则主程序将退出
		select {}
	}

}
