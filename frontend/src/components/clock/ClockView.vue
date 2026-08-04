<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { Service as ClockService } from '../../../bindings/working/internal/modules/clock'
import type { Settings, PomodoroSettings } from '../../../bindings/working/internal/modules/clock/types/models'
import StopwatchPanel from './StopwatchPanel.vue'
import TimerPanel from './TimerPanel.vue'
import PomodoroPanel from './PomodoroPanel.vue'

type Tab = 'now' | 'world' | 'stopwatch' | 'timer' | 'pomodoro'

const tabs: Array<{ id: Tab; label: string }> = [
  { id: 'now', label: '현재 시간' },
  { id: 'world', label: '세계 시간' },
  { id: 'stopwatch', label: '스톱워치' },
  { id: 'timer', label: '타이머' },
  { id: 'pomodoro', label: '뽀모도로' },
]

const activeTab = ref<Tab>('now')
const now = ref(new Date())
const settings = ref<Settings | null>(null)
const error = ref('')
const zoneToAdd = ref('')
let clockTicker: ReturnType<typeof setInterval> | undefined

const localZone = Intl.DateTimeFormat().resolvedOptions().timeZone

// 시간대 목록은 브라우저가 알려 주는 값을 쓰고, 지원하지 않으면 대표 도시만 보여 준다.
const availableZones = computed<string[]>(() => {
  const supported = (Intl as unknown as { supportedValuesOf?: (key: string) => string[] }).supportedValuesOf
  const all = supported ? supported('timeZone') : fallbackZones
  const used = new Set(settings.value?.worldClocks || [])
  return all.filter(zone => !used.has(zone))
})

const fallbackZones = [
  'Asia/Seoul', 'Asia/Tokyo', 'Asia/Shanghai', 'Asia/Singapore', 'Asia/Kolkata', 'Asia/Dubai',
  'Europe/London', 'Europe/Paris', 'Europe/Berlin', 'Europe/Moscow',
  'America/New_York', 'America/Chicago', 'America/Denver', 'America/Los_Angeles', 'America/Sao_Paulo',
  'Australia/Sydney', 'Pacific/Auckland', 'UTC',
]

const worldClocks = computed(() =>
  (settings.value?.worldClocks || []).map(zone => ({
    zone,
    label: zone.split('/').pop()?.replace(/_/g, ' ') || zone,
    time: formatTime(zone),
    date: formatDate(zone),
    offset: offsetLabel(zone),
  }))
)

const localTime = computed(() => formatTime(localZone))
const localSeconds = computed(() => String(now.value.getSeconds()).padStart(2, '0'))
const localDate = computed(() =>
  new Intl.DateTimeFormat('ko-KR', { dateStyle: 'full', timeZone: localZone }).format(now.value)
)

function formatTime(zone: string): string {
  try {
    return new Intl.DateTimeFormat('ko-KR', { hour: '2-digit', minute: '2-digit', hour12: false, timeZone: zone }).format(now.value)
  } catch {
    return '--:--'
  }
}

function formatDate(zone: string): string {
  try {
    return new Intl.DateTimeFormat('ko-KR', { month: 'long', day: 'numeric', weekday: 'short', timeZone: zone }).format(now.value)
  } catch {
    return ''
  }
}

// 내 시간과 몇 시간 차이인지 보여 준다. 같은 순간을 각 시간대로 포맷해 비교한다.
function offsetLabel(zone: string): string {
  try {
    const target = zonedOffsetMinutes(zone)
    const local = zonedOffsetMinutes(localZone)
    const diff = Math.round((target - local) / 60 * 10) / 10
    if (diff === 0) return '같은 시간'
    return `${diff > 0 ? '+' : ''}${diff}시간`
  } catch {
    return ''
  }
}

// 지정한 시간대의 UTC 기준 분 단위 오프셋을 구한다.
function zonedOffsetMinutes(zone: string): number {
  const formatted = new Intl.DateTimeFormat('en-US', {
    timeZone: zone,
    year: 'numeric', month: '2-digit', day: '2-digit',
    hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false,
  }).formatToParts(now.value)
  const part = (type: string) => Number(formatted.find(p => p.type === type)?.value || 0)
  const asUTC = Date.UTC(part('year'), part('month') - 1, part('day'), part('hour') % 24, part('minute'), part('second'))
  return (asUTC - Math.floor(now.value.getTime() / 1000) * 1000) / 60_000
}

async function loadSettings() {
  try {
    settings.value = await ClockService.Settings()
  } catch (e) {
    error.value = (e as Error).message
  }
}

async function persist(next: Settings) {
  try {
    settings.value = await ClockService.SaveSettings(next)
    error.value = ''
  } catch (e) {
    error.value = (e as Error).message
  }
}

async function addZone() {
  const zone = zoneToAdd.value
  if (!zone || !settings.value) return
  await persist({ ...settings.value, worldClocks: [...(settings.value.worldClocks || []), zone] })
  zoneToAdd.value = ''
}

async function removeZone(zone: string) {
  if (!settings.value) return
  await persist({ ...settings.value, worldClocks: (settings.value.worldClocks || []).filter(item => item !== zone) })
}

async function savePomodoro(pomodoro: PomodoroSettings) {
  if (!settings.value) return
  await persist({ ...settings.value, pomodoro })
}

onMounted(async () => {
  clockTicker = setInterval(() => { now.value = new Date() }, 1000)
  await loadSettings()
})

onBeforeUnmount(() => {
  if (clockTicker) clearInterval(clockTicker)
})
</script>

