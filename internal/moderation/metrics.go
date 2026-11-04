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

func RegisterMetricsCommands(d *ext.Dispatcher) {
	d.AddHandler(handlers.NewCommand("metrics", Metrics))
}

func parseMetricsWindow(args []string) (time.Duration, string, error) {
	if len(args) == 0 || (len(args) == 1 && strings.EqualFold(args[0], "week")) {
		return 7 * 24 * time.Hour, "7 days", nil
	}
	switch {
	case len(args) == 1 && strings.EqualFold(args[0], "day"):
		return 24 * time.Hour, "24 hours", nil
	case len(args) == 1 && strings.EqualFold(args[0], "month"):
		return 30 * 24 * time.Hour, "30 days", nil
	default:
		return 0, "", fmt.Errorf("usage: /metrics [day|week|month]")
	}
}

// Metrics reports aggregate counts from confirmed PostgreSQL moderation audit
// actions. It deliberately omits member identifiers and free-form reasons.
func Metrics(b *gotgbot.Bot, c *ext.Context) error {
	chatID, err := groupChat(c)
	if err != nil {
		return reply(b, c, err.Error())
	}
	actor, err := actorID(c)
	if err != nil {
		return reply(b, c, err.Error())
	}
	if err = RequireActorAdmin(b, chatID, actor); err != nil {
		return reply(b, c, err.Error())
	}
	window, label, err := parseMetricsWindow(commandArgs(c.EffectiveMessage))
	if err != nil {
		return reply(b, c, err.Error())
	}
	if auditPool == nil {
		return reply(b, c, "Moderation metrics require PostgreSQL audit storage. Configure DATABASE_URL and apply migrations 001–005.")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	since := time.Now().UTC().Add(-window)

	var total, moderators, targets int64
	err = auditPool.QueryRow(ctx, `SELECT COUNT(*), COUNT(DISTINCT actor_user_id), COUNT(DISTINCT target_user_id)
		FROM moderation_actions WHERE bot_id=$1 AND chat_id=$2 AND created_at >= $3`,
		b.User.Id, chatID, since).Scan(&total, &moderators, &targets)
	if err != nil {
		return reply(b, c, "I couldn't generate moderation metrics. Check PostgreSQL and the audit migration.")
	}

	rows, err := auditPool.Query(ctx, `SELECT action, COUNT(*) FROM moderation_actions
		WHERE bot_id=$1 AND chat_id=$2 AND created_at >= $3
		GROUP BY action ORDER BY COUNT(*) DESC, action LIMIT 10`, b.User.Id, chatID, since)
	if err != nil {
		return reply(b, c, "I couldn't retrieve the moderation action breakdown.")
	}
	defer rows.Close()

	var out strings.Builder
	fmt.Fprintf(&out, "📈 Moderation metrics — %s\nRecorded actions: %d\nDistinct moderators: %d\nDistinct affected members: %d",
		label, total, moderators, targets)
	for rows.Next() {
		var action string
		var count int64
		if err = rows.Scan(&action, &count); err != nil {
			return reply(b, c, "I couldn't read the moderation action breakdown.")
		}
		fmt.Fprintf(&out, "\n• %s: %d", action, count)
	}
	if err = rows.Err(); err != nil {
		return reply(b, c, "I couldn't read the moderation action breakdown.")
	}
	if total == 0 {
		out.WriteString("\nNo confirmed moderation actions were recorded in this period.")
	}
	out.WriteString("\n\nMetrics include actions recorded in the moderation audit log; automated message deletions and unlogged protection events are not included.")
	return reply(b, c, out.String())
}
