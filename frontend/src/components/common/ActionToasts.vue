<script setup lang="ts">
import { cancelAction, completeAction, pendingActions } from '../../toasts'

// 남은 시간을 초 단위로 올림해서 보여 준다. 1초 미만이라도 0초로 보이지 않게 한다.
function remainingSeconds(remaining: number): number {
  return Math.max(1, Math.ceil(remaining / 1000))
}
</script>

<template>
  <!-- 되돌릴 수 있는 작업 알림. 화면 오른쪽 아래에 쌓아 둔다.
       클래스 이름에 action- 을 붙인 이유는 public/style.css에 남아 있는
       템플릿 시절 .toast 규칙(opacity: 0, 화면 위쪽 고정)에 묻히지 않기
       위해서다. scoped 스타일이라도 우리가 지정하지 않은 속성은 전역 규칙이
       그대로 먹는다. -->
  <div v-if="pendingActions.length" class="action-toast-stack" role="status" aria-live="polite">
    <div v-for="action in pendingActions" :key="action.id" class="action-toast">
      <div class="action-toast-row">
        <span class="action-toast-message">{{ action.message }}</span>
        <span class="action-toast-remaining">{{ remainingSeconds(action.remaining) }}초</span>
        <button class="action-toast-button" type="button" @click="cancelAction(action.id)">{{ action.actionLabel }}</button>
        <!-- 닫기는 되돌리지 않겠다는 뜻이므로 기다리지 않고 그 자리에서 마무리한다. -->
        <button
          class="action-toast-close"
          type="button"
          title="기다리지 않고 지금 처리"
          aria-label="알림 닫기"
          @click="completeAction(action.id)"
        >✕</button>
      </div>
      <div class="action-toast-progress" aria-hidden="true">
        <span :style="{ width: `${(action.remaining / action.total) * 100}%` }"></span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.action-toast-stack {
  position: fixed;
  right: 18px;
  bottom: 18px;
  z-index: 80;
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-width: min(420px, calc(100vw - 36px));
}
.action-toast {
  overflow: hidden;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--panel);
  box-shadow: 0 12px 32px rgba(0, 0, 0, 0.4);
  color: var(--text);
}
.action-toast-row {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 10px 12px;
}
.action-toast-message { flex: 1; min-width: 0; font-size: 13px; overflow-wrap: anywhere; }
.action-toast-remaining { flex: 0 0 auto; color: var(--muted); font-size: 12px; font-variant-numeric: tabular-nums; }
.action-toast-button {
  flex: 0 0 auto;
  padding: 4px 10px;
  border: 1px solid var(--accent);
  border-radius: 6px;
  background: transparent;
  color: var(--accent);
  font-size: 12px;
  white-space: nowrap;
}
.action-toast-button:hover { background: var(--accent); color: var(--on-accent); }
.action-toast-close {
  flex: 0 0 auto;
  width: 22px;
  height: 22px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: none;
  border-radius: 5px;
  background: transparent;
  color: var(--muted);
  font-size: 11px;
  line-height: 1;
}
.action-toast-close:hover { background: var(--panel-2); color: var(--text); }
/* 남은 시간을 줄어드는 막대로도 보여 준다. */
.action-toast-progress { height: 2px; background: var(--panel-2); }
.action-toast-progress span { display: block; height: 100%; background: var(--accent); }
</style>
