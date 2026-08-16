# WebView2 한국어 입력기 문제 해결 기록

다른 창을 다녀오면 한국어 입력이 망가지던 문제를 고친 과정이다.
네 번 헛짚은 뒤 다섯 번째에 해결했고, 그 네 번을 어떻게 배제했는지까지 남긴다.

관련 커밋: `a3b6d98` → `a687e4d` → `0fe256a` → `2ee8364`(되돌림) → **`50642fa`(해결)**

---

## 요약

Wails는 창이 포커스를 받으면 WebView2에 `MoveFocus(PROGRAMMATIC)`만 알려 준다.
이것은 웹뷰 **안에서** 논리적으로 포커스를 옮길 뿐이라, 정작 **Windows가 보는
키보드 포커스**는 웹뷰의 입력 창으로 돌아오지 않는다. 입력기는 Windows가 보는
포커스를 기준으로 문맥을 붙이므로, 붙을 곳이 없으면 조합 중인 글자를 그릴 자리도
정해지지 않는다.

같은 WebView2를 쓰는 Tauri(wry)는 `WM_SETFOCUS`에서 자식 창에 Win32 `SetFocus`를
직접 부른다. 그 한 줄이 차이였다. 같은 일을 하도록 고쳤다.

---

## 증상

- 문서 모듈에서 한글을 치던 중 다른 창을 클릭한다.
- 다시 앱으로 돌아와 이어서 타이핑한다.
- 조합 중인 글자가 입력란이 아니라 **모니터 왼쪽 위**에 그려진다.
- 창을 최소화했다가 켜면 잠시 정상으로 돌아온다.

---

## 원인

포커스는 세 단계를 거친다.

```
최상위 창 (Wails가 만든 창)
   └─ WebView2 컨테이너 창
        └─ Chrome_RenderWidgetHostHWND   ← 실제로 키 입력을 받는 창
                                            입력기 문맥도 여기에 붙는다
```

Wails가 `WM_SETFOCUS`에서 하는 일은 다음과 같다.

```go
case w32.WM_SETFOCUS:
    w.focus()   // SetForegroundWindow(최상위) + chromium.Focus()
```

`chromium.Focus()`는 `MoveFocus(COREWEBVIEW2_MOVE_FOCUS_REASON_PROGRAMMATIC)`이다.
웹뷰 내부의 논리적 포커스만 움직이고, **Win32 키보드 포커스는 최상위 창에 머문다.**
그래서 입력기가 문맥을 붙일 창이 없다.

측정으로 확인한 것:

| 확인 항목 | 결과 |
| --- | --- |
| 창 복귀 시 `document.activeElement` | `BODY` (쓰던 입력란이 아님) |
| 입력 창의 `ImmGetContext` | `0` — 문맥 없음 (12회 관측, 전부 0) |
| `WindowSetFocus` 이벤트 발생 여부 | 정상 발생 (복귀 1회당 2번) |

---

## 수정

`internal/platform/ime_windows.go`

```go
// 창이 포커스를 받는 순간, 웹뷰가 실제 키보드 포커스를 가져가게 한다.
window.OnWindowEvent(events.Windows.WindowSetFocus, func(*application.WindowEvent) {
    focusWebview(window)
})

// SetFocus는 그 창을 만든 스레드에서만 통하므로 메인 스레드로 넘긴다.
func focusWebview(window *application.WebviewWindow) {
    handle := window.NativeWindow()
    if handle == nil {
        return
    }
    application.InvokeAsync(func() {
        child, _, _ := procGetWindow.Call(uintptr(handle), gwChild) // GW_CHILD
        if child == 0 {
            return
        }
        _, _, _ = procSetFocus.Call(child)
    })
}
```

핵심은 두 가지다.

1. **Win32 `SetFocus`를 자식 창에 직접 부른다.** `MoveFocus`로는 부족하다.
2. **메인 스레드에서 부른다.** `SetFocus`는 창을 소유한 스레드에서만 동작한다.
   Wails 이벤트 콜백은 다른 고루틴에서 실행되므로 `InvokeAsync`로 넘겨야 한다.

### 결과

| | 창 복귀 후 `document.activeElement` |
| --- | --- |
| 수정 전 | `BODY` |
| 수정 후 | `INPUT` |

---

## 왜 Tauri는 멀쩡했나

이 단서가 문제를 풀었다. 같은 WebView2를 쓰는데 한쪽만 정상이면 차이는 호스트 창
처리에 있다. wry 소스(`src/webview2/mod.rs`)를 받아 비교했다.

```rust
if msg == WM_SETFOCUS {
  // Fix https://github.com/DioxusLabs/dioxus/issues/2900
  let child = GetWindow(hwnd, GW_CHILD).ok();
  if child.is_some() {
    let _ = SetFocus(child);   // ← Wails에 없는 처리
  }
}
```

wry가 참조한 Dioxus #2900도 같은 계열이다. "다른 앱으로 갔다가 돌아오면 시스템이
포커스를 웹 문서로 리셋해 준다"는 우회법이 이슈 본문에 적혀 있는데, 이 문서의
증상에서 "최소화했다 켜면 잠시 괜찮아진다"와 정확히 같은 이야기다.

---

## 진단 기록

각 가설은 추측이 아니라 측정으로 배제했다.

### 시도 1 — 창 위치를 웹뷰에 다시 알리기 · 기각

WebView2는 부모 창의 화면 좌표를 따로 기억해 두고 그 자리를 기준으로 입력기 창을
띄운다. 이 좌표가 옛것이라 화면 원점에 그려진다고 보았다.

