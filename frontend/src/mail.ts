import { ref, watch } from 'vue'

/**
 * 메일 모듈의 사용자 설정. 설정 화면과 이메일 화면이 함께 본다.
 */
const UNDO_SEND_KEY = 'working:mail-undo-send'
const UNDO_SEND_SECONDS_KEY = 'working:mail-undo-send-seconds'

/** 전송 취소로 쓸 수 있는 시간(초). 설정 화면의 선택지이기도 하다. */
export const UNDO_SEND_CHOICES = [5, 10, 20, 30]

function loadEnabled(): boolean {
  try {
    // 기본은 꺼 둔다. 바로 보내던 지금까지의 동작이 그대로 유지된다.
    return localStorage.getItem(UNDO_SEND_KEY) === 'true'
  } catch {
    return false
  }
}

function loadSeconds(): number {
  try {
    const raw = Number(localStorage.getItem(UNDO_SEND_SECONDS_KEY))
    return UNDO_SEND_CHOICES.includes(raw) ? raw : 10
  } catch {
    return 10
  }
}

/** undoSendEnabled가 켜져 있으면 보내기를 누른 뒤 잠시 취소할 수 있다. */
export const undoSendEnabled = ref(loadEnabled())

/** undoSendSeconds는 그 취소 가능 시간이다. */
export const undoSendSeconds = ref(loadSeconds())

function remember(key: string, value: string) {
  try {
    localStorage.setItem(key, value)
  } catch {
    // 저장소를 쓸 수 없어도 현재 세션의 설정은 그대로 유지된다.
  }
}

watch(undoSendEnabled, value => remember(UNDO_SEND_KEY, String(value)))
watch(undoSendSeconds, value => remember(UNDO_SEND_SECONDS_KEY, String(value)))
