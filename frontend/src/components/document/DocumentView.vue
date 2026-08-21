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
import { Dialogs, Events } from '@wailsio/runtime'
import { isDarkMode } from '../../theme'
import ResizeHandle from '../common/ResizeHandle.vue'
import FolderPlusIcon from './FolderPlusIcon.vue'
import LockIcon from './LockIcon.vue'
import NoEditIcon from './NoEditIcon.vue'
import BlankScreenIcon from './BlankScreenIcon.vue'
import KeyboardIcon from './KeyboardIcon.vue'
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

const editorElement = ref<HTMLElement | null>(null)
let editor: Editor | null = null
// setMarkdown이 change 이벤트를 다시 일으키므로, 문서를 불러오는 동안은 저장하지 않는다.
let loadingDocument = false
let saveTimer: ReturnType<typeof setTimeout> | undefined

const selectedDocument = computed(() => documents.value.find(d => d.id === selectedId.value) || null)

/* ---------- 오류 알림 ---------- */

// 자동 저장은 눈에 띄지 않게 흘러가므로, 실패했을 때만 모달로 분명히 알린다.
// 저장 말고 다른 작업의 실패도 같은 곳에서 보여 준다.
const errorMessage = ref('')

// 잠금 화면처럼 모달 밖에서도 오류를 보여 주는 곳이 있어 메시지 만들기를 나눠 둔다.
function messageOf(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

function reportError(e: unknown) {
  errorMessage.value = messageOf(e)
}

function closeError() {
  errorMessage.value = ''
}

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
    reportError(e)
  }
}

function onPlainInput() {
  if (loadingDocument) return
  scheduleSave()
}

/* ---------- 읽기 전용 ---------- */

// 다 쓴 문서를 실수로 고치거나 지우지 않도록 문서마다 읽기 전용을 켤 수 있다.
// 설정은 문서에 저장되므로 앱을 다시 켜도 그대로 유지된다.
const readOnly = computed(() => !!selectedDocument.value?.readOnly)

// TUI Editor에는 읽기 전용 설정이 없으므로 내부 ProseMirror 뷰의 editable
// 속성을 직접 바꾼다. 마크다운과 리치 텍스트는 각각 다른 뷰라 둘 다 손댄다.
function applyEditable() {
  const editable = !readOnly.value
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const internals = editor as any
  for (const part of [internals?.mdEditor, internals?.wwEditor]) {
    part?.view?.setProps({ editable: () => editable })
  }
}

watch(readOnly, () => applyEditable())

async function toggleReadOnly() {
  const current = selectedDocument.value
  if (!current) return
  // 읽기 전용으로 바꾸면 그 뒤의 저장이 막히므로 편집 중이던 내용을 먼저 저장한다.
  await flushSave()
  try {
    const saved = await DocumentService.SetReadOnly(current.id, !current.readOnly)
    if (saved) replaceInList(saved)
  } catch (e) {
    reportError(e)
  }
}

/* ---------- 암호 잠금 ---------- */

// 문서에 암호를 걸면 본문이 암호화된 채로 저장된다. 암호를 확인하기 전에는
// 목록·검색·미리보기에도 본문이 나오지 않고, 앱을 다시 켜면 다시 물어본다.
const locked = computed(() => !!selectedDocument.value?.locked)
const needsUnlock = computed(() => locked.value && !selectedDocument.value?.unlocked)

// 편집기 자리에 대신 띄우는 암호 입력 화면의 상태.
const unlockPassword = ref('')
const unlockError = ref('')
const unlockBusy = ref(false)
const unlockInput = ref<HTMLInputElement | null>(null)

// 잠긴 문서를 고르면 바로 암호를 칠 수 있게 입력칸에 초점을 준다.
watch(needsUnlock, value => {
  if (!value) return
  nextTick(() => unlockInput.value?.focus())
})

// 암호를 걸거나 없애는 모달의 상태.
const lockOpen = ref(false)
const lockPassword = ref('')
const lockConfirm = ref('')
const lockHint = ref('')
const lockError = ref('')
const lockBusy = ref(false)

function openLock() {
  if (!selectedId.value || needsUnlock.value) return
  lockPassword.value = ''
  lockConfirm.value = ''
  lockHint.value = selectedDocument.value?.hint || ''
  lockError.value = ''
  lockOpen.value = true
}

function closeLock() {
  lockOpen.value = false
  lockPassword.value = ''
  lockConfirm.value = ''
  lockError.value = ''
}

// 암호를 새로 걸어 문서를 잠근다.
// 방금 정한 암호이므로 잠근 뒤에도 이번 실행에서는 그대로 편집을 이어간다.
async function submitLock() {
  const current = selectedDocument.value
  if (!current || lockBusy.value) return
  if (!lockPassword.value) {
    lockError.value = '암호를 입력하세요'
    return
  }
  if (lockPassword.value !== lockConfirm.value) {
    lockError.value = '두 번 입력한 암호가 다릅니다'
    return
  }
  // 편집 중이던 내용까지 함께 잠기도록 먼저 저장한다.
  await flushSave()
  lockBusy.value = true
  try {
    const saved = await DocumentService.Lock(current.id, lockPassword.value, lockHint.value.trim())
    if (saved) replaceInList(saved)
    closeLock()
  } catch (e) {
    lockError.value = messageOf(e)
  } finally {
    lockBusy.value = false
  }
}

// 암호를 확인해 이번 실행 동안 문서를 열어 둔다.
async function submitUnlock() {
  const current = selectedDocument.value
  if (!current || unlockBusy.value) return
  unlockBusy.value = true
  unlockError.value = ''
  try {
    const opened = await DocumentService.Unlock(current.id, unlockPassword.value)
    if (opened) {
      replaceInList(opened)
      titleInput.value = opened.title
      applyType(typeInfo(opened.type).id, opened.content || '')
    }
    unlockPassword.value = ''
  } catch (e) {
    unlockError.value = messageOf(e)
  } finally {
    unlockBusy.value = false
  }
}

// 자리를 비울 때 다시 잠근다. 암호는 그대로 남고 화면에서도 본문을 지운다.
async function relock() {
  const current = selectedDocument.value
  if (!current) return
  await flushSave()
  closeLock()
  try {
    await DocumentService.Relock(current.id)
    const doc = await DocumentService.Get(current.id)
    if (doc) replaceInList(doc)
    // 편집기에 남아 있던 본문도 지워야 잠근 의미가 있다.
    applyType(docType.value, '')
  } catch (e) {
    reportError(e)
  }
}

// 암호를 확인한 뒤 잠금을 아예 없앤다. 본문은 다시 평문으로 저장된다.
async function removeLock() {
  const current = selectedDocument.value
  if (!current || lockBusy.value) return
  if (!lockPassword.value) {
    lockError.value = '암호를 입력하세요'
    return
  }
  lockBusy.value = true
  try {
    const saved = await DocumentService.RemoveLock(current.id, lockPassword.value)
    if (saved) {
      replaceInList(saved)
      applyType(typeInfo(saved.type).id, saved.content || '')
    }
    closeLock()
  } catch (e) {
    lockError.value = messageOf(e)
  } finally {
    lockBusy.value = false
  }
}

/* ---------- 빈 화면 ---------- */

