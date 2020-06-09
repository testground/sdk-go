package peek

import (
	"fmt"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/go-redis/redis/v7"
	"github.com/testground/sdk-go/runtime"
)

var DefaultRedisOpts = redis.Options{
	MinIdleConns:       2,               // allow the pool to downsize to 0 conns.
	PoolSize:           5,               // one for subscriptions, one for nonblocking operations.
	PoolTimeout:        3 * time.Minute, // amount of time a waiter will wait for a conn to become available.
	MaxRetries:         30,
	MinRetryBackoff:    1 * time.Second,
	MaxRetryBackoff:    3 * time.Second,
	DialTimeout:        10 * time.Second,
	ReadTimeout:        10 * time.Second,
	WriteTimeout:       10 * time.Second,
	IdleCheckFrequency: 30 * time.Second,
	MaxConnAge:         2 * time.Minute,
}

func MonitorBarriers(rp *runtime.RunParams) {
	host := "testground-infra-redis-headless"
	port := 6379
	opts := DefaultRedisOpts
	opts.Addr = fmt.Sprintf("%s:%d", host, port)
	client := redis.NewClient(&opts)

	key := fmt.Sprintf("run:%s:plan:%s:case:%s", rp.TestRun, rp.TestPlan, rp.TestCase)

	members, err := client.SMembers(key).Result()
	if err != nil {
		panic(err)
	}

	spew.Dump(members)

	//vals, err := client.MGet(key).Result()
	//if err != nil {
	//panic(err)
	//}

	//v := vals[0]

	//if v == nil {
	//return
	//}

	//curr, err := strconv.ParseInt(v.(string), 10, 64)
	//if err != nil {
	//panic(err)
	//}

	//fmt.Println(curr)
	//spew.Dump(curr)
}
