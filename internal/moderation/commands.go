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
	if ctx.EffectiveChat == nil {
		return 0, fmt.Errorf("this command can only be used in a group")
	}
	if ctx.EffectiveChat.Type != "group" && ctx.EffectiveChat.Type != "supergroup" {
		return 0, fmt.Errorf("this command can only be used in a group")
	}
	return ctx.EffectiveChat.Id, nil
}

func actorID(ctx *ext.Context) (int64, error) {
	// Anonymous admins and messages sent on behalf of channels do not expose
	// a verifiable human user ID. Fail closed rather than trust the sender chat.
	if ctx.EffectiveUser == nil || ctx.EffectiveUser.Id == 0 {
		return 0, fmt.Errorf("I cannot verify your personal admin account; disable anonymous admin mode to use this command")
	}
	return ctx.EffectiveUser.Id, nil
}

func commandArgs(message *gotgbot.Message) []string {
	if message == nil || message.Text == "" {
		return nil
	}
	fields := strings.Fields(message.Text)
	if len(fields) == 0 {
		return nil
	}
	return fields[1:]
}

func resolveTarget(message *gotgbot.Message, args []string) (int64, string, error) {
	if message != nil && message.ReplyToMessage != nil && message.ReplyToMessage.From != nil {
		reason := strings.Join(args, " ")
		return message.ReplyToMessage.From.Id, reason, nil
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

// Ban bans a member after verifying the human administrator, bot rights and
// target status. It intentionally supports reply targeting and numeric IDs only.
func Ban(b *gotgbot.Bot, ctx *ext.Context) error {
	chatID, err := groupChat(ctx)
	if err != nil { return reply(b, ctx, err.Error()) }
	actor, err := actorID(ctx)
	if err != nil { return reply(b, ctx, err.Error()) }
	if err = RequireActorAdmin(b, chatID, actor); err != nil { return reply(b, ctx, err.Error()) }
	if err = RequireBotRestrictPermission(b, chatID); err != nil { return reply(b, ctx, err.Error()) }

	targetID, reason, err := resolveTarget(ctx.EffectiveMessage, commandArgs(ctx.EffectiveMessage))
	if err != nil { return reply(b, ctx, "Usage: /ban (reply to a user) [reason] or /ban <user_id> [reason]\n"+err.Error()) }
	if b.User != nil && targetID == b.User.Id {
		return reply(b, ctx, "I cannot ban myself.")
	}
	if err = RequireTargetIsModeratable(b, chatID, targetID); err != nil { return reply(b, ctx, err.Error()) }

	ok, err := b.BanChatMember(chatID, targetID, &gotgbot.BanChatMemberOpts{RevokeMessages: true})
	if err != nil { return reply(b, ctx, "Telegram could not ban that user: "+err.Error()) }
	if !ok { return reply(b, ctx, "Telegram did not confirm the ban. No success was recorded.") }
	text := fmt.Sprintf("Banned user <code>%d</code>.", targetID)
	if reason != "" { text += "\nReason: " + reason }
	return reply(b, ctx, text)
}

// Unban removes a ban using a numeric ID or a reply to a visible message.
// Telegram's only_if_banned option avoids removing a currently joined member.
func Unban(b *gotgbot.Bot, ctx *ext.Context) error {
	chatID, err := groupChat(ctx)
	if err != nil { return reply(b, ctx, err.Error()) }
	actor, err := actorID(ctx)
	if err != nil { return reply(b, ctx, err.Error()) }
	if err = RequireActorAdmin(b, chatID, actor); err != nil { return reply(b, ctx, err.Error()) }
	if err = RequireBotRestrictPermission(b, chatID); err != nil { return reply(b, ctx, err.Error()) }

	targetID, _, err := resolveTarget(ctx.EffectiveMessage, commandArgs(ctx.EffectiveMessage))
	if err != nil { return reply(b, ctx, "Usage: /unban <user_id> (or reply to a user's message)\n"+err.Error()) }
	if b.User != nil && targetID == b.User.Id {
		return reply(b, ctx, "I cannot unban myself.")
	}
	// A currently present administrator is protected. A GetChatMember error
	// is allowed here because the user may be banned/absent; Telegram's unban
	// endpoint will perform the authoritative operation.
	if member, lookupErr := b.GetChatMember(chatID, targetID, nil); lookupErr == nil && IsProtectedTarget(member.GetStatus()) {
		return reply(b, ctx, "I won't change the membership status of a group owner or administrator.")
	}
	ok, err := b.UnbanChatMember(chatID, targetID, &gotgbot.UnbanChatMemberOpts{OnlyIfBanned: true})
	if err != nil { return reply(b, ctx, "Telegram could not unban that user: "+err.Error()) }
	if !ok { return reply(b, ctx, "Telegram did not confirm the unban.") }
	return reply(b, ctx, fmt.Sprintf("Unbanned user <code>%d</code>. They can now rejoin using an invite link.", targetID))
}
