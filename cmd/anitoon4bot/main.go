package main

import (
 "context"
 "log/slog"
 "os"
 "os/signal"
 "syscall"
 "time"

 "github.com/PaulSonOfLars/gotgbot/v2"
 "github.com/PaulSonOfLars/gotgbot/v2/ext"
 "github.com/jackc/pgx/v5/pgxpool"
 "github.com/redis/go-redis/v9"

 "github.com/ummidipradeep909-gif/AniToon_4Bot/internal/aiassistant"
 "github.com/ummidipradeep909-gif/AniToon_4Bot/internal/bot"
 "github.com/ummidipradeep909-gif/AniToon_4Bot/internal/clones"
 "github.com/ummidipradeep909-gif/AniToon_4Bot/internal/config"
 "github.com/ummidipradeep909-gif/AniToon_4Bot/internal/moderation"
 "github.com/ummidipradeep909-gif/AniToon_4Bot/migrations"
)

func main() {
 cfg,err:=config.Load()
 if err!=nil { slog.Error("configuration error","error",err); os.Exit(1) }
 logger:=slog.New(slog.NewTextHandler(os.Stdout,&slog.HandlerOptions{Level:slog.LevelInfo}))
 b,err:=gotgbot.NewBot(cfg.BotToken,nil)
 if err!=nil { logger.Error("could not initialise Telegram bot","error",err); os.Exit(1) }

 var redisClient *redis.Client
 redisURL:=cfg.RedisURL
 if redisURL!="" {
  redisOpts,parseErr:=redis.ParseURL(redisURL)
  if parseErr==nil {
   redisClient=redis.NewClient(redisOpts)
   pingCtx,cancel:=context.WithTimeout(context.Background(),5*time.Second)
   err=redisClient.Ping(pingCtx).Err()
   cancel()
  } else { err=parseErr }
  if err!=nil {
   logger.Error("Redis unavailable; refusing startup","error",err)
   os.Exit(1)
   if redisClient!=nil { _=redisClient.Close(); redisClient=nil }
  } else { logger.Info("Redis connected"); defer redisClient.Close() }
 } else { logger.Error("Redis unavailable; refusing startup"); os.Exit(1) }

 var db *pgxpool.Pool
 databaseURL:=cfg.DatabaseURL
 if databaseURL!="" {
  db,err=pgxpool.New(context.Background(),databaseURL)
  if err==nil {
   pingCtx,cancel:=context.WithTimeout(context.Background(),5*time.Second)
   err=db.Ping(pingCtx)
   cancel()
  }
  if err!=nil {
   logger.Error("PostgreSQL unavailable; refusing startup","error",err)
   os.Exit(1)
   if db!=nil { db.Close(); db=nil }
  } else {
   logger.Info("PostgreSQL connected")
   migrationCtx,migrationCancel:=context.WithTimeout(context.Background(),5*time.Minute)
   migrationErr:=migrations.Apply(migrationCtx,db)
   migrationCancel()
   if migrationErr!=nil {
    logger.Error("database migrations failed; refusing to start bot","error",migrationErr)
    db.Close()
    os.Exit(1)
   }
   logger.Info("database migrations applied")
   defer db.Close()
  }
 } else { logger.Error("PostgreSQL unavailable; refusing startup"); os.Exit(1) }

 dispatcher:=ext.NewDispatcher(&ext.DispatcherOpts{
  Error:func(_ *gotgbot.Bot,_ *ext.Context,err error) ext.DispatcherAction {
   logger.Error("update handler failed","error",err); return ext.DispatcherActionNoop
  },
  MaxRoutines:ext.DefaultMaxRoutines,Logger:logger,
 })
 updater:=ext.NewUpdater(dispatcher,&ext.UpdaterOpts{Logger:logger})
 bot.SetPrimaryBotID(b.User.Id)
 clones.RegisterCommands(dispatcher)
 moderation.RegisterWarningCommands(dispatcher,db)
 moderation.RegisterAuditCommands(dispatcher,db)
 moderation.RegisterMetricsCommands(dispatcher)
 moderation.RegisterReportCommands(dispatcher)
 moderation.RegisterUserInfoCommands(dispatcher)
 moderation.RegisterPurgeCommands(dispatcher)
 moderation.RegisterLockCommands(dispatcher,db)
 moderation.RegisterSlowModeCommands(dispatcher,db,redisClient)
 aiClient := aiassistant.NewClient(cfg.PolicyServiceURL, cfg.AIServiceToken)
 moderation.RegisterAntiFloodCommands(dispatcher,db,redisClient,aiClient)
 moderation.RegisterAntiRaidCommands(dispatcher,db,redisClient,aiClient)
 moderation.RegisterLinkProtectionCommands(dispatcher,db,redisClient,aiClient)
 moderation.RegisterForwardProtectionCommands(dispatcher,db)
 moderation.RegisterBlacklistCommands(dispatcher,db,redisClient,aiClient)
 moderation.RegisterGreetingCommands(dispatcher,db)
 moderation.RegisterRulesCommands(dispatcher,db)
 moderation.RegisterCommunityCommands(dispatcher,db)
 cloneManager:=clones.NewManager(updater,b.User.Id,logger,db,cfg.CloneEncryptionKey)
 moderation.RegisterOwnerDashboard(dispatcher,cfg.OwnerID,db,redisClient,cloneManager)
 cloneManager.Register(dispatcher)
 healthCtx,healthCancel:=context.WithCancel(context.Background())
 defer healthCancel()
 go cloneManager.RunHealthMonitor(healthCtx,60*time.Second)
 recoveryCtx,recoveryCancel:=context.WithTimeout(context.Background(),30*time.Second)
 cloneManager.Recover(recoveryCtx)
 recoveryCancel()

 logger.Info("AniToon_4Bot starting","username",b.User.Username,"mode",cfg.Mode)
 if cfg.Mode=="webhook" {
  if err:=updater.StartWebhook(b,cfg.WebhookPath,ext.WebhookOpts{
   ListenAddr:cfg.WebhookListenAddr,
   ReadHeaderTimeout:10*time.Second,
   ReadTimeout:15*time.Second,
   SecretToken:cfg.WebhookSecretToken,
  }); err!=nil {
   logger.Error("webhook server failed to start","error",err)
   if db!=nil { db.Close() }; if redisClient!=nil { redisClient.Close() }
   os.Exit(1)
  }
  webhookURL:=cfg.WebhookURL+cfg.WebhookPath
  if _,err:=b.SetWebhook(webhookURL,&gotgbot.SetWebhookOpts{
   SecretToken:cfg.WebhookSecretToken,
   AllowedUpdates:[]string{"message","chat_member"},
  }); err!=nil {
   logger.Error("Telegram webhook registration failed","error",err)
   _=updater.Stop()
   if db!=nil { db.Close() }; if redisClient!=nil { redisClient.Close() }
   os.Exit(1)
  }
  logger.Info("Telegram webhook registered","listen_addr",cfg.WebhookListenAddr,"path",cfg.WebhookPath)
 } else {
  if err:=updater.StartPolling(b,&ext.PollingOpts{
   DropPendingUpdates:false,
   GetUpdatesOpts:&gotgbot.GetUpdatesOpts{Timeout:30,AllowedUpdates:[]string{"message","chat_member"},RequestOpts:&gotgbot.RequestOpts{Timeout:35*time.Second}},
  }); err!=nil {
   logger.Error("polling failed to start","error",err)
   if db!=nil { db.Close() }; if redisClient!=nil { redisClient.Close() }
   os.Exit(1)
  }
 }
 signalCtx,stopSignals:=signal.NotifyContext(context.Background(),os.Interrupt,syscall.SIGTERM)
 defer stopSignals()
 // The updater workers run independently; waiting on the signal context avoids
 // an Idle/Stop startup race while still keeping the process alive.
 <-signalCtx.Done()
 logger.Info("shutdown signal received; stopping webhook server and all bot update loops")
 if err:=updater.Stop(); err!=nil {
  logger.Error("updater shutdown failed","error",err)
 }
 logger.Info("AniToon_4Bot shutdown complete")
}
