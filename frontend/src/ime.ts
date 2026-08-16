import { Events } from '@wailsio/runtime'

/**
 * 창을 다녀왔을 때 한국어 입력이 어긋나는 문제를 보정한다.
 *
 * WebView2에서는 글을 쓰다가 다른 창에 갔다 오면 두 가지가 어긋난다.
 *
 * 하나는 조합이다. 한글을 조합하는 도중에 창을 벗어나면 조합이 끝나지 않은 채
 * 남는다. compositionend가 오지 않는 경우도 있어, 웹 쪽에서는 아직 조합 중인
 * 것처럼 보인다.
 *
 * 다른 하나는 포커스다. 웹뷰는 창이 다시 활성화되어도 안쪽 포커스를 되살리지
 * 않고 document.body로 되돌려 놓는다. 글 쓰던 자리가 사라지는데, 이것이 입력기
 * 문제의 뿌리다. Chromium은 글을 쓸 수 있는 요소에 포커스가 있을 때만 입력기
 * 문맥을 창에 붙여 두기 때문이다. 실제로 웹뷰의 입력 창을 확인해 보면 포커스가
 * 빠진 상태에서는 입력기 문맥이 아예 없다. 문맥이 없으니 조합 중인 글자를
 * 그릴 자리도 정해지지 않아 화면 왼쪽 위의 기본 입력기 창에 그려진다.
 *
 * 그래서 창으로 돌아올 때 쓰던 자리로 포커스를 되돌린다. 그러면 Chromium이
 * 입력기 문맥을 다시 붙이고 조합 창 위치도 새로 잡는다.
 */

interface FocusSnapshot {
  element: HTMLElement
  start: number | null
  end: number | null
  direction: 'forward' | 'backward' | 'none' | null
}

let snapshot: FocusSnapshot | null = null
// 조합 중인지 여부. 창을 벗어날 때는 조합이 끝난 것으로 보고 반드시 되돌린다.
// 이 값이 참인 채로 남으면 아래 보정이 통째로 멈추므로 한곳에서만 관리한다.
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

/**
 * deactivate는 창을 벗어나기 직전에 글 쓰던 자리를 기억하고 포커스를 뗀다.
 * 포커스를 떼면 조합 중이던 글자가 확정된다.
 *
 * 다만 창을 벗어난 사실을 웹 쪽이 알았을 때는 이미 웹뷰가 포커스를 놓아 버린
 * 뒤일 때가 많다. 그때는 여기서 할 일이 없고, 아래 restore가 제자리를 되찾는다.
 */
function deactivate() {
  const active = document.activeElement
  if (!isEditable(active)) return
  capture(active)
  active.blur()
  // 조합 중에 창을 벗어나면 compositionend가 오지 않을 수 있어 여기서 정리한다.
  composing = false
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

  // 글을 쓰던 요소와 커서 자리를 계속 기억해 둔다.
  document.addEventListener('focusin', event => {
    if (isEditable(event.target)) capture(event.target)
  }, true)
  document.addEventListener('focusout', event => {
    if (isEditable(event.target)) capture(event.target)
    // 포커스가 빠지면 그 요소에서 조합이 이어질 수 없다.
    composing = false
  }, true)

  // 창을 벗어날 때. 웹뷰의 blur가 먼저 오고 Wails 알림이 뒤따르는데, 어느
  // 쪽이 먼저 와도 한 번만 처리되도록 deactivate가 스스로 확인한다.
  window.addEventListener('blur', deactivate)
  Events.On('windows:WindowInactive', deactivate)

  // 창으로 돌아올 때. 웹뷰의 focus 이벤트만으로는 순간을 놓칠 때가 있어
  // Wails가 창에서 직접 받아 넘겨 주는 활성화 알림도 함께 듣는다.
  window.addEventListener('focus', restoreSoon)
  Events.On('windows:WindowActive', restoreSoon)
  Events.On('windows:WindowClickActive', restoreSoon)
  Events.On('windows:WindowSetFocus', restoreSoon)
}
