package store

import (
	"testing"

	"working/internal/modules/email/types"
)

func TestMatchMessageIdentifiesByIDOrUID(t *testing.T) {
	tests := []struct {
		name    string
		message types.Message
		id      string
		uid     uint32
		want    bool
	}{
		{name: "Gmail 원격 ID 일치", message: types.Message{ID: "abc", UID: 1}, id: "abc", uid: 0, want: true},
		{name: "Gmail 원격 ID 불일치", message: types.Message{ID: "abc", UID: 1}, id: "xyz", uid: 1, want: false},
		{name: "IMAP UID 일치", message: types.Message{UID: 7}, id: "", uid: 7, want: true},
		{name: "IMAP UID 불일치", message: types.Message{UID: 7}, id: "", uid: 8, want: false},
		{name: "ID를 넘겨도 캐시에 ID가 없으면 UID로 비교", message: types.Message{UID: 7}, id: "abc", uid: 7, want: true},
		{name: "식별자가 모두 비어 있으면 일치하지 않음", message: types.Message{}, id: "", uid: 0, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := matchMessage(tt.message, tt.id, tt.uid); got != tt.want {
				t.Fatalf("메시지 식별 결과가 다릅니다: got %v, want %v", got, tt.want)
			}
		})
	}
}
