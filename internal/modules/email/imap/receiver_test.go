package imap

import (
	"testing"

	"github.com/emersion/go-imap"
)

func TestToMessageMapsSeenFlagToUnread(t *testing.T) {
	tests := []struct {
		name       string
		flags      []string
		wantUnread bool
	}{
		{name: "\\Seen 있으면 읽음", flags: []string{imap.SeenFlag}, wantUnread: false},
		{name: "\\Seen 없으면 읽지 않음", flags: nil, wantUnread: true},
		{name: "다른 플래그만 있으면 읽지 않음", flags: []string{imap.FlaggedFlag}, wantUnread: true},
		{name: "\\Seen과 다른 플래그 혼재", flags: []string{imap.FlaggedFlag, imap.SeenFlag}, wantUnread: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toMessage(&imap.Message{Uid: 42, Flags: tt.flags}, &imap.BodySectionName{Peek: true})
			if got.Unread != tt.wantUnread {
				t.Fatalf("읽지 않음 여부가 다릅니다: got %v, want %v", got.Unread, tt.wantUnread)
			}
			if got.UID != 42 {
				t.Fatalf("UID가 다릅니다: got %d, want 42", got.UID)
			}
		})
	}
}
