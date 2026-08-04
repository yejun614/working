<script setup lang="ts">
import { ref, computed, onBeforeUnmount } from 'vue'
import { playAlarm } from '../../alarm'
import { notifyDesktop } from './notify'

const presets = [
  { label: '1분', seconds: 60 },
  { label: '3분', seconds: 180 },
  { label: '5분', seconds: 300 },
  { label: '10분', seconds: 600 },
  { label: '30분', seconds: 1800 },
]

const inputHours = ref(0)
const inputMinutes = ref(5)
const inputSeconds = ref(0)

const running = ref(false)
const finished = ref(false)
// 목표 시각을 잡아 두고 남은 시간을 계산한다. 틱을 세면 창이 가려졌을 때 어긋난다.
const endsAt = ref(0)
const remaining = ref(0)
const totalMs = ref(0)
let ticker: ReturnType<typeof setInterval> | undefined

const configuredMs = computed(() =>
  (Math.max(0, inputHours.value) * 3600 + Math.max(0, inputMinutes.value) * 60 + Math.max(0, inputSeconds.value)) * 1000
)
const started = computed(() => totalMs.value > 0)
const progress = computed(() => (totalMs.value ? 1 - remaining.value / totalMs.value : 0))

function startTicker() {
  if (ticker) return
  ticker = setInterval(tick, 100)
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
  remaining.value = 0
  running.value = false
  finished.value = true
  stopTicker()
  playAlarm()
  notifyDesktop('타이머 완료', `${formatDuration(totalMs.value)} 타이머가 끝났습니다.`)
}

// 알림 문구에 쓸 전체 길이를 사람이 읽는 형태로 만든다.
function formatDuration(ms: number): string {
  const total = Math.max(0, Math.round(ms / 1000))
  const hours = Math.floor(total / 3600)
  const minutes = Math.floor(total / 60) % 60
  const seconds = total % 60
  const parts: string[] = []
  if (hours) parts.push(`${hours}시간`)
  if (minutes) parts.push(`${minutes}분`)
  if (seconds || !parts.length) parts.push(`${seconds}초`)
  return parts.join(' ')
}

function start() {
  const duration = started.value && remaining.value > 0 ? remaining.value : configuredMs.value
  if (duration <= 0) return
  if (!started.value || remaining.value <= 0) totalMs.value = duration
  finished.value = false
  remaining.value = duration
  endsAt.value = Date.now() + duration
  running.value = true
  startTicker()
}

function pause() {
  if (!running.value) return
  remaining.value = Math.max(0, endsAt.value - Date.now())
  running.value = false
  stopTicker()
}

function reset() {
  running.value = false
  finished.value = false
  stopTicker()
  remaining.value = 0
  totalMs.value = 0
  endsAt.value = 0
}

function applyPreset(seconds: number) {
  reset()
  inputHours.value = Math.floor(seconds / 3600)
  inputMinutes.value = Math.floor(seconds / 60) % 60
  inputSeconds.value = seconds % 60
}

// 남은 시간은 초 단위로 올림해 표시한다. 1초가 남았는데 0으로 보이지 않게 한다.
function formatRemaining(ms: number): string {
  const total = Math.max(0, Math.ceil(ms / 1000))
  const pad = (v: number) => String(v).padStart(2, '0')
  const hours = Math.floor(total / 3600)
  const minutes = Math.floor(total / 60) % 60
  const seconds = total % 60
  return hours > 0 ? `${hours}:${pad(minutes)}:${pad(seconds)}` : `${pad(minutes)}:${pad(seconds)}`
}

onBeforeUnmount(stopTicker)
</script>

<template>
  <div class="panel">
    <div v-if="finished" class="alert done" role="alert">타이머가 끝났습니다.</div>

    <div class="readout" :class="{ running, finished }">
      {{ formatRemaining(started ? remaining : configuredMs) }}
    </div>
    <div v-if="started" class="progress" aria-hidden="true">
      <div class="progress-fill" :style="{ width: `${Math.min(100, progress * 100)}%` }"></div>
    </div>

    <div v-if="!started" class="inputs">
      <label>시<input v-model.number="inputHours" type="number" min="0" max="23" /></label>
      <label>분<input v-model.number="inputMinutes" type="number" min="0" max="59" /></label>
      <label>초<input v-model.number="inputSeconds" type="number" min="0" max="59" /></label>
    </div>

    <div v-if="!started" class="presets">
      <button v-for="preset in presets" :key="preset.seconds" class="btn sm" @click="applyPreset(preset.seconds)">
        {{ preset.label }}
      </button>
    </div>

    <div class="controls">
      <button v-if="!running" class="btn primary lg" :disabled="!started && configuredMs <= 0" @click="start">
        {{ started && remaining > 0 ? '계속' : '시작' }}
      </button>
      <button v-else class="btn primary lg" @click="pause">일시정지</button>
      <button class="btn lg" :disabled="!started" @click="reset">초기화</button>
    </div>
  </div>
</template>

<style scoped>
.panel { display: flex; flex-direction: column; align-items: center; gap: 18px; padding: 32px 24px; }
.readout {
  font-size: 64px;
  font-weight: 300;
  font-variant-numeric: tabular-nums;
  letter-spacing: 1px;
}
.readout.running { color: var(--accent); }
.readout.finished { color: var(--danger); }

.progress { width: 100%; max-width: 420px; height: 4px; border-radius: 2px; background: var(--border); overflow: hidden; }
.progress-fill { height: 100%; background: var(--accent); transition: width 0.1s linear; }

.inputs { display: flex; gap: 10px; }
.inputs label { display: flex; flex-direction: column; gap: 4px; color: var(--muted); font-size: 11px; text-align: center; }
.inputs input {
  width: 72px;
  padding: 8px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--panel-2);
  color: var(--text);
  font: inherit;
  font-size: 18px;
  text-align: center;
  font-variant-numeric: tabular-nums;
}
.presets { display: flex; flex-wrap: wrap; justify-content: center; gap: 6px; }
.controls { display: flex; gap: 8px; }

.alert.done {
  padding: 8px 14px;
  border-radius: 6px;
  background: rgba(255, 90, 106, 0.12);
  color: var(--danger);
  font-size: 13px;
}
</style>
