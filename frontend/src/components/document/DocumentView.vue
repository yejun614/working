<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch, nextTick } from 'vue'
import Editor from '@toast-ui/editor'
import '@toast-ui/editor/dist/toastui-editor.css'
import '@toast-ui/editor/dist/theme/toastui-editor-dark.css'
// TOAST UI Editor가 제공하는 플러그인을 모두 켠다.
// 각 플러그인은 자체 스타일시트를 함께 요구하므로 CSS도 같이 불러온다.
import chart from '@toast-ui/editor-plugin-chart'
import '@toast-ui/chart/dist/toastui-chart.css'
import codeSyntaxHighlight from '@toast-ui/editor-plugin-code-syntax-highlight/dist/toastui-editor-plugin-code-syntax-highlight-all'
import '@toast-ui/editor-plugin-code-syntax-highlight/dist/toastui-editor-plugin-code-syntax-highlight.css'
import 'prismjs/themes/prism.css'
import colorSyntax from '@toast-ui/editor-plugin-color-syntax'
import 'tui-color-picker/dist/tui-color-picker.css'
import '@toast-ui/editor-plugin-color-syntax/dist/toastui-editor-plugin-color-syntax.css'
import tableMergedCell from '@toast-ui/editor-plugin-table-merged-cell'
import '@toast-ui/editor-plugin-table-merged-cell/dist/toastui-editor-plugin-table-merged-cell.css'
import uml from '@toast-ui/editor-plugin-uml'
import { Service as DocumentService } from '../../../bindings/working/internal/modules/document'
import type { DocType, Document, Folder } from '../../../bindings/working/internal/modules/document/types/models'
import { isDarkMode } from '../../theme'
import ResizeHandle from '../common/ResizeHandle.vue'
import { usePaneVisible, usePaneWidth } from '../../panes'

const sidebarWidth = usePaneWidth('document:sidebar', 240)
// 탭 바의 패널 버튼으로 문서 목록을 접을 수 있다.
const sidebarVisible = usePaneVisible('document:sidebar')
const layoutColumns = computed(() => ({
  gridTemplateColumns: sidebarVisible.value
    ? `${sidebarWidth.value}px auto minmax(0, 1fr)`
    : 'minmax(0, 1fr)',
}))

const documents = ref<Document[]>([])
const folders = ref<Folder[]>([])
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

/* ---------- 문서 형식 ---------- */

// 형식은 문서마다 따로 저장한다. 같은 본문을 어떤 편집기로 열지만 달라진다.
interface DocTypeInfo {
  id: DocType
  label: string
  icon: string
  description: string
}

const DOC_TYPES: DocTypeInfo[] = [
  { id: 'wysiwyg', label: '리치 텍스트', icon: 'A', description: '서식을 바로 보며 편집합니다' },
  { id: 'markdown', label: '마크다운', icon: 'M', description: '원문을 편집하고 옆에서 미리 봅니다' },
  { id: 'text', label: '일반 텍스트', icon: 'T', description: '서식 없이 원문만 편집합니다' },
]

const WRAP_KEY = 'working:document-wrap'

function loadWrap(): boolean {
  try {
    return localStorage.getItem(WRAP_KEY) === 'true'
  } catch {
    return false
  }
}

// 지금 열어 둔 문서의 형식. 문서를 바꿀 때마다 그 문서의 값으로 맞춘다.
const docType = ref<DocType>('markdown')
// 일반 텍스트 형식의 본문. 에디터와 따로 들고 있다가 형식을 바꿀 때 주고받는다.
const plainText = ref('')
// Windows 메모장처럼 자동 줄바꿈은 기본으로 꺼 둔다. 이 설정만 앱 전체에서 공유한다.
const wrapText = ref(loadWrap())
const typeMenuOpen = ref(false)

function typeInfo(type?: DocType): DocTypeInfo {
  return DOC_TYPES.find(item => item.id === type) ?? DOC_TYPES[1]
}

const currentType = computed(() => typeInfo(docType.value))

function toggleWrap() {
  wrapText.value = !wrapText.value
  try {
    localStorage.setItem(WRAP_KEY, String(wrapText.value))
  } catch {
    // 저장소를 쓸 수 없어도 현재 세션의 설정은 그대로 유지된다.
  }
}

// 지금 보고 있는 형식에서 편집 중인 본문을 가져온다.
function currentContent(): string {
  if (docType.value === 'text') return plainText.value
  return editor?.getMarkdown() ?? ''
}

// 형식과 본문을 함께 적용한다. 두 편집기에 같은 본문을 넣어 두어야
// 형식을 바꿔도 내용이 이어진다.
function applyType(type: DocType, content: string) {
  loadingDocument = true
  docType.value = type
  plainText.value = content
  if (type !== 'text') {
    editor?.setMarkdown(content)
    editor?.changeMode(type === 'wysiwyg' ? 'wysiwyg' : 'markdown', true)
  }
  loadingDocument = false
}

