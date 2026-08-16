//go:build windows

// Package platform은 특정 운영체제에서만 필요한 보정을 모아 둔다.
package platform

import (
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// 다루는 Windows 메시지들.
const (
	wmMove         = 0x0003
	wmSetFocus     = 0x0007
	wmNCDestroy    = 0x0082
	wmExitSizeMove = 0x0232
)

// GetWindow에 넘기는 값. 첫 자식과, 형제 중 다음 창을 뜻한다.
const (
	gwHwndNext = 2
	gwChild    = 5
)

// webviewClass는 WebView2가 만드는 창의 클래스 이름 앞부분이다.
const webviewClass = "Chrome_WidgetWin"

// subclassID는 이 창에 걸어 둔 우리 서브클래스를 구분하는 표시다.
const subclassID = 0x7766

var (
	user32           = syscall.NewLazyDLL("user32.dll")
	procPostMessage  = user32.NewProc("PostMessageW")
	procGetWindow    = user32.NewProc("GetWindow")
	procSetFocus     = user32.NewProc("SetFocus")
	procGetClassName = user32.NewProc("GetClassNameW")

	comctl32                 = syscall.NewLazyDLL("comctl32.dll")
	procSetWindowSubclass    = comctl32.NewProc("SetWindowSubclass")
	procRemoveWindowSubclass = comctl32.NewProc("RemoveWindowSubclass")
	procDefSubclassProc      = comctl32.NewProc("DefSubclassProc")
)

/*
FixWebviewFocus는 웹뷰가 키보드 포커스를 놓치지 않도록 한다.

한국어 입력기는 Windows가 보는 키보드 포커스를 기준으로 문맥을 붙인다. 포커스가
웹뷰의 입력 창이 아니라 최상위 창에 있으면 문맥이 붙을 곳이 없고, 그러면 조합 중인
글자를 그릴 자리도 정해지지 않아 화면 왼쪽 위에 그려진다.

Wails는 포커스를 최상위 창에 두는 곳이 두 군데 있다.

  - WM_SETFOCUS에서 MoveFocus(PROGRAMMATIC)만 부른다. 웹뷰 안에서 논리적으로만
    포커스를 옮기므로 Windows가 보는 포커스는 최상위 창에 남는다.
  - WM_ENTERSIZEMOVE에서 SetFocus(최상위)를 부른다. 창을 옮기거나 크기를 조절할 때
    열려 있는 드롭다운을 닫기 위해서인데, 끝나고 되돌리지 않는다.

그래서 창 프로시저를 가로채 두 시점에서 웹뷰 자식 창으로 포커스를 넘긴다.
같은 WebView2를 쓰는 Tauri(wry)도 WM_SETFOCUS에서 같은 일을 한다.

이 일은 반드시 창 프로시저 안에서 곧바로 해야 한다. Wails 이벤트로 받아 처리하면
채널과 메인 스레드 큐를 거치느라 0.5~40ms 늦게 실행되는데, 그 사이에 포커스가 또
바뀌면 엉뚱한 시점에 손대게 되어 빠르게 창을 오갈 때 간헐적으로 어긋난다.

참고: wry src/webview2/mod.rs의 WM_SETFOCUS 처리, DioxusLabs/dioxus#2900,
wailsapp/wails#3783.
*/
func FixWebviewFocus(window *application.WebviewWindow) {
	var installed sync.Once
	install := func(*application.WindowEvent) {
		handle := window.NativeWindow()
		if handle == nil {
			return
		}
		installed.Do(func() {
			// 서브클래스는 창을 만든 스레드에서 걸어야 한다.
			application.InvokeAsync(func() {
				_, _, _ = procSetWindowSubclass.Call(uintptr(handle), focusSubclass, subclassID, 0)
				focusChild(uintptr(handle))
			})
		})
	}
	// 창이 준비되는 시점은 상황에 따라 다르므로 이른 이벤트 몇 개에서 시도하고,
	// 실제 설치는 sync.Once가 한 번만 하도록 막는다.
	window.OnWindowEvent(events.Windows.WindowShow, install)
	window.OnWindowEvent(events.Windows.WindowSetFocus, install)
	window.OnWindowEvent(events.Windows.WebViewNavigationCompleted, install)

	// 웹뷰가 기억하는 창 위치를 다시 알려 줘야 하는 시점들. WebView2는 부모 창의
	// 화면 좌표를 따로 들고 있다가 그 자리를 기준으로 드롭다운이나 입력기 창을
	// 띄우는데, 이 좌표는 NotifyParentWindowPositionChanged를 불러야 갱신된다.
	// Wails는 창이 움직일 때(WM_MOVE)만 이를 호출하므로, 창을 옮기지 않아도 좌표가
	// 어긋날 수 있는 경우에는 우리가 WM_MOVE를 보내 대신 알리도록 한다.
	//
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
		events.Windows.WindowRestore,
		events.Windows.WindowUnMinimise,
		events.Windows.WindowEndResize,
		events.Windows.WindowDPIChanged,
		events.Windows.WindowShow,
	} {
		window.OnWindowEvent(event, notify)
	}
}

