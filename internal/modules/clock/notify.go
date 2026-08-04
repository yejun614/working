package clock

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/services/notifications"
)

// notifier는 데스크톱 알림 전송자이다.
// Wails 알림 서비스가 이 인터페이스를 만족하며, 테스트에서는 대역으로 바꿔 끼운다.
type notifier interface {
	SendNotification(options notifications.NotificationOptions) error
}

// desktopNotifier는 알림 서비스를 처음 쓸 때 준비한다.
//
// Wails 서비스로 등록하면 초기화 실패가 앱 실행 자체를 막는다(Windows에서는
// 레지스트리와 COM 등록을 한다). 알림은 있으면 좋은 기능이므로, 처음 알림을
// 보낼 때 준비하고 실패는 알림 기능에만 한정한다.
func (s *Service) desktopNotifier() (notifier, error) {
	s.notifyOnce.Do(func() {
		if s.notify != nil {
			return
		}
		service := notifications.New()
		if err := service.ServiceStartup(context.Background(), application.ServiceOptions{}); err != nil {
			s.notifyErr = fmt.Errorf("데스크톱 알림을 준비할 수 없습니다: %w", err)
			return
		}
		s.notify = service
		s.notifyShutdown = service.ServiceShutdown
	})
	if s.notifyErr != nil {
		return nil, s.notifyErr
	}
	return s.notify, nil
}

// Notify는 타이머나 뽀모도로 구간이 끝났음을 데스크톱 알림으로 알린다.
// 앱이 최소화되어 있거나 다른 창을 보고 있어도 알 수 있게 한다.
// 알림을 쓸 수 없는 환경에서는 오류를 반환하며, 화면은 소리와 문구로 안내한다.
func (s *Service) Notify(title, body string) error {
	if strings.TrimSpace(title) == "" {
		return fmt.Errorf("알림 제목이 필요합니다")
	}
	target, err := s.desktopNotifier()
	if err != nil {
		return err
	}
	return target.SendNotification(notifications.NotificationOptions{
		// 같은 ID로 보내면 이전 알림을 대체할 수 있어 매번 새 ID를 쓴다.
		// Windows의 시계 해상도가 낮아 시각만으로는 겹칠 수 있으므로 일련번호를 더한다.
		ID:    fmt.Sprintf("clock-%d-%d", time.Now().UnixNano(), s.notifySeq.Add(1)),
		Title: title,
		Body:  body,
	})
}
