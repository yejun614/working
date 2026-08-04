<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch, nextTick } from 'vue'
import Editor from '@toast-ui/editor'
import '@toast-ui/editor/dist/toastui-editor.css'
import '@toast-ui/editor/dist/theme/toastui-editor-dark.css'
import { Service as DocumentService } from '../../../bindings/working/internal/modules/document'
import type { Document } from '../../../bindings/working/internal/modules/document/types/models'
import { isDarkMode } from '../../theme'

const documents = ref<Document[]>([])
const backlinks = ref<Document[]>([])
const selectedId = ref('')
const titleInput = ref('')
const filter = ref('')
const error = ref('')
const saveState = ref<'idle' | 'saving' | 'saved'>('idle')

const editorElement = ref<HTMLElement | null>(null)
let editor: Editor | null = null
// setMarkdown이 change 이벤트를 다시 일으키므로, 문서를 불러오는 동안은 저장하지 않는다.
let loadingDocument = false
let saveTimer: ReturnType<typeof setTimeout> | undefined
let saveStateTimer: ReturnType<typeof setTimeout> | undefined

const selectedDocument = computed(() => documents.value.find(d => d.id === selectedId.value) || null)

const filteredDocuments = computed(() => {
  const keyword = filter.value.trim().toLowerCase()
  if (!keyword) return documents.value
  return documents.value.filter(doc =>
    doc.title.toLowerCase().includes(keyword) || (doc.content || '').toLowerCase().includes(keyword)
  )
})

// 렌더러 콜백에서 참조하는 제목 집합. 없는 문서로 향하는 링크를 다르게 표시한다.
const knownTitles = new Set<string>()
function refreshKnownTitles() {
  knownTitles.clear()
  documents.value.forEach(doc => knownTitles.add(doc.title.trim().toLowerCase()))
}

// 마크다운 미리보기에서 [[문서 제목]]을 클릭 가능한 링크로 바꾼다.
// TUI Editor는 위키 링크를 모르므로 text 노드를 직접 토큰으로 쪼갠다.
const wikiLinkPattern = /\[\[([^[\]\r\n]+)\]\]/g
function renderWikiLinks(node: { literal: string | null }) {
  const literal = node.literal || ''
  const tokens: Array<Record<string, unknown>> = []
  let lastIndex = 0
  let match: RegExpExecArray | null

  wikiLinkPattern.lastIndex = 0
  while ((match = wikiLinkPattern.exec(literal)) !== null) {
    const title = match[1].trim()
    if (!title) continue
    if (match.index > lastIndex) {
      tokens.push({ type: 'text', content: literal.slice(lastIndex, match.index) })
    }
    const missing = !knownTitles.has(title.toLowerCase())
    tokens.push({
      type: 'openTag',
      tagName: 'a',
      attributes: { 'data-doc-link': title, href: '#', title: missing ? '새 문서로 만들기' : title },
      classNames: missing ? ['doc-link', 'missing'] : ['doc-link'],
    })
    tokens.push({ type: 'text', content: title })
    tokens.push({ type: 'closeTag', tagName: 'a' })
    lastIndex = match.index + match[0].length
  }

  if (!tokens.length) return { type: 'text', content: literal }
  if (lastIndex < literal.length) tokens.push({ type: 'text', content: literal.slice(lastIndex) })
  return tokens
}

function createEditor(initialContent: string) {
  if (!editorElement.value) return
  editor = new Editor({
    el: editorElement.value,
    height: '100%',
    initialEditType: 'markdown',
    previewStyle: 'vertical',
    // 위키 링크는 마크다운 미리보기에서만 렌더링하므로 WYSIWYG 전환은 감춘다.
    hideModeSwitch: true,
    usageStatistics: false,
    theme: isDarkMode.value ? 'dark' : 'default',
    initialValue: initialContent,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    customHTMLRenderer: { text: renderWikiLinks as any },
  })
  editor.on('change', () => {
    if (loadingDocument) return
    scheduleSave()
  })
}

