#### 通用查询采集工具，对接categraf或promethues。查询结果入时序库，实现监控告警



### 目前支持类型

1. MySQL
2. Oracle
3. PostgreSQL
4. Redis
5. MongoDB
6. ElasticSearch

#### 

#### 1.编译

下载或clone 代码，执行 ` go build -o queryCollector main.go`，会在当前目录生成 `queryCollector` 文件。

#### 2.准备配置文件 config.toml

将`config/config\_example.toml` 复制成 `config/config.toml`  ，并修改其中的内容

```
配置项请看注释说明，按需修改
```

#### 3.运行

将 编译好的二进制文件`queryCollector`、`config/config.toml` 2个文件放到某个目录（比如`/data/queryCollector`）

执行`/data/queryCollector/queryCollector -config=/data/queryCollector/conf/config.toml`测试

#### 4\. 观察日志

