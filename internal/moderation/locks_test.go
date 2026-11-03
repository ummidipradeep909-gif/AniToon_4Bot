package moderation

import (
	"reflect"
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

func TestParseLockType(t *testing.T) {
	for _, tc := range []struct{ args []string; want string; wantErr bool }{
		{nil, "all", false},
		{[]string{"MEDIA"}, "media", false},
		{[]string{"all"}, "all", false},
		{[]string{"unknown"}, "", true},
		{[]string{"media", "polls"}, "", true},
	} {
		got, err := parseLockType(tc.args, "all")
		if (err != nil) != tc.wantErr || got != tc.want {
			t.Fatalf("parseLockType(%v) = %q, %v; want %q, error=%v", tc.args, got, err, tc.want, tc.wantErr)
		}
	}
}

func TestUpdateLockTypes(t *testing.T) {
	tests := []struct{ initial []string; value string; locking bool; want []string }{
		{nil, "media", true, []string{"media"}},
		{[]string{"media"}, "polls", true, []string{"media", "polls"}},
		{[]string{"media", "polls"}, "media", false, []string{"polls"}},
		{[]string{"all"}, "media", false, []string{"messages", "stickers", "polls", "previews", "info", "invites", "pin", "topics", "reactions"}},
		{[]string{"media"}, "all", false, nil},
		{[]string{"media"}, "all", true, []string{"all"}},
	}
	for _, tc := range tests {
		got := updateLockTypes(tc.initial, tc.value, tc.locking)
		if !reflect.DeepEqual(got, tc.want) {
			t.Fatalf("updateLockTypes(%v,%q,%v) = %v, want %v", tc.initial, tc.value, tc.locking, got, tc.want)
		}
	}
}

func TestApplyPermissionLocks(t *testing.T) {
	original := gotgbot.ChatPermissions{
		CanSendMessages: true, CanSendAudios: true, CanSendDocuments: true,
		CanSendPhotos: true, CanSendVideos: true, CanSendVideoNotes: true,
		CanSendVoiceNotes: true, CanSendPolls: true, CanSendOtherMessages: true,
		CanAddWebPagePreviews: true, CanChangeInfo: true, CanInviteUsers: true,
		CanPinMessages: true,
	}
	got := applyPermissionLocks(original, []string{"media", "polls", "previews"})
	if !got.CanSendAudios || got.CanSendMessages != true { // messages remain enabled
		t.Fatal("media lock unexpectedly changed message permissions")
	}
	if got.CanSendDocuments || got.CanSendPhotos || got.CanSendVideos || got.CanSendVideoNotes || got.CanSendVoiceNotes {
		t.Fatal("media lock did not disable every media permission")
	}
	if got.CanSendPolls || got.CanAddWebPagePreviews {
		t.Fatal("poll or preview lock was not applied")
	}
	if !original.CanSendPhotos || !original.CanSendPolls {
		t.Fatal("applyPermissionLocks mutated its input")
	}
}

func TestPermissionPayloadIncludesFalseValues(t *testing.T) {
	payload := permissionPayload(gotgbot.ChatPermissions{})
	for _, key := range []string{"can_send_messages", "can_send_photos", "can_send_other_messages", "can_invite_users"} {
		value, ok := payload[key]
		if !ok || value != false {
			t.Fatalf("payload[%q] = %v, present=%v; expected explicit false", key, value, ok)
		}
	}
}
