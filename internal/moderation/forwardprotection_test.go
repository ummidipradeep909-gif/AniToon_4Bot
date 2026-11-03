package moderation

import (
	"testing"

	"github.com/PaulSonOfLars/gotgbot/v2"
)

func TestIsForwardedMessage(t *testing.T) {
	tests := []struct {
		name string
		origin gotgbot.MessageOrigin
		want bool
	}{
		{name: "not forwarded", origin: nil, want: false},
		{name: "forwarded from user", origin: gotgbot.MessageOriginUser{Date: 1, SenderUser: gotgbot.User{Id: 42}}, want: true},
		{name: "forwarded from hidden user", origin: gotgbot.MessageOriginHiddenUser{Date: 1, SenderUserName: "Hidden"}, want: true},
		{name: "forwarded from channel", origin: gotgbot.MessageOriginChannel{Date: 1, MessageId: 9}, want: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			msg := &gotgbot.Message{ForwardOrigin: tc.origin}
			got := msg.ForwardOrigin != nil
			if got != tc.want { t.Fatalf("forward detection = %v, want %v", got, tc.want) }
		})
	}
}
