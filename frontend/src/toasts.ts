import { ref } from 'vue'

/**
 * 되돌릴 수 있는 작업을 잠시 미뤄 두는 곳이다.
 *
 * 지우거나 보내기 전에 한 번 더 묻는 대신, 화면에서는 먼저 처리한 것처럼
 * 보여 주고 실제 작업은 잠깐 미룬다. 그 사이 오른쪽 아래 알림에서 되돌릴 수
 * 있고, 시간이 지나면 그때 진짜로 실행한다. 되돌리기가 서버에 다시 요청을
 * 보내는 방식이 아니라 아직 실행하지 않은 것이므로 실패할 일이 없다.
 */
export interface PendingAction {
  id: number

  /** 알림에 보여 줄 문구 */
  message: string

  /** 되돌리기 버튼에 쓸 말 (예: 되돌리기, 전송 취소) */
  actionLabel: string

  /** 남은 시간(ms). 알림이 남은 시간을 막대와 숫자로 보여 준다. */
  remaining: number

  /** 처음 정한 시간(ms) */
  total: number
}

interface Entry extends PendingAction {
  deadline: number
  commit: () => void | Promise<void>
  cancel?: () => void
}

const entries = new Map<number, Entry>()
let nextId = 1
let ticker: ReturnType<typeof setInterval> | undefined

/** pendingActions는 화면에 띄울 알림 목록이다. 먼저 넣은 것이 앞에 온다. */
export const pendingActions = ref<PendingAction[]>([])

function publish() {
  pendingActions.value = [...entries.values()].map(entry => ({
    id: entry.id,
    message: entry.message,
    actionLabel: entry.actionLabel,
    remaining: entry.remaining,
    total: entry.total,
  }))
  if (entries.size && !ticker) ticker = setInterval(tick, 200)
  if (!entries.size && ticker) {
    clearInterval(ticker)
    ticker = undefined
  }
}

function tick() {
  const now = Date.now()
  for (const entry of [...entries.values()]) {
    entry.remaining = Math.max(0, entry.deadline - now)
    if (entry.remaining === 0) void run(entry.id)
  }
  publish()
}

// run은 미뤄 둔 작업을 실제로 실행한다. 목록에서 먼저 빼서 두 번 실행되지 않게 한다.
async function run(id: number) {
  const entry = entries.get(id)
  if (!entry) return
  entries.delete(id)
  publish()
  await entry.commit()
}

export interface ScheduleOptions {
  message: string
  actionLabel: string

  /** 되돌릴 수 있는 시간(초) */
  seconds: number

  /** 시간이 지나면 실행할 작업. 실패 처리는 이 안에서 한다. */
  commit: () => void | Promise<void>

  /** 사용자가 되돌리기를 눌렀을 때 화면을 원래대로 되돌리는 작업 */
  cancel?: () => void
}

/** scheduleAction은 작업을 미뤄 두고 알림을 띄운다. */
export function scheduleAction(options: ScheduleOptions): number {
  const id = nextId++
  const total = Math.max(1, Math.round(options.seconds)) * 1000
  entries.set(id, {
    id,
    message: options.message,
    actionLabel: options.actionLabel,
    remaining: total,
    total,
    deadline: Date.now() + total,
    commit: options.commit,
    cancel: options.cancel,
  })
  publish()
  return id
}

/**
 * completeAction은 남은 시간을 기다리지 않고 지금 실행한다.
 * 알림을 닫는 것은 되돌리지 않겠다는 뜻이므로, 알림만 감추고 타이머를
 * 남겨 두는 대신 그 자리에서 마무리한다.
 */
export function completeAction(id: number) {
  void run(id)
}

/** cancelAction은 미뤄 둔 작업을 실행하지 않고 되돌린다. */
export function cancelAction(id: number) {
  const entry = entries.get(id)
  if (!entry) return
  entries.delete(id)
  publish()
  entry.cancel?.()
}

/**
 * flushActions는 남은 작업을 기다리지 않고 모두 실행한다.
 * 앱을 닫을 때처럼 더 기다릴 수 없는 시점에 부른다.
 */
export function flushActions() {
  for (const id of [...entries.keys()]) void run(id)
}
