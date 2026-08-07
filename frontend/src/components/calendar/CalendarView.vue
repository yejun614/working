<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import TuiCalendar from '@toast-ui/calendar'
import '@toast-ui/calendar/dist/toastui-calendar.min.css'
import { Service as CalendarService } from '../../../bindings/working/internal/modules/calendar'
import { Service as AccountService } from '../../../bindings/working/internal/modules/account'
import type { Account } from '../../../bindings/working/internal/modules/account/types/models'
import type { Event, CalendarInfo } from '../../../bindings/working/internal/modules/calendar/types/models'
import EventForm from './EventForm.vue'

const accounts = ref<Account[]>([])
type DisplayCalendar = CalendarInfo & { accountId: string; displayColor: string }
const calendarsByAccount = ref<Record<string, DisplayCalendar[]>>({})
const calendarVisibility = ref<Record<string, boolean>>({})
const events = ref<Event[]>([])
const selectedAccountId = ref<string>('')
const loading = ref(false)
const error = ref('')
const info = ref('')
const copyStatus = ref('')
let copyStatusTimer: ReturnType<typeof setTimeout> | undefined

const showEventForm = ref(false)
const editingEvent = ref<Event | null>(null)
const eventFormDefaultDate = ref('')

// 월 캘린더 상태
const viewYear = ref(new Date().getFullYear())
const viewMonth = ref(new Date().getMonth()) // 0-based
const selectedDate = ref<string>(todayStr())

const selectedEvent = ref<Event | null>(null)
const tuiCalendarElement = ref<HTMLElement | null>(null)
let tuiCalendar: TuiCalendar | null = null
let immediateSelectedCell: Element | null = null
// 앱 시작 시 캐시가 비어 있을 때만 원격 캘린더 자동 새로고침을 한 번 수행한다.
const startupAutoRefreshKey = 'app-startup-auto-refresh:calendar'
let startupAutoRefreshAttempted = sessionStorage.getItem(startupAutoRefreshKey) === '1'

const monthLabel = computed(() => `${viewYear.value}년 ${viewMonth.value + 1}월`)

const selectedAccount = computed(() =>
  accounts.value.find(a => a.id === selectedAccountId.value) || null
)

// OAuth 토큰이 만료·철회되어 재인증이 필요한 계정. 백엔드가 계정에 사유를 기록한다.
const reauthAccounts = computed(() => accounts.value.filter(a => !!a.authError))
const reconnectingAccountId = ref('')

// Google 재인증을 다시 수행한다. 기본 브라우저에서 Google 로그인 창이 열리고,
// 인증이 끝나면 키체인의 토큰만 교체된 뒤 동기화까지 이어서 진행한다.
async function reconnectAccount(acc: Account) {
  if (reconnectingAccountId.value) return
  reconnectingAccountId.value = acc.id
  try {
    await AccountService.GoogleReconnect(acc.id)
    await CalendarService.SyncNow(acc.id)
    await refreshAccounts()
    await loadEvents()
    setInfo(`"${acc.name}" 계정을 다시 인증했습니다.`)
  } catch (e) {
    setError((e as Error).message)
    await refreshAccounts()
  } finally {
    reconnectingAccountId.value = ''
  }
}

function tuiCalendarId(calendar: DisplayCalendar): string {
  return calendarKey(calendar.accountId, { href: calendar.href })
}

function allDisplayCalendars(): DisplayCalendar[] {
  return Object.values(calendarsByAccount.value).flat()
}

function syncTuiDate() {
  if (!tuiCalendar) return
  const date = tuiCalendar.getDate().toDate()
  viewYear.value = date.getFullYear()
  viewMonth.value = date.getMonth()
  selectedDate.value = dateStr(date)
}

// Toast UI Calendar는 클릭 선택을 누적할 수 있으므로 새 셀을 선택하기 전에 이전 선택을 지운다.
function clearTuiGridSelections() {
  tuiCalendar?.clearGridSelections()
}

// Toast UI Calendar는 날짜를 mouseup 때 확정하므로, mousedown 단계에서 상세 날짜를 먼저 바꾼다.
// 월 그리드의 셀 순서는 getDateRangeStart()부터 이어지는 날짜 순서와 동일하다.
function selectTuiDateOnMouseDown(mouseEvent: MouseEvent) {
  clearTuiGridSelections()
  if (!tuiCalendar || !tuiCalendarElement.value) return

  const target = mouseEvent.target as Element | null
  const cell = target?.closest('.daygrid-cell')
  if (!cell) return

  const cells = Array.from(tuiCalendarElement.value.querySelectorAll('.daygrid-cell'))
  const cellIndex = cells.indexOf(cell)
  if (cellIndex < 0) return

  immediateSelectedCell?.classList.remove('hermes-immediate-selected')
  immediateSelectedCell = cell
  cell.classList.add('hermes-immediate-selected')

  const date = tuiCalendar.getDateRangeStart().toDate()
  date.setDate(date.getDate() + cellIndex)
  selectCalendarDate(date)
}