function destroyEditor() {
  editor?.destroy()
  editor = null
}

// 테마를 바꾸면 에디터를 다시 만들어야 하므로 편집 중인 본문을 옮겨 담는다.
watch(isDarkMode, () => {
  const content = editor?.getMarkdown() ?? ''
  destroyEditor()
  nextTick(() => createEditor(content))
})

async function refreshDocuments() {
  try {
    documents.value = (await DocumentService.List()) || []
    refreshKnownTitles()
  } catch (e) {
    error.value = (e as Error).message
  }
}

async function selectDocument(id: string) {
  if (id === selectedId.value) return
  await flushSave()
  try {
    const doc = await DocumentService.Get(id)
    if (!doc) return
    loadingDocument = true
    selectedId.value = doc.id
    titleInput.value = doc.title
    editor?.setMarkdown(doc.content || '')
    loadingDocument = false
    await refreshBacklinks()
  } catch (e) {
    loadingDocument = false
    error.value = (e as Error).message
  }
}

async function refreshBacklinks() {
  if (!selectedId.value) {
    backlinks.value = []
    return
  }
  try {
    backlinks.value = (await DocumentService.Backlinks(selectedId.value)) || []
  } catch (e) {
    error.value = (e as Error).message
  }
}

function scheduleSave() {
  if (saveTimer) clearTimeout(saveTimer)
  saveTimer = setTimeout(() => void save(), 700)
}

async function flushSave() {
  if (!saveTimer) return
  clearTimeout(saveTimer)
  saveTimer = undefined
  await save()
}

async function save() {
  saveTimer = undefined
  const current = selectedDocument.value
  if (!current || !editor) return

  const payload: Document = {
    ...current,
    title: titleInput.value.trim() || current.title,
    content: editor.getMarkdown(),
  }
  saveState.value = 'saving'
  error.value = ''
  try {
    const saved = await DocumentService.Save(payload)
    if (!saved) return
    // 목록 전체를 다시 받지 않고 해당 항목만 갱신해 편집 흐름을 끊지 않는다.
    const index = documents.value.findIndex(d => d.id === saved.id)
    if (index >= 0) documents.value[index] = saved
    titleInput.value = saved.title
    refreshKnownTitles()
    markSaved()
    await refreshBacklinks()
  } catch (e) {
    saveState.value = 'idle'
    error.value = (e as Error).message
  }
}

function markSaved() {
  saveState.value = 'saved'
  if (saveStateTimer) clearTimeout(saveStateTimer)
  saveStateTimer = setTimeout(() => { if (saveState.value === 'saved') saveState.value = 'idle' }, 2000)
}

async function createDocument(title = '') {
  await flushSave()
  try {
    const doc = await DocumentService.Create(title)
    if (!doc) return
    documents.value = [doc, ...documents.value]
    refreshKnownTitles()
    loadingDocument = true
    selectedId.value = doc.id
    titleInput.value = doc.title
    editor?.setMarkdown('')
    loadingDocument = false
    await refreshBacklinks()
  } catch (e) {
    error.value = (e as Error).message
  }
}

async function deleteDocument(doc: Document) {
  if (!confirm(`문서 "${doc.title}" 을(를) 삭제할까요?`)) return
  try {
    await DocumentService.Delete(doc.id)
    if (selectedId.value === doc.id) {
      selectedId.value = ''
      titleInput.value = ''
      editor?.setMarkdown('')
      backlinks.value = []
    }
    await refreshDocuments()
  } catch (e) {
    error.value = (e as Error).message
  }
}

// 미리보기의 [[링크]] 클릭을 가로채 해당 문서로 이동한다.
// 없는 문서를 가리키는 링크는 그 제목으로 새 문서를 만든다.
async function onEditorClick(mouseEvent: MouseEvent) {
  const target = mouseEvent.target as HTMLElement | null
  const anchor = target?.closest('a[data-doc-link]') as HTMLElement | null
  if (!anchor) return
  mouseEvent.preventDefault()

  const title = anchor.dataset.docLink?.trim()
  if (!title) return
  try {
    const found = await DocumentService.FindByTitle(title)
    if (found) await selectDocument(found.id)
    else await createDocument(title)
  } catch (e) {
    error.value = (e as Error).message
  }
}

