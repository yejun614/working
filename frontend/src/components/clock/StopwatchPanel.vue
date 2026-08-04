<script setup lang="ts">
import { ref, computed, onBeforeUnmount } from 'vue'

// 경과 시간은 틱을 세지 않고 Date.now() 차이로 구한다.
// 창이 가려져 타이머가 느려지거나 건너뛰어도 시간이 어긋나지 않는다.
const running = ref(false)
const accumulated = ref(0)
const startedAt = ref(0)
const now = ref(Date.now())
const laps = ref<number[]>([])
let ticker: ReturnType<typeof setInterval> | undefined

const elapsed = computed(() => accumulated.value + (running.value ? now.value - startedAt.value : 0))
const canReset = computed(() => elapsed.value > 0 || laps.value.length > 0)

// 마지막 구간과의 차이를 함께 보여 주기 위해 랩 사이 간격을 계산한다.
const lapRows = computed(() =>
  laps.value.map((total, index) => ({
    index: laps.value.length - index,
    total,
    split: total - (laps.value[index + 1] ?? 0),
  }))
)

function startTicker() {
  if (ticker) return
  // 1/100초 자리를 보여 주므로 짧은 주기로 갱신한다.
  ticker = setInterval(() => { now.value = Date.now() }, 40)
}

function stopTicker() {
  if (!ticker) return
  clearInterval(ticker)
  ticker = undefined
}

function toggle() {
  if (running.value) {
    accumulated.value = elapsed.value
    running.value = false
    stopTicker()
    return
  }
  startedAt.value = Date.now()
  now.value = startedAt.value
  running.value = true
  startTicker()
}

function reset() {
  running.value = false
  stopTicker()
  accumulated.value = 0
  startedAt.value = 0
  laps.value = []
}

function lap() {
  if (!running.value) return
  laps.value = [elapsed.value, ...laps.value]
}

// 밀리초를 시:분:초.1/100초로 만든다. 한 시간이 넘을 때만 시를 붙인다.
function formatDuration(ms: number): string {
  const total = Math.max(0, Math.floor(ms))
  const centis = Math.floor((total % 1000) / 10)
  const seconds = Math.floor(total / 1000) % 60
  const minutes = Math.floor(total / 60_000) % 60
  const hours = Math.floor(total / 3_600_000)
  const pad = (v: number) => String(v).padStart(2, '0')
  const base = `${pad(minutes)}:${pad(seconds)}.${pad(centis)}`
  return hours > 0 ? `${hours}:${base}` : base
}

onBeforeUnmount(stopTicker)
</script>

<template>
  <div class="panel">
    <div class="readout" :class="{ running }">{{ formatDuration(elapsed) }}</div>

    <div class="controls">
      <button class="btn primary lg" @click="toggle">{{ running ? '일시정지' : elapsed > 0 ? '계속' : '시작' }}</button>
      <button class="btn lg" :disabled="!running" @click="lap">랩</button>
      <button class="btn lg" :disabled="!canReset" @click="reset">초기화</button>
    </div>

    <div v-if="lapRows.length" class="laps">
      <div class="lap-head"><span>랩</span><span>구간</span><span>누적</span></div>
      <ul>
        <li v-for="row in lapRows" :key="row.index">
          <span class="lap-index">{{ row.index }}</span>
          <span>{{ formatDuration(row.split) }}</span>
          <span class="lap-total">{{ formatDuration(row.total) }}</span>
        </li>
      </ul>
    </div>
  </div>
</template>

<style scoped>
.panel { display: flex; flex-direction: column; align-items: center; gap: 20px; padding: 32px 24px; }
.readout {
  font-size: 56px;
  font-weight: 300;
  font-variant-numeric: tabular-nums;
  letter-spacing: 1px;
  color: var(--text);
}
.readout.running { color: var(--accent); }
.controls { display: flex; gap: 8px; }

.laps { width: 100%; max-width: 420px; }
.lap-head, .laps li {
  display: grid;
  grid-template-columns: 48px 1fr 1fr;
  gap: 8px;
  padding: 6px 10px;
  font-variant-numeric: tabular-nums;
}
.lap-head { color: var(--muted); font-size: 11px; text-transform: uppercase; letter-spacing: 0.6px; }
.laps ul { max-height: 240px; overflow: auto; margin: 0; padding: 0; list-style: none; }
.laps li { border-top: 1px solid var(--border); font-size: 13px; }
.lap-index { color: var(--muted); }
.lap-total { color: var(--muted); }
</style>
