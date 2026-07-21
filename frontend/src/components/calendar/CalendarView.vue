<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { Service as CalendarService } from '../../../bindings/working/internal/modules/calendar'
import type { Account } from '../../../bindings/working/internal/modules/calendar/account/models'
import type { Event } from '../../../bindings/working/internal/modules/calendar/types/models'
import CalendarAccountForm from './CalendarAccountForm.vue'
import EventForm from './EventForm.vue'

const accounts = ref<Account[]>([])
const events = ref<Event[]>([])
const selectedAccountId = ref<string>('')
const loading = ref(false)
const error = ref('')
const info = ref('')

const showAccountForm = ref(false)
const editingAccount = ref<Account | null>(null)
const showEventForm = ref(false)
const editingEvent = ref<Event | null>(null)
const eventFormDefaultDate = ref('')

// 월 캘린더 상태
const viewYear = ref(new Date().getFullYear())
const viewMonth = ref(new Date().getMonth()) // 0-based
const selectedDate = ref<string>(todayStr())

const selectedEvent = ref<Event | null>(null)

const monthLabel = computed(() => `${viewYear.value}년 ${viewMonth.value + 1}월`)

const selectedAccount = computed(() =>
  accounts.value.find(a => a.id === selectedAccountId.value) || null
)

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
    const eStart = new Date(e.start)
    const eEnd = new Date(e.end)
    const dayStart = new Date(ds + 'T00:00:00')
    const dayEnd = new Date(ds + 'T23:59:59')
    return eStart <= dayEnd && eEnd >= dayStart
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
  if (viewMonth.value === 0) {
    viewMonth.value = 11
    viewYear.value--
  } else {
    viewMonth.value--
  }
}

function nextMonth() {
  if (viewMonth.value === 11) {
    viewMonth.value = 0
    viewYear.value++
  } else {
    viewMonth.value++
  }
}

function goToday() {
  const now = new Date()
  viewYear.value = now.getFullYear()
  viewMonth.value = now.getMonth()
  selectedDate.value = todayStr()
}

function selectDate(date: string) {
  selectedDate.value = date
  selectedEvent.value = null
}

function accountColor(accId: string): string {
  const acc = accounts.value.find(a => a.id === accId)
  return acc?.color || '#4f7cff'
}

async function refreshAccounts() {
  try {
    const list = await CalendarService.AccountList()
    accounts.value = list || []
    if (!selectedAccountId.value && accounts.value.length > 0) {
      selectedAccountId.value = accounts.value[0].id
    }
  } catch (e) {
    setError((e as Error).message)
  }
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
  } catch (e) {
    setError((e as Error).message)
  } finally {
    loading.value = false
  }
}

async function onAccountChanged() {
  await loadEvents()
}

function openNewAccount() {
  editingAccount.value = null
  showAccountForm.value = true
}

function openEditAccount(acc: Account) {
  editingAccount.value = acc
  showAccountForm.value = true
}

async function deleteAccount(acc: Account) {
  if (!confirm(`계정 "${acc.name}" 을(를) 삭제할까요? 해당 계정의 로컬 일정도 함께 삭제됩니다.`)) return
  try {
    await CalendarService.AccountDelete(acc.id)
    setInfo('계정이 삭제되었습니다.')
    await refreshAccounts()
    await loadEvents()
  } catch (e) {
    setError((e as Error).message)
  }
}

async function onAccountSaved() {
  showAccountForm.value = false
  await refreshAccounts()
  setInfo('계정이 저장되었습니다.')
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
  }
}

function setError(msg: string) {
  error.value = msg
  info.value = ''
}
function setInfo(msg: string) {
  info.value = msg
  error.value = ''
  setTimeout(() => { if (info.value === msg) info.value = '' }, 3500)
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
  await refreshAccounts()
  await loadEvents()
})
</script>