function allDayEndForTui(startValue: string, endValue: string): string {
  // iCalendar의 종일 DTEND는 종료일 미포함이지만 Toast UI Calendar는 포함하므로 하루를 뺀다.
  const start = startValue.slice(0, 10)
  const end = endValue.slice(0, 10) > start ? endValue.slice(0, 10) : start
  const [year, month, day] = end.split('-').map(Number)
  const tuiEnd = new Date(Date.UTC(year, month - 1, day - 1))
  return tuiEnd.toISOString().slice(0, 10) < start ? start : tuiEnd.toISOString().slice(0, 10)
}

function renderTuiCalendar() {
  if (!tuiCalendar) return

  const displayCalendars = allDisplayCalendars()
  tuiCalendar.setCalendars(displayCalendars.map(calendar => ({
    id: tuiCalendarId(calendar),
    name: calendar.name || '이름 없는 캘린더',
    color: '#ffffff',
    backgroundColor: calendar.displayColor,
    dragBackgroundColor: calendar.displayColor,
    borderColor: calendar.displayColor,
  })))
  tuiCalendar.clear()

  const tuiEvents = events.value.map(event => {
    const calendar = displayCalendars.find(candidate =>
      candidate.accountId === event.calendarId && candidate.href === event.calendarHref
    )
    const calendarId = calendar ? tuiCalendarId(calendar) : event.calendarId
    return {
      id: `${calendarId}:${event.uid}`,
      calendarId,
      title: event.title,
      body: event.description || '',
      location: event.location || '',
      start: event.allDay ? event.start.slice(0, 10) : event.start,
      end: event.allDay ? allDayEndForTui(event.start, event.end) : event.end,
      category: event.allDay ? 'allday' as const : 'time' as const,
      isAllday: !!event.allDay,
      isReadOnly: true,
      isVisible: isCalendarVisible(event.calendarId, event.calendarHref),
      raw: event,
    }
  })
  tuiCalendar.createEvents(tuiEvents)
  tuiCalendar.render()
}

function initializeTuiCalendar() {
  if (!tuiCalendarElement.value) return
  tuiCalendar = new TuiCalendar(tuiCalendarElement.value, {
    defaultView: 'month',
    // 전역 읽기 전용에서는 빈 날짜 셀의 gridSelection도 비활성화되므로,
    // 일정 객체별로 읽기 전용을 적용하고 날짜 선택만 허용한다.
    isReadOnly: false,
    usageStatistics: false,
    useFormPopup: false,
    useDetailPopup: false,
    gridSelection: { enableClick: true, enableDblClick: false },
    month: { isAlways6Weeks: true, visibleEventCount: 4 },
    theme: {
      common: {
        backgroundColor: 'var(--panel)',
        border: '1px solid var(--border)',
        dayName: { color: 'var(--muted)' },
        holiday: { color: 'var(--danger)' },
        saturday: { color: 'var(--accent)' },
        today: { color: 'var(--accent)' },
      },
      month: {
        dayExceptThisMonth: { color: 'var(--muted)' },
        dayName: { borderLeft: '1px solid var(--border)', backgroundColor: 'var(--panel-2)' },
        weekend: { backgroundColor: 'var(--panel-2)' },
      },
    },
  })
  tuiCalendar.on('selectDateTime', ({ start }) => {
    selectCalendarDate(start)
  })
  tuiCalendar.on('clickEvent', ({ event }) => {
    const original = event.raw as Event | undefined
    if (!original) return
    selectedEvent.value = original
    selectedDate.value = dateStr(new Date(original.start))
  })
  renderTuiCalendar()
}

// Toast UI Calendar의 날짜 선택과 상세 패널이 같은 날짜 상태를 공유하도록 한다.
// 선택한 날짜에 일정이 없어도 기존 일정 상세가 남지 않도록 선택 이벤트도 초기화한다.
function selectCalendarDate(date: Date) {
  selectedDate.value = dateStr(date)
  selectedEvent.value = null
}