// 아무 문서도 띄우지 않고 화면을 비워 둘 수 있다. 생각을 정리할 때 쓰므로,
// 모듈을 오갔다 돌아와도 비워 둔 상태가 유지되도록 기억해 둔다.
const BLANK_KEY = 'working:document-blank'

function loadBlank(): boolean {
  try {
    return localStorage.getItem(BLANK_KEY) === 'true'
  } catch {
    return false
  }
}

const blankScreen = ref(loadBlank())

watch(blankScreen, value => {
  try {
    localStorage.setItem(BLANK_KEY, String(value))
  } catch {
    // 저장소를 쓸 수 없어도 현재 세션의 상태는 그대로 유지된다.
  }
})

// 열어 둔 문서를 닫고 화면을 비운다. 문서는 그대로 남는다.
async function clearScreen() {
  hidePreview()
  await flushSave()
  selectedId.value = ''
  titleInput.value = ''
  applyType(docType.value, '')
  blankScreen.value = true
}

/* ---------- 폴더 트리 ---------- */

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

function rememberCollapsed() {
  try {
    localStorage.setItem(COLLAPSED_KEY, JSON.stringify(collapsedFolders.value))
  } catch {
    // 저장소를 쓸 수 없어도 현재 세션의 펼침 상태는 그대로 유지된다.
  }
}

function toggleFolder(id: string) {
  collapsedFolders.value = isCollapsed(id)
    ? collapsedFolders.value.filter(item => item !== id)
    : [...collapsedFolders.value, id]
  rememberCollapsed()
}

function expandFolder(id: string) {
  if (!id || !isCollapsed(id)) return
  collapsedFolders.value = collapsedFolders.value.filter(item => item !== id)
  rememberCollapsed()
}

const folderIds = computed(() => new Set(folders.value.map(folder => folder.id)))

// 없는 폴더를 가리키는 값은 최상위로 본다. 목록을 다시 받기 전에도 화면이 깨지지 않는다.
function parentOf(folder: Folder): string {
  return folder.parentId && folderIds.value.has(folder.parentId) ? folder.parentId : ''
}

function folderOf(doc: Document): string {
  return doc.folderId && folderIds.value.has(doc.folderId) ? doc.folderId : ''
}

// 백엔드가 이미 형제 순서대로 정렬해 보내므로 걸러 내기만 하면 순서가 유지된다.
function childFolders(parentId: string): Folder[] {
  return folders.value.filter(folder => parentOf(folder) === parentId)
}

function childDocuments(folderId: string): Document[] {
  return documents.value.filter(doc => folderOf(doc) === folderId)
}

// 트리는 한 줄짜리 목록으로 펼쳐 그린다. 깊이는 들여쓰기로만 나타내므로
// 재귀 컴포넌트 없이도 폴더 안 폴더를 얼마든지 표현할 수 있고,
// 끌어 놓을 자리를 계산하기도 쉽다.
type RowKind = 'folder' | 'document' | 'empty'

interface TreeRow {
  key: string
  kind: RowKind
  depth: number
  folder?: Folder
  document?: Document
}

function collectRows(parentId: string, depth: number, out: TreeRow[]) {
  for (const folder of childFolders(parentId)) {
    out.push({ key: `f:${folder.id}`, kind: 'folder', depth, folder })
    if (isCollapsed(folder.id)) continue
    collectRows(folder.id, depth + 1, out)
    const docs = childDocuments(folder.id)
    for (const doc of docs) {
      out.push({ key: `d:${doc.id}`, kind: 'document', depth: depth + 1, document: doc })
    }
    if (!docs.length && !childFolders(folder.id).length) {
      out.push({ key: `e:${folder.id}`, kind: 'empty', depth: depth + 1, folder })
    }
  }
}

const treeRows = computed(() => {
  const out: TreeRow[] = []
  collectRows('', 0, out)
  for (const doc of childDocuments('')) {
    out.push({ key: `d:${doc.id}`, kind: 'document', depth: 0, document: doc })
  }
  return out
})

// 폴더 안 문서 수는 하위 폴더까지 합쳐 센다.
function folderCount(id: string): number {
  let total = childDocuments(id).length
  for (const child of childFolders(id)) total += folderCount(child.id)
  return total
}

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
    reportError(e)
  }
}

async function createFolder(parentId = '') {
  const name = prompt(parentId ? '새 하위 폴더 이름' : '새 폴더 이름')
  if (name === null) return
  try {
    await DocumentService.CreateFolder(name.trim(), parentId)
    expandFolder(parentId)
    await refreshFolders()
  } catch (e) {
    reportError(e)
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
    reportError(e)
  }
}

async function deleteFolder(folder: Folder) {
  if (!confirm(`폴더 "${folder.name}" 을(를) 지울까요?\n안에 있던 하위 폴더와 문서는 지워지지 않고 한 단계 위로 나옵니다.`)) return
  try {
    await DocumentService.DeleteFolder(folder.id)
    await refreshTree()
  } catch (e) {
    reportError(e)
  }
}

/* ---------- 끌어 놓기로 옮기기 ---------- */

// 문서와 폴더를 끌어서 다른 폴더로 옮기거나 형제끼리 순서를 바꾼다.
// 줄의 위/아래 가장자리에 놓으면 그 자리에 끼워 넣고, 폴더 줄 가운데에
// 놓으면 그 폴더 안으로 들어간다.
type DropPosition = 'before' | 'after' | 'inside'

const dragged = ref<{ kind: 'folder' | 'document'; id: string } | null>(null)
const dropHint = ref<{ key: string; position: DropPosition } | null>(null)

// 끌 수 있는 줄은 누른 채 몇 픽셀만 움직여도 브라우저가 끌기를 시작하고
// click 이벤트를 없앤다. 그래서 줄을 고르는 일은 click이 아니라 누르고 떼는
// 과정에서 직접 처리한다. 이 정도(px)까지 움직인 것은 클릭으로 본다.
const DRAG_SLOP = 12

// 누른 지점과, 뗄 때 접거나 펼 폴더. 문서와 달리 폴더는 끌기가 아니라고
// 판명된 뒤에 접어야 끌고 가는 도중에 하위 항목이 나타나 줄이 밀리지 않는다.
let pressPoint: { x: number; y: number } | null = null
let pendingFolder = ''

// 문서는 파일 탐색기처럼 누르는 순간 바로 고른다. 끌어서 옮기려는 경우에도
// 그 문서가 선택되는 것이 자연스럽다.
// 버튼 위에서 누른 것은 버튼이 알아서 처리하도록 그냥 넘긴다.
function onRowPointerDown(row: TreeRow, event: PointerEvent) {
  pendingFolder = ''
  pressPoint = null
  if (event.button !== 0) return
  if ((event.target as HTMLElement | null)?.closest('button')) return

  pressPoint = { x: event.clientX, y: event.clientY }
  if (row.kind === 'folder') pendingFolder = row.folder!.id
  else if (row.kind === 'document') void selectDocument(row.document!.id)
}

// 끌기가 시작되지 않은 보통의 클릭. 여기서 폴더를 접거나 편다.
function onRowPointerUp() {
  if (pendingFolder) toggleFolder(pendingFolder)
  pendingFolder = ''
  pressPoint = null
}