<template>
  <div class="layout">
    <aside class="sidebar">
      <div class="sidebar-header">
        <h1>working</h1>
        <span class="module-tag">캘린더 모듈</span>
      </div>

      <div class="account-section">
        <div class="section-title">
          <span>캘린더</span>
          <button class="icon-btn" title="계정 추가" @click="openNewAccount">+</button>
        </div>
        <ul class="account-list">
          <li
            v-for="a in accounts"
            :key="a.id"
            :class="{ active: a.id === selectedAccountId }"
            @click="selectedAccountId = a.id; onAccountChanged()"
          >
            <div class="account-row">
              <span class="color-dot" :style="{ background: a.color || '#4f7cff' }"></span>
              <div class="account-name">{{ a.name }}</div>
              <div class="account-actions">
                <button v-if="a.source === 'caldav'" class="icon-btn sm" title="동기화" @click.stop="syncAccount(a)">↻</button>
                <button class="icon-btn sm" title="편집" @click.stop="openEditAccount(a)">✎</button>
                <button class="icon-btn sm danger" title="삭제" @click.stop="deleteAccount(a)">✕</button>
              </div>
            </div>
            <div class="account-sub">
              <span class="badge">{{ a.source === 'caldav' ? 'CalDAV' : '로컬' }}</span>
              <span v-if="a.lastSyncAt" class="last-sync">{{ formatDate(a.lastSyncAt) }}</span>
            </div>
          </li>
          <li v-if="accounts.length === 0" class="empty">등록된 캘린더가 없습니다</li>
        </ul>
      </div>
    </aside>

    <section class="calendar-pane">
      <div v-if="error" class="alert error">{{ error }}</div>
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

      <div class="week-header">
        <div v-for="w in weekDays" :key="w" class="week-cell">{{ w }}</div>
      </div>

      <div class="cal-grid">
        <div
          v-for="cell in calendarCells"
          :key="cell.date"
          class="cal-cell"
          :class="{ 'other-month': !cell.current, today: cell.date === todayStr(), selected: cell.date === selectedDate }"
          @click="selectDate(cell.date)"
          @dblclick="openNewEvent(cell.date)"
        >
          <div class="cell-date">{{ cell.day }}</div>
          <div class="cell-events">
            <div
              v-for="e in cell.events.slice(0, 3)"
              :key="e.uid"
              class="cell-event"
              :style="{ borderLeftColor: accountColor(e.calendarId) }"
              :title="e.title"
              @click.stop="selectedEvent = e"
            >
              <span class="ev-time">{{ e.allDay ? '종일' : formatTime(e.start) }}</span>
              <span class="ev-title">{{ e.title }}</span>
            </div>
            <div v-if="cell.events.length > 3" class="more">+{{ cell.events.length - 3 }} 더보기</div>
          </div>
        </div>
      </div>
    </section>

    <section class="detail-pane">
      <div class="detail-header">
        <span class="detail-date">{{ selectedDate }}</span>
        <button class="btn primary sm" @click="openNewEvent(selectedDate)">+ 일정</button>
      </div>

      <div class="detail-body">
        <div v-if="selectedDayEvents.length === 0" class="state">이 날의 일정이 없습니다</div>
        <ul v-else class="day-event-list">
          <li
            v-for="e in selectedDayEvents"
            :key="e.uid"
            :class="{ selected: selectedEvent?.uid === e.uid }"
            @click="selectedEvent = e"
          >
            <span class="time-col">{{ e.allDay ? '종일' : formatTime(e.start) }}</span>
            <span class="color-bar" :style="{ background: accountColor(e.calendarId) }"></span>
            <span class="ev-title">{{ e.title }}</span>
          </li>
        </ul>

        <div v-if="selectedEvent" class="event-detail">
          <div class="detail-actions">
            <button class="btn sm" @click="openEditEvent(selectedEvent)">편집</button>
            <button class="btn sm danger" @click="deleteEvent(selectedEvent)">삭제</button>
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

    <CalendarAccountForm
      v-if="showAccountForm"
      :account="editingAccount"
      @close="showAccountForm = false"
      @saved="onAccountSaved"
    />
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
  grid-template-columns: 220px 1fr 300px;
  height: 100vh;
}
.sidebar {
  background: var(--panel);
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  overflow: auto;
}
.sidebar-header {
  padding: 18px 16px 12px;
  border-bottom: 1px solid var(--border);
}
.sidebar-header h1 { margin: 0; font-size: 18px; letter-spacing: 0.5px; }
.module-tag { font-size: 11px; color: var(--muted); }
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
.account-row { display: flex; align-items: center; gap: 8px; }
.color-dot { width: 10px; height: 10px; border-radius: 50%; flex-shrink: 0; }
.account-name { font-weight: 600; flex: 1; }
.account-actions { display: none; gap: 4px; }
.account-list li:hover .account-actions { display: flex; }
.account-sub { font-size: 11px; color: var(--muted); margin-top: 4px; padding-left: 18px; display: flex; gap: 8px; }
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
.btn.primary { background: var(--accent); border-color: var(--accent); }
.btn.primary:hover { background: var(--accent-hover); }
.btn.danger { color: var(--danger); }
.btn:disabled { opacity: 0.5; cursor: not-allowed; }

.alert {
  margin: 8px 14px;
  padding: 8px 12px;
  border-radius: 6px;
  font-size: 13px;
}
.alert.error { background: rgba(255,90,106,0.12); color: var(--danger); }
.alert.info { background: rgba(56,211,159,0.12); color: var(--ok); }

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