async function changeType(next: DocType) {
  typeMenuOpen.value = false
  const current = selectedDocument.value
  if (!current || next === docType.value) return

  // 형식을 바꾸기 전에 편집 중이던 내용을 먼저 저장해 목록의 문서와 어긋나지 않게 한다.
  await flushSave()
  applyType(next, currentContent())
  try {
    const saved = await DocumentService.SetType(current.id, next)
    if (saved) replaceInList(saved)
  } catch (e) {
    error.value = (e as Error).message
  }
}

function onPlainInput() {
  if (loadingDocument) return
  scheduleSave()
}

/* ---------- 폴더 ---------- */

const COLLAPSED_KEY = 'working:document-collapsed-folders'

function loadCollapsed(): string[] {
  try {
    const raw = localStorage.getItem(COLLAPSED_KEY)
    const parsed = raw ? JSON.parse(raw) : []
    return Array.isArray(parsed) ? parsed.filter((id): id is string => typeof id === 'string') : []
  } catch {
    return []
  }
}

const collapsedFolders = ref<string[]>(loadCollapsed())

function isCollapsed(id: string): boolean {
  return collapsedFolders.value.includes(id)
}

function toggleFolder(id: string) {
  collapsedFolders.value = isCollapsed(id)
    ? collapsedFolders.value.filter(item => item !== id)
    : [...collapsedFolders.value, id]
  try {
    localStorage.setItem(COLLAPSED_KEY, JSON.stringify(collapsedFolders.value))
  } catch {
    // 저장소를 쓸 수 없어도 현재 세션의 펼침 상태는 그대로 유지된다.
  }
}

// 폴더별로 담긴 문서. 목록은 이미 최근 수정순이라 그대로 나눠 담기만 한다.
const folderGroups = computed(() =>
  folders.value.map(folder => ({
    folder,
    documents: documents.value.filter(doc => doc.folderId === folder.id),
  })),
)

// 폴더에 넣지 않은 문서. 지워진 폴더를 가리키는 문서도 여기로 모은다.
const rootDocuments = computed(() =>
  documents.value.filter(doc => !doc.folderId || !folders.value.some(f => f.id === doc.folderId)),
)

// 검색 중에는 폴더 구분을 접고 결과만 한 줄로 보여 준다.
const searchResults = computed(() => {
  const keyword = filter.value.trim().toLowerCase()
  if (!keyword) return []
  return documents.value.filter(doc =>
    doc.title.toLowerCase().includes(keyword) || (doc.content || '').toLowerCase().includes(keyword)
  )
})

const isSearching = computed(() => filter.value.trim().length > 0)

async function refreshFolders() {
  try {
    folders.value = (await DocumentService.Folders()) || []
  } catch (e) {
    error.value = (e as Error).message
  }
}

async function createFolder() {
  const name = prompt('새 폴더 이름')
  if (name === null) return
  try {
    await DocumentService.CreateFolder(name.trim())
    await refreshFolders()
  } catch (e) {
    error.value = (e as Error).message
  }
}

async function renameFolder(folder: Folder) {
  const name = prompt('폴더 이름', folder.name)
  if (name === null) return
  const trimmed = name.trim()
  if (!trimmed || trimmed === folder.name) return
  try {
    await DocumentService.RenameFolder(folder.id, trimmed)
    await refreshFolders()
  } catch (e) {
    error.value = (e as Error).message
  }
}

async function deleteFolder(folder: Folder) {
  if (!confirm(`폴더 "${folder.name}" 을(를) 지울까요?\n안에 있던 문서는 지워지지 않고 폴더 밖으로 나옵니다.`)) return
  try {
    await DocumentService.DeleteFolder(folder.id)
    await refreshFolders()
    await refreshDocuments()
  } catch (e) {
    error.value = (e as Error).message
  }
}

/* ---------- 문서 옮기기 ---------- */

// 문서를 끌어 폴더에 놓으면 그 폴더로 옮긴다. 폴더 밖 영역에 놓으면 폴더에서 뺀다.
const draggedDocumentId = ref('')
// 놓을 자리 표시. 빈 문자열은 폴더 밖 영역이고, null은 놓을 곳이 없다는 뜻이다.
const dropTarget = ref<string | null>(null)

function startDocumentDrag(doc: Document) {
  hidePreview()
  draggedDocumentId.value = doc.id
}

function previewDrop(folderId: string) {
  if (!draggedDocumentId.value) return
  dropTarget.value = folderId
}

function endDocumentDrag() {
  draggedDocumentId.value = ''
  dropTarget.value = null
}

async function dropOnFolder(folderId: string) {
  const id = draggedDocumentId.value
  endDocumentDrag()
  if (!id) return
  const doc = documents.value.find(item => item.id === id)
  if (!doc || (doc.folderId || '') === folderId) return
  await moveDocument(id, folderId)
}

// 끌어 놓기 말고 문서 정보 모달에서도 폴더를 고를 수 있게 같은 동작을 함수로 둔다.
async function moveDocument(id: string, folderId: string) {
  try {
    const moved = await DocumentService.MoveDocument(id, folderId)
    if (moved) replaceInList(moved)
  } catch (e) {
    error.value = (e as Error).message
  }
}