function onTitleInput() {
  scheduleSave()
}

function formatDate(value?: string): string {
  if (!value) return ''
  const date = new Date(value)
  return isNaN(date.getTime()) ? value : date.toLocaleString()
}

onMounted(async () => {
  createEditor('')
  await refreshDocuments()
  if (documents.value.length) await selectDocument(documents.value[0].id)
})

onBeforeUnmount(() => {
  if (saveTimer) clearTimeout(saveTimer)
  if (saveStateTimer) clearTimeout(saveStateTimer)
  destroyEditor()
})
</script>

<template>
  <div class="document-layout">
    <aside class="document-sidebar">
      <div class="sidebar-header">
        <h1>문서</h1>
        <button class="icon-btn" title="새 문서" @click="createDocument()">+</button>
      </div>
      <input v-model="filter" class="search" type="search" placeholder="제목·본문 검색" />
      <ul class="document-list">
        <li
          v-for="doc in filteredDocuments"
          :key="doc.id"
          :class="{ active: doc.id === selectedId }"
          @click="selectDocument(doc.id)"
        >
          <div class="doc-row">
            <span class="doc-title">{{ doc.title }}</span>
            <button class="icon-btn sm danger doc-delete" title="문서 삭제" @click.stop="deleteDocument(doc)">✕</button>
          </div>
          <span class="doc-date">{{ formatDate(doc.updatedAt) }}</span>
        </li>
        <li v-if="!filteredDocuments.length" class="empty">
          {{ documents.length ? '검색 결과가 없습니다' : '문서가 없습니다' }}
        </li>
      </ul>
    </aside>

    <section class="document-main">
      <header class="document-header">
        <input
          v-model="titleInput"
          class="title-input"
          placeholder="문서 제목"
          :disabled="!selectedId"
          @input="onTitleInput"
        />
        <span class="save-state" role="status">
          {{ saveState === 'saving' ? '저장 중…' : saveState === 'saved' ? '저장됨' : '' }}
        </span>
      </header>

      <div v-if="error" class="alert error">{{ error }}</div>

      <div v-show="!selectedId" class="state">
        <p>왼쪽에서 문서를 고르거나 새 문서를 만드세요.</p>
        <p class="hint">본문에 <code>[[다른 문서 제목]]</code> 을 적으면 미리보기에서 눌러 이동할 수 있습니다.</p>
      </div>

      <!-- 에디터는 한 번만 만들고 문서를 바꿔 끼우므로 v-if 대신 v-show로 감춘다. -->
      <div v-show="selectedId" class="editor-wrap">
        <div ref="editorElement" class="editor-host" @click="onEditorClick"></div>
        <section class="backlinks">
          <h2>이 문서를 링크한 문서 <span class="count">{{ backlinks.length }}</span></h2>
          <ul v-if="backlinks.length">
            <li v-for="doc in backlinks" :key="doc.id">
              <button class="backlink" type="button" @click="selectDocument(doc.id)">{{ doc.title }}</button>
            </li>
          </ul>
          <p v-else class="hint">아직 이 문서를 링크한 문서가 없습니다.</p>
        </section>
      </div>
    </section>
  </div>
</template>

