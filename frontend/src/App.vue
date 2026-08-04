<script setup lang="ts">
import { ref } from 'vue'
import EmailView from './components/email/EmailView.vue'
import CalendarView from './components/calendar/CalendarView.vue'
import KanbanView from './components/kanban/KanbanView.vue'
import DocumentView from './components/document/DocumentView.vue'
import AccountsView from './components/account/AccountsView.vue'
import { applyTheme, isDarkMode } from './theme'

const activeModule = ref<'email' | 'calendar' | 'kanban' | 'document' | 'account'>('calendar')
const showSettings = ref(false)
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
      <button
        :class="{ active: activeModule === 'kanban' }"
        @click="activeModule = 'kanban'"
      >칸반</button>
      <button
        :class="{ active: activeModule === 'document' }"
        @click="activeModule = 'document'"
      >문서</button>
      <button
        :class="{ active: activeModule === 'account' }"
        @click="activeModule = 'account'"
      >계정</button>
      <button class="settings-button" @click="showSettings = !showSettings">⚙ 설정</button>
    </nav>
    <section v-if="showSettings" class="settings-panel" aria-label="앱 설정">
      <label class="setting-row">
        <input v-model="isDarkMode" type="checkbox" @change="applyTheme()" />
        <span>Dark Mode</span>
      </label>
      <p class="setting-description">앱 전체 화면과 이메일 본문에 적용됩니다.</p>
    </section>
    <main class="module-container">
      <CalendarView v-show="activeModule === 'calendar'" />
      <EmailView v-show="activeModule === 'email'" />
      <KanbanView v-show="activeModule === 'kanban'" />
      <DocumentView v-if="activeModule === 'document'" />
      <AccountsView v-if="activeModule === 'account'" />
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
:root[data-theme='light'] {
  --bg: #f4f6fb;
  --panel: #ffffff;
  --panel-2: #eef1f7;
  --border: #d8deea;
  --text: #202633;
  --muted: #697386;
  --accent: #3d68d8;
  --accent-hover: #3158bf;
  --danger: #d94355;
  --ok: #168c68;
}

* {
  box-sizing: border-box;
}

html, body, #app {
  height: 100%;
  margin: 0;
  background: var(--bg);
  color: var(--text);
  font-family: "Pretendard", "Pretendard JP", sans-serif;
  font-size: 14px;
}

button {
  font-family: inherit;
  cursor: pointer;
}
</style>

<style scoped>
.app-shell {
  position: relative;
  display: flex;
  flex-direction: column;
  height: 100vh;
  /* 스크롤은 각 모듈이 자기 영역 안에서만 처리한다. 앱 껍데기가 스크롤되면
     100vh 아래로 배경이 드러나므로 여기서 막는다. */
  overflow: hidden;
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
.settings-button {
  margin-left: auto;
}
.settings-panel {
  position: absolute;
  top: 42px;
  right: 8px;
  z-index: 20;
  min-width: 220px;
  padding: 14px;
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 8px;
  box-shadow: 0 12px 30px rgba(0, 0, 0, 0.18);
}
.setting-row {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--text);
  cursor: pointer;
}
.setting-description {
  margin: 8px 0 0 24px;
  color: var(--muted);
  font-size: 12px;
  line-height: 1.4;
}
.module-container {
  flex: 1;
  width: 100%;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
}
</style>