function startDrag(row: TreeRow, event: DragEvent) {
  hidePreview()
  if (row.kind === 'folder') dragged.value = { kind: 'folder', id: row.folder!.id }
  else if (row.kind === 'document') dragged.value = { kind: 'document', id: row.document!.id }
  else return
  // 일부 브라우저는 데이터가 없으면 끌기를 시작하지 않는다.
  event.dataTransfer?.setData('text/plain', dragged.value.id)
  if (event.dataTransfer) event.dataTransfer.effectAllowed = 'move'
}

// 끌기가 시작되면 pointerup 대신 dragend가 온다. 손이 조금 흔들려 끌기로
// 넘어갔을 뿐이라면 클릭으로 보고 폴더를 접거나 편다.
function endDrag(event?: DragEvent) {
  if (pendingFolder && pressPoint && event) {
    const distance = Math.hypot(event.clientX - pressPoint.x, event.clientY - pressPoint.y)
    if (distance <= DRAG_SLOP) toggleFolder(pendingFolder)
  }
  pendingFolder = ''
  pressPoint = null
  dragged.value = null
  dropHint.value = null
}

// 폴더를 자기 하위로 옮기면 트리가 끊기므로 미리 막는다.
function isDescendantFolder(candidateId: string, ancestorId: string): boolean {
  const parents = new Map(folders.value.map(folder => [folder.id, parentOf(folder)]))
  const seen = new Set<string>()
  let current = candidateId
  while (current && !seen.has(current)) {
    if (current === ancestorId) return true
    seen.add(current)
    current = parents.get(current) ?? ''
  }
  return false
}

// 이 줄의 어느 높이에 놓았는지로 무엇을 하려는지 정한다.
function dropIntent(row: TreeRow, ratio: number): DropPosition | null {
  const item = dragged.value
  if (!item) return null

  if (row.kind === 'empty') {
    return item.kind === 'document' || !isDescendantFolder(row.folder!.id, item.id) ? 'inside' : null
  }

  if (row.kind === 'folder') {
    const folder = row.folder!
    if (item.kind === 'folder') {
      if (folder.id === item.id) return null
      if (isDescendantFolder(folder.id, item.id)) return null
      return ratio < 0.3 ? 'before' : ratio > 0.7 ? 'after' : 'inside'
    }
    // 문서는 폴더 줄 어디에 놓아도 그 폴더 안으로 들어간다.
    return 'inside'
  }

  // 문서 줄에는 문서만 앞뒤로 끼워 넣는다.
  if (item.kind !== 'document' || row.document!.id === item.id) return null
  return ratio < 0.5 ? 'before' : 'after'
}

function onRowDragOver(row: TreeRow, event: DragEvent) {
  if (!dragged.value) return
  const element = event.currentTarget as HTMLElement
  const rect = element.getBoundingClientRect()
  const ratio = rect.height ? (event.clientY - rect.top) / rect.height : 0.5
  const position = dropIntent(row, ratio)
  dropHint.value = position ? { key: row.key, position } : null
  if (event.dataTransfer) event.dataTransfer.dropEffect = position ? 'move' : 'none'
}

async function onRowDrop(row: TreeRow) {
  const item = dragged.value
  const hint = dropHint.value
  endDrag()
  if (!item || !hint || hint.key !== row.key) return

  if (item.kind === 'document') {
    if (hint.position === 'inside') {
      const folderId = row.folder!.id
      expandFolder(folderId)
      await moveDocument(item.id, folderId, childDocuments(folderId).length)
      return
    }
    const target = row.document!
    const folderId = folderOf(target)
    const siblings = childDocuments(folderId).filter(doc => doc.id !== item.id)
    const at = siblings.findIndex(doc => doc.id === target.id)
    const index = at < 0 ? siblings.length : hint.position === 'after' ? at + 1 : at
    await moveDocument(item.id, folderId, index)
    return
  }

  if (hint.position === 'inside') {
    const parentId = row.folder!.id
    expandFolder(parentId)
    await moveFolder(item.id, parentId, childFolders(parentId).length)
    return
  }
  const target = row.folder!
  const parentId = parentOf(target)
  const siblings = childFolders(parentId).filter(folder => folder.id !== item.id)
  const at = siblings.findIndex(folder => folder.id === target.id)
  const index = at < 0 ? siblings.length : hint.position === 'after' ? at + 1 : at
  await moveFolder(item.id, parentId, index)
}

function onRootDragOver(event: DragEvent) {
  if (!dragged.value) return
  dropHint.value = { key: 'root', position: 'inside' }
  if (event.dataTransfer) event.dataTransfer.dropEffect = 'move'
}

async function onRootDrop() {
  const item = dragged.value
  endDrag()
  if (!item) return
  if (item.kind === 'document') await moveDocument(item.id, '', childDocuments('').length)
  else await moveFolder(item.id, '', childFolders('').length)
}

// 순서를 바꾸면 형제 전체의 순서가 다시 매겨지므로 목록을 다시 받아 온다.
async function moveDocument(id: string, folderId: string, index: number) {
  try {
    await DocumentService.MoveDocument(id, folderId, index)
    await refreshDocuments()
  } catch (e) {
    reportError(e)
  }
}

async function moveFolder(id: string, parentId: string, index: number) {
  try {
    await DocumentService.MoveFolder(id, parentId, index)
    await refreshFolders()
  } catch (e) {
    reportError(e)
  }
}