// 월 캘린더 그리드: 일~토 6주 체웅
const calendarCells = computed(() => {
  const first = new Date(viewYear.value, viewMonth.value, 1)
  const startDay = first.getDay() // 0=Sun
  const daysInMonth = new Date(viewYear.value, viewMonth.value + 1, 0).getDate()
  const prevMonthDays = new Date(viewYear.value, viewMonth.value, 0).getDate()

  const cells: { date: string; day: number; current: boolean; events: Event[] }[] = []
  // 이전 달 여백
  for (let i = startDay - 1; i >= 0; i--) {
    const d = prevMonthDays - i
    const dt = new Date(viewYear.value, viewMonth.value - 1, d)
    cells.push({ date: dateStr(dt), day: d, current: false, events: eventsForDate(dt) })
  }
  // 현재 달
  for (let d = 1; d <= daysInMonth; d++) {
    const dt = new Date(viewYear.value, viewMonth.value, d)
    cells.push({ date: dateStr(dt), day: d, current: true, events: eventsForDate(dt) })
  }
  // 다음 달 여백 (6주 채우기)
  let next = 1
  while (cells.length < 42) {
    const dt = new Date(viewYear.value, viewMonth.value + 1, next)
    cells.push({ date: dateStr(dt), day: next, current: false, events: eventsForDate(dt) })
    next++
  }
  return cells
})

const selectedDayEvents = computed(() => {
  if (!selectedDate.value) return []
  const dt = new Date(selectedDate.value)
  return eventsForDate(dt).sort((a, b) => a.start.localeCompare(b.start))
})

function eventsForDate(dt: Date): Event[] {
  const ds = dateStr(dt)
  return events.value.filter(e => {
    if (!isCalendarVisible(e.calendarId, e.calendarHref)) return false

    // 종일 일정의 DTSTART/DTEND는 시간대가 없는 날짜 값이다.
    // 이를 Date로 변환해 시간 구간을 비교하면 UTC 자정이 로컬 시간으로
    // 이동하면서 전날 일정이 다음 날 상세 패널에 섞일 수 있다.
    // 따라서 종일 일정은 종료일 미포함의 날짜 구간으로만 비교한다.
    if (e.allDay) {
      const startDate = e.start.slice(0, 10)
      const endDate = e.end.slice(0, 10)
      if (!endDate || endDate <= startDate) return ds === startDate
      return startDate <= ds && ds < endDate
    }

    const eStart = new Date(e.start)
    const eEnd = new Date(e.end)
    const dayStart = new Date(ds + 'T00:00:00')
    const nextDayStart = new Date(dayStart)
    nextDayStart.setDate(nextDayStart.getDate() + 1)
    // 종료 시각은 미포함으로 비교한다.
    return eStart < nextDayStart && eEnd > dayStart
  })
}

function dateStr(dt: Date): string {
  const y = dt.getFullYear()
  const m = String(dt.getMonth() + 1).padStart(2, '0')
  const d = String(dt.getDate()).padStart(2, '0')
  return `${y}-${m}-${d}`
}

function todayStr(): string {
  return dateStr(new Date())
}

function prevMonth() {
  if (tuiCalendar) {
    tuiCalendar.prev()
    syncTuiDate()
    return
  }
  if (viewMonth.value === 0) {
    viewMonth.value = 11
    viewYear.value--
  } else {
    viewMonth.value--
  }
}

function nextMonth() {
  if (tuiCalendar) {
    tuiCalendar.next()
    syncTuiDate()
    return
  }
  if (viewMonth.value === 11) {
    viewMonth.value = 0
    viewYear.value++
  } else {
    viewMonth.value++
  }
}

function goToday() {
  if (tuiCalendar) {
    tuiCalendar.today()
    syncTuiDate()
    return
  }
  const now = new Date()
  viewYear.value = now.getFullYear()
  viewMonth.value = now.getMonth()
  selectedDate.value = todayStr()
}

function selectDate(date: string) {
  selectCalendarDate(new Date(`${date}T00:00:00`))
}

function accountColor(accId: string): string {
  const acc = accounts.value.find(a => a.id === accId)
  return acc?.calendar?.color || '#4f7cff'
}

const calendarPalette = ['#4f7cff', '#a855f7', '#14b8a6', '#f97316', '#ef476f', '#eab308', '#06b6d4', '#84cc16']

function calendarColor(event: Event): string {
  if (event.calendarHref) {
    const calendars = calendarsByAccount.value[event.calendarId] || []
    const calendar = calendars.find(c => c.href === event.calendarHref)
    if (calendar) return calendar.displayColor
  }
  return accountColor(event.calendarId)
}

