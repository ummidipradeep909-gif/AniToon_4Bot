package moderation

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"
)

func integrationRedis(t *testing.T) *redis.Client {
	t.Helper()
	raw := os.Getenv("REDIS_TEST_URL")
	if raw == "" { t.Skip("REDIS_TEST_URL is not configured") }
	opts, err := redis.ParseURL(raw)
	if err != nil { t.Fatalf("parse REDIS_TEST_URL: %v", err) }
	opts.PoolSize = 32
	opts.MaxRetries = 3
	client := redis.NewClient(opts)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil { client.Close(); t.Fatalf("Redis is not ready: %v", err) }
	t.Cleanup(func(){ _ = client.Close() })
	return client
}

func TestFloodWindowScriptConcurrentBurst(t *testing.T) {
	client := integrationRedis(t)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	key := fmt.Sprintf("test:flood:%d", time.Now().UnixNano())
	t.Cleanup(func(){ _ = client.Del(context.Background(), key).Err() })

	const workers = 256
	now := time.Now().UnixMilli()
	var wg sync.WaitGroup
	counts := make(chan int64, workers)
	errs := make(chan error, workers)
	for i:=0; i<workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			member := fmt.Sprintf("message:%d", i)
			n, err := floodWindowScript.Run(ctx, client, []string{key},
				now-10_000, now, member, 10_000).Int64()
			if err != nil { errs <- err; return }
			counts <- n
		}(i)
	}
	wg.Wait()
	close(counts); close(errs)
	for err := range errs { t.Errorf("concurrent flood script: %v", err) }
	var max int64
	seen := 0
	for n := range counts { seen++; if n > max { max = n } }
	if seen != workers { t.Fatalf("completed %d/%d concurrent requests", seen, workers) }
	if max != workers { t.Fatalf("maximum atomic burst count = %d, want %d", max, workers) }
	if got, err := client.ZCard(ctx,key).Result(); err != nil || got != workers {
		t.Fatalf("stored burst size = %d, err=%v; want %d", got, err, workers)
	}
}

func TestFloodAndRaidCountersRemainIsolated(t *testing.T) {
	client := integrationRedis(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	suffix := time.Now().UnixNano()
	floodA := fmt.Sprintf("test:flood:a:%d", suffix)
	floodB := fmt.Sprintf("test:flood:b:%d", suffix)
	raidA := fmt.Sprintf("test:raid:a:%d", suffix)
	t.Cleanup(func(){ _ = client.Del(context.Background(), floodA, floodB, raidA).Err() })
	now := time.Now().UnixMilli()
	if _,err:=floodWindowScript.Run(ctx,client,[]string{floodA},now-1000,now,"one",5000).Int64();err!=nil{t.Fatal(err)}
	if _,err:=floodWindowScript.Run(ctx,client,[]string{floodB},now-1000,now,"two",5000).Int64();err!=nil{t.Fatal(err)}
	if _,err:=raidWindowScript.Run(ctx,client,[]string{raidA},now-1000,now,"three",5000).Int64();err!=nil{t.Fatal(err)}
	for key,want:=range map[string]int64{floodA:1,floodB:1,raidA:1} {
		if got,err:=client.ZCard(ctx,key).Result();err!=nil||got!=want{t.Fatalf("%s count=%d err=%v want=%d",key,got,err,want)}
	}
}

func TestRedisClientRecoversAfterConnectionsAreClosed(t *testing.T) {
	client := integrationRedis(t)
	raw := os.Getenv("REDIS_TEST_URL")
	opts, err := redis.ParseURL(raw)
	if err != nil { t.Fatal(err) }
	admin := redis.NewClient(opts)
	defer admin.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil { t.Fatal(err) }
	// Close the test client's pooled TCP connections without stopping Redis.
	if _,err:=admin.Do(ctx,"CLIENT","KILL","TYPE","normal","SKIPME","yes").Result();err!=nil{t.Fatalf("close pooled Redis connections: %v",err)}
	if err:=client.Ping(ctx).Err();err!=nil{t.Fatalf("Redis client did not reconnect after its pooled connections were closed: %v",err)}
}
