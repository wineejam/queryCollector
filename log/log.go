package log

import (
	rotatelogs "github.com/lestrrat/go-file-rotatelogs"
	"github.com/rifflock/lfshook"
	"github.com/sirupsen/logrus"
	"os"
	"path"
	"queryCollector/conf"
	"time"
)

func Writer(logPath, level, logKey string, save, rotationHour uint) *rotatelogs.RotateLogs {
	logFullPath := path.Join(logPath, level)
	var cstSh, _ = time.LoadLocation("Asia/Shanghai") // 上海
	fileSuffix := time.Now().In(cstSh).Format("2006-01-02") + ".log"

	logier, err := rotatelogs.New(
		logFullPath+"-"+fileSuffix,
		rotatelogs.WithLinkName(logKey+"-"+level),                               // 生成软链，指向最新日志文件
		rotatelogs.WithRotationCount(int(save)),                                 // 文件最大保存份数
		rotatelogs.WithRotationTime(time.Hour*time.Duration(int(rotationHour))), // 日志切割时间间隔
	)

	if err != nil {
		panic(err)
	}
	return logier
}

func checkDirAndCreate(dirName string) {
	if _, err := os.Stat(dirName); err != nil {
		err := os.MkdirAll(dirName, 0711)
		if err != nil {
			logrus.Errorln("Error creating directory:", dirName)
			os.Exit(1)
			return
		}
	}
}

func SetLogger(config conf.Config, logPath string, logLevelSetting string, logFormatter logrus.Formatter) {
	/*
		logrus对象统一设置
	*/

	checkDirAndCreate(logPath)

	lfHook := lfshook.NewHook(lfshook.WriterMap{
		logrus.DebugLevel: Writer(logPath, "debug", "log", config.LogSetting.Save, config.LogSetting.Rotation), // 为不同级别设置不同的输出目的
		logrus.InfoLevel:  Writer(logPath, "info", "log", config.LogSetting.Save, config.LogSetting.Rotation),
		logrus.WarnLevel:  Writer(logPath, "warn", "log", config.LogSetting.Save, config.LogSetting.Rotation),
		logrus.ErrorLevel: Writer(logPath, "error", "log", config.LogSetting.Save, config.LogSetting.Rotation),
		logrus.FatalLevel: Writer(logPath, "fatal", "log", config.LogSetting.Save, config.LogSetting.Rotation),
		logrus.PanicLevel: Writer(logPath, "panic", "log", config.LogSetting.Save, config.LogSetting.Rotation),
	}, logFormatter) // 设置日志文件的日志格式
	logrus.AddHook(lfHook)

	// 根据配置文件中的日志级别 或者命令行参数 设置日志级别
	switch logLevelSetting {
	case "debug":
		logrus.SetLevel(logrus.DebugLevel)
	case "info":
		logrus.SetLevel(logrus.InfoLevel)
	case "warn":
		logrus.SetLevel(logrus.WarnLevel)
	case "error":
		logrus.SetLevel(logrus.ErrorLevel)
	default:
		logrus.SetLevel(logrus.InfoLevel)
	}
}
