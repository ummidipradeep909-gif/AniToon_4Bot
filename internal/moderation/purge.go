package moderation

import (
	"fmt"
	"strconv"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
)

const maxPurgeMessages int64 = 100

// RegisterPurgeCommands installs /del and /purge on the shared dispatcher,
// making the commands available to the main bot and connected clone bots.
func RegisterPurgeCommands(d *ext.Dispatcher) {
	d.AddHandler(handlers.NewCommand("del", DeleteRepliedMessage))
	d.AddHandler(handlers.NewCommand("purge", PurgeMessages))
}

// RequireBotDeletePermission fails closed unless Telegram confirms that the
// bot is an administrator with the explicit delete-messages privilege.
func RequireBotDeletePermission(b *gotgbot.Bot, chatID int64) error {
	if b.User.Id == 0 {
		return fmt.Errorf("bot identity is unavailable")
	}
	member, err := b.GetChatMember(chatID, b.User.Id, nil)
	if err != nil {
		return fmt.Errorf("could not verify the bot's permissions: %w", err)
	}
	if member.GetStatus() != StatusAdmin {
		return fmt.Errorf("I must be a group administrator to delete messages")
	}
	admin, ok := member.(gotgbot.ChatMemberAdministrator)
	if !ok || !admin.CanDeleteMessages {
		return fmt.Errorf("I need Telegram's 'Delete messages' administrator permission")
	}
	return nil
}

// purgeRange returns an inclusive message-ID range. A reply-based purge starts
// at the replied-to message; a count-based purge includes the /purge command.
func purgeRange(message *gotgbot.Message, args []string) (int64, int64, error) {
	if message == nil || message.MessageId <= 0 {
		return 0, 0, fmt.Errorf("could not determine the command message ID")
	}
	if len(args) > 1 {
		return 0, 0, fmt.Errorf("usage: /purge [count] — reply to a message or provide a count from 1 to 100")
	}

	var start int64
	if len(args) == 1 {
		count, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil || count < 1 {
			return 0, 0, fmt.Errorf("purge count must be a number from 1 to 100")
		}
		if count > maxPurgeMessages {
			return 0, 0, fmt.Errorf("one purge can delete at most 100 messages")
		}
		start = message.MessageId - count + 1
		if start <= 0 {
			return 0, 0, fmt.Errorf("there are not enough earlier message IDs for that count")
		}
	} else {
		if message.ReplyToMessage == nil || message.ReplyToMessage.MessageId <= 0 {
			return 0, 0, fmt.Errorf("reply to the first message to purge, or use /purge <count> (1–100)")
		}
		start = message.ReplyToMessage.MessageId
		if start > message.MessageId {
			return 0, 0, fmt.Errorf("the replied-to message must come before the purge command")
		}
	}

	if message.MessageId-start+1 > maxPurgeMessages {
		return 0, 0, fmt.Errorf("one purge can delete at most 100 messages; use a smaller range")
	}
	return start, message.MessageId, nil
}

// DeleteRepliedMessage deletes only the message being replied to.
func DeleteRepliedMessage(b *gotgbot.Bot, ctx *ext.Context) error {
	chatID, err := groupChat(ctx)
	if err != nil {
		return reply(b, ctx, err.Error())
	}
	actor, err := actorID(ctx)
	if err != nil {
		return reply(b, ctx, err.Error())
	}
	if err = RequireActorAdmin(b, chatID, actor); err != nil {
		return reply(b, ctx, err.Error())
	}
	if err = RequireBotDeletePermission(b, chatID); err != nil {
		return reply(b, ctx, err.Error())
	}
	message := ctx.EffectiveMessage
	if message == nil || message.ReplyToMessage == nil || message.ReplyToMessage.MessageId <= 0 {
		return reply(b, ctx, "Usage: reply to the message you want to delete, then send /del.")
	}
	ok, err := b.DeleteMessage(chatID, message.ReplyToMessage.MessageId, nil)
	if err != nil {
		return reply(b, ctx, "Telegram could not delete the replied-to message: "+err.Error())
	}
	if !ok {
		return reply(b, ctx, "Telegram did not confirm deletion of the replied-to message.")
	}
	return nil
}

// PurgeMessages deletes an inclusive sequence of message IDs, one request at a
// time so Telegram's partial failures can be counted and reported honestly.
func PurgeMessages(b *gotgbot.Bot, ctx *ext.Context) error {
	chatID, err := groupChat(ctx)
	if err != nil {
		return reply(b, ctx, err.Error())
	}
	actor, err := actorID(ctx)
	if err != nil {
		return reply(b, ctx, err.Error())
	}
	if err = RequireActorAdmin(b, chatID, actor); err != nil {
		return reply(b, ctx, err.Error())
	}
	if err = RequireBotDeletePermission(b, chatID); err != nil {
		return reply(b, ctx, err.Error())
	}

	start, end, err := purgeRange(ctx.EffectiveMessage, commandArgs(ctx.EffectiveMessage))
	if err != nil {
		return reply(b, ctx, err.Error())
	}

	deleted, failed := int64(0), int64(0)
	// Delete newest-first so the /purge command itself can be removed early.
	for id := end; id >= start; id-- {
		ok, deleteErr := b.DeleteMessage(chatID, id, nil)
		if deleteErr != nil || !ok {
			failed++
			continue
		}
		deleted++
	}

	if failed == 0 {
		_, err = b.SendMessage(chatID, fmt.Sprintf("🧹 Purge complete: deleted %d message(s).", deleted), nil)
		return err
	}
	report := fmt.Sprintf("🧹 Purge partially completed: deleted %d message(s); %d message ID(s) could not be deleted. Telegram may reject messages older than 48 hours, missing IDs, or otherwise non-deletable messages.", deleted, failed)
	_, err = b.SendMessage(chatID, report, nil)
	return err
}