function fallbackCalendar(acc: Account): DisplayCalendar {
  return {
    accountId: acc.id,
    name: acc.name,
    color: acc.calendar?.color,
    displayColor: acc.calendar?.color || calendarPalette[0],
  }
}

function calendarKey(accountId: string, calendar?: Partial<Pick<CalendarInfo, 'href' | 'name'>>): string {
  return `${accountId}:${calendar?.href || calendar?.name || 'default'}`
}

function restoreCalendarVisibility(accountId: string, calendar: CalendarInfo): boolean {
  const key = calendarKey(accountId, { href: calendar.href })
  const saved = localStorage.getItem(`calendar-visible:${key}`)
  const visible = saved === null ? true : saved === 'true'
  calendarVisibility.value[key] = visible
  return visible
}

function isCalendarVisible(accountId: string, href?: string): boolean {
  return calendarVisibility.value[calendarKey(accountId, { href })] !== false
}

function toggleCalendar(calendar: DisplayCalendar) {
  const key = calendarKey(calendar.accountId, { href: calendar.href })
  const visible = calendarVisibility.value[key] === false
  calendarVisibility.value[key] = visible
  localStorage.setItem(`calendar-visible:${key}`, String(visible))
  renderTuiCalendar()
}

async function refreshAccounts() {
  try {
    const list = await CalendarService.AccountList()
    accounts.value = list || []
    const nextCalendars: Record<string, DisplayCalendar[]> = {}
    let paletteOffset = 0
    await Promise.all(accounts.value.map(async (acc) => {
      try {
        const list = await CalendarService.Calendars(acc.id)
        const calendars = (list || []).map((calendar, index) => ({
          ...calendar,
          accountId: acc.id,
          displayColor: calendar.color || calendarPalette[(paletteOffset + index) % calendarPalette.length],
        }))
        paletteOffset += Math.max(calendars.length, 1)
        nextCalendars[acc.id] = calendars.length > 0 ? calendars : [fallbackCalendar(acc)]
        nextCalendars[acc.id].forEach(calendar => restoreCalendarVisibility(acc.id, calendar))
      } catch {
        nextCalendars[acc.id] = [fallbackCalendar(acc)]
        restoreCalendarVisibility(acc.id, nextCalendars[acc.id][0])
      }
    }))
    calendarsByAccount.value = nextCalendars
    // Calendars 호출 중 인증 만료가 기록될 수 있으므로 계정을 다시 읽어 안내 상태를 반영한다.
    accounts.value = (await CalendarService.AccountList()) || accounts.value
    if (!selectedAccountId.value && accounts.value.length > 0) {
      selectedAccountId.value = accounts.value[0].id
    }
  } catch (e) {
    setError((e as Error).message)
  }
}

async function autoRefreshEmptyCacheOnce() {
  if (startupAutoRefreshAttempted || events.value.length > 0) return
  startupAutoRefreshAttempted = true
  sessionStorage.setItem(startupAutoRefreshKey, '1')

  const remoteAccounts = accounts.value.filter(acc => acc.calendar?.source === 'caldav')
  if (remoteAccounts.length === 0) return

  // 계정별로 한 번씩만 동기화한 뒤 캐시를 다시 읽는다.
  await Promise.all(remoteAccounts.map(acc => CalendarService.SyncNow(acc.id)))
  await loadEvents()
}

async function loadEvents() {
  loading.value = true
  error.value = ''
  try {
    // 현재 월 범위 +- 1개월 여유
    const from = new Date(viewYear.value, viewMonth.value - 1, 1)
    const to = new Date(viewYear.value, viewMonth.value + 2, 0)
    const list = await CalendarService.EventList(from.toISOString(), to.toISOString())
    events.value = list || []
    renderTuiCalendar()
    await autoRefreshEmptyCacheOnce()
  } catch (e) {
    setError((e as Error).message)
    // 시작 시 자동 동기화가 인증 만료로 실패했을 수 있어 재인증 안내 상태를 다시 읽는다.
    await refreshAccounts()
  } finally {
    loading.value = false
  }
}

async function onAccountChanged() {
  await loadEvents()
}

function openNewEvent(date?: string) {
  editingEvent.value = null
  eventFormDefaultDate.value = date || selectedDate.value || todayStr()
  showEventForm.value = true
}

function openEditEvent(ev: Event) {
  editingEvent.value = ev
  showEventForm.value = true
}

async function onEventSaved() {
  showEventForm.value = false
  setInfo('일정이 저장되었습니다.')
  await loadEvents()
}

