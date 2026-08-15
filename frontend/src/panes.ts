import { ref, watch, type Ref } from 'vue'
import type { ModuleId } from './modules'

const PREFIX = 'working:pane-width:'
const VISIBLE_PREFIX = 'working:pane-visible:'

function load(key: string, fallback: number): number {
  try {
    const raw = localStorage.getItem(PREFIX + key)
    if (raw === null) return fallback
    const parsed = Number(raw)
    return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback
  } catch {
    return fallback
  }
}

/**
 * 드래그로 조절한 패널 너비(px)를 기억한다.
 * 모듈을 오갈 때마다 컴포넌트가 새로 만들어지므로 localStorage에 남겨 둔다.
 */
export function usePaneWidth(key: string, fallback: number): Ref<number> {
  const width = ref(load(key, fallback))
  watch(width, (value) => {
    try {
      localStorage.setItem(PREFIX + key, String(value))
    } catch {
      // 저장소를 쓸 수 없어도 현재 세션의 너비는 그대로 유지된다.
    }
  })
  return width
}

/** SidePane은 모듈이 접을 수 있는 측면 패널 하나이다. */
export interface SidePane {
  /** key는 usePaneWidth와 같은 값을 써서 너비와 가시성을 한 패널로 묶는다. */
  key: string

  /**
   * icon은 탭 바 버튼에 그릴 글자이다.
   * 화면 어느 쪽인지보다 그 패널에 무엇이 들어 있는지가 알아보기 쉬우므로,
   * 내용이 뚜렷한 패널은 위치 기호 대신 그 내용을 나타내는 기호를 쓴다.
   */
  icon: string

  /** label은 버튼 툴팁에 쓰는 패널 이름이다. */
  label: string
}

/**
 * MODULE_PANES는 모듈마다 접을 수 있는 측면 패널을 적어 둔 표이다.
 * 탭 바는 지금 보고 있는 모듈의 항목만 버튼으로 그린다.
 */
export const MODULE_PANES: Partial<Record<ModuleId, SidePane[]>> = {
  calendar: [
    { key: 'calendar:sidebar', icon: '◧', label: '캘린더 목록' },
    { key: 'calendar:detail', icon: '◨', label: '일정 상세' },
  ],
  email: [
    { key: 'email:sidebar', icon: '◧', label: '계정·폴더' },
    // 메일 목록은 가운데 칸이라 위치 기호를 쓰면 오른쪽 패널로 오해하기 쉽다.
    { key: 'email:list', icon: '✉', label: '메일 목록' },
  ],
  kanban: [{ key: 'kanban:sidebar', icon: '◧', label: '보드 목록' }],
  document: [{ key: 'document:sidebar', icon: '◧', label: '문서 목록' }],
}

// 가시성은 탭 바 버튼과 모듈 화면이 함께 보므로 키마다 하나의 ref를 돌려 쓴다.
// 너비와 달리 컴포넌트 밖에서도 바꾸기 때문에 여기서 모아 둔다.
const visibilityRefs = new Map<string, Ref<boolean>>()

/** usePaneVisible은 패널을 펼쳐 둘지 여부를 돌려준다. 기본값은 펼침이다. */
export function usePaneVisible(key: string): Ref<boolean> {
  const cached = visibilityRefs.get(key)
  if (cached) return cached

  let initial = true
  try {
    initial = localStorage.getItem(VISIBLE_PREFIX + key) !== 'false'
  } catch {
    // 저장소를 쓸 수 없으면 펼친 상태로 시작한다.
  }
  const visible = ref(initial)
  watch(visible, (value) => {
    try {
      localStorage.setItem(VISIBLE_PREFIX + key, String(value))
    } catch {
      // 저장소를 쓸 수 없어도 현재 세션의 상태는 그대로 유지된다.
    }
  })
  visibilityRefs.set(key, visible)
  return visible
}

/** togglePane은 패널을 접거나 펼친다. 탭 바 버튼이 쓴다. */
export function togglePane(key: string) {
  const visible = usePaneVisible(key)
  visible.value = !visible.value
}