<template>
  <div class="clock-layout">
    <nav class="clock-tabs">
      <button
        v-for="tab in tabs"
        :key="tab.id"
        :class="{ active: activeTab === tab.id }"
        @click="activeTab = tab.id"
      >{{ tab.label }}</button>
    </nav>

    <div v-if="error" class="alert error">{{ error }}</div>

    <div class="clock-body">
      <section v-show="activeTab === 'now'" class="panel now-panel">
        <div class="now-time">
          {{ localTime }}<span class="now-seconds">:{{ localSeconds }}</span>
        </div>
        <div class="now-date">{{ localDate }}</div>
        <div class="now-zone">{{ localZone }}</div>
      </section>

      <section v-show="activeTab === 'world'" class="panel world-panel">
        <div class="world-add">
          <select v-model="zoneToAdd" class="zone-select">
            <option value="">시간대 추가…</option>
            <option v-for="zone in availableZones" :key="zone" :value="zone">{{ zone }}</option>
          </select>
          <button class="btn" :disabled="!zoneToAdd" @click="addZone">추가</button>
        </div>

        <ul v-if="worldClocks.length" class="world-list">
          <li v-for="clock in worldClocks" :key="clock.zone">
            <div class="world-place">
              <strong>{{ clock.label }}</strong>
              <span class="world-zone">{{ clock.zone }} · {{ clock.offset }}</span>
            </div>
            <div class="world-when">
              <span class="world-time">{{ clock.time }}</span>
              <span class="world-date">{{ clock.date }}</span>
            </div>
            <button class="icon-btn sm danger" title="삭제" @click="removeZone(clock.zone)">✕</button>
          </li>
        </ul>
        <p v-else class="state">추가한 시간대가 없습니다.</p>
      </section>

      <!-- 스톱워치·타이머·뽀모도로는 탭을 옮겨도 계속 돌아가야 하므로 v-show로 유지한다. -->
      <StopwatchPanel v-show="activeTab === 'stopwatch'" />
      <TimerPanel v-show="activeTab === 'timer'" />
      <PomodoroPanel
        v-if="settings"
        v-show="activeTab === 'pomodoro'"
        :settings="settings.pomodoro"
        @save="savePomodoro"
      />
    </div>
  </div>
</template>

<style scoped>
.clock-layout {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  min-width: 0;
  overflow: hidden;
  color: var(--text);
}
.clock-tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 2px;
  padding: 10px 16px 0;
  border-bottom: 1px solid var(--border);
}
.clock-tabs button {
  padding: 7px 14px;
  border: 0;
  border-radius: 6px 6px 0 0;
  background: transparent;
  color: var(--muted);
  font-size: 13px;
}
.clock-tabs button:hover { color: var(--text); }
.clock-tabs button.active { color: var(--text); border-bottom: 2px solid var(--accent); }

.clock-body { flex: 1; min-height: 0; overflow: auto; }
.alert { margin: 8px 16px; padding: 8px 12px; border-radius: 6px; font-size: 13px; }
.alert.error { background: rgba(255, 90, 106, 0.12); color: var(--danger); overflow-wrap: anywhere; }
.state { padding: 40px 24px; text-align: center; color: var(--muted); }

.now-panel { display: flex; flex-direction: column; align-items: center; gap: 8px; padding: 56px 24px; }
.now-time { font-size: 84px; font-weight: 200; font-variant-numeric: tabular-nums; letter-spacing: 2px; }
.now-seconds { font-size: 40px; color: var(--muted); }
.now-date { font-size: 16px; color: var(--text); }
.now-zone { color: var(--muted); font-size: 12px; }

.world-panel { display: flex; flex-direction: column; gap: 14px; padding: 24px; }
.world-add { display: flex; gap: 8px; }
.zone-select {
  flex: 1;
  min-width: 0;
  padding: 8px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--panel-2);
  color: var(--text);
  font: inherit;
  font-size: 13px;
}
.world-list { display: flex; flex-direction: column; gap: 8px; margin: 0; padding: 0; list-style: none; }
.world-list li {
  display: flex;
  align-items: center;
  gap: 12px;
  padding: 12px 14px;
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 8px;
}
.world-place { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 2px; }
.world-place strong { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.world-zone { color: var(--muted); font-size: 11px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.world-when { flex-shrink: 0; display: flex; flex-direction: column; align-items: flex-end; gap: 2px; }
.world-time { font-size: 22px; font-variant-numeric: tabular-nums; }
.world-date { color: var(--muted); font-size: 11px; }

.icon-btn {
  flex-shrink: 0;
  background: transparent;
  border: 1px solid var(--border);
  color: var(--text);
  border-radius: 4px;
  width: 24px;
  height: 24px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.icon-btn.sm { width: 20px; height: 20px; font-size: 11px; }
.icon-btn.danger:hover { color: var(--danger); border-color: var(--danger); }
</style>

<style>
/* 시계 패널들이 공유하는 버튼 모양. 자식 컴포넌트가 그린 DOM에도 적용되도록 scoped 밖에 둔다. */
.clock-layout .btn {
  padding: 6px 12px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--panel-2);
  color: var(--text);
  font: inherit;
}
.clock-layout .btn:hover:not(:disabled) { background: var(--border); }
.clock-layout .btn.primary { background: var(--accent); border-color: var(--accent); }
.clock-layout .btn.primary:hover:not(:disabled) { background: var(--accent-hover); }
.clock-layout .btn.sm { padding: 4px 9px; font-size: 12px; }
.clock-layout .btn.lg { padding: 9px 20px; font-size: 14px; }
.clock-layout .btn:disabled { opacity: 0.5; cursor: not-allowed; }
</style>
