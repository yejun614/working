//go:build !windows

package platform

import "github.com/wailsapp/wails/v3/pkg/application"

// FixIMEPosition은 Windows 밖에서는 할 일이 없다.
// 입력기 후보 창 위치가 어긋나는 문제는 WebView2에서만 생긴다.
func FixIMEPosition(window *application.WebviewWindow) {}