async function refreshTree() {
  await Promise.all([refreshDocuments(), refreshFolders()])
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

/* ---------- 단축키 ---------- */

// 글자와 문단 서식을 손을 옮기지 않고 바로 적용할 수 있게 단축키를 쓴다.
// 굵게·기울임처럼 편집기에 이미 있는 것은 그대로 두고, 제목·링크처럼 빠진
// 것만 onEditorKeydown에서 채운다. 아래 목록은 도움말 모달에 그대로 보여 준다.
interface ShortcutGroup {
  title: string
  items: Array<{ keys: string; label: string }>
}

const SHORTCUT_GROUPS: ShortcutGroup[] = [
  {
    title: '글자 효과',
    items: [
      { keys: 'Ctrl+B', label: '굵게' },
      { keys: 'Ctrl+I', label: '기울임' },
      { keys: 'Ctrl+S', label: '취소선' },
      { keys: 'Ctrl+E', label: '인라인 코드' },
    ],
  },
  {
    title: '문단',
    items: [
      { keys: 'Ctrl+1 ~ Ctrl+6', label: '제목 1~6단계' },
      { keys: 'Ctrl+0', label: '제목을 떼고 본문으로' },
      { keys: 'Alt+Q', label: '인용' },
      { keys: 'Ctrl+Shift+P', label: '코드 블록' },
      { keys: 'Ctrl+L', label: '구분선' },
    ],
  },
  {
    title: '목록',
    items: [
      { keys: 'Ctrl+U', label: '순서 없는 목록' },
      { keys: 'Ctrl+O', label: '순서 있는 목록' },
      { keys: 'Alt+T', label: '체크리스트' },
      { keys: 'Tab / Shift+Tab', label: '한 단계 안으로 / 밖으로' },
    ],
  },
  {
    title: '링크',
    items: [
      { keys: 'Ctrl+K', label: '고른 글자에 링크 주소 넣기' },
      { keys: 'Ctrl+Shift+K', label: '고른 글자를 [[문서 링크]]로' },
    ],
  },
  {
    title: '편집',
    items: [
      { keys: 'Ctrl+Z', label: '되돌리기' },
      { keys: 'Ctrl+Shift+Z', label: '다시 실행' },
    ],
  },
]

const shortcutsOpen = ref(false)

// 편집기 안에서 누른 키를 받아 편집기에 없는 단축키를 채운다.
// 읽기 전용이거나 보조키가 맞지 않으면 그냥 넘긴다.
function onEditorKeydown(_mode: 'markdown' | 'wysiwyg', event: KeyboardEvent) {
  if (readOnly.value) return
  // 한글을 조합하는 중의 키 입력은 입력기가 쓰고 있으므로 건드리지 않는다.
  if (event.isComposing || event.keyCode === 229) return
  if (!(event.ctrlKey || event.metaKey) || event.altKey) return

  const key = event.key.toLowerCase()

  // Ctrl+1~6은 제목 단계, Ctrl+0은 제목을 떼고 본문으로 되돌린다.
  if (!event.shiftKey && /^[1-6]$/.test(key)) {
    event.preventDefault()
    editor?.exec('heading', { level: Number(key) })
    return
  }
  if (!event.shiftKey && key === '0') {
    event.preventDefault()
    clearHeading()
    return
  }
  if (key === 'k') {
    event.preventDefault()
    if (event.shiftKey) insertWikiLink()
    else insertLink()
    return
  }
  if (key === 'e' && !event.shiftKey) {
    event.preventDefault()
    editor?.exec('code')
  }
}

// Ctrl+0: 제목을 떼고 본문으로 되돌린다.
// 마크다운 모드에서는 편집기의 heading 명령이 줄 앞에 공백 하나를 남기므로,
// 고른 줄들의 제목 기호만 직접 떼어 낸다.
function clearHeading() {
  if (!editor) return
  if (!editor.isMarkdownMode()) {
    editor.exec('heading', { level: 0 })
    return
  }
  // 마크다운 모드의 선택 위치는 [[시작 줄, 칸], [끝 줄, 칸]] 형태다.
  const [from, to] = editor.getSelection() as [[number, number], [number, number]]
  const lines = editor.getMarkdown().split('\n')
  const firstLine = from[0]
  const lastLine = to[0]
  const stripped = lines
    .slice(firstLine - 1, lastLine)
    .map(line => line.replace(/^\s{0,3}#{1,6}\s*/, ''))
  if (!stripped.length) return
  editor.setSelection([firstLine, 1], [lastLine, (lines[lastLine - 1] || '').length + 1])
  editor.replaceSelection(stripped.join('\n'))
}

// Ctrl+K: 고른 글자를 링크 글자로 삼고 주소만 물어본다.
function insertLink() {
  const text = editor?.getSelectedText() || ''
  const url = prompt('링크 주소', 'https://')
  if (url === null) return
  const trimmed = url.trim()
  if (!trimmed) return
  editor?.exec('addLink', { linkUrl: trimmed, linkText: text || trimmed })
}

// Ctrl+Shift+K: 이 앱의 문서 링크는 [[제목]] 형식이라 고른 글자를 감싸 준다.
function insertWikiLink() {
  const selected = (editor?.getSelectedText() || '').trim()
  editor?.replaceSelection(`[[${selected}]]`)
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
  // 편집기에 없는 단축키를 채운다. 마크다운·리치 텍스트 두 모드에서 모두 온다.
  editor.on('keydown', onEditorKeydown)
  applyEditable()
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
    applyEditable()
  })
})

/* ---------- 목록/문서 ---------- */

// replaceInList는 목록 안의 문서 하나만 갈아 끼운다.
// 목록 전체를 다시 받으면 편집 흐름이 끊기므로 저장 뒤에는 이 방법을 쓴다.
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
    reportError(e)
  }
}

async function selectDocument(id: string) {
  hidePreview()
  if (id === selectedId.value) return
  await flushSave()
  try {
    const doc = await DocumentService.Get(id)
    if (!doc) return
    // 읽기 전용·잠금 상태가 그 사이 바뀌었을 수 있으므로 목록도 함께 갱신한다.
    replaceInList(doc)
    selectedId.value = doc.id
    titleInput.value = doc.title
    blankScreen.value = false
    unlockPassword.value = ''
    unlockError.value = ''
    // 잠긴 문서는 본문이 비어 오므로 편집기 대신 암호 입력 화면이 뜬다.
    applyType(typeInfo(doc.type).id, doc.content || '')
  } catch (e) {
    reportError(e)
  }
}

function scheduleSave() {
  // 읽기 전용과 아직 열지 않은 잠긴 문서는 저장 자체를 시작하지 않는다.
  if (readOnly.value || needsUnlock.value) return
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

  // 저장 뒤 입력칸을 되돌려도 되는지 판단하려고 보낸 제목을 기억해 둔다.
  const sentTitle = titleInput.value
  const payload: Document = {
    ...current,
    title: sentTitle.trim() || current.title,
    type: docType.value,
    content: currentContent(),
  }
  try {
    const saved = await DocumentService.Save(payload)
    if (!saved) return
    replaceInList(saved)
    // 제목을 지우고 새 제목을 고민하는 중이거나 저장하는 사이에 더 고쳤다면 입력칸을 두고 본다.
    // 원래 제목이 저절로 다시 채워지면 편집을 방해한다.
    if (sentTitle.trim() && titleInput.value === sentTitle) titleInput.value = saved.title
  } catch (e) {
    reportError(e)
  }
}

async function createDocument(title = '', folderId = '') {
  hidePreview()
  await flushSave()
  try {
    // 새 문서는 지금 열어 둔 문서와 같은 형식으로 시작한다.
    const doc = await DocumentService.Create(title, folderId, docType.value)
    if (!doc) return
    expandFolder(folderId)
    await refreshDocuments()
    selectedId.value = doc.id
    titleInput.value = doc.title
    blankScreen.value = false
    applyType(typeInfo(doc.type).id, '')
  } catch (e) {
    reportError(e)
  }
}

/* ---------- 외부 파일 가져오기 / 내보내기 ---------- */

// 파일 선택 창에서 문서로 가져올 텍스트 계열 확장자를 안내한다.
const IMPORT_FILTER = { DisplayName: '문서 파일', Pattern: '*.md;*.markdown;*.txt' }

// 다이얼로그 취소는 오류가 아니라 사용자가 그만둔 것이므로 조용히 무시한다.
function isDialogCancel(e: unknown): boolean {
  return e instanceof Error && /cancelled by user|cancelled/i.test(e.message)
}

// 가져오기 버튼: 파일 선택 창을 열어 선택한 파일을 문서로 만든다.
// 지금 보고 있는 폴더 안으로 가져오면 목록이 그대로 유지된다.
async function importFile() {
  await flushSave()
  try {
    const path = await Dialogs.OpenFile({
      Title: '문서로 가져올 파일 선택',
      Filters: [IMPORT_FILTER],
      AllowsMultipleSelection: false,
    })
    if (!path) return
    const targetFolder = selectedDocument.value?.folderId || ''
    const doc = await DocumentService.Import(path, targetFolder)
    if (!doc) return
    expandFolder(targetFolder)
    await refreshDocuments()
    selectedId.value = doc.id
    titleInput.value = doc.title
    applyType(typeInfo(doc.type).id, doc.content || '')
  } catch (e) {
    if (!isDialogCancel(e)) reportError(e)
  }
}

