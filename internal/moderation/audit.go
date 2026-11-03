package moderation

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/jackc/pgx/v5/pgxpool"
)

var auditPool *pgxpool.Pool

// RegisterAuditCommands installs /modlog and configures the shared audit store
// used by the main bot and all clones on the shared dispatcher.
func RegisterAuditCommands(d *ext.Dispatcher, pool *pgxpool.Pool) {
	auditPool = pool
	d.AddHandler(handlers.NewCommand("modlog", ModLog))
}

func parseAuditLimit(args []string) (int, error) {
	if len(args) == 0 { return 10, nil }
	if len(args) != 1 { return 0, fmt.Errorf("usage: /modlog [1-20]") }
	n, err := strconv.Atoi(args[0])
	if err != nil || n < 1 || n > 20 { return 0, fmt.Errorf("log count must be between 1 and 20") }
	return n, nil
}

// writeModerationAudit records confirmed Telegram actions. Audit failures are
// surfaced to the admin but never pretend that the Telegram action was undone.
func writeModerationAudit(botID, chatID, actorID, targetID int64, action, reason string, expiresAt *time.Time) error {
	if auditPool == nil { return fmt.Errorf("PostgreSQL audit storage is not configured") }
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := auditPool.Exec(ctx, `INSERT INTO chats(chat_id) VALUES($1) ON CONFLICT(chat_id) DO NOTHING`, chatID); err != nil {
		return fmt.Errorf("ensure audit chat: %w", err)
	}
	if len(reason) > 300 { reason = reason[:300] }
	_, err := auditPool.Exec(ctx, `INSERT INTO moderation_actions
		(bot_id,chat_id,target_user_id,actor_user_id,action,reason,expires_at)
		VALUES($1,$2,$3,$4,$5,$6,$7)`,
		botID, chatID, targetID, actorID, action, reason, expiresAt)
	if err != nil { return fmt.Errorf("insert moderation audit: %w", err) }
	return nil
}

func auditWarning(err error) string {
	if err == nil { return "" }
	return "\n⚠️ Action succeeded, but its audit record could not be saved."
}

// ModLog shows recent moderation actions by this bot in the current group.
func ModLog(b *gotgbot.Bot, c *ext.Context) error {
	chatID, err := groupChat(c)
	if err != nil { return reply(b,c,err.Error()) }
	actor, err := actorID(c)
	if err != nil { return reply(b,c,err.Error()) }
	if err = RequireActorAdmin(b,chatID,actor); err != nil { return reply(b,c,err.Error()) }
	limit, err := parseAuditLimit(commandArgs(c.EffectiveMessage))
	if err != nil { return reply(b,c,err.Error()) }
	if auditPool == nil { return reply(b,c,"Moderation audit storage is unavailable. Configure DATABASE_URL and apply migration 005.") }

	ctx,cancel := context.WithTimeout(context.Background(),5*time.Second)
	defer cancel()
	rows,err := auditPool.Query(ctx,`SELECT actor_user_id,target_user_id,action,reason,created_at
		FROM moderation_actions WHERE bot_id=$1 AND chat_id=$2
		ORDER BY created_at DESC,id DESC LIMIT $3`,b.User.Id,chatID,limit)
	if err != nil { return reply(b,c,"I couldn't retrieve the moderation log. Check the database and migration 005.") }
	defer rows.Close()

	var out strings.Builder
	fmt.Fprintf(&out,"🧾 Recent moderation actions (latest %d):",limit)
	count:=0
	for rows.Next() {
		var adminID,targetID int64
		var action,reason string
		var created time.Time
		if err=rows.Scan(&adminID,&targetID,&action,&reason,&created); err != nil { return reply(b,c,"I couldn't read the moderation log.") }
		fmt.Fprintf(&out,"\n\n%s — %s\nTarget: %d · Admin: %d",created.UTC().Format("2006-01-02 15:04 UTC"),action,targetID,adminID)
		if reason!="" { fmt.Fprintf(&out,"\nReason: %s",reason) }
		count++
	}
	if err=rows.Err(); err != nil { return reply(b,c,"I couldn't read the moderation log.") }
	if count==0 { out.WriteString("\nNo recorded moderation actions yet.") }
	return reply(b,c,out.String())
}
