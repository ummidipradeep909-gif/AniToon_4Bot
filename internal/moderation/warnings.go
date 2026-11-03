package moderation

import (
 "context"
 "fmt"
 "strings"
 "time"

 "github.com/PaulSonOfLars/gotgbot/v2"
 "github.com/PaulSonOfLars/gotgbot/v2/ext"
 "github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
 "github.com/jackc/pgx/v5/pgxpool"
)

// RegisterWarningCommands installs durable warning commands on the shared
// dispatcher, so the main bot and connected clones use the same policy store.
func RegisterWarningCommands(d *ext.Dispatcher, pool *pgxpool.Pool) {
 d.AddHandler(handlers.NewCommand("warn", func(b *gotgbot.Bot, c *ext.Context) error {
  if pool == nil { return reply(b,c,"Warning storage is unavailable. Configure DATABASE_URL and restart the bot."); }
  return warnMember(b,c,pool)
 }))
 d.AddHandler(handlers.NewCommand("warnings", func(b *gotgbot.Bot, c *ext.Context) error {
  if pool == nil { return reply(b,c,"Warning storage is unavailable. Configure DATABASE_URL and restart the bot."); }
  return listWarnings(b,c,pool)
 }))
 d.AddHandler(handlers.NewCommand("resetwarn", func(b *gotgbot.Bot, c *ext.Context) error {
  if pool == nil { return reply(b,c,"Warning storage is unavailable. Configure DATABASE_URL and restart the bot."); }
  return resetWarnings(b,c,pool)
 }))
}

func warnMember(b *gotgbot.Bot,c *ext.Context,pool *pgxpool.Pool) error {
 chatID,err:=groupChat(c); if err!=nil{return reply(b,c,err.Error())}
 actor,err:=actorID(c); if err!=nil{return reply(b,c,err.Error())}
 if err=RequireActorAdmin(b,chatID,actor); err!=nil{return reply(b,c,err.Error())}
 targetID,reason,err:=resolveTarget(c.EffectiveMessage,commandArgs(c.EffectiveMessage))
 if err!=nil{return reply(b,c,"Usage: reply with /warn [reason], or /warn <user_id> [reason]. "+err.Error())}
 if targetID==actor{return reply(b,c,"You cannot warn yourself.")}
 if targetID==b.User.Id{return reply(b,c,"I cannot warn myself.")}
 if err=RequireTargetIsModeratable(b,chatID,targetID); err!=nil{return reply(b,c,err.Error())}
 reason=strings.TrimSpace(reason)
 if len(reason)>300 { reason=reason[:300] }
 if reason=="" { reason="No reason provided" }

 ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second); defer cancel()
 if _,err=pool.Exec(ctx,`INSERT INTO chats (chat_id) VALUES ($1) ON CONFLICT (chat_id) DO NOTHING`,chatID); err!=nil {
  return reply(b,c,"I couldn't save the warning because the database is unavailable.")
 }
 if _,err=pool.Exec(ctx,`INSERT INTO chat_warnings (chat_id,user_id,actor_user_id,reason) VALUES ($1,$2,$3,$4)`,chatID,targetID,actor,reason); err!=nil {
  return reply(b,c,"I couldn't save the warning because the database is unavailable.")
 }
 var count int
 if err=pool.QueryRow(ctx,`SELECT COUNT(*) FROM chat_warnings WHERE chat_id=$1 AND user_id=$2`,chatID,targetID).Scan(&count); err!=nil {
  return reply(b,c,"Warning was saved, but I couldn't retrieve the updated count.")
 }
 result:=fmt.Sprintf("⚠️ User %d has been warned.\nWarnings: %d/3\nReason: %s",targetID,count,reason)
 if count>=3 { result+="\n⚠️ Warning limit reached. An administrator should review this member; no automatic punishment has been applied." }
 return reply(b,c,result)
}

func listWarnings(b *gotgbot.Bot,c *ext.Context,pool *pgxpool.Pool) error {
 chatID,err:=groupChat(c); if err!=nil{return reply(b,c,err.Error())}
 actor,err:=actorID(c); if err!=nil{return reply(b,c,err.Error())}
 if err=RequireActorAdmin(b,chatID,actor); err!=nil{return reply(b,c,err.Error())}
 targetID,_,err:=resolveTarget(c.EffectiveMessage,commandArgs(c.EffectiveMessage))
 if err!=nil{return reply(b,c,"Usage: reply to a user with /warnings, or /warnings <user_id>. "+err.Error())}
 ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second); defer cancel()
 var count int
 if err=pool.QueryRow(ctx,`SELECT COUNT(*) FROM chat_warnings WHERE chat_id=$1 AND user_id=$2`,chatID,targetID).Scan(&count); err!=nil {
  return reply(b,c,"I couldn't read warning history because the database is unavailable.")
 }
 rows,err:=pool.Query(ctx,`SELECT actor_user_id,reason,created_at FROM chat_warnings WHERE chat_id=$1 AND user_id=$2 ORDER BY created_at DESC,id DESC LIMIT 5`,chatID,targetID)
 if err!=nil{return reply(b,c,"I couldn't read warning history because the database is unavailable.")}
 defer rows.Close()
 var out strings.Builder
 fmt.Fprintf(&out,"⚠️ Warnings for user %d: %d\n",targetID,count)
 if count==0 { out.WriteString("No warnings recorded in this group.") } else {
  for rows.Next() {
   var adminID int64; var reason string; var created time.Time
   if err=rows.Scan(&adminID,&reason,&created); err!=nil{return reply(b,c,"I couldn't read warning history.")}
   fmt.Fprintf(&out,"\n• %s — by admin %d (%s)",reason,adminID,created.UTC().Format("2006-01-02 15:04 UTC"))
  }
  if err=rows.Err(); err!=nil{return reply(b,c,"I couldn't read warning history.")}
  if count>5 { out.WriteString("\nShowing the latest 5 warnings.") }
 }
 return reply(b,c,out.String())
}

func resetWarnings(b *gotgbot.Bot,c *ext.Context,pool *pgxpool.Pool) error {
 chatID,err:=groupChat(c); if err!=nil{return reply(b,c,err.Error())}
 actor,err:=actorID(c); if err!=nil{return reply(b,c,err.Error())}
 if err=RequireActorAdmin(b,chatID,actor); err!=nil{return reply(b,c,err.Error())}
 targetID,_,err:=resolveTarget(c.EffectiveMessage,commandArgs(c.EffectiveMessage))
 if err!=nil{return reply(b,c,"Usage: reply to a user with /resetwarn, or /resetwarn <user_id>. "+err.Error())}
 ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second); defer cancel()
 tag,err:=pool.Exec(ctx,`DELETE FROM chat_warnings WHERE chat_id=$1 AND user_id=$2`,chatID,targetID)
 if err!=nil{return reply(b,c,"I couldn't reset warnings because the database is unavailable.")}
 auditErr:=writeModerationAudit(b.User.Id,chatID,actor,targetID,"resetwarn",fmt.Sprintf("Removed %d warning(s)",tag.RowsAffected()),nil)
 return reply(b,c,fmt.Sprintf("Reset %d warning(s) for user %d in this group.%s",tag.RowsAffected(),targetID,auditWarning(auditErr)))
}
