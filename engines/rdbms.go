package engines

import (
	"database/sql"
	"errors"
	_ "github.com/go-sql-driver/mysql" // 导入包但不使用，init()
	"github.com/godror/godror"
	_ "github.com/lib/pq"
	"github.com/sirupsen/logrus"
	"strconv"
	"strings"
)

func InitDB(dbType, dsn string, maxConn int) (db *sql.DB, err error) {
	// var db *sql.DB // 连接池对象
	// 数据库
	// 用户名:密码啊@tcp(ip:端口)/数据库的名字
	// dsn := "root:123@tcp(127.0.0.1:3306)/test"
	// 连接数据集
	// dbType取值 mysql|postgres|oracle
	if dbType == "mysql" || dbType == "postgres" {
		db, err = sql.Open(dbType, dsn) // open不会检验用户名和密码
	} else if dbType == "oracle" {
		db, err = sql.Open("godror", dsn)
	} else {
		return nil, errors.New("not support yet")
	}
	if err != nil {
		return nil, err
	}
	err = db.Ping() // 尝试连接数据库
	if err != nil {
		return nil, err
	}
	// fmt.Println("连接数据库成功~")
	// 设置数据库连接池的最大连接数
	// db.SetMaxIdleConns(maxConn)
	// 设置连接池的最大空闲连接数和最大打开连接数
	db.SetMaxIdleConns(maxConn) // 最大空闲连接数
	db.SetMaxOpenConns(maxConn) // 最大打开连接数
	logrus.Debugf("数据库[%s]连接池最大连接数maxConn：%d", strings.Split(dsn, "@")[1], maxConn)
	return db, nil
}

func interfaceToString(value interface{}) string {
	// 断言是否为int64
	if val, ok := value.(int64); ok {
		return strconv.FormatInt(val, 10)
	}
	// oracle查询结果
	if val, ok := value.(godror.Number); ok {
		return val.String()
	}
	// 断言是否为float64
	if val, ok := value.(float64); ok {
		return strconv.FormatFloat(val, 'f', -1, 64)
	}
	// 断言是否为string
	if val, ok := value.(string); ok {
		return val
	}
	if val, ok := value.([]byte); ok {
		return string(val)
	}
	return "nil"
}

// QueryDb Query 统一对外的查询方法
func QueryDb(db *sql.DB, querySQL string) ([]map[string]string, error) {
	// 执行查询
	rows, err := db.Query(querySQL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// 获取列名
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	// 创建用于存储查询结果的切片
	var results []map[string]string

	// 创建用于存储每一行的临时变量
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range columns {
		valuePtrs[i] = &values[i]
	}

	// 遍历结果
	for rows.Next() {
		// 将查询结果扫描到临时变量中
		err := rows.Scan(valuePtrs...)
		if err != nil {
			return nil, err
		}

		// 创建用于存储每一行数据的映射
		rowData := make(map[string]string)

		// 处理每一列的值
		for i, col := range columns {
			val := values[i]
			// fmt.Printf("DEBUG-val:%v,%T\n", val, val)
			var value string
			value = interfaceToString(val)
			rowData[col] = value
		}

		// 将每一行数据添加到结果切片中
		results = append(results, rowData)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}
