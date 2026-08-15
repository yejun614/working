<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { applyTheme, isDarkMode } from './theme'
import { MODULE_PANES, togglePane, usePaneVisible } from './panes'
import { flushActions } from './toasts'
import ActionToasts from './components/common/ActionToasts.vue'
import {
  canDisableModule,
  enabledModules,
  moduleEntries,
  moveModule,
  moveModuleTo,
  resetModuleLayout,
  setModuleEnabled,
  type ModuleId,
} from './modules'

// 처음 열 화면은 캘린더로 하되, 캘린더를 껐다면 남은 첫 모듈을 연다.
function initialModule(): ModuleId {
  const modules = enabledModules.value
  const calendar = modules.find((module) => module.id === 'calendar')
  return (calendar ?? modules[0]).id
}

const activeModule = ref<ModuleId>(initialModule())
const showSettings = ref(false)

// 보고 있던 모듈을 설정에서 끄면 빈 화면이 남으므로 첫 모듈로 옮긴다.
watch(enabledModules, (modules) => {
  if (!modules.some((module) => module.id === activeModule.value)) {
    activeModule.value = modules[0].id
  }
})

// 지금 보고 있는 모듈이 가진 측면 패널. 탭 바 오른쪽에 접기 버튼으로 그린다.
const activePanes = computed(() =>
  (MODULE_PANES[activeModule.value] ?? []).map((pane) => ({ ...pane, visible: usePaneVisible(pane.key) })),
)

const draggedModule = ref<ModuleId | null>(null)
const dropTargetModule = ref<ModuleId | null>(null)

function previewModuleDrop(id: ModuleId) {
  if (!draggedModule.value) return
  dropTargetModule.value = id === draggedModule.value ? null : id
}

function dropModule(id: ModuleId) {
  const dragged = draggedModule.value
  draggedModule.value = null
  dropTargetModule.value = null
  if (dragged) moveModuleTo(dragged, id)
}

function cancelModuleDrag() {
  draggedModule.value = null
  dropTargetModule.value = null
}

// 되돌릴 수 있게 미뤄 둔 작업(메일 삭제·전송)은 앱을 닫기 전에 마저 실행한다.
function flushBeforeUnload() {
  flushActions()
}
onMounted(() => window.addEventListener('beforeunload', flushBeforeUnload))
onBeforeUnmount(() => window.removeEventListener('beforeunload', flushBeforeUnload))
</script>

<template>
  <div class="app-shell">
    <nav class="tab-bar">
      <button
        v-for="module in enabledModules"
        :key="module.id"
        :class="{ active: activeModule === module.id }"
        @click="activeModule = module.id"
      >{{ module.label }}</button>
      <div class="tab-bar-tools">
        <!-- 패널 접기 버튼은 지금 보고 있는 모듈이 가진 패널만 보여 준다. -->
        <button
          v-for="pane in activePanes"
          :key="pane.key"
          class="pane-button"
          :class="{ on: pane.visible.value }"
          :aria-pressed="pane.visible.value"
          :title="`${pane.label} 패널 ${pane.visible.value ? '접기' : '펼치기'}`"
          @click="togglePane(pane.key)"
        >{{ pane.icon }}</button>
        <button class="settings-button" @click="showSettings = !showSettings">⚙ 설정</button>
      </div>
    </nav>
    <section v-if="showSettings" class="settings-panel" aria-label="앱 설정">
      <label class="setting-row">
        <input v-model="isDarkMode" type="checkbox" @change="applyTheme()" />
        <span>Dark Mode</span>
      </label>
      <p class="setting-description">앱 전체 화면과 이메일 본문에 적용됩니다.</p>
      <div class="settings-divider"></div>
      <div class="setting-row setting-heading">
        <span>모듈</span>
        <button class="reset-button" @click="resetModuleLayout()">기본값</button>
      </div>
      <p class="setting-description module-description">
        체크를 끄면 탭이 숨겨지고, 항목을 끌어 놓거나 화살표를 누르면 탭 순서가 바뀝니다.
      </p>
      <ul class="module-list">
        <li
          v-for="(entry, index) in moduleEntries"
          :key="entry.id"
          class="module-row"
          :class="{ dragging: draggedModule === entry.id, 'drop-target': dropTargetModule === entry.id }"
          draggable="true"
          @dragstart="draggedModule = entry.id"
          @dragover.prevent="previewModuleDrop(entry.id)"
          @drop.prevent="dropModule(entry.id)"
          @dragend="cancelModuleDrag"
        >
          <span class="drag-handle" aria-hidden="true">⠿</span>
          <label class="module-name">
            <input
              type="checkbox"
              :checked="entry.enabled"
              :disabled="entry.enabled && !canDisableModule(entry.id)"
              :title="entry.enabled && !canDisableModule(entry.id) ? '최소 한 개의 모듈은 켜 두어야 합니다.' : ''"
              @change="setModuleEnabled(entry.id, ($event.target as HTMLInputElement).checked)"
            />
            <span :class="{ disabled: !entry.enabled }">{{ entry.definition.label }}</span>
          </label>
          <button class="order-button" :disabled="index === 0" title="앞으로" @click="moveModule(entry.id, -1)">↑</button>
          <button
            class="order-button"
            :disabled="index === moduleEntries.length - 1"
            title="뒤로"
            @click="moveModule(entry.id, 1)"
          >↓</button>
        </li>
      </ul>
    </section>
    <main class="module-container">
      <!-- 켜져 있는 모듈만 만들고, 그중 보고 있는 하나만 보여 준다.
           lazy 모듈은 볼 때만 만들어 두므로 탭을 벗어나면 다시 정리된다. -->
      <template v-for="module in enabledModules" :key="module.id">
        <component
          :is="module.component"
          v-if="!module.lazy || activeModule === module.id"
          v-show="activeModule === module.id"
        />
      </template>
    </main>
    <!-- 되돌릴 수 있는 작업 알림은 어느 모듈에서든 보이도록 앱 껍데기에 둔다. -->
    <ActionToasts />
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
  /* accent 배경 위에 올리는 글자색. --text는 라이트 모드에서 검게 변해 대비가 무너진다. */
  --on-accent: #ffffff;
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
  --on-accent: #ffffff;
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
/* 탭은 아래 테두리까지 닿아야 하므로 늘어나 있고, 오른쪽 도구 버튼만
   따로 세로 가운데에 맞춘다. 버튼 높이를 하나로 묶어 가로줄을 맞춘다. */
