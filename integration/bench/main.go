//go:build integration

// 压测基准：对 5 种存储（memory/redis/mysql/etcd/leveldb）跑分布式锁的
// 延迟与吞吐基准，输出 markdown 报告。
//
// 运行：go run ./bench [--services]  （在 go-storage-lock-test 目录）
// 依赖外部服务：Redis(6379)、MySQL(13307)、etcd(2379)；缺失的自动跳过。
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/storage-lock/go-etcd-storage"
	"github.com/storage-lock/go-leveldb-storage"
	"github.com/storage-lock/go-memory-storage"
	mysql_storage "github.com/storage-lock/go-mysql-storage"
	"github.com/storage-lock/go-redis-storage"
	"github.com/storage-lock/go-storage"
	storage_lock "github.com/storage-lock/go-storage-lock"
)

type backend struct {
	name    string
	storage storage.Storage
}

func main() {
	iters := flag.Int("iters", 2000, "operations per latency benchmark")
	concurrent := flag.Int("concurrent", 8, "goroutines for throughput benchmark")
	flag.Parse()

	ctx := context.Background()

	backends := []backend{}
	add := func(name string, s storage.Storage, err error) {
		if err != nil {
			fmt.Printf("[skip] %s: %v\n", name, err)
			return
		}
		backends = append(backends, backend{name, s})
	}

	add("memory", memory_storage.NewMemoryStorage(), nil)

	// Redis（缺省用本机无密码实例；可用 STORAGE_LOCK_REDIS_URI 覆盖）
	uri := os.Getenv("STORAGE_LOCK_REDIS_URI")
	if uri == "" {
		uri = "redis://localhost:6379/0"
	}
	mgr := redis_storage.NewRedisConnectionManagerFromURI(uri)
	s, err := redis_storage.NewRedisStorage(ctx, redis_storage.NewRedisStorageOptions().SetConnectionManager(mgr))
	add("redis", s, err)

	// MySQL（DSN 缺省用本地测试容器）
	dsn := os.Getenv("STORAGE_LOCK_MYSQL_DSN")
	if dsn == "" {
		dsn = "root:rootpass@tcp(127.0.0.1:13307)/storage_lock_test?parseTime=true&charset=utf8mb4&loc=Local"
	}
	mysqlMgr := mysql_storage.NewMysqlConnectionManagerFromDsn(dsn)
	if s, err := mysql_storage.NewMysqlStorage(ctx, mysql_storage.NewMysqlStorageOptions().SetConnectionManager(mysqlMgr)); err == nil {
		backends = append(backends, backend{"mysql", s})
	} else {
		fmt.Printf("[skip] mysql: %v\n", err)
	}

	// etcd
	endpoints := []string{"127.0.0.1:2379"}
	if s, err := etcd_storage.NewEtcdStorage(ctx, etcd_storage.NewEtcdStorageOptionsWithEndpoints(endpoints)); err == nil {
		backends = append(backends, backend{"etcd", s})
	} else {
		fmt.Printf("[skip] etcd: %v\n", err)
	}

	// LevelDB
	dir, _ := os.MkdirTemp("", "storage-lock-bench-leveldb")
	defer os.RemoveAll(dir)
	if s, err := leveldb_storage.NewLevelDBStorage(ctx, leveldb_storage.NewLevelDBStorageOptionsWithPath(dir)); err == nil {
		backends = append(backends, backend{"leveldb", s})
	} else {
		fmt.Printf("[skip] leveldb: %v\n", err)
	}

	// 每个后端都要 Init（建表/建目录）
	for _, b := range backends {
		if err := b.storage.Init(ctx); err != nil {
			fmt.Printf("[skip] %s init: %v\n", b.name, err)
		}
	}

	fmt.Printf("# Storage Lock 压测基准报告\n\n")
	fmt.Printf("生成时间：%s\n\n", time.Now().Format("2006-01-02 15:04:05"))
	fmt.Printf("环境：Linux %s, Go %s, 单机本进程内测量（不含跨机网络）\n\n", os.Getenv("HOSTNAME"), goVersion())
	fmt.Println("| 存储 | 无竞争加锁延迟 (μs) | 无竞争解锁延迟 (μs) | 并发竞争吞吐 (ops/s) | 加锁 P99 (μs) |")
	fmt.Println("|------|---------------------|---------------------|---------------------|---------------|")

	var results []result

	for _, b := range backends {
		r := benchBackend(ctx, b, *iters, *concurrent)
		results = append(results, r)
		fmt.Printf("| %s | %.1f | %.1f | %d | %.1f |\n",
			b.name,
			float64(r.lockAvg.Microseconds()),
			float64(r.unlockAvg.Microseconds()),
			r.throughput,
			float64(r.lockP99.Microseconds()),
		)
	}

	fmt.Println("\n## 方法与解读")
	fmt.Println("")
	fmt.Println("- **无竞争延迟**：单 goroutine 串行 Lock/UnLock 各 `iters` 次，取平均。衡量单次 CAS 的固有代价。")
	fmt.Println("- **并发竞争吞吐**：`concurrent` 个 goroutine 并发抢同一把锁（Lock→立即 UnLock），统计每秒成功次数。")
	fmt.Println("- **P99**：无竞争延迟的第 99 百分位，衡量尾延迟。")
	fmt.Println("- 看门狗续租不计入（测量的是临界区外的锁操作本身）。")
	fmt.Println("- 同一介质的结果受部署（本机 vs 容器）影响，跨机器比较才有意义；本报告用于相对对比。")
}

