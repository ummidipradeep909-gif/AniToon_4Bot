package moderation

import (
	"fmt"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

// Kick removes a member without leaving a lasting ban. Telegram has no
// standalone kickChatMember method: the supported pattern is ban then unban.
// If the unban fails after a successful ban, report that the user remains
// banned so an administrator can explicitly run /unban.
func Kick(b *gotgbot.Bot, ctx *ext.Context) error {
	chatID, err := groupChat(ctx)
	if err != nil { return reply(b, ctx, err.Error()) }
	actor, err := actorID(ctx)
	if err != nil { return reply(b, ctx, err.Error()) }
	if err = RequireActorAdmin(b, chatID, actor); err != nil { return reply(b, ctx, err.Error()) }
	if err = RequireBotRestrictPermission(b, chatID); err != nil { return reply(b, ctx, err.Error()) }

	targetID, reason, err := resolveTarget(ctx.EffectiveMessage, commandArgs(ctx.EffectiveMessage))
	if err != nil {
		return reply(b, ctx, "Usage: reply with /kick [reason], or /kick <user_id> [reason]. "+err.Error())
	}
	if targetID == b.User.Id { return reply(b, ctx, "I cannot kick myself.") }
	if err = RequireTargetIsModeratable(b, chatID, targetID); err != nil { return reply(b, ctx, err.Error()) }

	banned, err := b.BanChatMember(chatID, targetID, &gotgbot.BanChatMemberOpts{RevokeMessages: false})
	if err != nil { return reply(b, ctx, "Telegram could not remove that user: "+err.Error()) }
	if !banned { return reply(b, ctx, "Telegram did not confirm the removal.") }

	unbanned, err := b.UnbanChatMember(chatID, targetID, &gotgbot.UnbanChatMemberOpts{OnlyIfBanned: true})
	if err != nil {
		return reply(b, ctx, fmt.Sprintf("User %d was removed but could not be unbanned automatically. They remain banned; use /unban %d after resolving the Telegram error: %v", targetID, targetID, err))
	}
	if !unbanned {
		return reply(b, ctx, fmt.Sprintf("User %d was removed, but Telegram did not confirm unbanning. They may remain banned; check with /unban %d.", targetID, targetID))
	}

	auditErr := writeModerationAudit(b.User.Id, chatID, actor, targetID, "kick", reason, nil)
	notifyModerationEvent(b, b.User.Id, chatID, targetID, "kick")
	result := fmt.Sprintf("Kicked user %d. They may rejoin using a group invite link.", targetID)
	if reason != "" { result += "\nReason: " + reason }
	result += auditWarning(auditErr)
	return reply(b, ctx, result)
}
