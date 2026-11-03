package moderation

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
)

func parseMuteTarget(message *gotgbot.Message, args []string) (int64, time.Duration, string, error) {
	var targetID int64
	var rest []string
	if message != nil && message.ReplyToMessage != nil && message.ReplyToMessage.From != nil {
		targetID = message.ReplyToMessage.From.Id
		rest = args
	} else {
		if len(args) == 0 {
			return 0, 0, "", fmt.Errorf("reply to a user's message or provide their numeric Telegram user ID")
		}
		id, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil || id <= 0 {
			return 0, 0, "", fmt.Errorf("provide a valid numeric user ID, or reply to the user's message")
		}
		targetID = id
		rest = args[1:]
	}

	var duration time.Duration
	if len(rest) > 0 {
		first := strings.ToLower(rest[0])
		if first[0] >= '0' && first[0] <= '9' {
			duration, err := ParseMuteDuration(first)
			if err != nil {
				return 0, 0, "", err
			}
			rest = rest[1:]
		}
	}
	return targetID, duration, strings.Join(rest, " "), nil
}

// Mute restricts all message-sending permissions for a member. With no
// duration the restriction is permanent; otherwise Telegram lifts it at the
// requested Unix timestamp.
func Mute(b *gotgbot.Bot, ctx *ext.Context) error {
	chatID, err := groupChat(ctx)
	if err != nil { return reply(b, ctx, err.Error()) }
	actor, err := actorID(ctx)
	if err != nil { return reply(b, ctx, err.Error()) }
	if err = RequireActorAdmin(b, chatID, actor); err != nil { return reply(b, ctx, err.Error()) }
	if err = RequireBotRestrictPermission(b, chatID); err != nil { return reply(b, ctx, err.Error()) }

	targetID, duration, reason, err := parseMuteTarget(ctx.EffectiveMessage, commandArgs(ctx.EffectiveMessage))
	if err != nil {
		return reply(b, ctx, "Usage: reply with /mute [duration] [reason], or /mute <user_id> [duration] [reason]. "+err.Error())
	}
	if b.User != nil && targetID == b.User.Id { return reply(b, ctx, "I cannot mute myself.") }
	if err = RequireTargetIsModeratable(b, chatID, targetID); err != nil { return reply(b, ctx, err.Error()) }

	untilDate := int64(0)
	if duration > 0 { untilDate = time.Now().Add(duration).Unix() }
	permissions := gotgbot.ChatPermissions{UseIndependentChatPermissions: true}
	ok, err := b.RestrictChatMember(chatID, targetID, permissions, &gotgbot.RestrictChatMemberOpts{
		UseIndependentChatPermissions: true,
		UntilDate: untilDate,
	})
	if err != nil { return reply(b, ctx, "Telegram could not mute that user: "+err.Error()) }
	if !ok { return reply(b, ctx, "Telegram did not confirm the mute.") }

	result := fmt.Sprintf("Muted user %d", targetID)
	if duration > 0 { result += " for " + duration.String() } else { result += " permanently" }
	if reason != "" { result += ". Reason: " + reason } else { result += "." }
	return reply(b, ctx, result)
}

// Unmute restores all Telegram chat permissions. Telegram's chat defaults
// may be broader or narrower than this; a future settings-aware permission
// layer will preserve per-chat defaults rather than assume them.
func Unmute(b *gotgbot.Bot, ctx *ext.Context) error {
	chatID, err := groupChat(ctx)
	if err != nil { return reply(b, ctx, err.Error()) }
	actor, err := actorID(ctx)
	if err != nil { return reply(b, ctx, err.Error()) }
	if err = RequireActorAdmin(b, chatID, actor); err != nil { return reply(b, ctx, err.Error()) }
	if err = RequireBotRestrictPermission(b, chatID); err != nil { return reply(b, ctx, err.Error()) }

	targetID, _, err := resolveTarget(ctx.EffectiveMessage, commandArgs(ctx.EffectiveMessage))
	if err != nil { return reply(b, ctx, "Usage: reply with /unmute, or /unmute <user_id>. "+err.Error()) }
	if b.User != nil && targetID == b.User.Id { return reply(b, ctx, "I cannot unmute myself.") }
	member, err := b.GetChatMember(chatID, targetID, nil)
	if err != nil { return reply(b, ctx, "Could not find that user in this group: "+err.Error()) }
	if IsProtectedTarget(member.GetStatus()) { return reply(b, ctx, "I won't change the permissions of a group owner or administrator.") }
	if member.GetStatus() != gotgbot.ChatMemberStatusRestricted {
		return reply(b, ctx, "That user is not currently restricted in this group.")
	}
	if restricted, ok := member.(gotgbot.ChatMemberRestricted); ok && !restricted.IsMember {
		return reply(b, ctx, "That user is no longer a member of this group.")
	}

	// Telegram's documented unrestrict operation requires all permissions true.
	permissions := gotgbot.ChatPermissions{
		CanSendMessages: true,
		CanSendAudios: true,
		CanSendDocuments: true,
		CanSendPhotos: true,
		CanSendVideos: true,
		CanSendVideoNotes: true,
		CanSendVoiceNotes: true,
		CanSendPolls: true,
		CanSendOtherMessages: true,
		CanAddWebPagePreviews: true,
		CanChangeInfo: true,
		CanInviteUsers: true,
		CanPinMessages: true,
		CanManageTopics: boolPtr(true),
		CanReactToMessages: boolPtr(true),
		CanEditTag: boolPtr(true),
	}
	ok, err := b.RestrictChatMember(chatID, targetID, permissions, &gotgbot.RestrictChatMemberOpts{UseIndependentChatPermissions: true})
	if err != nil { return reply(b, ctx, "Telegram could not unmute that user: "+err.Error()) }
	if !ok { return reply(b, ctx, "Telegram did not confirm the unmute.") }
	return reply(b, ctx, fmt.Sprintf("Unmuted user %d.", targetID))
}

func boolPtr(v bool) *bool { return &v }