func goVersion() string {
	if v := os.Getenv("GOVERSION"); v != "" {
		return v
	}
	return "unknown"
}

func newLock(b backend, lockId string) *storage_lock.StorageLock {
	options := storage_lock.NewStorageLockOptionsWithLockId(lockId)
	options.LeaseExpireAfter = 3 * time.Second
	options.LeaseRefreshInterval = time.Second
	options.DisableWatchDog = true // 压测不跑看门狗，锁操作本身才是测量对象
	lock, err := storage_lock.NewStorageLockWithOptions(b.storage, options)
	if err != nil {
		panic(err)
	}
	return lock
}

func benchBackend(ctx context.Context, b backend, iters, concurrent int) result {
	lock := newLock(b, "bench-lock")

	// 预热
	_ = lock.Lock(ctx, "warm")
	_ = lock.UnLock(ctx, "warm")

	// 无竞争延迟
	var lockTimes, unlockTimes []time.Duration
	owner := "bench-owner"
	for i := 0; i < iters; i++ {
		t0 := time.Now()
		if err := lock.Lock(ctx, owner); err != nil {
			panic(err)
		}
		lockTimes = append(lockTimes, time.Since(t0))
		t0 = time.Now()
		if err := lock.UnLock(ctx, owner); err != nil {
			panic(err)
		}
		unlockTimes = append(unlockTimes, time.Since(t0))
	}

	// 并发竞争吞吐
	var wg sync.WaitGroup
	var ops atomic.Int64
	start := time.Now()
	for g := 0; g < concurrent; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			owner := fmt.Sprintf("conc-%d", g)
			roundCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			for {
				if err := lock.Lock(roundCtx, owner); err != nil {
					return // 超时结束
				}
				_ = lock.UnLock(roundCtx, owner)
				ops.Add(1)
			}
		}(g)
	}
	wg.Wait()
	elapsed := time.Since(start)

	// P99
	sort.Slice(lockTimes, func(i, j int) bool { return lockTimes[i] < lockTimes[j] })
	p99 := lockTimes[len(lockTimes)*99/100]

	return result{
		name:       b.name,
		lockAvg:    avg(lockTimes),
		unlockAvg:  avg(unlockTimes),
		lockP99:    p99,
		throughput: int(float64(ops.Load()) / elapsed.Seconds()),
	}
}

type result struct {
	name       string
	lockAvg    time.Duration
	unlockAvg  time.Duration
	lockP99    time.Duration
	throughput int
}

func avg(ds []time.Duration) time.Duration {
	var sum time.Duration
	for _, d := range ds {
		sum += d
	}
	return sum / time.Duration(len(ds))
}
