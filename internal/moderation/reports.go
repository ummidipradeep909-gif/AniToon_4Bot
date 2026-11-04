package moderation

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
)

func RegisterReportCommands(d *ext.Dispatcher) {
	d.AddHandler(handlers.NewCommand("report", Report))
	d.AddHandler(handlers.NewCommand("reportnotify", ReportNotify))
}

func parseReportWindow(args []string) (time.Duration, string, error) {
	if len(args) == 0 || (len(args) == 1 && strings.EqualFold(args[0], "day")) {
		return 24 * time.Hour, "24-hour", nil
	}
	if len(args) == 1 && strings.EqualFold(args[0], "week") {
		return 7 * 24 * time.Hour, "7-day", nil
	}
	return 0, "", fmt.Errorf("usage: /report [day|week]")
}

func Report(b *gotgbot.Bot, c *ext.Context) error {
	chatID, err := groupChat(c)
	if err != nil { return reply(b,c,err.Error()) }
	actor, err := actorID(c)
	if err != nil { return reply(b,c,err.Error()) }
	if err = RequireActorAdmin(b,chatID,actor); err != nil { return reply(b,c,err.Error()) }
	window, label, err := parseReportWindow(commandArgs(c.EffectiveMessage))
	if err != nil { return reply(b,c,err.Error()) }
	if auditPool == nil { return reply(b,c,"Reports require PostgreSQL audit storage. Configure DATABASE_URL and apply migrations 001–005.") }

	ctx,cancel := context.WithTimeout(context.Background(),5*time.Second)
	defer cancel()
	since := time.Now().UTC().Add(-window)
	var total int64
	if err = auditPool.QueryRow(ctx,`SELECT COUNT(*) FROM moderation_actions WHERE bot_id=$1 AND chat_id=$2 AND created_at >= $3`,b.User.Id,chatID,since).Scan(&total); err != nil {
		return reply(b,c,"I couldn't generate the report. Check PostgreSQL and the moderation audit migration.")
	}
	rows,err := auditPool.Query(ctx,`SELECT action,COUNT(*) FROM moderation_actions
		WHERE bot_id=$1 AND chat_id=$2 AND created_at >= $3 GROUP BY action ORDER BY COUNT(*) DESC,action LIMIT 20`,b.User.Id,chatID,since)
	if err != nil { return reply(b,c,"I couldn't generate the report.") }
	defer rows.Close()
	var out strings.Builder
	fmt.Fprintf(&out,"📊 Moderation report — %s\nSince: %s UTC\nTotal recorded actions: %d",label,since.Format("2006-01-02 15:04"),total)
	for rows.Next() {
		var action string
		var count int64
		if err=rows.Scan(&action,&count); err != nil { return reply(b,c,"I couldn't read the report.") }
		fmt.Fprintf(&out,"\n• %s: %d",action,count)
	}
	if err=rows.Err(); err != nil { return reply(b,c,"I couldn't read the report.") }
	if total==0 { out.WriteString("\nNo moderation actions were recorded in this period.") }
	return reply(b,c,out.String())
}

func ReportNotify(b *gotgbot.Bot,c *ext.Context) error {
	chatID,err:=groupChat(c)
	if err!=nil{return reply(b,c,err.Error())}
	actor,err:=actorID(c)
	if err!=nil{return reply(b,c,err.Error())}
	if err=RequireActorAdmin(b,chatID,actor);err!=nil{return reply(b,c,err.Error())}
	if auditPool==nil{return reply(b,c,"Notification settings require PostgreSQL.")}
	args:=commandArgs(c.EffectiveMessage)
	if len(args)!=1 || (!strings.EqualFold(args[0],"on")&&!strings.EqualFold(args[0],"off")) {
		return reply(b,c,"Usage: /reportnotify on|off")
	}
	enabled:=strings.EqualFold(args[0],"on")
	ctx,cancel:=context.WithTimeout(context.Background(),5*time.Second);defer cancel()
	if _,err=auditPool.Exec(ctx,`INSERT INTO chats(chat_id) VALUES($1) ON CONFLICT(chat_id) DO NOTHING`,chatID);err!=nil{return reply(b,c,"I couldn't update notification settings.")}
	_,err=auditPool.Exec(ctx,`INSERT INTO chat_report_settings(bot_id,chat_id,notifications_enabled,updated_by)
		VALUES($1,$2,$3,$4) ON CONFLICT(bot_id,chat_id) DO UPDATE
		SET notifications_enabled=EXCLUDED.notifications_enabled,updated_by=EXCLUDED.updated_by,updated_at=NOW()`,
		b.User.Id,chatID,enabled,actor)
	if err!=nil{return reply(b,c,"I couldn't update notification settings. Apply migration 015 and check PostgreSQL.")}
	if enabled{return reply(b,c,"Moderation event notifications are now enabled for this bot in this group.")}
	return reply(b,c,"Moderation event notifications are now disabled for this bot in this group.")
}

func notifyModerationEvent(b *gotgbot.Bot,botID,chatID,targetID int64,action string) {
	if auditPool==nil || b==nil || b.User.Id!=botID{return}
	ctx,cancel:=context.WithTimeout(context.Background(),2*time.Second);defer cancel()
	var enabled bool
	err:=auditPool.QueryRow(ctx,`SELECT notifications_enabled FROM chat_report_settings WHERE bot_id=$1 AND chat_id=$2`,botID,chatID).Scan(&enabled)
	if err!=nil||!enabled{return}
	_,_ = b.SendMessage(chatID,fmt.Sprintf("🔔 Moderation event: %s\nTarget user ID: %d",action,targetID),nil)
}
