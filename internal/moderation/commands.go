package moderation

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func reply(b *gotgbot.Bot, ctx *ext.Context, message string) error {
	_, err := ctx.EffectiveMessage.Reply(b, message, nil)
	return err
}

func groupChat(ctx *ext.Context) (int64, error) {
	if ctx.EffectiveChat == nil || (ctx.EffectiveChat.Type != "group" && ctx.EffectiveChat.Type != "supergroup") {
		return 0, fmt.Errorf("this command can only be used in a group")
	}
	return ctx.EffectiveChat.Id, nil
}

func actorID(ctx *ext.Context) (int64, error) {
	// Anonymous admins and channel senders do not expose a verifiable human ID.
	if ctx.EffectiveUser == nil || ctx.EffectiveUser.Id == 0 {
		return 0, fmt.Errorf("I cannot verify your personal admin account; disable anonymous admin mode to use this command")
	}
	return ctx.EffectiveUser.Id, nil
}

func commandArgs(message *gotgbot.Message) []string {
	if message == nil || message.Text == "" { return nil }
	fields := strings.Fields(message.Text)
	if len(fields) == 0 { return nil }
	return fields[1:]
}

func resolveTarget(message *gotgbot.Message, args []string) (int64, string, error) {
	if message != nil && message.ReplyToMessage != nil && message.ReplyToMessage.From != nil {
		return message.ReplyToMessage.From.Id, strings.Join(args, " "), nil
	}
	if len(args) == 0 {
		return 0, "", fmt.Errorf("reply to a user's message or provide their numeric Telegram user ID")
	}
	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil || id <= 0 {
		return 0, "", fmt.Errorf("please provide a valid numeric Telegram user ID, or reply to their message")
	}
	return id, strings.Join(args[1:], " "), nil
}

func Ban(b *gotgbot.Bot, ctx *ext.Context) error {
	chatID, err := groupChat(ctx)
	if err != nil { return reply(b, ctx, err.Error()) }
	actor, err := actorID(ctx)
	if err != nil { return reply(b, ctx, err.Error()) }
	if err = RequireActorAdmin(b, chatID, actor); err != nil { return reply(b, ctx, err.Error()) }
	if err = RequireBotRestrictPermission(b, chatID); err != nil { return reply(b, ctx, err.Error()) }
	targetID, reason, err := resolveTarget(ctx.EffectiveMessage, commandArgs(ctx.EffectiveMessage))
	if err != nil { return reply(b, ctx, "Usage: /ban (reply to a user) [reason] or /ban <user_id> [reason]. "+err.Error()) }
	if b.User != nil && targetID == b.User.Id { return reply(b, ctx, "I cannot ban myself.") }
	if err = RequireTargetIsModeratable(b, chatID, targetID); err != nil { return reply(b, ctx, err.Error()) }
	ok, err := b.BanChatMember(chatID, targetID, &gotgbot.BanChatMemberOpts{RevokeMessages: true})
	if err != nil { return reply(b, ctx, "Telegram could not ban that user: "+err.Error()) }
	if !ok { return reply(b, ctx, "Telegram did not confirm the ban.") }
	text := fmt.Sprintf("Banned user %d.", targetID)
	if reason != "" { text += "\nReason: " + reason }
	return reply(b, ctx, text)
}

func Unban(b *gotgbot.Bot, ctx *ext.Context) error {
	chatID, err := groupChat(ctx)
	if err != nil { return reply(b, ctx, err.Error()) }
	actor, err := actorID(ctx)
	if err != nil { return reply(b, ctx, err.Error()) }
	if err = RequireActorAdmin(b, chatID, actor); err != nil { return reply(b, ctx, err.Error()) }
	if err = RequireBotRestrictPermission(b, chatID); err != nil { return reply(b, ctx, err.Error()) }
	targetID, _, err := resolveTarget(ctx.EffectiveMessage, commandArgs(ctx.EffectiveMessage))
	if err != nil { return reply(b, ctx, "Usage: /unban <user_id> (or reply to a user's message). "+err.Error()) }
	if b.User != nil && targetID == b.User.Id { return reply(b, ctx, "I cannot unban myself.") }
	if member, lookupErr := b.GetChatMember(chatID, targetID, nil); lookupErr == nil {
		if IsProtectedTarget(member.GetStatus()) {
			return reply(b, ctx, "I won't change the membership status of a group owner or administrator.")
		}
		if member.GetStatus() != gotgbot.ChatMemberStatusKicked {
			return reply(b, ctx, "That user is not currently banned in this group.")
		}
	}
	ok, err := b.UnbanChatMember(chatID, targetID, &gotgbot.UnbanChatMemberOpts{OnlyIfBanned: true})
	if err != nil { return reply(b, ctx, "Telegram could not unban that user: "+err.Error()) }
	if !ok { return reply(b, ctx, "Telegram did not confirm the unban.") }
	return reply(b, ctx, fmt.Sprintf("Unbanned user %d. They can now rejoin using an invite link.", targetID))
}