async function deleteEvent(ev: Event) {
  if (!confirm(`일정 "${ev.title}" 을(를) 삭제할까요?`)) return
  try {
    await CalendarService.EventDelete(ev.calendarId, ev.uid)
    setInfo('일정이 삭제되었습니다.')
    selectedEvent.value = null
    await loadEvents()
  } catch (e) {
    setError((e as Error).message)
  }
}

async function syncAccount(acc: Account) {
  try {
    await CalendarService.SyncNow(acc.id)
    setInfo('동기화가 완료되었습니다.')
    await loadEvents()
  } catch (e) {
    setError((e as Error).message)
    // 인증 만료로 실패했을 수 있으므로 계정을 다시 읽어 재인증 안내를 띄운다.
    await refreshAccounts()
  }
}

function setError(msg: string) {
  error.value = msg
  info.value = ''
  copyStatus.value = ''
}
function setInfo(msg: string) {
  info.value = msg
  error.value = ''
  copyStatus.value = ''
  setTimeout(() => { if (info.value === msg) info.value = '' }, 3500)
}

async function copyError() {
  if (!error.value) return

  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(error.value)
    } else {
      const textarea = document.createElement('textarea')
      textarea.value = error.value
      textarea.setAttribute('readonly', '')
      textarea.style.position = 'fixed'
      textarea.style.opacity = '0'
      document.body.appendChild(textarea)
      textarea.select()
      const copied = document.execCommand('copy')
      textarea.remove()
      if (!copied) throw new Error('copy command failed')
    }

    copyStatus.value = '복사됨'
    if (copyStatusTimer) clearTimeout(copyStatusTimer)
    copyStatusTimer = setTimeout(() => { copyStatus.value = '' }, 2500)
  } catch {
    copyStatus.value = '복사 실패'
  }
}

function formatTime(s: string): string {
  const d = new Date(s)
  if (isNaN(d.getTime())) return s
  return d.toTimeString().slice(0, 5)
}

function formatDate(s: string): string {
  if (!s) return ''
  const d = new Date(s)
  if (isNaN(d.getTime())) return s
  return d.toLocaleDateString()
}

const weekDays = ['일', '월', '화', '수', '목', '금', '토']

onMounted(async () => {
  initializeTuiCalendar()
  await refreshAccounts()
  await loadEvents()
})

onBeforeUnmount(() => {
  tuiCalendar?.destroy()
  tuiCalendar = null
})
</script>

