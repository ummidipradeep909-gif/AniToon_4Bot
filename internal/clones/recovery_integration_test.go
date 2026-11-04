package clones

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

var integrationSchemaCounter atomic.Uint64

func cloneIntegrationPool(t *testing.T) (*pgxpool.Pool, string, *pgxpool.Pool) {
	t.Helper()
	dsn:=os.Getenv("CLONE_TEST_DATABASE_URL")
	if dsn=="" { t.Skip("CLONE_TEST_DATABASE_URL is not configured") }
	ctx,cancel:=context.WithTimeout(context.Background(),45*time.Second); defer cancel()
	admin,err:=pgxpool.New(ctx,dsn); if err!=nil { t.Fatalf("connect PostgreSQL admin pool: %v",err) }
	schema:=fmt.Sprintf("clone_recovery_%d_%d",os.Getpid(),integrationSchemaCounter.Add(1))
	if _,err=admin.Exec(ctx,"CREATE SCHEMA "+schema);err!=nil{admin.Close();t.Fatalf("create isolated schema: %v",err)}
	cfg,err:=pgxpool.ParseConfig(dsn);if err!=nil{admin.Close();t.Fatal(err)}
	cfg.ConnConfig.RuntimeParams["search_path"]=schema
	cfg.ConnConfig.RuntimeParams["application_name"]=schema
	pool,err:=pgxpool.NewWithConfig(ctx,cfg);if err!=nil{_,_=admin.Exec(ctx,"DROP SCHEMA "+schema+" CASCADE");admin.Close();t.Fatal(err)}
	if err=migrations.Apply(ctx,pool);err!=nil{pool.Close();_,_=admin.Exec(ctx,"DROP SCHEMA "+schema+" CASCADE");admin.Close();t.Fatalf("apply schema migrations: %v",err)}
	t.Cleanup(func(){pool.Close();_,_=admin.Exec(context.Background(),"DROP SCHEMA "+schema+" CASCADE");admin.Close()})
	return pool,schema,admin
}

func TestConcurrentCloneCredentialRegistrationEnforcesQuota(t *testing.T) {
	pool,_,_:=cloneIntegrationPool(t)
	store:=credentialStore{db:pool}
	const workers,quota=32,7
	var wg sync.WaitGroup
	var successes atomic.Int64
	var quotaErrors atomic.Int64
	errs:=make(chan error,workers)
	for i:=0;i<workers;i++{
		wg.Add(1)
		go func(i int){
			defer wg.Done()
			row:=credential{BotID:int64(900000+i),OwnerID:int64(800000+i),Ciphertext:[]byte(fmt.Sprintf("cipher-%d",i)),Nonce:[]byte("0123456789ab")}
			err:=store.insert(context.Background(),row,quota)
			switch err {
			case nil: successes.Add(1)
			case errCloneQuotaReached: quotaErrors.Add(1)
			default: errs<-err
			}
		}(i)
	}
	wg.Wait();close(errs)
	for err:=range errs{t.Errorf("unexpected concurrent registration error: %v",err)}
	if got:=successes.Load();got!=quota{t.Fatalf("successful registrations=%d want=%d",got,quota)}
	if got:=quotaErrors.Load();got!=workers-quota{t.Fatalf("quota rejections=%d want=%d",got,workers-quota)}
	var rows int
	if err:=pool.QueryRow(context.Background(),"SELECT COUNT(*) FROM clone_credentials").Scan(&rows);err!=nil{t.Fatal(err)}
	if rows!=quota{t.Fatalf("persisted clone credentials=%d want=%d",rows,quota)}
}

func TestCloneCredentialLifecycleAndOwnerIsolation(t *testing.T) {
	pool,_,_:=cloneIntegrationPool(t)
	store:=credentialStore{db:pool}
	ctx:=context.Background()
	first:=credential{BotID:910001,OwnerID:810001,Ciphertext:[]byte("cipher-a"),Nonce:[]byte("0123456789ab")}
	second:=credential{BotID:910002,OwnerID:810002,Ciphertext:[]byte("cipher-b"),Nonce:[]byte("0123456789ab")}
	if err:=store.insert(ctx,first,10);err!=nil{t.Fatal(err)}
	if err:=store.insert(ctx,second,10);err!=nil{t.Fatal(err)}
	if err:=store.setEnabled(ctx,first.BotID,first.OwnerID,false);err!=nil{t.Fatal(err)}
	got,err:=store.getByOwner(ctx,first.OwnerID);if err!=nil{t.Fatal(err)}
	if got.BotID!=first.BotID||got.Enabled{t.Fatalf("first owner's clone state crossed or was not persisted: %+v",got)}
	other,err:=store.getByOwner(ctx,second.OwnerID);if err!=nil{t.Fatal(err)}
	if other.BotID!=second.BotID||!other.Enabled{t.Fatalf("second owner's clone was affected: %+v",other)}
}

func TestPostgresPoolRecoversAfterBackendTermination(t *testing.T) {
	pool,schema,admin:=cloneIntegrationPool(t)
	ctx,cancel:=context.WithTimeout(context.Background(),10*time.Second);defer cancel()
	if err:=pool.Ping(ctx);err!=nil{t.Fatal(err)}
	tag,err:=admin.Exec(ctx,"SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE application_name=$1 AND pid<>pg_backend_pid()",schema)
	if err!=nil{t.Fatalf("terminate test pool connections: %v",err)}
	_ = tag
	deadline:=time.Now().Add(8*time.Second)
	for {
		err=pool.Ping(ctx)
		if err==nil{break}
		if time.Now().After(deadline){t.Fatalf("pool failed to reconnect after backend termination: %v",err)}
		time.Sleep(100*time.Millisecond)
	}
	var one int
	if err:=pool.QueryRow(ctx,"SELECT 1").Scan(&one);err!=nil||one!=1{t.Fatalf("query after pool recovery = %d, err=%v",one,err)}
}