<style scoped>
.document-layout {
  display: grid;
  grid-template-columns: minmax(0, 240px) minmax(0, 1fr);
  width: 100%;
  min-width: 0;
  height: 100%;
  color: var(--text);
}
.document-sidebar {
  display: flex;
  flex-direction: column;
  min-width: 0;
  background: var(--panel);
  border-right: 1px solid var(--border);
  overflow: hidden;
}
.sidebar-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 18px 16px 12px;
  border-bottom: 1px solid var(--border);
}
.sidebar-header h1 { margin: 0; font-size: 18px; letter-spacing: 0.5px; }
.search {
  margin: 10px 12px;
  padding: 7px 9px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--panel-2);
  color: var(--text);
  font: inherit;
  font-size: 12px;
}
.document-list { flex: 1; list-style: none; margin: 0; padding: 0; overflow: auto; }
.document-list li {
  padding: 8px 16px;
  border-bottom: 1px solid transparent;
  cursor: pointer;
}
.document-list li:hover { background: var(--panel-2); }
.document-list li.active {
  background: var(--panel-2);
  border-left: 3px solid var(--accent);
  padding-left: 13px;
}
.doc-row { display: flex; align-items: center; gap: 6px; min-width: 0; }
.doc-title {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 600;
  font-size: 13px;
}
.doc-delete { display: none; flex: 0 0 auto; }
.document-list li:hover .doc-delete { display: inline-flex; }
.doc-date { display: block; margin-top: 2px; color: var(--muted); font-size: 11px; }
.empty { color: var(--muted); font-style: italic; cursor: default; }
.empty:hover { background: transparent; }

.icon-btn {
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

.document-main {
  display: flex;
  flex-direction: column;
  min-width: 0;
  overflow: hidden;
}
.document-header {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 12px 16px;
  border-bottom: 1px solid var(--border);
}
.title-input {
  flex: 1;
  min-width: 0;
  padding: 6px 8px;
  border: 1px solid transparent;
  border-radius: 6px;
  background: transparent;
  color: var(--text);
  font: inherit;
  font-size: 17px;
  font-weight: 600;
}
.title-input:hover:not(:disabled), .title-input:focus {
  border-color: var(--border);
  background: var(--panel-2);
}
.title-input:disabled { opacity: 0.5; }
.save-state { flex-shrink: 0; color: var(--muted); font-size: 11px; min-width: 44px; text-align: right; }

.alert { margin: 8px 16px; padding: 8px 12px; border-radius: 6px; font-size: 13px; }
.alert.error { background: rgba(255, 90, 106, 0.12); color: var(--danger); overflow-wrap: anywhere; }

.state { padding: 60px 24px; text-align: center; color: var(--muted); }
.hint { color: var(--muted); font-size: 12px; }
.hint code {
  padding: 1px 5px;
  border-radius: 3px;
  background: var(--panel-2);
  border: 1px solid var(--border);
}

.editor-wrap { display: flex; flex-direction: column; flex: 1; min-height: 0; }
.editor-host { flex: 1; min-height: 0; overflow: hidden; }

.backlinks {
  flex-shrink: 0;
  max-height: 140px;
  overflow: auto;
  padding: 10px 16px 14px;
  border-top: 1px solid var(--border);
  background: var(--panel);
}
.backlinks h2 { margin: 0 0 8px; color: var(--muted); font-size: 11px; text-transform: uppercase; letter-spacing: 0.6px; }
.backlinks .count { color: var(--accent); }
.backlinks ul { display: flex; flex-wrap: wrap; gap: 6px; margin: 0; padding: 0; list-style: none; }
.backlink {
  padding: 3px 9px;
  border: 1px solid var(--border);
  border-radius: 12px;
  background: var(--panel-2);
  color: var(--text);
  font-size: 12px;
  cursor: pointer;
}
.backlink:hover { border-color: var(--accent); color: var(--accent); }
</style>

<style>
/* 미리보기 안의 위키 링크. scoped 밖에 두어야 에디터가 그린 DOM에 적용된다. */
.toastui-editor-contents a.doc-link {
  color: var(--accent);
  text-decoration: none;
  border-bottom: 1px solid currentColor;
  cursor: pointer;
}
.toastui-editor-contents a.doc-link:hover { opacity: 0.8; }
.toastui-editor-contents a.doc-link.missing {
  color: var(--danger);
  border-bottom-style: dashed;
}
</style>