// 내보내기 버튼: 저장 창을 열어 지금 문서의 본문을 파일로 쓴다.
async function exportFile() {
  const current = selectedDocument.value
  if (!current) return
  try {
    const path = await Dialogs.SaveFile({
      Title: '문서를 내보낼 위치 선택',
      Filename: `${current.title}.md`,
      Filters: [IMPORT_FILTER],
    })
    if (!path) return
    await DocumentService.Export(path, current)
  } catch (e) {
    if (!isDialogCancel(e)) reportError(e)
  }
}

// 파일을 창에 끌어다 놓았을 때 백엔드가 가져온 뒤 목록을 다시 그린다.
function onFilesImported() {
  refreshDocuments()
}

async function deleteDocument(doc: Document) {
  hidePreview()
  // 잠긴 문서는 본문을 확인할 수 없으므로 지우기 전에 한 번 더 알려 준다.
  const warning = doc.locked ? '\n암호로 잠긴 문서입니다. 지우면 되돌릴 수 없습니다.' : ''
  if (!confirm(`문서 "${doc.title}" 을(를) 삭제할까요?${warning}`)) return
  try {
    await DocumentService.Delete(doc.id)
    if (selectedId.value === doc.id) {
      selectedId.value = ''
      titleInput.value = ''
      applyType(docType.value, '')
    }
    await refreshDocuments()
  } catch (e) {
    reportError(e)
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
    reportError(e)
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
    folderId: folderOf(current),
    // 읽기 전용과 암호 잠금은 문서마다 다르므로 정보에서도 한눈에 확인한다.
    protection: current.readOnly && current.locked
      ? '읽기 전용 · 암호 잠금'
      : current.readOnly
        ? '읽기 전용'
        : current.locked
          ? '암호 잠금'
          : '없음',
    hint: current.hint || '',
    size: formatBytes(new TextEncoder().encode(content).length),
    characters: [...content].length,
    createdAt: formatDate(current.createdAt),
    updatedAt: formatDate(current.updatedAt),
  }
})

// 폴더 선택 상자에 트리 모양을 그대로 보여 주기 위해 깊이만큼 들여쓴다.
const folderOptions = computed(() => {
  const out: Array<{ id: string; label: string }> = []
  const walk = (parentId: string, depth: number) => {
    for (const folder of childFolders(parentId)) {
      out.push({ id: folder.id, label: `${'   '.repeat(depth)}${depth ? '└ ' : ''}${folder.name}` })
      walk(folder.id, depth + 1)
    }
  }
  walk('', 0)
  return out
})

async function openInfo() {
  if (!selectedId.value) return
  infoOpen.value = true
  backlinks.value = []
  try {
    backlinks.value = (await DocumentService.Backlinks(selectedId.value)) || []
  } catch (e) {
    reportError(e)
  }
}

function closeInfo() {
  infoOpen.value = false
}

async function openBacklink(doc: Document) {
  closeInfo()
  await selectDocument(doc.id)
}

// 모달(정보·단축키·잠금·오류)과 형식 드롭다운은 Esc로 닫는다.
// 한글을 조합하는 중의 Esc는 입력기가 조합을 취소하려는 것이므로 넘긴다.
function onKeydown(event: KeyboardEvent) {
  if (event.isComposing || event.keyCode === 229) return
  if (event.key !== 'Escape') return
  if (errorMessage.value) closeError()
  else if (shortcutsOpen.value) shortcutsOpen.value = false
  else if (lockOpen.value) closeLock()
  else if (infoOpen.value) closeInfo()
  typeMenuOpen.value = false
}

/* ---------- 목록 호버 미리보기 ---------- */

// 사이드바 항목에 마우스를 올리면 본문 앞부분을 팝오버로 보여 준다.
// 사이드바는 overflow: hidden이라 팝오버는 body로 내보내 띄운다.
const PREVIEW_LIMIT = 240
const PREVIEW_MAX_HEIGHT = 220

const hoverDocument = ref<Document | null>(null)
const previewPosition = ref({ top: 0, left: 0 })

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

// 줄 어디에 마우스를 올려도 미리보기가 뜨도록 줄 전체에서 받는다.
// 문서가 아닌 줄(폴더·빈 안내)로 옮겼으면 닫는다.
function onRowEnter(row: TreeRow, event: MouseEvent) {
  if (row.kind === 'document' && row.document) showPreview(row.document, event)
  else hidePreview()
}

// 기다리지 않고 마우스를 올린 즉시 띄운다.
function showPreview(doc: Document, event: MouseEvent) {
  const item = event.currentTarget as HTMLElement | null
  if (!item || dragged.value) return
  const rect = item.getBoundingClientRect()
  // 아래쪽 항목에서도 팝오버가 화면 밖으로 나가지 않도록 위로 밀어 올린다.
  const top = Math.min(rect.top, window.innerHeight - PREVIEW_MAX_HEIGHT - 16)
  previewPosition.value = { top: Math.max(12, top), left: rect.right + 10 }
  hoverDocument.value = doc
}

function hidePreview() {
  hoverDocument.value = null
}

onMounted(async () => {
  createEditor('')
  window.addEventListener('keydown', onKeydown)
  // 창으로 끌어다 놓은 파일이 문서로 가져와지면 목록을 다시 그린다.
  Events.On('document:files-imported', onFilesImported)
  await refreshTree()
  // 화면을 비워 둔 채 나갔다면 돌아와도 비워 둔다.
  if (!blankScreen.value && documents.value.length) await selectDocument(documents.value[0].id)
})

onBeforeUnmount(() => {
  if (saveTimer) clearTimeout(saveTimer)
  window.removeEventListener('keydown', onKeydown)
  Events.Off('document:files-imported')
  hidePreview()
  destroyEditor()
})
</script>

