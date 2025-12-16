package engines

import (
	"fmt"
	"github.com/gomodule/redigo/redis"
	"github.com/sirupsen/logrus"
	"strings"
	"time"
)

// MyError 自定义错误类型
type MyError struct {
	ErrorMessage string
}

// 实现 error 接口的 Error 方法
func (e *MyError) Error() string {
	return fmt.Sprintf("Error:%s", e.ErrorMessage)
}

func RedisNewPool(conn string, pass string, db int) *redis.Pool {
	// 建立连接池
	return &redis.Pool{
		MaxIdle:     4, // 最初连接数量
		MaxActive:   0, // 连接池最大连接数量,不确定可以用0（0表示自动定义），按需分配
		IdleTimeout: 300,
		// Wait:        true,
		Dial: func() (redis.Conn, error) {
			con, err := redis.Dial("tcp", conn,
				redis.DialPassword(pass),
				redis.DialDatabase(db),
				redis.DialConnectTimeout(3*time.Second),
				redis.DialReadTimeout(3*time.Second),
				redis.DialWriteTimeout(3*time.Second))
			if err != nil {
				return nil, err
			}
			return con, nil
		},
	}
}

// QueryRedis Query 统一对外的查询方法
func QueryRedis(conn redis.Conn, key string, redisType string) (v float64, err error) {
	tmp := strings.SplitN(key, ",", -1)
	// for _, e := range tmp {
	// 	logrus.Debugf("TMP each:--%s--", e)
	// }
	//
	key = strings.TrimSpace(tmp[0])
	// 使用 EXISTS 命令检查键是否存在
	exists, _ := redis.Bool(conn.Do("EXISTS", key))
	if !exists {
		err1 := &MyError{ErrorMessage: fmt.Sprintf("%s Not Exists.", key)}
		return -1, err1
	}

	if redisType == "string" {
		v, err = redis.Float64(conn.Do("GET", key))
	} else if redisType == "hash" {
		if len(tmp) > 1 {
			// 获取hash key
			hashKey := ""
			for _, e := range tmp[1:] {
				if e != "" {
					hashKey = strings.TrimSpace(e)
					break
				}
			}
			if hashKey != "" {
				logrus.Debugf("hashKey:%s", hashKey)
				v, err = redis.Float64(conn.Do("HGET", key, hashKey))
				logrus.Debugf("value:%v", v)
			} else {
				err1 := &MyError{ErrorMessage: "获取hash key名失败"}
				return -1, err1
			}
		} else {
			err1 := &MyError{ErrorMessage: "Key格式错误，请使用,分隔Redis key名和hash key名"}
			return -1, err1
		}

	} else if redisType == "list" {
		res, err1 := redis.Int64(conn.Do("LLEN", key))
		v, err = float64(res), err1
	} else if redisType == "set" {
		res, err1 := redis.Int64(conn.Do("SCARD", key))
		v, err = float64(res), err1
	} else if redisType == "zset" {
		res, err1 := redis.Int64(conn.Do("ZCARD", key))
		v, err = float64(res), err1
	} else {
		// 创建一个自定义错误
		err1 := &MyError{ErrorMessage: "Not supported yet."}
		return -1, err1
	}

	if err != nil {
		return -1, err
	} else {
		return v, nil
	}
}
