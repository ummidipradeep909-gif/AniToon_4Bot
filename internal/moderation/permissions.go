package moderation

import (
	"fmt"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

const (
	StatusCreator = "creator"
	StatusAdmin   = "administrator"
)

// IsChatAdmin reports whether Telegram identifies the member as a chat creator
// or administrator. Unknown and ordinary member states are never privileged.
func IsChatAdmin(status string) bool {
	return status == StatusCreator || status == StatusAdmin
}

// IsProtectedTarget prevents routine moderation commands from targeting chat
// creators or administrators. Telegram remains the final authority.
func IsProtectedTarget(status string) bool {
	return IsChatAdmin(status)
}

// RequireActorAdmin verifies the invoking user against Telegram's live member
// state. It deliberately fails closed if Telegram cannot verify the user.
func RequireActorAdmin(b *gotgbot.Bot, chatID, actorID int64) error {
	member, err := b.GetChatMember(chatID, actorID, nil)
	if err != nil {
		return fmt.Errorf("could not verify your admin status: %w", err)
	}
	if !IsChatAdmin(member.GetStatus()) {
		return fmt.Errorf("only chat administrators can use this command")
	}
	return nil
}

// RequireBotRestrictPermission verifies that the bot is an administrator with
// the explicit can_restrict_members privilege required by ban/unban actions.
func RequireBotRestrictPermission(b *gotgbot.Bot, chatID int64) error {
	if b.User == nil {
		return fmt.Errorf("bot identity is unavailable")
	}
	member, err := b.GetChatMember(chatID, b.User.Id, nil)
	if err != nil {
		return fmt.Errorf("could not verify the bot's permissions: %w", err)
	}
	if member.GetStatus() != StatusAdmin {
		return fmt.Errorf("I must be an administrator in this group")
	}
	admin, ok := member.(*gotgbot.ChatMemberAdministrator)
	if !ok || !admin.CanRestrictMembers {
		return fmt.Errorf("I need the 'Restrict members' permission to do that")
	}
	return nil
}

// RequireTargetIsMember prevents ban commands from silently targeting an
// already absent user and prevents routine commands from targeting admins.
func RequireTargetIsModeratable(b *gotgbot.Bot, chatID, targetID int64) error {
	member, err := b.GetChatMember(chatID, targetID, nil)
	if err != nil {
		return fmt.Errorf("could not find that user in this group: %w", err)
	}
	if IsProtectedTarget(member.GetStatus()) {
		return fmt.Errorf("I won't moderate a group owner or administrator")
	}
	if member.GetStatus() == gotgbot.ChatMemberStatusLeft || member.GetStatus() == gotgbot.ChatMemberStatusKicked {
		return fmt.Errorf("that user is not currently a member; use /unban with their numeric ID if needed")
	}
	return nil
}
