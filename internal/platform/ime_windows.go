//go:build windows

// Package platform은 특정 운영체제에서만 필요한 보정을 모아 둔다.
package platform

import (
	"syscall"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// wmMove는 창이 움직였다고 알리는 Windows 메시지이다.
const wmMove = 0x0003

var (
	user32          = syscall.NewLazyDLL("user32.dll")
	procPostMessage = user32.NewProc("PostMessageW")
)

// FixIMEPosition은 한국어 입력기 후보 창이 모니터 왼쪽 위에 뜨는 문제를 줄인다.
//
// WebView2는 부모 창의 화면 좌표를 따로 기억해 두고 그 자리를 기준으로 입력기
// 후보 창이나 드롭다운을 띄운다. 이 좌표는 NotifyParentWindowPositionChanged를
// 불러야 갱신되는데, Wails는 창이 움직일 때(WM_MOVE)만 이를 호출한다. 그래서
// 창을 옮기지 않고도 화면 좌표가 달라지는 경우 — 모니터 배치나 배율 변경,
// 최소화 복원, 크기 변경 — 에는 좌표가 옛것으로 남아 후보 창이 화면 원점에
// 그려진다. 창을 최소화했다 켜면 괜찮아지는 것도 그때 WM_MOVE가 오기 때문이다.
//
// Wails가 창 밖으로 WebView2 객체를 내주지 않으므로, 좌표가 흔들릴 만한
// 시점마다 창에 WM_MOVE를 한 번 보내 Wails가 대신 알리도록 한다. Wails의
// WM_MOVE 처리는 위치 통지와 이동 이벤트 발생뿐이라 다른 부작용은 없다.
func FixIMEPosition(window *application.WebviewWindow) {
	notify := func(*application.WindowEvent) {
		handle := window.NativeWindow()
		if handle == nil {
			return
		}
		_, _, _ = procPostMessage.Call(uintptr(handle), wmMove, 0, 0)
	}

	// 이동(WindowDidMove)은 Wails가 이미 처리하므로 넣지 않는다.
	// 넣으면 우리가 보낸 WM_MOVE가 다시 이 콜백을 불러 끝없이 돈다.
	for _, event := range []events.WindowEventType{
		events.Windows.WindowSetFocus,
		events.Windows.WindowRestore,
		events.Windows.WindowUnMinimise,
		events.Windows.WindowEndResize,
		events.Windows.WindowDPIChanged,
		events.Windows.WindowShow,
	} {
		window.OnWindowEvent(event, notify)
	}
}
