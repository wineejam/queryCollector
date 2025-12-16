package conf

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	PrometheusUrl   string
	TargetFileNames []string
	LogSetting      LogSetting
	Targets         Targets
}

type SubConfig struct {
	Targets Targets
}

type Targets struct {
	Redis   []RedisConfig
	Rdbms   []RdbmsConfig
	Es      []EsConfig
	MongoDB []MongodbConfig
}
type EsConfig struct {
	Conn    string
	Metrics []EsMetric
}
type EsMetric struct {
	Cron        string
	Name        string
	Description string
	Indices     string
	Content     string
	Labels      map[string]string
}

type TargetFile struct {
	TargetFileNames []string
}

type LogSetting struct {
	ConsoleLog bool
	Level      string
	Rotation   uint
	Save       uint
}
type RedisConfig struct {
	Conn    string
	Pass    string
	Db      int
	Metrics []RedisMetric
}
type RedisMetric struct {
	Cron        string
	Type        string
	Key         string
	Name        string
	Description string
	Labels      map[string]string
}
type RdbmsConfig struct {
	Conn    string
	Type    string
	MaxConn int
	Metrics []DbMetric
}
type DbMetric struct {
	Cron        string
	Name        string
	Description string
	IsTag       bool
	Content     string
	Labels      map[string]string
}
type MongodbConfig struct {
	Conn    string
	Metrics []MongodbMetric
}
type MongodbMetric struct {
	Cron        string
	Name        string
	Description string
	Collection  string
	QueryType   string
	QueryField  string
	QueryJson   string
	Labels      map[string]string
}

func FileExist(path string) bool {
	_, err := os.Lstat(path)
	return !os.IsNotExist(err)
}

func getConfigPath(configFileName string) string {
	//
	ex, err := os.Executable()
	if err != nil {
		panic(err)
	}
	exPath := filepath.Dir(ex)
	realPath, _ := filepath.EvalSymlinks(exPath)
	// 获取配置文件绝对路径
	configPath := filepath.Join(realPath, configFileName)
	return configPath
}

func (c Config) ReadConfig(configFileName string) Config {
	configFull := configFileName
	if !filepath.IsAbs(configFull) { // 判断路径是否为绝对路径
		configFull = getConfigPath(configFileName) // 相对路径则拼接当前路径
	}
	if !FileExist(configFileName) { // 判断是否存在
		fmt.Printf("Error:配置文件 %s 不存在。\n", configFull)
		os.Exit(1)
	}
	configPath := filepath.Dir(configFileName)
	fileName := filepath.Base(configFileName)
	configName := strings.Split(fileName, ".")[0]
	viper.SetConfigName(configName) // 读取指定配置文件名
	viper.SetConfigType("toml")     // 设置配置文件后缀为toml
	viper.AddConfigPath(configPath) // 配置文件搜索路径
	err := viper.ReadInConfig()
	if err != nil {
		logrus.Fatalf("read config failed: %v", err)
	}

	err = viper.Unmarshal(&c)
	if err != nil {
		logrus.Fatalf("read config failed: %v", err)
	}

	return c
}

func (sc SubConfig) ReadSubConfig(configFileName string) SubConfig {
	configFull := configFileName
	if !filepath.IsAbs(configFull) { // 判断路径是否为绝对路径
		configFull = getConfigPath(configFileName) // 相对路径则拼接当前路径
	}
	if !FileExist(configFileName) { // 判断是否存在
		fmt.Printf("Error:配置文件 %s 不存在。\n", configFull)
		os.Exit(1)
	}
	configPath := filepath.Dir(configFileName)
	fileName := filepath.Base(configFileName)
	configName := strings.Split(fileName, ".")[0]
	viper.SetConfigName(configName) // 读取指定配置文件名
	viper.SetConfigType("toml")     // 设置配置文件后缀为toml
	viper.AddConfigPath(configPath) // 配置文件搜索路径
	if err := viper.ReadInConfig(); err != nil {
		logrus.Fatalf("read config failed: %v", err)
	}

	if err := viper.Unmarshal(&sc); err != nil {
		logrus.Fatalf("read config failed: %v", err)
	}
	return sc
}
