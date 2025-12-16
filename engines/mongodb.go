package engines

import (
	"context"
	"encoding/json"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

// MergeBSONMaps 合并两个 bson.M 对象并返回一个新的合并后的对象
func MergeBSONMaps(m1, m2 bson.M) bson.M {
	result := make(bson.M)
	// 遍历第一个 bson.M，将其键值对添加到结果中
	for key, value := range m1 {
		result[key] = value
	}
	// 遍历第二个 bson.M，将其键值对添加到结果中
	for key, value := range m2 {
		result[key] = value
	}
	return result
}
func JsonStr2json(jsonStr string) (jsonMap map[string]interface{}, err error) {
	// 定义一个空接口，用于存储解析后的 JSON 数据
	var jsonData interface{}
	// 使用 json.Unmarshal 将 JSON 字符串解析为 JSON 对象
	err = json.Unmarshal([]byte(jsonStr), &jsonData)
	if err != nil {
		return nil, err
	}

	// 使用类型断言将 jsonData 转换为 map[string]interface{}
	jsonMap, ok := jsonData.(map[string]interface{})
	if !ok {
		err1 := &MyError{ErrorMessage: "无法转换为 map[string]interface{}"}
		return nil, err1
	}

	return jsonMap, nil
}

func InitMongo(connDsn string) (*mongo.Client, error) {
	// 连接到 MongoDB 服务器
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	clientOptions := options.Client().ApplyURI(connDsn)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		logrus.Fatal("连接失败，", err)
		return nil, err
	}
	// fmt.Println("Connected to MongoDB!")
	// defer client.Disconnect(ctx)  // 加上 这句连接不上MongoDB
	return client, err
}

// FindDocuments 函数执行通用的 MongoDB 查询
func FindDocuments(collection *mongo.Collection, query interface{}, projection bson.M, sort bson.D) ([]bson.M, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	findOptions := options.Find()
	if projection != nil {
		findOptions.SetProjection(projection)
	}
	if sort != nil {
		findOptions.SetSort(sort)
	}

	cursor, err := collection.Find(ctx, query, findOptions)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []bson.M
	for cursor.Next(ctx) {
		var result bson.M
		err := cursor.Decode(&result)
		if err != nil {
			return nil, err
		}
		results = append(results, result)
	}

	if err := cursor.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

// 将jsonMap转和Bson
func jsonMap2Bson(jsonMap map[string]interface{}) bson.M {
	bsonData := make(bson.M)
	for key, val := range jsonMap {
		if t, ok := val.(map[string]interface{}); ok {
			return bson.M{key: jsonMap2Bson(t)}
		} else if valT, ok := val.(float64); ok {
			tmp := bson.M{key: valT}
			bsonData = MergeBSONMaps(bsonData, tmp)
		} else if valT, ok := val.(string); ok {
			tmp := bson.M{key: valT}
			bsonData = MergeBSONMaps(bsonData, tmp)
		}
	}
	return bsonData
}

func QueryMongo(client *mongo.Client, databaseName, collectionName string, filter string) []bson.M {
	// ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	// defer cancel()
	// fmt.Println("databaseName:", databaseName)
	// 检查集合是否存在
	collectionNames, _ := client.Database(databaseName).ListCollectionNames(context.Background(), bson.D{})
	logrus.Debugf("所有集合:%v", collectionNames)
	exists := false
	for _, name := range collectionNames {
		if name == collectionName {
			exists = true
			break
		}
	}

	if !exists {
		logrus.Errorf("MongoDB Collection[%s] does not exist.", collectionName)
		return nil
	}

	// 获取对应的数据库和集合
	collection := client.Database(databaseName).Collection(collectionName)

	// string转jsonMap
	jsonMap, err := JsonStr2json(filter)
	// jsonMap转BSON
	bsonData := jsonMap2Bson(jsonMap)
	// fmt.Println("bsonData:", bsonData)
	query := bsonData
	// fmt.Printf("query:%+v\n", query)

	// 调用通用查询函数，传入不同的查询条件
	results, err := FindDocuments(collection, query, nil, nil)
	if err != nil {
		logrus.Errorln("查询失败，", err)
	}
	// // 打印查询结果
	// fmt.Printf("MongoDB(%s)查询结果:\n", filter)
	// for _, result := range results {
	// 	fmt.Printf("%+v\n", result)
	// }
	return results

}
