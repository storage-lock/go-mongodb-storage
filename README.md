# Mongodb Storage 

# 一、这是什么

以MongoDB为存储引擎的[Storage](https://github.com/storage-lock/go-storage)实现，当前仓库为比较底层的存储层实现，你可以与[storage-lock](https://github.com/storage-lock/go-storage-lock)结合使用。


# 二、安装

```bash
go get -u github.com/storage-lock/go-mongodb-storage
```

# 三、API示例

## 3.1 从URI创建MongodbStorage

```go
package main

import (
	"context"
	"fmt"
	mongodb_storage "github.com/storage-lock/go-mongodb-storage"
)

func main() {

	// 使用一个uri形式的数据库连接字符串创建ConnectionManager
	uri := "mongodb://root:UeGqAm8CxYGldMDLoNNt@192.168.128.206:27017/?connectTimeoutMS=300000"
	connectionManager := mongodb_storage.NewMongoConnectionManager(uri)

	// 然后从这个ConnectionManager创建MongodbStorage
	options := mongodb_storage.NewMongoStorageOptions().SetConnectionManager(connectionManager)
	storage, err := mongodb_storage.NewMongoStorage(context.Background(), options)
	if err != nil {
		panic(err)
	}
	fmt.Println(storage.GetName())

}
```

## 3.2 从mongo.Client创建MongodbStorage

```go
package main

import (
	"context"
	"fmt"
	mongodb_storage "github.com/storage-lock/go-mongodb-storage"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func main() {

	// 假设已经在其它地方初始化数据库连接得到了一个*mongo.Client
	uri := "mongodb://root:UeGqAm8CxYGldMDLoNNt@192.168.128.206:27017/?connectTimeoutMS=300000"
	client, err := mongo.Connect(context.Background(), options.Client().ApplyURI(uri))
	if err != nil {
		panic(err)
	}

	// 则可以从这个*mongo.Client中创建一个Mongodb Storage
	connectionManager := mongodb_storage.NewMongoConnectionManagerFromClient(client)
	options := mongodb_storage.NewMongoStorageOptions().SetConnectionManager(connectionManager)
	storage, err := mongodb_storage.NewMongoStorage(context.Background(), options)
	if err != nil {
		panic(err)
	}
	fmt.Println(storage.GetName())

}
```