.tab-bar-tools {
  display: flex;
  align-items: center;
  gap: 4px;
  margin-left: auto;
  align-self: center;
}
.tab-bar-tools button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  height: 28px;
  padding: 0 10px;
  border-radius: 6px;
  font-size: 13px;
  line-height: 1;
}
/* 패널 접기 버튼. 탭과 달리 눌린 상태를 배경으로 표시한다. */
.pane-button {
  width: 30px;
  padding: 0;
  font-size: 15px;
}
.tab-bar .pane-button.on {
  color: var(--accent);
  background: var(--panel-2);
}
.settings-panel {
  position: absolute;
  top: 42px;
  right: 8px;
  z-index: 20;
  min-width: 260px;
  max-height: calc(100vh - 60px);
  overflow-y: auto;
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
.settings-divider {
  margin: 14px 0 12px;
  border-top: 1px solid var(--border);
}
.setting-heading {
  justify-content: space-between;
  font-weight: 600;
  cursor: default;
}
.reset-button {
  background: transparent;
  border: 1px solid var(--border);
  border-radius: 5px;
  color: var(--muted);
  font-size: 11px;
  padding: 3px 8px;
}
.reset-button:hover {
  color: var(--text);
  border-color: var(--accent);
}
.module-description {
  margin-left: 0;
}
.module-list {
  margin: 10px 0 0;
  padding: 0;
  list-style: none;
}
.module-row {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 5px 6px;
  border: 1px solid transparent;
  border-radius: 6px;
  /* 끌어 놓기 중에는 li 안의 글자가 선택되지 않도록 한다. */
  user-select: none;
  cursor: grab;
  transition: opacity 0.12s, border-color 0.12s, background 0.12s;
}
.module-row:hover {
  background: var(--panel-2);
}
.module-row.dragging {
  opacity: 0.4;
}
.module-row.drop-target {
  border-color: var(--accent);
  background: rgba(79, 124, 255, 0.12);
}
.drag-handle {
  color: var(--muted);
  font-size: 13px;
}
.module-name {
  display: flex;
  align-items: center;
  gap: 8px;
  flex: 1;
  min-width: 0;
  cursor: pointer;
}
.module-name .disabled {
  color: var(--muted);
}
.order-button {
  width: 22px;
  padding: 2px 0;
  background: transparent;
  border: 1px solid var(--border);
  border-radius: 5px;
  color: var(--muted);
  font-size: 11px;
}
.order-button:hover:not(:disabled) {
  color: var(--text);
  border-color: var(--accent);
}
.order-button:disabled {
  opacity: 0.35;
  cursor: default;
}
.module-container {
  flex: 1;
  width: 100%;
  min-width: 0;
  min-height: 0;
  overflow: hidden;
}
</style>