<template>
  <div class="layout">
    <aside class="sidebar">
      <div class="sidebar-header">
        <h1>캘린더</h1>
      </div>

      <div class="account-section">
        <div class="section-title">
          <span>캘린더</span>
        </div>
        <ul class="account-list">
          <li
            v-for="a in accounts"
            :key="a.id"
            :class="{ active: a.id === selectedAccountId }"
            @click="selectedAccountId = a.id; onAccountChanged()"
          >
            <div class="account-row">
              <span class="color-dot" :style="{ background: a.calendar?.color || '#4f7cff' }"></span>
              <div class="account-name">{{ a.name }}</div>
              <span v-if="a.authError" class="auth-warning" :title="a.authError">⚠</span>
              <div class="account-actions">
                <button
                  v-if="a.authError"
                  class="icon-btn sm warning"
                  title="Google 재인증"
                  :disabled="!!reconnectingAccountId"
                  @click.stop="reconnectAccount(a)"
                >⟳</button>
                <button v-if="a.calendar?.source === 'caldav'" class="icon-btn sm" title="동기화" @click.stop="syncAccount(a)">↻</button>
              </div>
            </div>
            <div class="account-sub">
              <span class="badge">{{ a.calendar?.source === 'caldav' ? 'CalDAV' : '로컬' }}</span>
              <span v-if="a.calendar?.lastSyncAt" class="last-sync">{{ formatDate(a.calendar.lastSyncAt) }}</span>
            </div>
            <ul v-if="calendarsByAccount[a.id]?.length" class="calendar-list">
              <li v-for="calendar in calendarsByAccount[a.id]" :key="calendar.href || calendar.name" class="calendar-item">
                <span class="calendar-color-dot" :style="{ background: calendar.displayColor }"></span>
                <span class="calendar-name">{{ calendar.name || '이름 없는 캘린더' }}</span>
                <label class="calendar-toggle" :title="isCalendarVisible(calendar.accountId, calendar.href) ? '캘린더 숨기기' : '캘린더 표시하기'" @click.stop>
                  <input
                    type="checkbox"
                    :checked="isCalendarVisible(calendar.accountId, calendar.href)"
                    @change="toggleCalendar(calendar)"
                  />
                </label>
              </li>
            </ul>
          </li>
          <li v-if="accounts.length === 0" class="empty">계정 탭에서 캘린더를 추가하세요</li>
        </ul>
      </div>
    </aside>

    <section class="calendar-pane">
      <div v-if="reauthAccounts.length" class="alert reauth" role="alert">
        <div class="reauth-text">
          <strong>Google 계정 인증이 만료되었습니다</strong>
          <span>토큰이 만료되었거나 접근 권한이 철회되어 일정을 동기화할 수 없습니다. 재인증을 누르면 브라우저에서 Google 로그인 창이 열립니다.</span>
        </div>
        <div class="reauth-actions">
          <button
            v-for="a in reauthAccounts"
            :key="a.id"
            class="btn sm primary"
            type="button"
            :title="a.authError"
            :disabled="!!reconnectingAccountId"
            @click="reconnectAccount(a)"
          >{{ reconnectingAccountId === a.id ? '인증 중…' : `${a.name} 재인증` }}</button>
        </div>
      </div>
      <div v-if="error" class="alert error">
        <span class="alert-message">{{ error }}</span>
        <button class="btn sm alert-copy" type="button" @click="copyError">오류 복사</button>
        <span v-if="copyStatus" class="copy-status" role="status">{{ copyStatus }}</span>
      </div>
      <div v-if="info" class="alert info">{{ info }}</div>

      <div class="cal-header">
        <div class="cal-nav">
          <button class="icon-btn" @click="prevMonth">‹</button>
          <span class="month-label">{{ monthLabel }}</span>
          <button class="icon-btn" @click="nextMonth">›</button>
          <button class="btn sm" @click="goToday">오늘</button>
        </div>
        <div class="cal-actions">
          <button class="btn" @click="loadEvents" :disabled="loading">새로고침</button>
          <button class="btn primary" @click="openNewEvent()">일정 추가</button>
        </div>
      </div>

      <div
        ref="tuiCalendarElement"
        class="tui-calendar-host"
        @mousedown.capture="selectTuiDateOnMouseDown"
      ></div>
    </section>

    <section class="detail-pane">
      <div class="detail-header">
        <span class="detail-date">{{ selectedDate }}</span>
        <button class="btn primary sm" @click="openNewEvent(selectedDate)">+ 일정</button>
      </div>

      <div :key="selectedDate" class="detail-body">
        <div v-if="selectedDayEvents.length === 0" class="state">이 날의 일정이 없습니다</div>
        <ul v-else class="day-event-list">
          <li
            v-for="e in selectedDayEvents"
            :key="e.uid"
            :class="{ selected: selectedEvent?.uid === e.uid }"
            @click="selectedEvent = e"
          >
            <span class="time-col">{{ e.allDay ? '종일' : formatTime(e.start) }}</span>
            <span class="color-bar" :style="{ background: calendarColor(e) }"></span>
            <span class="ev-title">{{ e.title }}</span>
          </li>
        </ul>

        <div v-if="selectedEvent" class="event-detail">
          <div class="detail-actions">
            <template v-if="!selectedEvent.uid.startsWith('kanban:')">
              <button class="btn sm" @click="openEditEvent(selectedEvent)">편집</button>
              <button class="btn sm danger" @click="deleteEvent(selectedEvent)">삭제</button>
            </template>
            <span v-else class="kanban-event-note">칸반 카드에서 관리</span>
          </div>
          <h3>{{ selectedEvent.title }}</h3>
          <div class="meta"><span>시작:</span> {{ new Date(selectedEvent.start).toLocaleString() }}</div>
          <div class="meta"><span>종료:</span> {{ new Date(selectedEvent.end).toLocaleString() }}</div>
          <div class="meta" v-if="selectedEvent.location"><span>장소:</span> {{ selectedEvent.location }}</div>
          <div class="meta" v-if="selectedEvent.organizer"><span>주최:</span> {{ selectedEvent.organizer }}</div>
          <div class="meta" v-if="selectedEvent.attendees && selectedEvent.attendees.length">
            <span>참석자:</span> {{ selectedEvent.attendees.join(', ') }}
          </div>
          <div class="meta" v-if="selectedEvent.recurrenceRule"><span>반복:</span> {{ selectedEvent.recurrenceRule }}</div>
          <pre v-if="selectedEvent.description" class="desc">{{ selectedEvent.description }}</pre>
        </div>
      </div>
    </section>
    <EventForm
      v-if="showEventForm"
      :accounts="accounts"
      :event="editingEvent"
      :default-calendar-id="selectedAccountId"
      :default-date="eventFormDefaultDate"
      @close="showEventForm = false"
      @saved="onEventSaved"
    />
  </div>