<template>
  <div class="document-layout" :style="layoutColumns" data-file-drop-target>
    <aside v-if="sidebarVisible" class="document-sidebar">
      <div class="sidebar-header">
        <h1>문서</h1>
        <div class="header-buttons">
          <button class="icon-btn" title="새 폴더" aria-label="새 폴더" @click="createFolder()">
            <FolderPlusIcon />
          </button>
          <button class="icon-btn" title="파일 가져오기" aria-label="파일 가져오기" @click="importFile">⇩</button>
          <button class="icon-btn" title="새 문서" @click="createDocument()">+</button>
        </div>
      </div>
      <input v-model="filter" class="search" type="search" placeholder="제목·본문 검색" />

      <div class="sidebar-body" @mouseleave="hidePreview">
        <!-- 검색 중에는 폴더 구분 없이 결과만 보여 준다. -->
        <ul v-if="isSearching" class="tree">
          <li
            v-for="doc in searchResults"
            :key="doc.id"
            class="tree-row document"
            :class="{ active: doc.id === selectedId }"
            @click="selectDocument(doc.id)"
            @mouseenter="showPreview(doc, $event)"
          >
            <div class="row-main">
              <span class="doc-type" :title="typeInfo(doc.type).label">{{ typeInfo(doc.type).icon }}</span>
              <span class="row-name">{{ doc.title }}</span>
              <span v-if="doc.locked" class="row-mark" title="암호로 잠긴 문서"><LockIcon :size="13" /></span>
              <span v-else-if="doc.readOnly" class="row-mark" title="읽기 전용 문서"><NoEditIcon :size="13" /></span>
              <button class="icon-btn sm danger row-action" title="문서 삭제" @click.stop="deleteDocument(doc)">✕</button>
            </div>
            <span class="row-date">{{ formatDate(doc.updatedAt) }}</span>
          </li>
          <li v-if="!searchResults.length" class="empty">검색 결과가 없습니다</li>
        </ul>

        <template v-else>
          <ul class="tree">
            <li
              v-for="row in treeRows"
              :key="row.key"
              class="tree-row"
              :class="[
                row.kind,
                dropHint && dropHint.key === row.key ? `drop-${dropHint.position}` : '',
                row.kind === 'document' && row.document!.id === selectedId ? 'active' : '',
                dragged && ((row.kind === 'document' && dragged.id === row.document!.id) || (row.kind === 'folder' && dragged.id === row.folder!.id)) ? 'dragging' : '',
              ]"
              :style="{ paddingLeft: `${10 + row.depth * 14}px` }"
              :draggable="row.kind !== 'empty'"
              @mouseenter="onRowEnter(row, $event)"
              @pointerdown="onRowPointerDown(row, $event)"
              @pointerup="onRowPointerUp"
              @dragstart="startDrag(row, $event)"
              @dragend="endDrag($event)"
              @dragover.prevent="onRowDragOver(row, $event)"
              @drop.prevent="onRowDrop(row)"
            >
              <!-- 폴더 줄 -->
              <template v-if="row.kind === 'folder'">
                <div class="row-main">
                  <span class="folder-caret" aria-hidden="true">{{ isCollapsed(row.folder!.id) ? '▸' : '▾' }}</span>
                  <span
                    class="row-name folder-name"
                    title="두 번 눌러 이름 변경"
                    @dblclick.stop="renameFolder(row.folder!)"
                  >{{ row.folder!.name }}</span>
                  <span class="folder-count">{{ folderCount(row.folder!.id) }}</span>
                  <button class="icon-btn sm row-action" title="이 폴더에 새 문서" @click.stop="createDocument('', row.folder!.id)">+</button>
                  <button class="icon-btn sm row-action" title="하위 폴더 추가" aria-label="하위 폴더 추가" @click.stop="createFolder(row.folder!.id)">
                    <FolderPlusIcon :size="17" />
                  </button>
                  <button class="icon-btn sm danger row-action" title="폴더 삭제" @click.stop="deleteFolder(row.folder!)">✕</button>
                </div>
              </template>

              <!-- 문서 줄 -->
              <template v-else-if="row.kind === 'document'">
                <div class="row-main">
                  <span class="doc-type" :title="typeInfo(row.document!.type).label">{{ typeInfo(row.document!.type).icon }}</span>
                  <span class="row-name">{{ row.document!.title }}</span>
                  <span v-if="row.document!.locked" class="row-mark" title="암호로 잠긴 문서"><LockIcon :size="13" /></span>
                  <span v-else-if="row.document!.readOnly" class="row-mark" title="읽기 전용 문서"><NoEditIcon :size="13" /></span>
                  <button class="icon-btn sm danger row-action" title="문서 삭제" @click.stop="deleteDocument(row.document!)">✕</button>
                </div>
                <span class="row-date">{{ formatDate(row.document!.updatedAt) }}</span>
              </template>

              <!-- 빈 폴더 안내 -->
              <span v-else class="empty sm">비어 있음</span>
            </li>
          </ul>

          <!-- 남는 공간은 최상위로 빼는 자리로 쓴다. -->
          <div
            class="root-drop"
            :class="{ 'drop-target': dropHint && dropHint.key === 'root' }"
            @mouseenter="hidePreview"
            @dragover.prevent="onRootDragOver"
            @drop.prevent="onRootDrop"
          >
            <span v-if="!treeRows.length" class="empty">문서가 없습니다</span>
          </div>
        </template>
      </div>
    </aside>

    <!-- 사이드바가 잘라내지 않도록 팝오버는 body에 붙여 띄운다. -->
    <Teleport to="body">
      <div
        v-if="hoverDocument"
        class="doc-preview"
        :style="{ transform: `translate3d(${previewPosition.left}px, ${previewPosition.top}px, 0)` }"
      >
        <strong class="preview-title">{{ hoverDocument.title }}</strong>
        <span class="preview-date">{{ typeInfo(hoverDocument.type).label }} · {{ formatDate(hoverDocument.updatedAt) }}</span>
        <p v-if="hoverDocument.locked && !hoverDocument.unlocked" class="preview-body empty">암호로 잠긴 문서입니다</p>
        <p v-else-if="previewText(hoverDocument.content)" class="preview-body">{{ previewText(hoverDocument.content) }}</p>
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
            :disabled="!selectedId || needsUnlock"
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
          :placeholder="selectedDocument?.title || '문서 제목'"
          :disabled="!selectedId"
          :readonly="readOnly || needsUnlock"
          @input="onTitleInput"
        />
        <button
          v-if="docType === 'text'"
          type="button"
          class="header-icon-button"
          :class="{ active: wrapText }"
          :aria-pressed="wrapText"
          :title="`자동 줄바꿈 ${wrapText ? '끄기' : '켜기'}`"
          aria-label="자동 줄바꿈"
          @click="toggleWrap"
        >↵</button>
        <button
          class="header-icon-button"
          type="button"
          :class="{ active: readOnly }"
          :aria-pressed="readOnly"
          :disabled="!selectedId || needsUnlock"
          :title="readOnly ? '읽기 전용 끄기 (수정 허용)' : '읽기 전용 켜기 (수정 금지)'"
          aria-label="읽기 전용"
          @click="toggleReadOnly"
        ><NoEditIcon /></button>
        <button
          class="header-icon-button"
          type="button"
          :class="{ active: locked }"
          :disabled="!selectedId || needsUnlock"
          :title="locked ? '문서 잠금 관리 (다시 잠그기·잠금 없애기)' : '암호로 잠그기'"
          aria-label="암호 잠금"
          @click="openLock"
        ><LockIcon :open="!locked" /></button>
        <button
          class="header-icon-button"
          type="button"
          :disabled="!selectedId"
          title="파일로 내보내기"
          aria-label="파일로 내보내기"
          @click="exportFile"
        >⇪</button>
        <button
          class="header-icon-button"
          type="button"
          :disabled="!selectedId"
          title="문서를 닫고 화면 비우기"
          aria-label="화면 비우기"
          @click="clearScreen"
        ><BlankScreenIcon /></button>
        <button
          class="header-icon-button"
          type="button"
          title="단축키 보기"
          aria-label="단축키 보기"
          @click="shortcutsOpen = true"
        ><KeyboardIcon /></button>
        <button
          class="header-icon-button"
          type="button"
          :disabled="!selectedId || needsUnlock"
          title="문서 정보"
          aria-label="문서 정보"
          @click="openInfo"
        >ⓘ</button>
      </header>

      <div v-show="!selectedId" class="state">
        <!-- 스스로 화면을 비운 경우에는 안내를 줄여 생각을 방해하지 않는다. -->
        <template v-if="blankScreen">
          <p>화면을 비워 두었습니다.</p>
          <p class="hint">생각이 정리되면 왼쪽에서 문서를 고르거나 새 문서를 만드세요.</p>
        </template>
        <template v-else>
          <p>왼쪽에서 문서를 고르거나 새 문서를 만드세요.</p>
          <p class="hint">본문에 <code>[[다른 문서 제목]]</code> 을 적으면 미리보기에서 눌러 이동할 수 있습니다.</p>
        </template>
      </div>

      <!-- 잠긴 문서는 편집기 대신 암호 입력 화면을 띄운다. -->
      <div v-if="selectedId && needsUnlock" class="lock-gate">
        <form class="lock-card" @submit.prevent="submitUnlock">
          <LockIcon class="lock-mark" :size="34" />
          <h2>암호로 잠긴 문서</h2>
          <p class="hint">암호를 넣으면 이 문서를 볼 수 있습니다. 앱을 다시 켜면 다시 물어봅니다.</p>
          <p v-if="selectedDocument?.hint" class="lock-hint">힌트: {{ selectedDocument.hint }}</p>
          <div class="lock-form">
            <input
              ref="unlockInput"
              v-model="unlockPassword"
              class="lock-input"
              type="password"
              autocomplete="off"
              placeholder="암호"
            />
            <button class="modal-button primary" type="submit" :disabled="unlockBusy">열기</button>
          </div>
          <p v-if="unlockError" class="error-text">{{ unlockError }}</p>
        </form>
      </div>

      <!-- 에디터는 한 번만 만들고 문서를 바꿔 끼우므로 v-if 대신 v-show로 감춘다. -->
      <div v-show="selectedId && !needsUnlock" class="editor-wrap" :class="{ readonly: readOnly }">
        <p v-if="readOnly" class="readonly-banner">읽기 전용 문서입니다. 고치려면 머리글의 읽기 전용 버튼을 눌러 끄세요.</p>
        <div v-show="docType !== 'text'" ref="editorElement" class="editor-host" @click="onEditorClick"></div>
        <textarea
          v-show="docType === 'text'"
          v-model="plainText"
          class="plain-editor"
          :class="{ wrap: wrapText }"
          spellcheck="false"
          :readonly="readOnly"
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
                @change="moveDocument(selectedId, ($event.target as HTMLSelectElement).value, 0)"
              >
                <option value="">폴더 없음</option>
                <option v-for="option in folderOptions" :key="option.id" :value="option.id">{{ option.label }}</option>
              </select>
            </dd>
            <dt>보호</dt>
            <dd>
              {{ documentInfo.protection }}
              <span v-if="documentInfo.hint" class="muted">(힌트: {{ documentInfo.hint }})</span>
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

    <!-- 암호 잠금: 걸기 전에는 새 암호를, 걸린 뒤에는 다시 잠그기와 없애기를 다룬다. -->
    <Teleport to="body">
      <div v-if="lockOpen && selectedDocument" class="modal-overlay" @click.self="closeLock">
        <div class="modal" role="dialog" aria-modal="true" aria-label="문서 암호 잠금">
          <header class="modal-header">
            <h2>{{ locked ? '문서 잠금 관리' : '문서에 암호 걸기' }}</h2>
            <button class="icon-btn" type="button" title="닫기" @click="closeLock">✕</button>
          </header>

          <template v-if="!locked">
            <p class="hint">
              암호를 걸면 본문이 암호화되어 저장됩니다. 암호는 어디에도 저장하지 않으므로
              잊으면 본문을 되살릴 수 없습니다.
            </p>
            <form class="lock-fields" @submit.prevent="submitLock">
              <label>
                <span>암호</span>
                <input v-model="lockPassword" type="password" autocomplete="new-password" />
              </label>
              <label>
                <span>암호 확인</span>
                <input v-model="lockConfirm" type="password" autocomplete="new-password" />
              </label>
              <label>
                <span>힌트 (선택)</span>
                <input v-model="lockHint" type="text" placeholder="암호를 떠올릴 단서" />
              </label>
              <p v-if="lockError" class="error-text">{{ lockError }}</p>
              <div class="modal-actions">
                <button class="modal-button" type="button" @click="closeLock">취소</button>
                <button class="modal-button primary" type="submit" :disabled="lockBusy">잠그기</button>
              </div>
            </form>
          </template>

          <template v-else>
            <p class="hint">이 문서는 암호로 잠겨 있고, 지금은 열려 있어 편집할 수 있습니다.</p>
            <div class="modal-actions">
              <button class="modal-button primary" type="button" @click="relock">지금 다시 잠그기</button>
            </div>
            <form class="lock-fields spaced" @submit.prevent="removeLock">
              <label>
                <span>암호를 넣으면 잠금을 없앨 수 있습니다</span>
                <input v-model="lockPassword" type="password" autocomplete="off" />
              </label>
              <p v-if="lockError" class="error-text">{{ lockError }}</p>
              <div class="modal-actions">
                <button class="modal-button" type="button" @click="closeLock">취소</button>
                <button class="modal-button danger" type="submit" :disabled="lockBusy">잠금 없애기</button>
              </div>
            </form>
          </template>
        </div>
      </div>
    </Teleport>

    <!-- 단축키 도움말. 편집기에 원래 있던 것과 이 앱이 더한 것을 함께 적는다. -->
    <Teleport to="body">
      <div v-if="shortcutsOpen" class="modal-overlay" @click.self="shortcutsOpen = false">
        <div class="modal wide" role="dialog" aria-modal="true" aria-label="단축키">
          <header class="modal-header">
            <h2>단축키</h2>
            <button class="icon-btn" type="button" title="닫기" @click="shortcutsOpen = false">✕</button>
          </header>
          <p class="hint">
            리치 텍스트와 마크다운 문서에서 쓸 수 있습니다. 일반 텍스트 문서와
            읽기 전용 문서에는 적용되지 않습니다.
          </p>
          <section v-for="group in SHORTCUT_GROUPS" :key="group.title" class="shortcut-group">
            <h3>{{ group.title }}</h3>
            <dl class="shortcut-list">
              <template v-for="item in group.items" :key="item.keys">
                <dt><kbd>{{ item.keys }}</kbd></dt>
                <dd>{{ item.label }}</dd>
              </template>
            </dl>
          </section>
        </div>
      </div>
    </Teleport>

    <!-- 자동 저장은 조용히 흘러가므로 실패는 모달로 분명히 알린다. -->
    <Teleport to="body">
      <div v-if="errorMessage" class="modal-overlay" @click.self="closeError">
        <div class="modal error" role="alertdialog" aria-modal="true" aria-label="오류">
          <header class="modal-header">
            <h2>문제가 생겼습니다</h2>
            <button class="icon-btn" type="button" title="닫기" @click="closeError">✕</button>
          </header>
          <p class="error-text">{{ errorMessage }}</p>
          <p class="hint">계속 실패하면 편집한 내용을 다른 곳에 복사해 두세요.</p>
          <div class="modal-actions">
            <button class="modal-button" type="button" @click="closeError">닫기</button>
          </div>
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
.sidebar-body { display: flex; flex-direction: column; flex: 1; min-height: 0; overflow: auto; }

