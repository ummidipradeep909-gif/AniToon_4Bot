package moderation

import (
 "context"
 "fmt"
 "runtime"
 "strings"
 "time"

 "github.com/PaulSonOfLars/gotgbot/v2"
 "github.com/PaulSonOfLars/gotgbot/v2/ext"
 "github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
 "github.com/jackc/pgx/v5/pgxpool"
 "github.com/redis/go-redis/v9"
)

type cloneHealthReporter interface { HealthSummary() (total,running,degraded,recovering int) }
var dashboardStartedAt=time.Now()

func RegisterOwnerDashboard(d *ext.Dispatcher, ownerID int64, db *pgxpool.Pool, cache *redis.Client, clones cloneHealthReporter) {
 if ownerID<=0 { return }
 d.AddHandler(handlers.NewCommand("dashboard",func(b *gotgbot.Bot,c *ext.Context)error{return ownerDashboard(b,c,ownerID,db,cache,clones)}))
}

func ownerDashboard(b *gotgbot.Bot,c *ext.Context,ownerID int64,db *pgxpool.Pool,cache *redis.Client,clones cloneHealthReporter)error{
 if c.EffectiveChat==nil||c.EffectiveUser==nil||c.EffectiveMessage==nil||c.EffectiveChat.Type!="private" { return nil }
 if c.EffectiveUser.Id!=ownerID { return nil }
 var mem runtime.MemStats;runtime.ReadMemStats(&mem)
 var out strings.Builder
 fmt.Fprintf(&out,"🖥️ AniToon System Dashboard\nUptime: %s\nGo routines: %d\nCPU cores: %d\nMemory in use: %.1f MiB / allocated: %.1f MiB\n",
  time.Since(dashboardStartedAt).Round(time.Second),runtime.NumGoroutine(),runtime.NumCPU(),float64(mem.HeapInuse)/1024/1024,float64(mem.Sys)/1024/1024)
 if db==nil { out.WriteString("PostgreSQL: unavailable\n") } else {
  ctx,cancel:=context.WithTimeout(context.Background(),2*time.Second);err:=db.Ping(ctx);cancel()
  stats:=db.Stat()
  state:="healthy";if err!=nil{state="unavailable"}
  fmt.Fprintf(&out,"PostgreSQL: %s (pool: %d total, %d idle, %d acquired)\n",state,stats.TotalConns(),stats.IdleConns(),stats.AcquiredConns())
 }
 if cache==nil { out.WriteString("Redis: unavailable\n") } else {
  ctx,cancel:=context.WithTimeout(context.Background(),2*time.Second);err:=cache.Ping(ctx).Err();cancel()
  state:="healthy";if err!=nil{state="unavailable"}
  fmt.Fprintf(&out,"Redis: %s\n",state)
 }
 if clones!=nil { total,running,degraded,recovering:=clones.HealthSummary();fmt.Fprintf(&out,"Clones: %d registered | %d running | %d degraded | %d recovering\n",total,running,degraded,recovering) }
 out.WriteString("\nMetrics are process-local snapshots. CPU core count is shown; per-process CPU percentage requires an OS-specific sampler.")
 _,err:=b.SendMessage(c.EffectiveChat.Id,out.String(),nil);return err
}
