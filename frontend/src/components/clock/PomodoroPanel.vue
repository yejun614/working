<script setup lang="ts">
import { ref, computed, watch, onBeforeUnmount } from 'vue'
import { playAlarm } from '../../alarm'
import type { PomodoroSettings } from '../../../bindings/working/internal/modules/clock/types/models'

const props = defineProps<{ settings: PomodoroSettings }>()
const emit = defineEmits<{ (e: 'save', settings: PomodoroSettings): void }>()

type Phase = 'focus' | 'shortBreak' | 'longBreak'

const phaseLabels: Record<Phase, string> = {
  focus: '집중',
  shortBreak: '짧은 휴식',
  longBreak: '긴 휴식',
}

const phase = ref<Phase>('focus')
const running = ref(false)
// 목표 시각을 기준으로 남은 시간을 구한다. 틱을 세면 창이 가려졌을 때 어긋난다.
const endsAt = ref(0)
const remaining = ref(0)
// 이번 세트에서 끝낸 집중 횟수. 긴 휴식을 마치면 0으로 돌아간다.
const completedFocus = ref(0)
const showSettings = ref(false)
const draft = ref<PomodoroSettings>({ ...props.settings })
let ticker: ReturnType<typeof setInterval> | undefined

const phaseMinutes = computed<Record<Phase, number>>(() => ({
  focus: props.settings.focusMinutes,
  shortBreak: props.settings.shortBreakMinutes,
  longBreak: props.settings.longBreakMinutes,
}))
const phaseMs = computed(() => phaseMinutes.value[phase.value] * 60_000)

// 이번 세트에서 채운 집중 횟수. 세트를 다 채우면 긴 휴식 동안 전부 채워진 채로 둔다.
const filledRounds = computed(() => {
  const rounds = props.settings.roundsBeforeLongBreak
  if (completedFocus.value === 0) return 0
  const remainder = completedFocus.value % rounds
  return remainder === 0 ? rounds : remainder
})
const progress = computed(() => (phaseMs.value ? 1 - remaining.value / phaseMs.value : 0))

// 설정이 바뀌면 진행 중이 아닐 때만 남은 시간을 새 길이로 맞춘다.
watch(
  () => props.settings,
  settings => {
    draft.value = { ...settings }
    if (!running.value) remaining.value = phaseMs.value
  },
  { immediate: true, deep: true }
)

function startTicker() {
  if (ticker) return
  ticker = setInterval(tick, 200)
}

function stopTicker() {
  if (!ticker) return
  clearInterval(ticker)
  ticker = undefined
}

function tick() {
  const left = endsAt.value - Date.now()
  if (left > 0) {
    remaining.value = left
    return
  }
  completePhase()
}

// 한 구간이 끝나면 알림음을 내고 다음 구간을 준비한다.
// 자동으로 이어서 시작하지 않고 멈춰서, 자리를 비운 사이 흘러가지 않게 한다.
function completePhase() {
  stopTicker()
  running.value = false
  playAlarm(phase.value === 'focus' ? 3 : 2)

  if (phase.value === 'focus') {
    completedFocus.value += 1
    const rounds = props.settings.roundsBeforeLongBreak
    phase.value = completedFocus.value % rounds === 0 ? 'longBreak' : 'shortBreak'
  } else {
    if (phase.value === 'longBreak') completedFocus.value = 0
    phase.value = 'focus'
  }
  remaining.value = phaseMs.value
}

function start() {
  if (running.value) return
  if (remaining.value <= 0) remaining.value = phaseMs.value
  endsAt.value = Date.now() + remaining.value
  running.value = true
  startTicker()
}

function pause() {
  if (!running.value) return
  remaining.value = Math.max(0, endsAt.value - Date.now())
  running.value = false
  stopTicker()
}

// 지금 구간을 건너뛰고 다음 구간으로 넘어간다.
function skip() {
  stopTicker()
  running.value = false
  endsAt.value = 0
  if (phase.value === 'focus') {
    completedFocus.value += 1
    phase.value = completedFocus.value % props.settings.roundsBeforeLongBreak === 0 ? 'longBreak' : 'shortBreak'
  } else {
    if (phase.value === 'longBreak') completedFocus.value = 0
    phase.value = 'focus'
  }
  remaining.value = phaseMs.value
}