/* ---------- 위키 링크 ---------- */

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

/* ---------- 에디터 ---------- */

function createEditor(initialContent: string) {
  if (!editorElement.value) return
  editor = new Editor({
    el: editorElement.value,
    height: '100%',
    // 일반 텍스트 문서에서도 에디터는 살려 두고 화면에서만 감추므로,
    // 그 경우에는 마크다운으로 만들어 둔다.
    initialEditType: docType.value === 'wysiwyg' ? 'wysiwyg' : 'markdown',
    previewStyle: 'vertical',
    // 형식 전환은 제목 왼쪽 아이콘이 담당하므로 에디터 기본 스위치는 감춘다.
    hideModeSwitch: true,
    usageStatistics: false,
    theme: isDarkMode.value ? 'dark' : 'default',
    initialValue: initialContent,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    customHTMLRenderer: { text: renderWikiLinks as any },
    plugins: [
      // 표 셀 병합, 글자 색, 코드 구문 강조(모든 언어 포함), 차트, UML.
      tableMergedCell,
      colorSyntax,
      codeSyntaxHighlight,
      [chart, { minWidth: 100, maxWidth: 900, minHeight: 100, maxHeight: 400 }],
      // UML은 plantuml.com 서버에서 그림을 받아오므로 오프라인에서는 표시되지 않는다.
      uml,
    ],
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
  const content = currentContent()
  destroyEditor()
  nextTick(() => {
    createEditor(content)
    applyType(docType.value, content)
  })
})

/* ---------- 목록/문서 ---------- */

// replaceInList는 목록 안의 문서 하나만 갈아 끼운다.
// 목록 전체를 다시 받으면 편집 흐름이 끊기므로 저장·이동 뒤에는 이 방법을 쓴다.
function replaceInList(doc: Document) {
  const index = documents.value.findIndex(item => item.id === doc.id)
  if (index >= 0) documents.value[index] = doc
  refreshKnownTitles()
}

async function refreshDocuments() {
  try {
    documents.value = (await DocumentService.List()) || []
    refreshKnownTitles()
  } catch (e) {
    error.value = (e as Error).message
  }
}

async function selectDocument(id: string) {
  hidePreview()
  if (id === selectedId.value) return
  await flushSave()
  try {
    const doc = await DocumentService.Get(id)
    if (!doc) return
    selectedId.value = doc.id
    titleInput.value = doc.title
    applyType(typeInfo(doc.type).id, doc.content || '')
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
  if (!current) return

  const payload: Document = {
    ...current,
    title: titleInput.value.trim() || current.title,
    type: docType.value,
    content: currentContent(),
  }
  saveState.value = 'saving'
  error.value = ''
  try {
    const saved = await DocumentService.Save(payload)
    if (!saved) return
    replaceInList(saved)
    titleInput.value = saved.title
    markSaved()
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

async function createDocument(title = '', folderId = '') {
  hidePreview()
  await flushSave()
  try {
    // 새 문서는 지금 열어 둔 문서와 같은 형식으로 시작한다.
    const doc = await DocumentService.Create(title, folderId, docType.value)
    if (!doc) return
    documents.value = [doc, ...documents.value]
    refreshKnownTitles()
    selectedId.value = doc.id
    titleInput.value = doc.title
    applyType(typeInfo(doc.type).id, '')
  } catch (e) {
    error.value = (e as Error).message
  }
}

async function deleteDocument(doc: Document) {
  hidePreview()
  if (!confirm(`문서 "${doc.title}" 을(를) 삭제할까요?`)) return
  try {
    await DocumentService.Delete(doc.id)
    if (selectedId.value === doc.id) {
      selectedId.value = ''
      titleInput.value = ''
      applyType(docType.value, '')
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
    else await createDocument(title, selectedDocument.value?.folderId || '')
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

/* ---------- 문서 정보 ---------- */

const infoOpen = ref(false)
const backlinks = ref<Document[]>([])

function formatBytes(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

// 정보 모달에 보여 줄 값. 저장 전 내용도 반영하도록 편집 중인 본문으로 센다.
const documentInfo = computed(() => {
  const current = selectedDocument.value
  if (!current) return null
  const content = infoOpen.value ? currentContent() : ''
  return {
    title: titleInput.value.trim() || current.title,
    type: typeInfo(current.type),
    folderId: current.folderId || '',
    size: formatBytes(new TextEncoder().encode(content).length),
    characters: [...content].length,
    createdAt: formatDate(current.createdAt),
    updatedAt: formatDate(current.updatedAt),
  }
})

async function openInfo() {
  if (!selectedId.value) return
  infoOpen.value = true
  backlinks.value = []
  try {
    backlinks.value = (await DocumentService.Backlinks(selectedId.value)) || []
  } catch (e) {
    error.value = (e as Error).message
  }
}

function closeInfo() {
  infoOpen.value = false
}

async function openBacklink(doc: Document) {
  closeInfo()
  await selectDocument(doc.id)
}

// 모달과 형식 드롭다운은 Esc로 닫는다.
function onKeydown(event: KeyboardEvent) {
  if (event.key !== 'Escape') return
  if (infoOpen.value) closeInfo()
  typeMenuOpen.value = false
}

/* ---------- 목록 호버 미리보기 ---------- */

// 사이드바 항목에 마우스를 올리면 본문 앞부분을 팝오버로 보여 준다.
// 사이드바는 overflow: hidden이라 팝오버는 body로 내보내 띄운다.
const PREVIEW_DELAY = 300
const PREVIEW_LIMIT = 240
const PREVIEW_MAX_HEIGHT = 220

const hoverDocument = ref<Document | null>(null)
const previewPosition = ref({ top: 0, left: 0 })
let hoverTimer: ReturnType<typeof setTimeout> | undefined

// 팝오버 요약. 마크다운 기호를 걷어내 읽을 수 있는 문장만 남긴다.
function previewText(content?: string): string {
  const plain = (content || '')
    .replace(/```[^\n]*\n?/g, '')             // 코드 펜스 표시
    .replace(/!\[[^\]]*\]\([^)]*\)/g, '')     // 이미지
    .replace(/\[\[([^[\]]+)\]\]/g, '$1')      // 위키 링크
    .replace(/\[([^\]]*)\]\([^)]*\)/g, '$1')  // 일반 링크
    .replace(/^\s{0,3}#{1,6}\s+/gm, '')       // 제목
    .replace(/^\s{0,3}>\s?/gm, '')            // 인용
    .replace(/^\s{0,3}([-*+]|\d+\.)\s+/gm, '') // 목록 기호
    .replace(/[*_~`]/g, '')                   // 강조 기호
    .replace(/\n{3,}/g, '\n\n')
    .trim()
  if (!plain) return ''
  return plain.length > PREVIEW_LIMIT ? `${plain.slice(0, PREVIEW_LIMIT)}…` : plain
}

function showPreview(doc: Document, event: MouseEvent) {
  const item = event.currentTarget as HTMLElement | null
  if (!item || draggedDocumentId.value) return
  if (hoverTimer) clearTimeout(hoverTimer)
  hoverTimer = setTimeout(() => {
    const rect = item.getBoundingClientRect()
    // 아래쪽 항목에서도 팝오버가 화면 밖으로 나가지 않도록 위로 밀어 올린다.
    const top = Math.min(rect.top, window.innerHeight - PREVIEW_MAX_HEIGHT - 16)
    previewPosition.value = { top: Math.max(12, top), left: rect.right + 10 }
    hoverDocument.value = doc
  }, PREVIEW_DELAY)
}

function hidePreview() {
  if (hoverTimer) clearTimeout(hoverTimer)
  hoverTimer = undefined
  hoverDocument.value = null
}

onMounted(async () => {
  createEditor('')
  window.addEventListener('keydown', onKeydown)
  await Promise.all([refreshDocuments(), refreshFolders()])
  if (documents.value.length) await selectDocument(documents.value[0].id)
})

onBeforeUnmount(() => {
  if (saveTimer) clearTimeout(saveTimer)
  if (saveStateTimer) clearTimeout(saveStateTimer)
  window.removeEventListener('keydown', onKeydown)
  hidePreview()
  destroyEditor()
})
</script>

<template>
  <div class="document-layout" :style="layoutColumns">
    <aside v-if="sidebarVisible" class="document-sidebar">
      <div class="sidebar-header">
        <h1>문서</h1>
        <div class="header-buttons">
          <button class="icon-btn" title="새 폴더" @click="createFolder">⊞</button>
          <button class="icon-btn" title="새 문서" @click="createDocument()">+</button>
        </div>
      </div>
      <input v-model="filter" class="search" type="search" placeholder="제목·본문 검색" />

      <div class="sidebar-body" @mouseleave="hidePreview">
        <!-- 검색 중에는 폴더 구분 없이 결과만 보여 준다. -->
        <ul v-if="isSearching" class="document-list">
          <li
            v-for="doc in searchResults"
            :key="doc.id"
            :class="{ active: doc.id === selectedId }"
            @click="selectDocument(doc.id)"
            @mouseenter="showPreview(doc, $event)"
            @mouseleave="hidePreview"
          >
            <div class="doc-row">
              <span class="doc-type" :title="typeInfo(doc.type).label">{{ typeInfo(doc.type).icon }}</span>
              <span class="doc-title">{{ doc.title }}</span>
              <button class="icon-btn sm danger doc-delete" title="문서 삭제" @click.stop="deleteDocument(doc)">✕</button>
            </div>
            <span class="doc-date">{{ formatDate(doc.updatedAt) }}</span>
          </li>
          <li v-if="!searchResults.length" class="empty">검색 결과가 없습니다</li>
        </ul>

        <template v-else>
          <!-- 폴더별 묶음. 폴더 머리글에 문서를 끌어다 놓으면 그 폴더로 옮긴다. -->
          <section
            v-for="group in folderGroups"
            :key="group.folder.id"
            class="folder"
            :class="{ 'drop-target': dropTarget === group.folder.id }"
            @dragover.prevent="previewDrop(group.folder.id)"
            @drop.prevent="dropOnFolder(group.folder.id)"
          >
            <div class="folder-row" @click="toggleFolder(group.folder.id)">
              <span class="folder-caret" aria-hidden="true">{{ isCollapsed(group.folder.id) ? '▸' : '▾' }}</span>
              <span class="folder-name">{{ group.folder.name }}</span>
              <span class="folder-count">{{ group.documents.length }}</span>
              <button class="icon-btn sm folder-action" title="폴더 이름 변경" @click.stop="renameFolder(group.folder)">✎</button>
              <button class="icon-btn sm danger folder-action" title="폴더 삭제" @click.stop="deleteFolder(group.folder)">✕</button>
            </div>
            <ul v-if="!isCollapsed(group.folder.id)" class="document-list nested">
              <li
                v-for="doc in group.documents"
                :key="doc.id"
                :class="{ active: doc.id === selectedId, dragging: draggedDocumentId === doc.id }"
                draggable="true"
                @click="selectDocument(doc.id)"
                @dragstart="startDocumentDrag(doc)"
                @dragend="endDocumentDrag"
                @mouseenter="showPreview(doc, $event)"
                @mouseleave="hidePreview"
              >
                <div class="doc-row">
                  <span class="doc-type" :title="typeInfo(doc.type).label">{{ typeInfo(doc.type).icon }}</span>
                  <span class="doc-title">{{ doc.title }}</span>
                  <button class="icon-btn sm danger doc-delete" title="문서 삭제" @click.stop="deleteDocument(doc)">✕</button>
                </div>
                <span class="doc-date">{{ formatDate(doc.updatedAt) }}</span>
              </li>
              <li v-if="!group.documents.length" class="empty sm">문서를 끌어다 놓으세요</li>
            </ul>
          </section>

          <!-- 폴더 밖 문서. 이 영역에 놓으면 폴더에서 빠져나온다. -->
          <div
            class="root-drop"
            :class="{ 'drop-target': dropTarget === '' }"
            @dragover.prevent="previewDrop('')"
            @drop.prevent="dropOnFolder('')"
          >
            <ul class="document-list">
              <li
                v-for="doc in rootDocuments"
                :key="doc.id"
                :class="{ active: doc.id === selectedId, dragging: draggedDocumentId === doc.id }"
                draggable="true"
                @click="selectDocument(doc.id)"
                @dragstart="startDocumentDrag(doc)"
                @dragend="endDocumentDrag"
                @mouseenter="showPreview(doc, $event)"
                @mouseleave="hidePreview"
              >
                <div class="doc-row">
                  <span class="doc-type" :title="typeInfo(doc.type).label">{{ typeInfo(doc.type).icon }}</span>
                  <span class="doc-title">{{ doc.title }}</span>
                  <button class="icon-btn sm danger doc-delete" title="문서 삭제" @click.stop="deleteDocument(doc)">✕</button>
                </div>
                <span class="doc-date">{{ formatDate(doc.updatedAt) }}</span>
              </li>
              <li v-if="!documents.length" class="empty">문서가 없습니다</li>
            </ul>
          </div>
        </template>
      </div>
    </aside>

    <!-- 사이드바가 잘라내지 않도록 팝오버는 body에 붙여 띄운다. -->
    <Teleport to="body">
      <div
        v-if="hoverDocument"
        class="doc-preview"
        :style="{ top: `${previewPosition.top}px`, left: `${previewPosition.left}px` }"
      >
        <strong class="preview-title">{{ hoverDocument.title }}</strong>
        <span class="preview-date">{{ typeInfo(hoverDocument.type).label }} · {{ formatDate(hoverDocument.updatedAt) }}</span>
        <p v-if="previewText(hoverDocument.content)" class="preview-body">{{ previewText(hoverDocument.content) }}</p>
        <p v-else class="preview-body empty">내용이 비어 있습니다</p>
      </div>
    </Teleport>

    <ResizeHandle
      v-if="sidebarVisible"
      v-model:width="sidebarWidth"
      :min="180"
      :max="480"
      label="문서 목록 너비 조절"
    />

    <section class="document-main">
      <header class="document-header">
        <!-- 제목 왼쪽 아이콘이 문서 형식이자 형식 바꾸기 버튼이다. -->
        <div class="type-picker">
          <button
            class="type-button"
            type="button"
            :disabled="!selectedId"
            :title="`형식: ${currentType.label} (눌러서 변경)`"
            :aria-expanded="typeMenuOpen"
            @click="typeMenuOpen = !typeMenuOpen"
          >{{ currentType.icon }}</button>
          <div v-if="typeMenuOpen" class="menu-backdrop" @click="typeMenuOpen = false"></div>
          <ul v-if="typeMenuOpen" class="type-menu" role="menu">
            <li v-for="type in DOC_TYPES" :key="type.id">
              <button
                type="button"
                role="menuitem"
                :class="{ active: type.id === docType }"
                @click="changeType(type.id)"
              >
                <span class="menu-icon">{{ type.icon }}</span>
                <span class="menu-text">
                  <span class="menu-label">{{ type.label }}</span>
                  <span class="menu-description">{{ type.description }}</span>
                </span>
              </button>
            </li>
          </ul>
        </div>

        <input
          v-model="titleInput"
          class="title-input"
          placeholder="문서 제목"
          :disabled="!selectedId"
          @input="onTitleInput"
        />
        <button
          v-if="docType === 'text'"
          type="button"
          class="wrap-toggle"
          :class="{ active: wrapText }"
          :aria-pressed="wrapText"
          title="자동 줄바꿈"
          @click="toggleWrap"
        >자동 줄바꿈</button>
        <span class="save-state" role="status">
          {{ saveState === 'saving' ? '저장 중…' : saveState === 'saved' ? '저장됨' : '' }}
        </span>
        <button
          class="info-button"
          type="button"
          :disabled="!selectedId"
          title="문서 정보"
          aria-label="문서 정보"
          @click="openInfo"
        >ⓘ</button>
      </header>

      <div v-if="error" class="alert error">{{ error }}</div>

      <div v-show="!selectedId" class="state">
        <p>왼쪽에서 문서를 고르거나 새 문서를 만드세요.</p>
        <p class="hint">본문에 <code>[[다른 문서 제목]]</code> 을 적으면 미리보기에서 눌러 이동할 수 있습니다.</p>
      </div>

      <!-- 에디터는 한 번만 만들고 문서를 바꿔 끼우므로 v-if 대신 v-show로 감춘다. -->
      <div v-show="selectedId" class="editor-wrap">
        <div v-show="docType !== 'text'" ref="editorElement" class="editor-host" @click="onEditorClick"></div>
        <textarea
          v-show="docType === 'text'"
          v-model="plainText"
          class="plain-editor"
          :class="{ wrap: wrapText }"
          spellcheck="false"
          placeholder="서식 없는 텍스트를 그대로 편집합니다."
          @input="onPlainInput"
        ></textarea>
      </div>
    </section>

    <Teleport to="body">
      <div v-if="infoOpen && documentInfo" class="modal-overlay" @click.self="closeInfo">
        <div class="modal" role="dialog" aria-modal="true" aria-label="문서 정보">
          <header class="modal-header">
            <h2>문서 정보</h2>
            <button class="icon-btn" type="button" title="닫기" @click="closeInfo">✕</button>
          </header>
          <dl class="info-grid">
            <dt>문서명</dt>
            <dd>{{ documentInfo.title }}</dd>
            <dt>형식</dt>
            <dd><span class="info-type">{{ documentInfo.type.icon }}</span> {{ documentInfo.type.label }}</dd>
            <dt>폴더</dt>
            <dd>
              <!-- 목록에서 끌어 놓는 대신 여기서도 폴더를 옮길 수 있다. -->
              <select
                class="folder-select"
                :value="documentInfo.folderId"
                @change="moveDocument(selectedId, ($event.target as HTMLSelectElement).value)"
              >
                <option value="">폴더 없음</option>
                <option v-for="folder in folders" :key="folder.id" :value="folder.id">{{ folder.name }}</option>
              </select>
            </dd>
            <dt>용량</dt>
            <dd>{{ documentInfo.size }} <span class="muted">({{ documentInfo.characters.toLocaleString() }}자)</span></dd>
            <dt>생성일</dt>
            <dd>{{ documentInfo.createdAt || '알 수 없음' }}</dd>
            <dt>마지막 수정일</dt>
            <dd>{{ documentInfo.updatedAt || '알 수 없음' }}</dd>
          </dl>
          <section class="modal-backlinks">
            <h3>이 문서를 링크한 문서 <span class="count">{{ backlinks.length }}</span></h3>
            <ul v-if="backlinks.length">
              <li v-for="doc in backlinks" :key="doc.id">
                <button class="backlink" type="button" @click="openBacklink(doc)">{{ doc.title }}</button>
              </li>
            </ul>
            <p v-else class="hint">아직 이 문서를 링크한 문서가 없습니다.</p>
          </section>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<style scoped>
/* grid-template-columns는 드래그한 너비를 반영하도록 인라인 스타일에서 지정한다. */
.document-layout {
  display: grid;
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
.header-buttons { display: flex; gap: 4px; }
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
.sidebar-body { flex: 1; min-height: 0; overflow: auto; }

/* 폴더 묶음 */
.folder { border-bottom: 1px solid var(--border); }
.folder.drop-target, .root-drop.drop-target { background: rgba(79, 124, 255, 0.12); }
.folder-row {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 7px 12px;
  cursor: pointer;
  color: var(--muted);
  font-size: 12px;
  font-weight: 600;
}
.folder-row:hover { background: var(--panel-2); color: var(--text); }
.folder-caret { flex: 0 0 auto; width: 10px; font-size: 10px; }
.folder-name { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.folder-count { flex: 0 0 auto; font-size: 11px; opacity: 0.7; }
.folder-action { display: none; flex: 0 0 auto; }
.folder-row:hover .folder-action { display: inline-flex; }
.root-drop { min-height: 60px; }

.document-list { list-style: none; margin: 0; padding: 0; }
.document-list.nested li { padding-left: 28px; }
.document-list.nested li.active { padding-left: 25px; }
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
.document-list li.dragging { opacity: 0.45; }
.doc-row { display: flex; align-items: center; gap: 6px; min-width: 0; }
/* 목록의 형식 아이콘은 눈으로 구분만 하는 용도라 누를 수 없다. */
.doc-type {
  flex: 0 0 auto;
  width: 15px;
  text-align: center;
  color: var(--muted);
  font-size: 11px;
  font-weight: 700;
}
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
.doc-date { display: block; margin-top: 2px; margin-left: 21px; color: var(--muted); font-size: 11px; }
.empty { color: var(--muted); font-style: italic; cursor: default; padding: 8px 16px; }
.empty.sm { font-size: 11px; padding-left: 28px; }
.empty:hover { background: transparent; }

/* 목록 호버 팝오버. body로 옮겨 그리므로 위치는 인라인 스타일이 정한다. */
.doc-preview {
  position: fixed;
  z-index: 60;
  width: 300px;
  max-height: 220px;
  overflow: hidden;
  padding: 10px 12px;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--panel);
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.35);
  color: var(--text);
  /* 팝오버가 마우스를 가로채면 목록 클릭이 막히므로 이벤트를 받지 않는다. */
  pointer-events: none;
}
.preview-title { display: block; font-size: 13px; overflow-wrap: anywhere; }
.preview-date { display: block; margin-top: 2px; color: var(--muted); font-size: 11px; }
.preview-body {
  margin: 8px 0 0;
  max-height: 150px;
  overflow: hidden;
  color: var(--muted);
  font-size: 12px;
  line-height: 1.5;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.preview-body.empty { font-style: italic; padding: 0; }

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

/* 형식 아이콘과 드롭다운 */
.type-picker { position: relative; flex: 0 0 auto; }
.type-button {
  width: 30px;
  height: 30px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--panel-2);
  color: var(--accent);
  font-size: 14px;
  font-weight: 700;
}
.type-button:hover:not(:disabled) { border-color: var(--accent); }
.type-button:disabled { opacity: 0.4; cursor: default; }
/* 드롭다운 밖을 누르면 닫히도록 화면 전체를 덮는 투명 판을 깐다. */
.menu-backdrop { position: fixed; inset: 0; z-index: 30; }
.type-menu {
  position: absolute;
  top: calc(100% + 6px);
  left: 0;
  z-index: 31;
  width: 220px;
  margin: 0;
  padding: 4px;
  list-style: none;
  border: 1px solid var(--border);
  border-radius: 8px;
  background: var(--panel);
  box-shadow: 0 10px 30px rgba(0, 0, 0, 0.35);
}
.type-menu button {
  display: flex;
  align-items: center;
  gap: 9px;
  width: 100%;
  padding: 7px 8px;
  border: none;
  border-radius: 6px;
  background: transparent;
  color: var(--text);
  text-align: left;
}
.type-menu button:hover { background: var(--panel-2); }
.type-menu button.active { color: var(--accent); }
.menu-icon {
  flex: 0 0 auto;
  width: 20px;
  text-align: center;
  font-size: 13px;
  font-weight: 700;
}
.menu-text { display: flex; flex-direction: column; min-width: 0; }
.menu-label { font-size: 13px; }
.menu-description { color: var(--muted); font-size: 11px; }

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

.wrap-toggle {
  flex-shrink: 0;
  padding: 5px 10px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--panel);
  color: var(--muted);
  font-size: 12px;
  white-space: nowrap;
}
.wrap-toggle:hover { color: var(--text); background: var(--panel-2); }
.wrap-toggle.active { border-color: var(--accent); color: var(--accent); }

.save-state { flex-shrink: 0; color: var(--muted); font-size: 11px; min-width: 44px; text-align: right; }

.info-button {
  flex: 0 0 auto;
  width: 30px;
  height: 30px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  border: 1px solid transparent;
  border-radius: 6px;
  background: transparent;
  color: var(--muted);
  font-size: 17px;
  line-height: 1;
}
.info-button:hover:not(:disabled) { color: var(--accent); border-color: var(--border); background: var(--panel-2); }
.info-button:disabled { opacity: 0.4; cursor: default; }

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

/* 일반 텍스트 형식은 Windows 메모장처럼 서식 없이 원문만 다룬다.
   자동 줄바꿈은 기본으로 꺼 두고 가로 스크롤로 긴 줄을 확인한다. */
.plain-editor {
  flex: 1;
  min-height: 0;
  padding: 12px 16px;
  border: none;
  outline: none;
  resize: none;
  background: var(--panel-2);
  color: var(--text);
  font-family: "D2Coding", "Courier New", monospace;
  font-size: 14px;
  line-height: 1.6;
  tab-size: 4;
  white-space: pre;
  overflow: auto;
}
.plain-editor.wrap { white-space: pre-wrap; overflow-wrap: anywhere; }

/* 문서 정보 모달 */
.modal-overlay {
  position: fixed;
  inset: 0;
  z-index: 70;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
  background: rgba(0, 0, 0, 0.5);
}
.modal {
  width: min(440px, 100%);
  max-height: 100%;
  overflow: auto;
  padding: 16px 20px 20px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--panel);
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.45);
  color: var(--text);
}
.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 14px;
}
.modal-header h2 { margin: 0; font-size: 15px; }
.info-grid {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 8px 14px;
  margin: 0;
  font-size: 13px;
}
.info-grid dt { color: var(--muted); font-size: 12px; white-space: nowrap; }
.info-grid dd { margin: 0; min-width: 0; overflow-wrap: anywhere; }
.info-grid .muted { color: var(--muted); }
.info-type { color: var(--accent); font-weight: 700; }
.folder-select {
  max-width: 100%;
  padding: 4px 6px;
  border: 1px solid var(--border);
  border-radius: 5px;
  background: var(--panel-2);
  color: var(--text);
  font: inherit;
  font-size: 12px;
}

.modal-backlinks { margin-top: 18px; padding-top: 14px; border-top: 1px solid var(--border); }
.modal-backlinks h3 { margin: 0 0 8px; color: var(--muted); font-size: 11px; text-transform: uppercase; letter-spacing: 0.6px; }
.modal-backlinks .count { color: var(--accent); }
.modal-backlinks ul { display: flex; flex-wrap: wrap; gap: 6px; margin: 0; padding: 0; list-style: none; }
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

/* 코드 블록과 인라인 코드는 고정폭 D2 Coding으로 그린다.
   편집 화면(마크다운·WYSIWYG)과 미리보기, 구문 강조 플러그인이 만드는
   Prism 요소까지 모두 같은 글꼴을 쓰도록 한 번에 지정한다. */
.toastui-editor-contents code,
.toastui-editor-contents pre,
.toastui-editor-contents pre code,
.toastui-editor-md-container .toastui-editor-md-code,
.toastui-editor-md-container .toastui-editor-md-code-block,
.toastui-editor-ww-container .toastui-editor-ww-code-block,
.toastui-editor-ww-container .toastui-editor-ww-code-block-highlighting,
.toastui-editor-code-block-language-input,
pre[class*="language-"],
code[class*="language-"] {
  font-family: "D2Coding", "Courier New", monospace;
}

/* Prism 기본 테마는 밝은 배경 기준이라 다크 모드에서 대비가 무너진다.
   앱 테마가 어두울 때는 tomorrow 계열 색으로 덮어쓴다. */
:root:not([data-theme="light"]) pre[class*="language-"],
:root:not([data-theme="light"]) code[class*="language-"] {
  color: #ccc;
  background: none;
  text-shadow: none;
}
:root:not([data-theme="light"]) pre[class*="language-"] { background: #2d2d2d; }
:root:not([data-theme="light"]) .token.comment,
:root:not([data-theme="light"]) .token.block-comment,
:root:not([data-theme="light"]) .token.prolog,
:root:not([data-theme="light"]) .token.doctype,
:root:not([data-theme="light"]) .token.cdata { color: #999; }
:root:not([data-theme="light"]) .token.punctuation { color: #ccc; }
:root:not([data-theme="light"]) .token.tag,
:root:not([data-theme="light"]) .token.attr-name,
:root:not([data-theme="light"]) .token.namespace,
:root:not([data-theme="light"]) .token.deleted { color: #e2777a; }
:root:not([data-theme="light"]) .token.function-name { color: #6196cc; }
:root:not([data-theme="light"]) .token.boolean,
:root:not([data-theme="light"]) .token.number,
:root:not([data-theme="light"]) .token.function { color: #f08d49; }
:root:not([data-theme="light"]) .token.property,
:root:not([data-theme="light"]) .token.class-name,
:root:not([data-theme="light"]) .token.constant,
:root:not([data-theme="light"]) .token.symbol { color: #f8c555; }
:root:not([data-theme="light"]) .token.selector,
:root:not([data-theme="light"]) .token.important,
:root:not([data-theme="light"]) .token.atrule,
:root:not([data-theme="light"]) .token.keyword,
:root:not([data-theme="light"]) .token.builtin { color: #cc99cd; }
:root:not([data-theme="light"]) .token.string,
:root:not([data-theme="light"]) .token.char,
:root:not([data-theme="light"]) .token.attr-value,
:root:not([data-theme="light"]) .token.regex,
:root:not([data-theme="light"]) .token.variable { color: #7ec699; }
:root:not([data-theme="light"]) .token.operator,
:root:not([data-theme="light"]) .token.entity,
:root:not([data-theme="light"]) .token.url { color: #67cdcc; }
:root:not([data-theme="light"]) .token.inserted { color: #8bc34a; }
</style>
