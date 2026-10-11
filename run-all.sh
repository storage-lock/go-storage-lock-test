#!/usr/bin/env bash
# 跑全部存储后端的分布式锁语义测试。
#
# 本地后端（memory、filesystem、leveldb、kv、http、sqlite3、rocksdb）不需要任何服务。
# 服务后端需要 docker/docker-compose.yml 里的实例已经在监听，环境变量指向它们。
# 某个服务没启动时，对应的 Test*StorageLockSemantics 会跳过而不是失败。
#
# 用法：
#   docker compose -f docker/docker-compose.yml up -d
#   ./run-all.sh
set -euo pipefail
cd "$(dirname "$0")"

export STORAGE_LOCK_REDIS_URI="${STORAGE_LOCK_REDIS_URI:-redis://localhost:16379}"
export STORAGE_LOCK_ETCD_ENDPOINTS="${STORAGE_LOCK_ETCD_ENDPOINTS:-localhost:2379}"
export STORAGE_LOCK_MYSQL_DSN="${STORAGE_LOCK_MYSQL_DSN:-root:rootpass@tcp(127.0.0.1:13307)/storage_lock_test?parseTime=true}"
export STORAGE_LOCK_POSTGRESQL_DSN="${STORAGE_LOCK_POSTGRESQL_DSN:-postgres://postgres:pgpass@127.0.0.1:15432/storage_lock_test?sslmode=disable}"
export STORAGE_LOCK_MONGO_URI="${STORAGE_LOCK_MONGO_URI:-mongodb://localhost:27017}"
export STORAGE_LOCK_S3_ENDPOINT="${STORAGE_LOCK_S3_ENDPOINT:-localhost:9000}"
export STORAGE_LOCK_S3_ACCESS_KEY="${STORAGE_LOCK_S3_ACCESS_KEY:-minioadmin}"
export STORAGE_LOCK_S3_SECRET_KEY="${STORAGE_LOCK_S3_SECRET_KEY:-minioadmin}"
export STORAGE_LOCK_ZOOKEEPER_HOSTS="${STORAGE_LOCK_ZOOKEEPER_HOSTS:-localhost:2181}"
export STORAGE_LOCK_MEMCACHE_SERVERS="${STORAGE_LOCK_MEMCACHE_SERVERS:-localhost:11212}"
export STORAGE_LOCK_LEDISDB_URI="${STORAGE_LOCK_LEDISDB_URI:-redis://localhost:16380}"

exec go test -count=1 -timeout 90m "$@" .
