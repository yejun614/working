<script setup lang="ts">
import { ref } from 'vue'
import EmailView from './components/email/EmailView.vue'
import CalendarView from './components/calendar/CalendarView.vue'

const activeModule = ref<'email' | 'calendar'>('calendar')
</script>

<template>
  <div class="app-shell">
    <nav class="tab-bar">
      <button
        :class="{ active: activeModule === 'calendar' }"
        @click="activeModule = 'calendar'"
      >캘린더</button>
      <button
        :class="{ active: activeModule === 'email' }"
        @click="activeModule = 'email'"
      >이메일</button>
    </nav>
    <main class="module-container">
      <CalendarView v-if="activeModule === 'calendar'" />
      <EmailView v-else-if="activeModule === 'email'" />
    </main>
  </div>
</template>

<style>
:root {
  --bg: #06070f;
  --panel: #11131f;
  --panel-2: #181b2b;
  --border: #232739;
  --text: #e7eaf3;
  --muted: #9aa3b8;
  --accent: #4f7cff;
  --accent-hover: #6a92ff;
  --danger: #ff5a6a;
  --ok: #38d39f;
}

* {
  box-sizing: border-box;
}

html, body, #app {
  height: 100%;
  margin: 0;
  background: var(--bg);
  color: var(--text);
  font-family: "Inter", system-ui, -apple-system, "Segoe UI", Roboto, sans-serif;
  font-size: 14px;
}

button {
  font-family: inherit;
  cursor: pointer;
}
</style>

<style scoped>
.app-shell {
  display: flex;
  flex-direction: column;
  height: 100vh;
}
.tab-bar {
  display: flex;
  gap: 2px;
  padding: 6px 6px 0;
  background: var(--panel);
  border-bottom: 1px solid var(--border);
}
.tab-bar button {
  background: transparent;
  border: none;
  color: var(--muted);
  padding: 8px 16px;
  border-radius: 6px 6px 0 0;
  font-size: 13px;
}
.tab-bar button:hover { color: var(--text); }
.tab-bar button.active {
  color: var(--text);
  border-bottom: 2px solid var(--accent);
}
.module-container {
  flex: 1;
  overflow: hidden;
}
</style>