임시 로그로 확인한 결과, 문제 상황에서 이미 통지가 나가고 있었다.

```
[IME] event=WindowSetFocus
[IME] event=WindowActive
[IME] event=WindowSetFocus
```

→ 좌표 문제가 아니었다. 다만 모니터 배치·배율 변경처럼 좌표가 실제로 어긋나는
경우에는 여전히 유효하므로 이 처리는 남겨 두었다.

### 시도 2 — 웹에서 포커스 복원 · 무효

창이 돌아올 때 직전에 쓰던 요소로 `element.focus()`를 걸었다.
**동작하지 않았다. 내 코드의 버그였다.**

```ts
if (!snap || composing) return   // ← composing이 true로 굳어 버림
```

조합 여부를 `compositionend`에서만 되돌렸는데, **한글 조합 도중에 창을 벗어나면
`compositionend`가 오지 않는다.** 그 값이 참으로 남아 복원이 영구히 막혔다.
하필 "조합 중 창 이탈"이 문제가 생기는 바로 그 상황이라, 보정이 필요한 순간마다
정확히 꺼져 있었다.

### 시도 3 — 위 버그 수정 · 역효과, 되돌림

`composing`을 정리하자 복원은 실행됐지만 **더 나빠졌다.** 복귀 후 입력이 영문으로
고정되고 한영 전환도 듣지 않았다.

웹에서 `focus()`를 부르면 Chromium이 입력기 문맥을 **새로 만드는데**, 그 문맥이
영문 상태로 시작하고 모드 전환이 먹지 않는다. WebView2 이슈 #5637이 이 현상을
그대로 기술한다 — IMM32 호출도, 합성된 IME 키도 무시되고 하드웨어 스캔코드만 통한다.

→ 원래 증상보다 나쁜 상태를 남길 수 없으므로 전부 되돌렸다(`2ee8364`).

**웹 레이어에서 포커스를 조작하는 접근 자체가 이 문제에 쓰면 안 되는 방법이었다.**

### 시도 4 — 네이티브에서 조합 강제 확정 · 불가

입력 창을 찾아 `ImmNotifyIME(NI_COMPOSITIONSTR, CPS_COMPLETE)`로 조합을 끝내려 했다.

```
[IME] renderWidget=50076344   ← 창은 찾았다
[IME] himc=0                  ← 문맥이 없다 (12회 전부)
```

`ImmGetContext`가 항상 0이었다. Chromium은 구형 IMM32가 아니라 **TSF**를 쓰므로
IMM32 호출은 애초에 닿지 않는다. 동작하지 않는 코드라 전부 제거했다.

### 그 밖에 배제한 것

**Wails 업그레이드** — `alpha2.117`에서 `beta.8`까지 나와 있어 해당 소스를 받아
비교했다. `WM_SETFOCUS`·`WM_MOVE` 처리가 **바이트 단위로 동일**해 업그레이드로는
해결되지 않는다.

---

## 관련 이슈

이 문제는 우리 앱만의 것이 아니다. Wails·Tauri·.NET MAUI에서 같은 계열이 보고돼 있고,
**WebView2 쪽은 전부 미해결**이다.

| 저장소 | 이슈 | 상태 | 내용 |
| --- | --- | --- | --- |
| WebView2 | [#5637](https://github.com/MicrosoftEdge/WebView2Feedback/issues/5637) | OPEN 2026-07 | 한영 전환 프로그램 제어 불가, IMM32 무시 |
| WebView2 | [#5475](https://github.com/MicrosoftEdge/WebView2Feedback/issues/5475) | OPEN 2025-12 | 한글 조합 중 포커스 상실 → 크래시 |
| WebView2 | [#2241](https://github.com/MicrosoftEdge/WebView2Feedback/issues/2241) | OPEN 2022-03 | 후보창 위치 오류 (백로그 등록 후 4년째) |
| WebView2 | [#1611](https://github.com/MicrosoftEdge/WebView2Feedback/issues/1611) | OPEN 2021-08 | 창 이동 시 후보창이 커서를 안 따라감 |
| Wails | [#3783](https://github.com/wailsapp/wails/issues/3783) | OPEN 2024-09 | 창 활성화 시 WebView 포커스 미복원 |
| Tauri | [#15436](https://github.com/tauri-apps/tauri/issues/15436) | OPEN 2026-05 | 내용 있는 입력란 첫 포커스 시 TSF 멈춤 |
| Dioxus | [#2900](https://github.com/DioxusLabs/dioxus/issues/2900) | CLOSED | wry가 `SetFocus` 처리를 넣게 된 원본 |

Wails에 한국어 IME 관련 이슈는 아직 없다. 이 기록을 근거로 올릴 수 있다.

---

## 배운 것

**재현할 수 없으면 추측하지 말고 측정한다.** 시도 1~4는 전부 그럴듯했지만, 실제로
문제를 좁힌 것은 로그와 프로브였다. `himc=0`, `activeElement=BODY` 같은 값 하나가
가설 하나를 깨끗이 죽였다.

**"다른 데서는 안 그런다"는 제보가 가장 값싼 단서다.** Tauri에서 겪지 않았다는 한
문장이 며칠치 추측을 대신했다. 같은 하부 기술을 쓰는 두 구현을 비교하면 차이는
코드에 그대로 드러난다.

**내 수정이 원인일 가능성을 먼저 의심한다.** 시도 2는 업스트림 버그가 아니라 내가
쓴 조건문 하나 때문에 죽어 있었다.

**나빠지면 즉시 되돌린다.** 시도 3은 원래 증상보다 나빴다. 원인을 더 파기 전에
되돌리는 것이 먼저다.
