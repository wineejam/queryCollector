package conf

import (
	"github.com/spf13/viper"
	"path/filepath"
	"strings"
	"testing"
)

type ConfigT struct {
	Targets Targets
}

func TestConfigRead(t *testing.T) {
	var sc ConfigT
	configFileName := "D:\\WorkSpace\\CODE\\GOLANG\\queryCollector\\config\\b.toml"
	configPath := filepath.Dir(configFileName)
	fileName := filepath.Base(configFileName)
	configName := strings.Split(fileName, ".")[0]
	viper.SetConfigName(configName) // 读取指定配置文件名
	viper.SetConfigType("toml")     // 设置配置文件后缀为toml
	viper.AddConfigPath(configPath) // 配置文件搜索路径
	if err := viper.ReadInConfig(); err != nil {
		t.Errorf("read config failed: %v", err)
	}
	// fmt.Println("all settings: ", viper.AllSettings())
	// fmt.Println("-----", viper.Get("targets"))

	// var sc Targets
	if err := viper.Unmarshal(&sc); err != nil {
		t.Errorf("read config failed: %v", err)
	}
	t.Log(sc)
}