</template>

<style scoped>
.layout {
  display: grid;
  grid-template-columns: minmax(0, 220px) minmax(0, 1fr) minmax(0, 300px);
  width: 100%;
  min-width: 0;
  height: 100%;
}
.sidebar {
  background: var(--panel);
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  min-width: 0;
  overflow: auto;
}
.sidebar-header {
  padding: 18px 16px 12px;
  border-bottom: 1px solid var(--border);
}
.sidebar-header h1 { margin: 0; font-size: 18px; letter-spacing: 0.5px; }
.section-title {
  display: flex; justify-content: space-between; align-items: center;
  padding: 12px 16px 6px;
  color: var(--muted); font-size: 12px;
  text-transform: uppercase; letter-spacing: 0.6px;
}
.account-list { list-style: none; margin: 0; padding: 0; }
.account-list li {
  padding: 8px 16px;
  cursor: pointer;
  border-bottom: 1px solid transparent;
}
.account-list li:hover { background: var(--panel-2); }
.account-list li.active {
  background: var(--panel-2);
  border-left: 3px solid var(--accent);
  padding-left: 13px;
}
.account-row { display: flex; align-items: center; gap: 8px; min-height: 20px; }
.color-dot { width: 10px; height: 10px; border-radius: 50%; flex-shrink: 0; }
.account-name { font-weight: 600; flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
/* 호버 시 레이아웃이 틀어지지 않도록 자리는 항상 차지하고 보이기만 전환한다. */
.account-actions { display: flex; gap: 4px; flex-shrink: 0; visibility: hidden; opacity: 0; }
.account-list li:hover .account-actions { visibility: visible; opacity: 1; }
.account-sub { font-size: 11px; color: var(--muted); margin-top: 4px; padding-left: 18px; display: flex; gap: 8px; }
.calendar-list { list-style: none; margin: 7px 0 0 18px; padding: 0; display: flex; flex-direction: column; gap: 5px; }
.calendar-item { display: flex; align-items: center; gap: 7px; padding: 0 !important; color: var(--muted); font-size: 11px; cursor: default !important; }
.calendar-item:hover { background: transparent !important; }
.calendar-color-dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
.calendar-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; flex: 1; }
.calendar-toggle { display: flex; align-items: center; cursor: pointer; }
.calendar-toggle input { accent-color: var(--accent); width: 13px; height: 13px; margin: 0; }
.badge {
  background: var(--border);
  padding: 1px 6px;
  border-radius: 3px;
  font-size: 10px;
}
.last-sync { font-size: 10px; }
.empty { color: var(--muted); font-style: italic; cursor: default; }
.empty:hover { background: transparent; }

.icon-btn {
  background: transparent;
  border: 1px solid var(--border);
  color: var(--text);
  border-radius: 4px;
  width: 24px; height: 24px;
  display: inline-flex; align-items: center; justify-content: center;
}
.icon-btn.sm { width: 20px; height: 20px; font-size: 11px; }
.icon-btn.danger:hover { color: var(--danger); border-color: var(--danger); }

.calendar-pane {
  display: flex;
  flex-direction: column;
  min-width: 0;
  overflow: hidden;
  border-right: 1px solid var(--border);
}
.cal-header {
  display: flex; justify-content: space-between; align-items: center;
  padding: 10px 14px;
  border-bottom: 1px solid var(--border);
}
.cal-nav { display: flex; align-items: center; gap: 6px; }
.month-label { font-weight: 600; min-width: 100px; text-align: center; }
.cal-actions { display: flex; gap: 6px; }
.btn {
  background: var(--panel-2);
  color: var(--text);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 6px 12px;
}
.btn.sm { padding: 3px 8px; font-size: 12px; }
.btn:hover { background: var(--border); }
.btn.primary { background: var(--accent); border-color: var(--accent); color: var(--on-accent); }
.btn.primary:hover { background: var(--accent-hover); }
.btn.danger { color: var(--danger); }
.btn:disabled { opacity: 0.5; cursor: not-allowed; }

