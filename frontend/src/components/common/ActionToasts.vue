<script setup lang="ts">
import { cancelAction, pendingActions } from '../../toasts'

// 남은 시간을 초 단위로 올림해서 보여 준다. 1초 미만이라도 0초로 보이지 않게 한다.
function remainingSeconds(remaining: number): number {
  return Math.max(1, Math.ceil(remaining / 1000))
}
</script>

<template>
  <!-- 되돌릴 수 있는 작업 알림. 화면 오른쪽 아래에 쌓아 둔다. -->
  <div v-if="pendingActions.length" class="toast-stack" role="status" aria-live="polite">
    <div v-for="action in pendingActions" :key="action.id" class="toast">
      <div class="toast-row">
        <span class="toast-message">{{ action.message }}</span>
        <span class="toast-remaining">{{ remainingSeconds(action.remaining) }}초</span>
        <button class="toast-action" type="button" @click="cancelAction(action.id)">{{ action.actionLabel }}</button>
      </div>
      <div class="toast-progress" aria-hidden="true">
        <span :style="{ width: `${(action.remaining / action.total) * 100}%` }"></span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.toast-stack {
  position: fixed;
  right: 18px;
  bottom: 18px;
  z-index: 80;
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-width: min(420px, calc(100vw - 36px));
}
.toast {
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--panel);
  box-shadow: 0 12px 32px rgba(0, 0, 0, 0.4);
  color: var(--text);
}
.toast-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
}
.toast-message { flex: 1; min-width: 0; font-size: 13px; overflow-wrap: anywhere; }
.toast-remaining { flex: 0 0 auto; color: var(--muted); font-size: 12px; font-variant-numeric: tabular-nums; }
.toast-action {
  flex: 0 0 auto;
  padding: 4px 10px;
  border: 1px solid var(--accent);
  border-radius: 6px;
  background: transparent;
  color: var(--accent);
  font-size: 12px;
  white-space: nowrap;
}
.toast-action:hover { background: var(--accent); color: var(--on-accent); }
/* 남은 시간을 줄어드는 막대로도 보여 준다. */
.toast-progress { height: 2px; background: var(--panel-2); }
.toast-progress span { display: block; height: 100%; background: var(--accent); }
</style>
