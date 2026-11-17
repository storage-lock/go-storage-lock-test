// Package backends 把每个具体存储实现登记进同一套锁回归用例。
//
// 不依赖外部服务的实现始终编译进来。需要服务、驱动或 CGO 的实现放在对应的
// 构建标签后面：没有那个环境时不去下载它的模块，也不把占位库算成通过。
//
// 没有声明 CapabilityCAS 的实现不在这里登记。ClickHouse 没有条件写，
// BerkeleyDB、COS 以及一批只有 NewX(ctx) 的占位库同样没有 CAS；把它们登记进来
// 只会在套件里失败，不能当成锁回归通过。S3 兼容包装复用 S3 的条件写，
// 所以跟 S3 用同一组 STORAGE_LOCK_S3_* 变量。DynamoDB 只接受现成客户端，
// 单有端点仍然跳过。
package backends
