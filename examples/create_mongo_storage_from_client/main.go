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
