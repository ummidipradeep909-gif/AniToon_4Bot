package moderation

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/PaulSonOfLars/gotgbot/v2"
	"github.com/PaulSonOfLars/gotgbot/v2/ext"
	"github.com/PaulSonOfLars/gotgbot/v2/ext/handlers"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var lockTypes = []string{"messages", "media", "stickers", "polls", "previews", "info", "invites", "pin", "topics", "reactions"}

// RegisterLockCommands installs per-chat permission lock commands on the
// shared dispatcher used by the primary bot and connected clones.
func RegisterLockCommands(d *ext.Dispatcher, pool *pgxpool.Pool) {
	d.AddHandler(handlers.NewCommand("lock", func(b *gotgbot.Bot, c *ext.Context) error {
		if pool == nil { return reply(b, c, "Lock settings are unavailable. Configure DATABASE_URL and restart the bot.") }
		return changeChatLock(b, c, pool, true)
	}))
	d.AddHandler(handlers.NewCommand("unlock", func(b *gotgbot.Bot, c *ext.Context) error {
		if pool == nil { return reply(b, c, "Lock settings are unavailable. Configure DATABASE_URL and restart the bot.") }
		return changeChatLock(b, c, pool, false)
	}))
	d.AddHandler(handlers.NewCommand("locks", func(b *gotgbot.Bot, c *ext.Context) error {
		if pool == nil { return reply(b, c, "Lock settings are unavailable. Configure DATABASE_URL and restart the bot.") }
		return listChatLocks(b, c, pool)
	}))
}

func parseLockType(args []string, defaultType string) (string, error) {
	if len(args) == 0 { return defaultType, nil }
	if len(args) != 1 { return "", fmt.Errorf("use one lock type: %s or all", strings.Join(lockTypes, ", ")) }
	value := strings.ToLower(strings.TrimSpace(args[0]))
	if value == "all" { return value, nil }
	for _, supported := range lockTypes {
		if value == supported { return value, nil }
	}
	return "", fmt.Errorf("unknown lock type %q. Supported: %s, all", value, strings.Join(lockTypes, ", "))
}

func expandedLockTypes(types []string) []string {
	for _, value := range types {
		if value == "all" { return append([]string(nil), lockTypes...) }
	}
	return append([]string(nil), types...)
}

func normalizeLockTypes(types []string) []string {
	for _, value := range types {
		if value == "all" { return []string{"all"} }
	}
	seen := make(map[string]bool, len(types))
	for _, value := range types { seen[value] = true }
	out := make([]string, 0, len(seen))
	for _, value := range lockTypes { if seen[value] { out = append(out, value) } }
	return out
}

func updateLockTypes(types []string, value string, locking bool) []string {
	current := normalizeLockTypes(types)
	if locking {
		if value == "all" { return []string{"all"} }
		for _, item := range current { if item == "all" { return current } }
		current = append(current, value)
		return normalizeLockTypes(current)
	}
	if value == "all" { return nil }
	if len(current) == 1 && current[0] == "all" {
		current = expandedLockTypes(current)
	}
	out := make([]string, 0, len(current))
	for _, item := range current { if item != value { out = append(out, item) } }
	return normalizeLockTypes(out)
}

func applyPermissionLocks(p gotgbot.ChatPermissions, types []string) gotgbot.ChatPermissions {
	for _, value := range expandedLockTypes(types) {
		switch value {
		case "messages": p.CanSendMessages = false
		case "media":
			p.CanSendAudios = false
			p.CanSendDocuments = false
			p.CanSendPhotos = false
			p.CanSendVideos = false
			p.CanSendVideoNotes = false
			p.CanSendVoiceNotes = false
		case "stickers": p.CanSendOtherMessages = false
		case "polls": p.CanSendPolls = false
		case "previews": p.CanAddWebPagePreviews = false
		case "info": p.CanChangeInfo = false
		case "invites": p.CanInviteUsers = false
		case "pin": p.CanPinMessages = false
		case "topics": p.CanManageTopics = boolPointer(false)
		case "reactions": p.CanReactToMessages = boolPointer(false)
		}
	}
	return p
}

func boolPointer(value bool) *bool { return &value }

// permissionPayload deliberately uses a map rather than ChatPermissions'
// generated JSON tags: the library marks bool fields omitempty, which would
// omit false values and make Telegram locks ineffective.
func permissionPayload(p gotgbot.ChatPermissions) map[string]any {
	payload := map[string]any{
		"can_send_messages": p.CanSendMessages,
		"can_send_audios": p.CanSendAudios,
		"can_send_documents": p.CanSendDocuments,
		"can_send_photos": p.CanSendPhotos,
		"can_send_videos": p.CanSendVideos,
		"can_send_video_notes": p.CanSendVideoNotes,
		"can_send_voice_notes": p.CanSendVoiceNotes,
		"can_send_polls": p.CanSendPolls,
		"can_send_other_messages": p.CanSendOtherMessages,
		"can_add_web_page_previews": p.CanAddWebPagePreviews,
		"can_change_info": p.CanChangeInfo,
		"can_invite_users": p.CanInviteUsers,
		"can_pin_messages": p.CanPinMessages,
	}
	if p.CanReactToMessages != nil { payload["can_react_to_messages"] = *p.CanReactToMessages }
	if p.CanEditTag != nil { payload["can_edit_tag"] = *p.CanEditTag }
	if p.CanManageTopics != nil { payload["can_manage_topics"] = *p.CanManageTopics }
	return payload
}

