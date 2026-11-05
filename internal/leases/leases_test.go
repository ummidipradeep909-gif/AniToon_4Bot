package leases

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ummidipradeep909-gif/AniToon_4Bot/migrations"
)

var leaseTestSchemaID atomic.Uint64

func leaseTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("CLONE_TEST_DATABASE_URL")
	if dsn == "" { t.Skip("CLONE_TEST_DATABASE_URL is not configured") }
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil { t.Fatalf("connect test PostgreSQL: %v", err) }
	schema := fmt.Sprintf("polling_lease_test_%d_%d", os.Getpid(), leaseTestSchemaID.Add(1))
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+schema); err != nil { admin.Close(); t.Fatalf("create isolated schema: %v", err) }
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil { admin.Close(); t.Fatal(err) }
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	cfg.ConnConfig.RuntimeParams["application_name"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil { _, _ = admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); admin.Close(); t.Fatal(err) }
	if err = migrations.Apply(ctx, pool); err != nil { pool.Close(); _, _ = admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); admin.Close(); t.Fatalf("apply migrations: %v", err) }
	t.Cleanup(func() { pool.Close(); _, _ = admin.Exec(context.Background(), "DROP SCHEMA "+schema+" CASCADE"); admin.Close() })
	return pool
}

func TestConcurrentAcquisitionHasOneWinner(t *testing.T) {
	pool := leaseTestPool(t)
	storeA, storeB := NewStore(pool), NewStore(pool)
	type result struct { lease Lease; ok bool; err error }
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for _, item := range []struct{ store *Store; id string }{{storeA, "instance-a"}, {storeB, "instance-b"}} {
		wg.Add(1)
		go func(s *Store, id string) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			lease, ok, err := s.Acquire(ctx, 700001, id, DefaultLeaseTTL)
			results <- result{lease:lease, ok:ok, err:err}
		}(item.store, item.id)
	}
	wg.Wait()
	close(results)
	var winner Lease
	wins := 0
	for r := range results {
		if r.err != nil { t.Fatalf("acquire lease: %v", r.err) }
		if r.ok { wins++; winner = r.lease }
	}
	if wins != 1 { t.Fatalf("got %d lease owners, want exactly one", wins) }
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if released, err := storeForInstance(winner.InstanceID, storeA, storeB).Release(ctx, winner); err != nil || !released {
		t.Fatalf("release winner: released=%v err=%v", released, err)
	}
}

func storeForInstance(id string, a, b *Store) *Store {
	// Both stores use the same database; the durable lease itself carries the owner.
	_ = id
	return a
}

func TestFencingTokenRejectsStaleRenewAndRelease(t *testing.T) {
	pool := leaseTestPool(t)
	a, b := NewStore(pool), NewStore(pool)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	old, ok, err := a.Acquire(ctx, 700002, "old-instance", 150*time.Millisecond)
	if err != nil || !ok { t.Fatalf("first acquire: ok=%v err=%v", ok, err) }
	if _, ok, err = b.Acquire(ctx, 700002, "new-instance", time.Second); err != nil || ok {
		t.Fatalf("live lease should block second instance: ok=%v err=%v", ok, err)
	}
	time.Sleep(250 * time.Millisecond)
	current, ok, err := b.Acquire(ctx, 700002, "new-instance", time.Second)
	if err != nil || !ok { t.Fatalf("takeover after expiry: ok=%v err=%v", ok, err) }
	if current.FencingToken <= old.FencingToken { t.Fatalf("fencing token did not increase: old=%d new=%d", old.FencingToken, current.FencingToken) }
	if renewed, err := a.Renew(ctx, old, time.Second); err != nil || renewed { t.Fatalf("stale owner renewed: renewed=%v err=%v", renewed, err) }
	if released, err := a.Release(ctx, old); err != nil || released { t.Fatalf("stale owner released new lease: released=%v err=%v", released, err) }
	if renewed, err := b.Renew(ctx, current, time.Second); err != nil || !renewed { t.Fatalf("current owner could not renew: renewed=%v err=%v", renewed, err) }
	if released, err := b.Release(ctx, current); err != nil || !released { t.Fatalf("current owner could not release: released=%v err=%v", released, err) }
}

func TestCoordinatorRejectsInvalidLeaseTiming(t *testing.T) {
	if _, err := newCoordinator(nil, "instance", DefaultLeaseTTL, DefaultRenewInterval); err == nil {
		t.Fatal("nil PostgreSQL pool accepted")
	}
}
