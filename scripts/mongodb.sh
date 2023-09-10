#!/usr/bin/env bash

# 删除之前已经启动过的容器
docker rm -f storage-lock-mongodb

# 启动Docker容器
docker run -d -p 27017:27017 --name storage-lock-mongodb -e MONGO_INITDB_ROOT_USERNAME=root -e MONGO_INITDB_ROOT_PASSWORD=UeGqAm8CxYGldMDLoNNt mongo

# 设置单元测试需要的环境变量
export STORAGE_LOCK_MONGO_URI="mongodb://root:UeGqAm8CxYGldMDLoNNt@127.0.0.1:27017/?connectTimeoutMS=300000"
