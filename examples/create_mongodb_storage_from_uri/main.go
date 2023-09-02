package main

import (
	"context"
	"fmt"
	mongodb_storage "github.com/storage-lock/go-mongodb-storage"
)

func main() {

	// 使用一个uri形式的数据库连接字符串创建ConnectionManager
	uri := "mongodb://root:UeGqAm8CxYGldMDLoNNt@192.168.128.206:27017/?connectTimeoutMS=300000"
	connectionManager := mongodb_storage.NewMongoConnectionManagerFromURI(uri)

	// 然后从这个ConnectionManager创建MongodbStorage
	options := mongodb_storage.NewMongoStorageOptions().SetConnectionManager(connectionManager)
	storage, err := mongodb_storage.NewMongoStorage(context.Background(), options)
	if err != nil {
		panic(err)
	}
	fmt.Println(storage.GetName())

}
