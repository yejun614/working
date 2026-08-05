import { computed, ref, type Component } from 'vue'
import EmailView from './components/email/EmailView.vue'
import CalendarView from './components/calendar/CalendarView.vue'
import KanbanView from './components/kanban/KanbanView.vue'
import DocumentView from './components/document/DocumentView.vue'
import ClockView from './components/clock/ClockView.vue'
import AccountsView from './components/account/AccountsView.vue'

export type ModuleId = 'calendar' | 'email' | 'kanban' | 'document' | 'clock' | 'account'

export interface ModuleDefinition {
  id: ModuleId

  /** label은 탭 바와 설정 화면에 보이는 이름이다. */
  label: string

  component: Component

  /**
   * lazy가 true면 탭을 열 때 처음 만들고 다른 탭으로 옮기면 없앤다.
   * 무거운 편집기를 쓰거나 열 때마다 다시 읽어야 하는 모듈에 쓴다.
   */
  lazy?: boolean
}

// MODULE_DEFINITIONS는 앱이 제공하는 모듈 전체 목록이자, 처음 실행했을 때의 기본 순서다.
// 모듈을 추가하려면 여기에만 넣으면 탭 바와 설정 화면에 함께 나타난다.
export const MODULE_DEFINITIONS: ModuleDefinition[] = [
  { id: 'calendar', label: '캘린더', component: CalendarView },
  { id: 'email', label: '이메일', component: EmailView },
  { id: 'kanban', label: '칸반', component: KanbanView },
  { id: 'document', label: '문서', component: DocumentView, lazy: true },
  { id: 'clock', label: '시계', component: ClockView },
  { id: 'account', label: '계정', component: AccountsView, lazy: true },
]

/** ModuleLayoutEntry는 모듈 하나의 활성 여부다. 순서는 배열 순서로 나타낸다. */
export interface ModuleLayoutEntry {
  id: ModuleId
  enabled: boolean
}

const STORAGE_KEY = 'working:module-layout'

function definitionOf(id: ModuleId): ModuleDefinition | undefined {
  return MODULE_DEFINITIONS.find((definition) => definition.id === id)
}

function defaultLayout(): ModuleLayoutEntry[] {
  return MODULE_DEFINITIONS.map((definition) => ({ id: definition.id, enabled: true }))
}

// loadLayout은 저장된 배치를 읽어 현재 모듈 목록과 맞춘다.
// 없어진 모듈은 버리고 새로 생긴 모듈은 활성 상태로 뒤에 붙이므로, 앱을 업데이트해도 설정이 깨지지 않는다.
function loadLayout(): ModuleLayoutEntry[] {
  let stored: unknown
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    if (!raw) return defaultLayout()
    stored = JSON.parse(raw)
  } catch {
    return defaultLayout()
  }
  if (!Array.isArray(stored)) return defaultLayout()

  const layout: ModuleLayoutEntry[] = []
  const seen = new Set<ModuleId>()
  for (const item of stored) {
    const entry = item as Partial<ModuleLayoutEntry> | null
    const id = entry?.id
    if (!id || seen.has(id) || !definitionOf(id)) continue
    seen.add(id)
    layout.push({ id, enabled: entry?.enabled !== false })
  }
  for (const definition of MODULE_DEFINITIONS) {
    if (!seen.has(definition.id)) layout.push({ id: definition.id, enabled: true })
  }
  // 모두 꺼져 있으면 본문이 비어 버리므로 첫 모듈은 반드시 켠다.
  if (!layout.some((entry) => entry.enabled)) layout[0].enabled = true
  return layout
}

const layout = ref<ModuleLayoutEntry[]>(loadLayout())

function persist() {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(layout.value))
  } catch {
    // 저장소를 쓸 수 없는 환경에서도 현재 세션의 배치는 그대로 쓴다.
  }
}

/** moduleEntries는 사용자가 정한 순서대로 모든 모듈을 돌려준다. 설정 화면이 쓴다. */
export const moduleEntries = computed(() =>
  layout.value.flatMap((entry) => {
    const definition = definitionOf(entry.id)
    return definition ? [{ id: entry.id, enabled: entry.enabled, definition }] : []
  }),
)

/** enabledModules는 탭 바와 본문에 실제로 보여 줄 모듈이다. */
export const enabledModules = computed(() =>
  moduleEntries.value.filter((entry) => entry.enabled).map((entry) => entry.definition),
)

/** canDisableModule은 지금 이 모듈을 꺼도 다른 활성 모듈이 남는지 알려준다. */
export function canDisableModule(id: ModuleId): boolean {
  return layout.value.some((entry) => entry.enabled && entry.id !== id)
}

// setModuleEnabled는 모듈을 켜거나 끈다.
// 마지막 활성 모듈을 끄면 보여 줄 화면이 없어지므로 그 경우에는 무시한다.
export function setModuleEnabled(id: ModuleId, enabled: boolean) {
  const entry = layout.value.find((item) => item.id === id)
  if (!entry || entry.enabled === enabled) return
  if (!enabled && !canDisableModule(id)) return
  entry.enabled = enabled
  persist()
}

// moveModule은 모듈을 목록에서 offset만큼 옮긴다. 음수는 앞(왼쪽), 양수는 뒤(오른쪽)다.
export function moveModule(id: ModuleId, offset: number) {
  const from = layout.value.findIndex((entry) => entry.id === id)
  if (from < 0) return
  const to = Math.min(Math.max(from + offset, 0), layout.value.length - 1)
  if (to === from) return
  const [entry] = layout.value.splice(from, 1)
  layout.value.splice(to, 0, entry)
  persist()
}

// moveModuleTo는 끌어 놓기로 옮길 때 쓴다. 끌고 온 모듈을 대상 모듈이 있던 자리에 넣는다.
export function moveModuleTo(id: ModuleId, targetId: ModuleId) {
  if (id === targetId) return
  const to = layout.value.findIndex((entry) => entry.id === targetId)
  const from = layout.value.findIndex((entry) => entry.id === id)
  if (from < 0 || to < 0) return
  moveModule(id, to - from)
}

/** resetModuleLayout은 모든 모듈을 켜고 기본 순서로 되돌린다. */
export function resetModuleLayout() {
  layout.value = defaultLayout()
  persist()
}
