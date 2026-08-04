package clock

import (
	"errors"
	"strings"
	"testing"

	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

// fakeNotifier는 보낸 알림을 기록하는 테스트용 전송자이다.
type fakeNotifier struct {
	sent []notifications.NotificationOptions
	err  error
}

func (f *fakeNotifier) SendNotification(options notifications.NotificationOptions) error {
	f.sent = append(f.sent, options)
	return f.err
}

// serviceWithNotifier는 데스크톱 알림 준비를 건너뛰고 대역을 끼운 서비스를 만든다.
func serviceWithNotifier(target notifier) *Service {
	s := &Service{notify: target}
	// 이미 준비된 것으로 표시해 실제 알림 서비스를 초기화하지 않게 한다.
	s.notifyOnce.Do(func() {})
	return s
}

func TestNotifySendsTitleAndBody(t *testing.T) {
	fake := &fakeNotifier{}
	service := serviceWithNotifier(fake)

	if err := service.Notify("타이머 완료", "설정한 시간이 끝났습니다."); err != nil {
		t.Fatalf("알림 전송 실패: %v", err)
	}
	if len(fake.sent) != 1 {
		t.Fatalf("보낸 알림 수 = %d, want 1", len(fake.sent))
	}
	sent := fake.sent[0]
	if sent.Title != "타이머 완료" || sent.Body != "설정한 시간이 끝났습니다." {
		t.Fatalf("알림 내용이 다릅니다: %+v", sent)
	}
	if !strings.HasPrefix(sent.ID, "clock-") {
		t.Fatalf("알림 ID = %q, want clock- 접두사", sent.ID)
	}
}

func TestNotifyUsesNewIDEachTime(t *testing.T) {
	fake := &fakeNotifier{}
	service := serviceWithNotifier(fake)

	for i := 0; i < 2; i++ {
		if err := service.Notify("집중 완료", "휴식 시간입니다."); err != nil {
			t.Fatalf("알림 전송 실패: %v", err)
		}
	}
	if fake.sent[0].ID == fake.sent[1].ID {
		t.Fatalf("알림 ID가 같습니다(%q). 이전 알림을 대체할 수 있습니다", fake.sent[0].ID)
	}
}

func TestNotifyRejectsEmptyTitle(t *testing.T) {
	fake := &fakeNotifier{}
	service := serviceWithNotifier(fake)

	if err := service.Notify("   ", "본문"); err == nil {
		t.Fatal("제목이 비어 있으면 오류여야 합니다")
	}
	if len(fake.sent) != 0 {
		t.Fatalf("제목이 없는데 알림을 보냈습니다: %+v", fake.sent)
	}
}

func TestNotifyReturnsSendError(t *testing.T) {
	fake := &fakeNotifier{err: errors.New("알림이 차단되었습니다")}
	service := serviceWithNotifier(fake)

	if err := service.Notify("타이머 완료", "끝났습니다."); err == nil {
		t.Fatal("전송 실패를 그대로 알려야 합니다")
	}
}