.alert {
  margin: 8px 14px;
  padding: 8px 12px;
  border-radius: 6px;
  font-size: 13px;
}
.alert.error {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  background: rgba(255,90,106,0.12);
  color: var(--danger);
}
.alert-message {
  flex: 1;
  min-width: 0;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.alert-copy { flex-shrink: 0; }
.copy-status { flex-shrink: 0; font-size: 11px; opacity: 0.85; }
.alert.info { background: rgba(56,211,159,0.12); color: var(--ok); }
.alert.reauth {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  background: rgba(255,198,92,0.14);
  color: #ffc65c;
  border: 1px solid rgba(255,198,92,0.35);
}
.reauth-text { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
.reauth-text span { color: var(--muted); font-size: 12px; }
.reauth-actions { display: flex; flex-shrink: 0; flex-wrap: wrap; gap: 6px; }
.auth-warning { flex-shrink: 0; color: #ffc65c; font-size: 12px; cursor: help; }
.icon-btn.warning { color: #ffc65c; border-color: rgba(255,198,92,0.5); }
.icon-btn.warning:hover:not(:disabled) { background: rgba(255,198,92,0.14); }
.icon-btn:disabled { opacity: 0.5; cursor: not-allowed; }

.tui-calendar-host {
  flex: 1;
  min-height: 0;
  overflow: hidden;
  background: var(--panel);
}
.tui-calendar-host :deep(.toastui-calendar-layout) { border-color: var(--border); }
.tui-calendar-host :deep(.hermes-immediate-selected) {
  box-shadow: inset 0 0 0 2px var(--accent);
}

.week-header {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
  border-bottom: 1px solid var(--border);
}
.week-cell {
  padding: 6px;
  text-align: center;
  font-size: 12px;
  color: var(--muted);
  text-transform: uppercase;
}
.cal-grid {
  display: grid;
  grid-template-columns: repeat(7, 1fr);
  grid-template-rows: repeat(6, 1fr);
  flex: 1;
  overflow: hidden;
}
.cal-cell {
  border-right: 1px solid var(--border);
  border-bottom: 1px solid var(--border);
  padding: 4px;
  cursor: pointer;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  min-height: 0;
}
.cal-cell.other-month { background: rgba(0,0,0,0.2); color: var(--muted); }
.cal-cell.today .cell-date {
  background: var(--accent);
  color: #fff;
  border-radius: 50%;
  width: 20px; height: 20px;
  display: flex; align-items: center; justify-content: center;
}
.cal-cell.selected { background: rgba(79,124,255,0.08); }
.cell-date { font-size: 12px; margin-bottom: 2px; }
.cell-events { flex: 1; overflow: hidden; display: flex; flex-direction: column; gap: 2px; }
.cell-event {
  font-size: 10px;
  padding: 1px 4px;
  border-left: 3px solid var(--accent);
  background: var(--panel-2);
  border-radius: 2px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.ev-time { color: var(--muted); margin-right: 4px; }
.ev-title { color: var(--text); }
.more { font-size: 10px; color: var(--muted); padding: 1px 4px; }

.detail-pane {
  display: flex;
  flex-direction: column;
  min-width: 0;
  overflow: hidden;
}
.detail-header {
  display: flex; justify-content: space-between; align-items: center;
  padding: 10px 14px;
  border-bottom: 1px solid var(--border);
}
.detail-date { font-weight: 600; }
.detail-body { flex: 1; overflow: auto; }
.state { padding: 24px; text-align: center; color: var(--muted); }
.day-event-list { list-style: none; margin: 0; padding: 0; }
.day-event-list li {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 14px;
  border-bottom: 1px solid var(--border);
  cursor: pointer;
}
.day-event-list li:hover { background: var(--panel-2); }
.day-event-list li.selected { background: var(--panel-2); border-left: 3px solid var(--accent); padding-left: 11px; }
.time-col { font-size: 11px; color: var(--muted); width: 44px; flex-shrink: 0; }
.color-bar { width: 3px; height: 14px; border-radius: 2px; flex-shrink: 0; }

.event-detail {
  padding: 14px;
  border-top: 2px solid var(--border);
}
.detail-actions { display: flex; gap: 6px; margin-bottom: 10px; }
.event-detail h3 { margin: 0 0 8px; font-size: 15px; }
.meta { font-size: 12px; margin: 3px 0; color: var(--muted); }
.meta span { color: var(--text); margin-right: 6px; }
.desc {
  margin-top: 10px;
  white-space: pre-wrap;
  word-wrap: break-word;
  font-family: inherit;
  font-size: 13px;
  line-height: 1.5;
  background: var(--panel-2);
  padding: 10px;
  border-radius: 6px;
  border: 1px solid var(--border);
}
</style>