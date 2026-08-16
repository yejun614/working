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

// gwChild는 GetWindow에게 첫 번째 자식 창을 달라고 하는 값이다.
const gwChild = 5

var (
	user32          = syscall.NewLazyDLL("user32.dll")
	procPostMessage = user32.NewProc("PostMessageW")
	procGetWindow   = user32.NewProc("GetWindow")
	procSetFocus    = user32.NewProc("SetFocus")
)

/*
FixWebviewFocus는 창을 다녀왔을 때 웹뷰가 키보드 포커스를 되찾도록 한다.

다른 창에 갔다가 돌아오면 한국어 입력기가 망가진다. 조합 중인 글자가 입력란이
아니라 화면 왼쪽 위에 그려지고, 입력 모드도 제자리를 찾지 못한다.

뿌리는 키보드 포커스다. Wails는 창이 포커스를 받으면 WebView2에
MoveFocus(PROGRAMMATIC)만 알려 준다. 이것은 웹뷰 안에서 논리적으로 포커스를
옮길 뿐이라, 정작 Windows가 보는 키보드 포커스는 웹뷰의 입력 창으로 돌아오지
않는다. 입력기는 Windows가 보는 포커스를 기준으로 문맥을 붙이므로, 문맥이 붙을
곳이 없어 조합 글자를 그릴 자리도 정해지지 않는다.

같은 WebView2를 쓰는 Tauri(wry)에서 이 증상이 없는 이유가 여기에 있다. wry는
WM_SETFOCUS에서 자식 창을 찾아 Win32 SetFocus를 직접 부른다. 여기서도 같은 일을
한다. SetFocus는 창을 가진 스레드에서만 통하므로 메인 스레드로 넘겨 실행한다.

참고: wry src/webview2/mod.rs의 WM_SETFOCUS 처리, DioxusLabs/dioxus#2900,
wailsapp/wails#3783.

창 위치를 웹뷰에 다시 알려 주는 일도 함께 한다. WebView2는 부모 창의 화면
좌표를 따로 기억해 두었다가 그 자리를 기준으로 입력기 창이나 드롭다운을 띄우는데,
이 좌표는 NotifyParentWindowPositionChanged를 불러야 갱신된다. Wails는 창이
움직일 때(WM_MOVE)만 이를 호출하므로, 창을 옮기지 않아도 좌표가 어긋날 수 있는
경우(모니터 배치·배율 변경, 최소화 복원 등)에는 옛 좌표가 남는다. Wails가 창
밖으로 WebView2 객체를 내주지 않으므로, 그럴 만한 시점마다 창에 WM_MOVE를 보내
Wails가 대신 알리도록 한다.
*/
func FixWebviewFocus(window *application.WebviewWindow) {
	// 창이 포커스를 받는 순간. 여기서 웹뷰가 실제 키보드 포커스를 가져가야 한다.
	window.OnWindowEvent(events.Windows.WindowSetFocus, func(*application.WindowEvent) {
		focusWebview(window)
	})

	// 창 위치가 어긋났을 수 있는 시점들.
	// 이동(WindowDidMove)은 Wails가 이미 처리하므로 넣지 않는다.
	// 넣으면 우리가 보낸 WM_MOVE가 다시 이 콜백을 불러 끝없이 돈다.
	notify := func(*application.WindowEvent) {
		handle := window.NativeWindow()
		if handle == nil {
			return
		}
		_, _, _ = procPostMessage.Call(uintptr(handle), wmMove, 0, 0)
	}
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

// focusWebview는 창의 첫 자식(웹뷰 입력 창)에 키보드 포커스를 준다.
// SetFocus는 그 창을 만든 스레드에서 불러야 하므로 메인 스레드에서 실행한다.
func focusWebview(window *application.WebviewWindow) {
	handle := window.NativeWindow()
	if handle == nil {
		return
	}
	application.InvokeAsync(func() {
		child, _, _ := procGetWindow.Call(uintptr(handle), gwChild)
		if child == 0 {
			return
		}
		_, _, _ = procSetFocus.Call(child)
	})
}