/* 트리는 폴더와 문서를 한 목록에 펼쳐 그리고 깊이는 들여쓰기로만 나타낸다.
   놓을 자리 표시는 가상 요소로 그려 줄 높이가 흔들리지 않게 한다. */
.tree { list-style: none; margin: 0; padding: 0; }
.tree-row {
  position: relative;
  padding: 7px 12px 7px 10px;
  cursor: pointer;
}
.tree-row:hover { background: var(--panel-2); }
.tree-row.active { background: var(--panel-2); box-shadow: inset 3px 0 0 var(--accent); }
.tree-row.dragging { opacity: 0.45; }
.tree-row.drop-before::after,
.tree-row.drop-after::after {
  content: '';
  position: absolute;
  left: 0;
  right: 0;
  height: 2px;
  background: var(--accent);
}
.tree-row.drop-before::after { top: 0; }
.tree-row.drop-after::after { bottom: 0; }
.tree-row.drop-inside { background: rgba(79, 124, 255, 0.16); box-shadow: inset 0 0 0 1px var(--accent); }

.row-main { display: flex; align-items: center; gap: 6px; min-width: 0; }
.row-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-weight: 600;
  font-size: 13px;
}
.row-date { display: block; margin-top: 2px; margin-left: 21px; color: var(--muted); font-size: 11px; }
.row-action { display: none; flex: 0 0 auto; }
.tree-row:hover .row-action { display: inline-flex; }