func setTelegramPermissions(b *gotgbot.Bot, chatID int64, permissions gotgbot.ChatPermissions) error {
	params := map[string]any{
		"chat_id": chatID,
		"permissions": permissionPayload(permissions),
		"use_independent_chat_permissions": true,
	}
	raw, err := b.Request("setChatPermissions", params, nil)
	if err != nil { return err }
	var ok bool
	if err = json.Unmarshal(raw, &ok); err != nil { return fmt.Errorf("could not parse Telegram's permission response: %w", err) }
	if !ok { return fmt.Errorf("Telegram did not confirm the permission update") }
	return nil
}

func changeChatLock(b *gotgbot.Bot, c *ext.Context, pool *pgxpool.Pool, locking bool) error {
	chatID, err := groupChat(c)
	if err != nil { return reply(b, c, err.Error()) }
	actor, err := actorID(c)
	if err != nil { return reply(b, c, err.Error()) }
	if err = RequireActorAdmin(b, chatID, actor); err != nil { return reply(b, c, err.Error()) }
	if err = RequireBotRestrictPermission(b, chatID); err != nil { return reply(b, c, err.Error()) }

	defaultType := "all"
	lockType, err := parseLockType(commandArgs(c.EffectiveMessage), defaultType)
	if err != nil { return reply(b, c, err.Error()) }

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	tx, err := pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil { return reply(b, c, "I couldn't access this group's lock settings.") }
	defer tx.Rollback(ctx)
	// Serialize lock changes per chat, including the first lock when no row exists yet.
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, chatID); err != nil {
		return reply(b, c, "I couldn't safely coordinate this group's lock update.")
	}

	if _, err = tx.Exec(ctx, `INSERT INTO chats (chat_id) VALUES ($1) ON CONFLICT (chat_id) DO NOTHING`, chatID); err != nil {
		return reply(b, c, "I couldn't prepare this group's lock settings.")
	}

	var snapshot []byte
	var active []string
	err = tx.QueryRow(ctx, `SELECT original_permissions, locked_types FROM chat_permission_locks WHERE chat_id=$1 FOR UPDATE`, chatID).Scan(&snapshot, &active)
	if err == pgx.ErrNoRows {
		if !locking { return reply(b, c, "This group has no saved permission locks.") }
		chat, getErr := b.GetChat(chatID, nil)
		if getErr != nil { return reply(b, c, "I couldn't read the group's current permissions: "+getErr.Error()) }
		if chat.Permissions == nil {
			return reply(b, c, "Telegram did not return the group's current default permissions. I won't change them because I cannot safely restore them later.")
		}
		snapshot, err = json.Marshal(chat.Permissions)
		if err != nil { return reply(b, c, "I couldn't save the group's original permissions.") }
		active = []string{}
		if _, err = tx.Exec(ctx, `INSERT INTO chat_permission_locks (chat_id, original_permissions, locked_types) VALUES ($1,$2,$3)`, chatID, snapshot, active); err != nil {
			return reply(b, c, "I couldn't save the group's original permissions.")
		}
	} else if err != nil {
		return reply(b, c, "I couldn't read this group's saved lock settings.")
	}

	next := updateLockTypes(active, lockType, locking)
	var original gotgbot.ChatPermissions
	if err = json.Unmarshal(snapshot, &original); err != nil {
		return reply(b, c, "Saved permission data is invalid. No Telegram permissions were changed.")
	}
	effective := applyPermissionLocks(original, next)
	if err = setTelegramPermissions(b, chatID, effective); err != nil {
		return reply(b, c, "Telegram rejected the permission change: "+err.Error())
	}

	if len(next) == 0 {
		_, err = tx.Exec(ctx, `DELETE FROM chat_permission_locks WHERE chat_id=$1`, chatID)
	} else {
		_, err = tx.Exec(ctx, `UPDATE chat_permission_locks SET locked_types=$2, updated_at=NOW() WHERE chat_id=$1`, chatID, next)
	}
	if err != nil { return reply(b, c, "Telegram permissions changed, but I couldn't save the lock state. Contact the bot owner before changing locks again.") }
	if err = tx.Commit(ctx); err != nil { return reply(b, c, "Telegram permissions changed, but the database commit failed. Contact the bot owner before changing locks again.") }

	if locking {
		return reply(b, c, fmt.Sprintf("🔒 Locked %s for regular members in this group.", lockType))
	}
	return reply(b, c, fmt.Sprintf("🔓 Unlocked %s. Remaining locks: %s", lockType, formatLockList(next)))
}

func listChatLocks(b *gotgbot.Bot, c *ext.Context, pool *pgxpool.Pool) error {
	chatID, err := groupChat(c)
	if err != nil { return reply(b, c, err.Error()) }
	actor, err := actorID(c)
	if err != nil { return reply(b, c, err.Error()) }
	if err = RequireActorAdmin(b, chatID, actor); err != nil { return reply(b, c, err.Error()) }

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var active []string
	err = pool.QueryRow(ctx, `SELECT locked_types FROM chat_permission_locks WHERE chat_id=$1`, chatID).Scan(&active)
	if err == pgx.ErrNoRows { return reply(b, c, "🔓 No active locks in this group.") }
	if err != nil { return reply(b, c, "I couldn't read this group's lock settings.") }
	return reply(b, c, "🔒 Active group locks: "+formatLockList(active))
}

func formatLockList(types []string) string {
	if len(types) == 0 { return "none" }
	if len(types) == 1 && types[0] == "all" { return "all" }
	ordered := normalizeLockTypes(types)
	sort.Strings(ordered)
	return strings.Join(ordered, ", ")
}