function resetAll() {
  stopTicker()
  running.value = false
  endsAt.value = 0
  phase.value = 'focus'
  completedFocus.value = 0
  remaining.value = phaseMs.value
}

function saveSettings() {
  emit('save', { ...draft.value })
  showSettings.value = false
}

function formatRemaining(ms: number): string {
  const total = Math.max(0, Math.ceil(ms / 1000))
  const pad = (v: number) => String(v).padStart(2, '0')
  return `${pad(Math.floor(total / 60))}:${pad(total % 60)}`
}

onBeforeUnmount(stopTicker)
</script>

<template>
  <div class="panel">
    <div class="phase" :class="phase">{{ phaseLabels[phase] }}</div>
    <div class="readout" :class="{ running }">{{ formatRemaining(remaining) }}</div>

    <div class="progress" aria-hidden="true">
      <div class="progress-fill" :class="phase" :style="{ width: `${Math.min(100, progress * 100)}%` }"></div>
    </div>

    <div class="rounds" :aria-label="`이번 세트에서 끝낸 집중 ${completedFocus}회`">
      <span
        v-for="round in props.settings.roundsBeforeLongBreak"
        :key="round"
        class="round-dot"
        :class="{ done: round <= filledRounds }"
      ></span>
    </div>

    <div class="controls">
      <button v-if="!running" class="btn primary lg" @click="start">시작</button>
      <button v-else class="btn primary lg" @click="pause">일시정지</button>
      <button class="btn lg" @click="skip">건너뛰기</button>
      <button class="btn lg" @click="resetAll">초기화</button>
      <button class="btn lg" @click="showSettings = !showSettings">설정</button>
    </div>

    <section v-if="showSettings" class="settings">
      <div class="settings-grid">
        <label>집중(분)<input v-model.number="draft.focusMinutes" type="number" min="1" max="180" /></label>
        <label>짧은 휴식(분)<input v-model.number="draft.shortBreakMinutes" type="number" min="1" max="180" /></label>
        <label>긴 휴식(분)<input v-model.number="draft.longBreakMinutes" type="number" min="1" max="180" /></label>
        <label>긴 휴식까지<input v-model.number="draft.roundsBeforeLongBreak" type="number" min="1" max="12" /></label>
      </div>
      <div class="settings-actions">
        <button class="btn sm" @click="showSettings = false">취소</button>
        <button class="btn sm primary" @click="saveSettings">저장</button>
      </div>
    </section>
  </div>
</template>

<style scoped>
.panel { display: flex; flex-direction: column; align-items: center; gap: 16px; padding: 28px 24px; }
.phase {
  padding: 3px 12px;
  border-radius: 12px;
  background: var(--panel-2);
  color: var(--muted);
  font-size: 12px;
  letter-spacing: 0.6px;
}
.phase.focus { background: rgba(79, 124, 255, 0.16); color: var(--accent); }
.phase.shortBreak { background: rgba(56, 211, 159, 0.16); color: var(--ok); }
.phase.longBreak { background: rgba(255, 198, 92, 0.16); color: #ffc65c; }

.readout { font-size: 64px; font-weight: 300; font-variant-numeric: tabular-nums; letter-spacing: 1px; }
.readout.running { color: var(--accent); }

.progress { width: 100%; max-width: 420px; height: 4px; border-radius: 2px; background: var(--border); overflow: hidden; }
.progress-fill { height: 100%; background: var(--accent); transition: width 0.2s linear; }
.progress-fill.shortBreak { background: var(--ok); }
.progress-fill.longBreak { background: #ffc65c; }

.rounds { display: flex; gap: 6px; }
.round-dot { width: 8px; height: 8px; border-radius: 50%; background: var(--border); }
.round-dot.done { background: var(--accent); }

.controls { display: flex; flex-wrap: wrap; justify-content: center; gap: 8px; }

.settings {
  width: 100%;
  max-width: 420px;
  padding: 14px 16px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--panel);
}
.settings-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; }
.settings-grid label { display: flex; flex-direction: column; gap: 4px; color: var(--muted); font-size: 11px; }
.settings-grid input {
  padding: 7px 8px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--panel-2);
  color: var(--text);
  font: inherit;
  font-variant-numeric: tabular-nums;
}
.settings-actions { display: flex; justify-content: flex-end; gap: 6px; margin-top: 12px; }
</style>