.tree-row.folder .row-name { color: var(--muted); font-size: 12px; }
.tree-row.folder:hover .row-name { color: var(--text); }
.folder-caret { flex: 0 0 auto; width: 10px; color: var(--muted); font-size: 10px; }
.folder-count { flex: 0 0 auto; color: var(--muted); font-size: 11px; }
.tree-row.empty { cursor: default; padding-top: 4px; padding-bottom: 6px; }

/* 목록의 형식 아이콘은 눈으로 구분만 하는 용도라 누를 수 없다. */
.doc-type {
  flex: 0 0 auto;
  width: 15px;
  text-align: center;
  color: var(--muted);
  font-size: 11px;
  font-weight: 700;
}

/* 목록의 잠금·읽기 전용 표시. 무엇인지 알아볼 정도로만 작게 둔다. */
.row-mark { display: inline-flex; flex: 0 0 auto; align-items: center; color: var(--muted); }

.root-drop { flex: 1; min-height: 48px; }
.root-drop.drop-target { background: rgba(79, 124, 255, 0.12); }
.empty { color: var(--muted); font-style: italic; font-size: 12px; padding: 8px 16px; }
.empty.sm { padding: 0; font-size: 11px; }

/* 목록 호버 팝오버. body로 옮겨 그리므로 위치는 인라인 스타일이 정한다. */
.doc-preview {
  position: fixed;
  top: 0;
  left: 0;
  /* 위아래 다른 항목으로 옮길 때 팝오버가 미끄러지듯 따라온다. */
  transition: transform 150ms ease-out;
  will-change: transform;
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
.preview-body.empty { font-style: italic; padding: 0; margin-top: 8px; }

/* 아이콘만 든 버튼은 안쪽 그림이 알아볼 만한 크기로 남게 넉넉히 잡는다. */
.icon-btn {
  background: transparent;
  border: 1px solid var(--border);
  color: var(--text);
  border-radius: 4px;
  width: 28px;
  height: 28px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
}
.icon-btn.sm { width: 24px; height: 24px; font-size: 12px; }
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

/* 머리글 오른쪽 아이콘 버튼(자동 줄바꿈, 문서 정보) */
.header-icon-button {
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
  font-size: 16px;
  line-height: 1;
}
.header-icon-button:hover:not(:disabled) { color: var(--accent); border-color: var(--border); background: var(--panel-2); }
.header-icon-button:disabled { opacity: 0.4; cursor: default; }
.header-icon-button.active { color: var(--accent); border-color: var(--accent); }

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

/* 읽기 전용은 편집기의 도구 모음까지 흐리게 해 못 누른다는 것을 분명히 한다. */
.readonly-banner {
  margin: 0;
  padding: 7px 16px;
  border-bottom: 1px solid var(--border);
  background: var(--panel-2);
  color: var(--muted);
  font-size: 12px;
}
.editor-wrap.readonly :deep(.toastui-editor-toolbar) {
  opacity: 0.45;
  pointer-events: none;
}

/* 잠긴 문서의 암호 입력 화면. 편집기 자리를 그대로 차지한다. */
.lock-gate {
  display: flex;
  flex: 1;
  align-items: center;
  justify-content: center;
  min-height: 0;
  padding: 24px;
}
.lock-card {
  width: min(360px, 100%);
  padding: 22px 24px 20px;
  border: 1px solid var(--border);
  border-radius: 10px;
  background: var(--panel);
  text-align: center;
}
.lock-mark { display: block; margin: 0 auto 10px; color: var(--accent); }
.lock-card h2 { margin: 0 0 8px; font-size: 15px; }
.lock-hint {
  margin: 10px 0 0;
  padding: 7px 10px;
  border-radius: 6px;
  background: var(--panel-2);
  color: var(--muted);
  font-size: 12px;
  overflow-wrap: anywhere;
}
.lock-form { display: flex; gap: 6px; margin-top: 14px; }
.lock-input {
  flex: 1;
  min-width: 0;
  padding: 7px 9px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--panel-2);
  color: var(--text);
  font: inherit;
  font-size: 13px;
}
.lock-input:focus { border-color: var(--accent); outline: none; }

/* 잠금 모달의 입력칸 묶음 */
.lock-fields { display: flex; flex-direction: column; gap: 10px; }
.lock-fields.spaced { margin-top: 16px; padding-top: 14px; border-top: 1px solid var(--border); }
.lock-fields label { display: flex; flex-direction: column; gap: 4px; }
.lock-fields label span { color: var(--muted); font-size: 12px; }
.lock-fields input {
  padding: 7px 9px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--panel-2);
  color: var(--text);
  font: inherit;
  font-size: 13px;
}
.lock-fields input:focus { border-color: var(--accent); outline: none; }

/* 단축키 목록 */
.shortcut-group { margin-top: 16px; }
.shortcut-group h3 {
  margin: 0 0 8px;
  color: var(--muted);
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.6px;
}
.shortcut-list {
  display: grid;
  grid-template-columns: auto minmax(0, 1fr);
  gap: 6px 14px;
  margin: 0;
  font-size: 13px;
}
.shortcut-list dt { white-space: nowrap; }
.shortcut-list dd { margin: 0; min-width: 0; color: var(--muted); }
kbd {
  display: inline-block;
  padding: 2px 6px;
  border: 1px solid var(--border);
  border-bottom-width: 2px;
  border-radius: 4px;
  background: var(--panel-2);
  color: var(--text);
  font-family: "D2Coding", "Courier New", monospace;
  font-size: 11px;
}

/* 모달(문서 정보·잠금·단축키·오류) */
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
.modal.error { border-color: var(--danger); }
.modal.wide { width: min(520px, 100%); }
.modal-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 14px;
}
.modal-header h2 { margin: 0; font-size: 15px; }
.error-text {
  margin: 0 0 10px;
  padding: 10px 12px;
  border-radius: 6px;
  background: rgba(255, 90, 106, 0.12);
  color: var(--danger);
  font-size: 13px;
  overflow-wrap: anywhere;
}
.modal-actions { display: flex; gap: 8px; justify-content: flex-end; margin-top: 16px; }
.modal-button {
  padding: 6px 14px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--panel-2);
  color: var(--text);
  font-size: 13px;
}
.modal-button:hover:not(:disabled) { border-color: var(--accent); color: var(--accent); }
.modal-button:disabled { opacity: 0.5; cursor: default; }
.modal-button.primary { border-color: var(--accent); color: var(--accent); }
.modal-button.danger { color: var(--danger); }
.modal-button.danger:hover:not(:disabled) { border-color: var(--danger); color: var(--danger); }

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
