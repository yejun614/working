import { Events } from '@wailsio/runtime'

/**
 * 창을 다녀왔을 때 글 쓰던 자리를 되찾아 주는 보정.
 *
 * WebView2는 다른 창에 갔다가 돌아오면 웹뷰 안의 포커스를 되살리지 않고
 * document.body로 되돌려 놓는다. 그래서 쓰던 입력란에서 커서가 빠지고, 다시
 * 글을 이어 쓸 때 Chromium이 입력기에 캐럿 위치를 알려 주지 못해 한국어 후보
 * 창이 캐럿이 아니라 모니터 왼쪽 위에 뜬다.
 *
 * 창이 다시 활성화될 때 직전에 글을 쓰던 요소로 포커스를 되돌려 준다. 그러면
 * 커서도 제자리로 오고, Chromium이 캐럿 위치를 다시 계산해 입력기에 알린다.
 */

interface FocusSnapshot {
  element: HTMLElement
  start: number | null
  end: number | null
  direction: 'forward' | 'backward' | 'none' | null
}

let snapshot: FocusSnapshot | null = null
// 조합 중에 포커스를 건드리면 쓰던 글자가 끊기므로 그때는 손대지 않는다.
let composing = false

function isEditable(node: EventTarget | null): node is HTMLElement {
  return (
    node instanceof HTMLInputElement ||
    node instanceof HTMLTextAreaElement ||
    (node instanceof HTMLElement && node.isContentEditable)
  )
}

// 선택 범위는 입력란에서만 기억한다. 리치 텍스트 편집기는 자기 선택 상태를
// 따로 들고 있어서, 포커스만 돌려주면 알아서 커서를 제자리에 놓는다.
function capture(element: HTMLElement) {
  if (element instanceof HTMLInputElement || element instanceof HTMLTextAreaElement) {
    snapshot = {
      element,
      start: element.selectionStart,
      end: element.selectionEnd,
      direction: element.selectionDirection,
    }
    return
  }
  snapshot = { element, start: null, end: null, direction: null }
}

function restore() {
  const snap = snapshot
  if (!snap || composing) return
  // 화면에서 사라진 요소면 되돌릴 곳이 없다.
  if (!snap.element.isConnected) {
    snapshot = null
    return
  }
  // 창으로 돌아오면서 사용자가 다른 곳을 눌렀다면 그 선택을 존중한다.
  const active = document.activeElement
  if (active && active !== document.body && active !== snap.element) return

  const element = snap.element
  element.focus()
  if (
    (element instanceof HTMLInputElement || element instanceof HTMLTextAreaElement) &&
    snap.start !== null &&
    snap.end !== null
  ) {
    element.setSelectionRange(snap.start, snap.end, snap.direction ?? undefined)
  }
}

// 창이 활성화된 직후에는 브라우저가 포커스를 정리하는 중이라 한 박자 뒤에 처리한다.
function restoreSoon() {
  setTimeout(restore, 0)
}

/** setupIMEFix는 앱이 시작할 때 한 번 부른다. */
export function setupIMEFix() {
  // 캡처 단계로 듣는다. 편집기가 이벤트를 멈춰도 조합 여부는 놓치지 않는다.
  document.addEventListener('compositionstart', () => { composing = true }, true)
  document.addEventListener('compositionend', () => { composing = false }, true)

  // 글을 쓰던 요소와 커서 자리를 계속 기억해 둔다. 창을 벗어날 때는 이미
  // 포커스가 빠진 뒤라 activeElement로는 알 수 없으므로 여기서 남긴다.
  document.addEventListener('focusin', event => {
    if (isEditable(event.target)) capture(event.target)
  }, true)
  document.addEventListener('focusout', event => {
    if (isEditable(event.target)) capture(event.target)
  }, true)

  window.addEventListener('focus', restoreSoon)

  // 웹뷰의 focus 이벤트만으로는 창이 다시 활성화된 순간을 놓칠 때가 있어,
  // Wails가 창에서 직접 받아 넘겨 주는 활성화 알림도 함께 듣는다.
  Events.On('windows:WindowActive', restoreSoon)
  Events.On('windows:WindowClickActive', restoreSoon)
  Events.On('windows:WindowSetFocus', restoreSoon)
}