// focusSubclass는 창 프로시저를 가로채는 콜백이다. syscall.NewCallback으로 만든
// 콜백은 되돌릴 수 없으므로 패키지에 하나만 둔다.
// 콜백 안에서 이 값을 다시 쓰므로(서브클래스 해제) init에서 만들어 순환을 피한다.
var focusSubclass uintptr

func init() {
	focusSubclass = syscall.NewCallback(onWindowMessage)
}

func onWindowMessage(hwnd, msg, wparam, lparam, id, refData uintptr) uintptr {
	// 원래 처리를 먼저 돌린다. Wails가 최상위 창으로 포커스를 가져가는 일까지
	// 끝난 뒤에 우리가 마지막으로 확정해야 덮어써지지 않는다.
	result, _, _ := procDefSubclassProc.Call(hwnd, msg, wparam, lparam)

	switch msg {
	case wmSetFocus, wmExitSizeMove:
		focusChild(hwnd)
	case wmNCDestroy:
		_, _, _ = procRemoveWindowSubclass.Call(hwnd, focusSubclass, subclassID)
	}
	return result
}

// focusChild는 웹뷰 창에 키보드 포커스를 준다.
// 창 프로시저 안에서 불리므로 이미 창을 가진 스레드 위에 있다.
func focusChild(hwnd uintptr) {
	child := webviewChild(hwnd)
	if child == 0 {
		return
	}
	_, _, _ = procSetFocus.Call(child)
}

// webviewChild는 자식 창 중에서 WebView2 창을 찾는다.
// 첫 자식을 그냥 쓰지 않는 이유는 GetWindow가 Z 순서를 기준으로 돌려주기 때문이다.
// 다른 자식 창이 잠깐 위로 올라오면 엉뚱한 창에 포커스를 주게 되고, 그것이
// 간헐적으로만 어긋나는 원인이 된다. 클래스 이름으로 확인해 고르고, 끝내 못
// 찾으면 첫 자식으로 되돌아간다.
func webviewChild(hwnd uintptr) uintptr {
	first, _, _ := procGetWindow.Call(hwnd, gwChild)
	for child := first; child != 0; {
		if strings.HasPrefix(classNameOf(child), webviewClass) {
			return child
		}
		child, _, _ = procGetWindow.Call(child, gwHwndNext)
	}
	return first
}

// classNameOf는 창 클래스 이름을 읽는다.
func classNameOf(hwnd uintptr) string {
	var name [64]uint16
	length, _, _ := procGetClassName.Call(hwnd, uintptr(unsafe.Pointer(&name[0])), uintptr(len(name)))
	if length == 0 {
		return ""
	}
	return syscall.UTF16ToString(name[:length])
}
