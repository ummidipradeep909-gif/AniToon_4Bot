package main

import (
 "context"
 "log/slog"
 "os"
 "time"

 "github.com/PaulSonOfLars/gotgbot/v2"
 "github.com/PaulSonOfLars/gotgbot/v2/ext"
 "github.com/jackc/pgx/v5/pgxpool"
 "github.com/redis/go-redis/v9"

 "github.com/ummidipradeep909-gif/AniToon_4Bot/internal/bot"
 "github.com/ummidipradeep909-gif/AniToon_4Bot/internal/clones"
 "github.com/ummidipradeep909-gif/AniToon_4Bot/internal/config"
 "github.com/ummidipradeep909-gif/AniToon_4Bot/internal/moderation"
)

func main() {
 cfg,err:=config.Load()
 if err!=nil { slog.Error("configuration error","error",err); os.Exit(1) }
 logger:=slog.New(slog.NewTextHandler(os.Stdout,&slog.HandlerOptions{Level:slog.LevelInfo}))
 b,err:=gotgbot.NewBot(cfg.BotToken,nil)
 if err!=nil { logger.Error("could not initialise Telegram bot","error",err); os.Exit(1) }

 var redisClient *redis.Client
 redisURL:=os.Getenv("REDIS_URL")
 if redisURL!="" {
  redisOpts,parseErr:=redis.ParseURL(redisURL)
  if parseErr==nil {
   redisClient=redis.NewClient(redisOpts)
   pingCtx,cancel:=context.WithTimeout(context.Background(),5*time.Second)
   err=redisClient.Ping(pingCtx).Err()
   cancel()
  } else { err=parseErr }
  if err!=nil {
   logger.Error("Redis unavailable; anti-flood and slow-mode detection will be disabled","error",err)
   if redisClient!=nil { _=redisClient.Close(); redisClient=nil }
  } else { logger.Info("Redis connected"); defer redisClient.Close() }
 } else { logger.Warn("REDIS_URL is empty; anti-flood detection will be disabled") }

 var db *pgxpool.Pool
 databaseURL:=os.Getenv("DATABASE_URL")
 if databaseURL!="" {
  db,err=pgxpool.New(context.Background(),databaseURL)
  if err==nil {
   pingCtx,cancel:=context.WithTimeout(context.Background(),5*time.Second)
   err=db.Ping(pingCtx)
   cancel()
  }
  if err!=nil {
   logger.Error("PostgreSQL unavailable; warning, lock, anti-flood, anti-raid and slow-mode configuration commands will be disabled","error",err)
   if db!=nil { db.Close(); db=nil }
  } else {
   logger.Info("PostgreSQL connected")
   defer db.Close()
  }
 } else {
  logger.Warn("DATABASE_URL is empty; warning, lock, anti-flood and slow-mode configuration commands will be disabled")
 }

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
 moderation.RegisterPurgeCommands(dispatcher)
 moderation.RegisterLockCommands(dispatcher,db)
 moderation.RegisterSlowModeCommands(dispatcher,db,redisClient)
 moderation.RegisterAntiFloodCommands(dispatcher,db,redisClient)
 moderation.RegisterAntiRaidCommands(dispatcher,db,redisClient)
 moderation.RegisterLinkProtectionCommands(dispatcher,db)
 moderation.RegisterForwardProtectionCommands(dispatcher,db)
 moderation.RegisterBlacklistCommands(dispatcher,db)
 cloneManager:=clones.NewManager(updater,b.User.Id,logger,db,os.Getenv("CLONE_ENCRYPTION_KEY"))
 cloneManager.Register(dispatcher)
 recoveryCtx,recoveryCancel:=context.WithTimeout(context.Background(),30*time.Second)
 cloneManager.Recover(recoveryCtx)
 recoveryCancel()

 logger.Info("AniToon_4Bot starting","username",b.User.Username)
 if err:=updater.StartPolling(b,&ext.PollingOpts{
  DropPendingUpdates:false,
  GetUpdatesOpts:&gotgbot.GetUpdatesOpts{Timeout:30,AllowedUpdates:[]string{"message","chat_member"},RequestOpts:&gotgbot.RequestOpts{Timeout:35*time.Second}},
 }); err!=nil { logger.Error("polling stopped","error",err); os.Exit(1) }
 updater.Idle()
}
