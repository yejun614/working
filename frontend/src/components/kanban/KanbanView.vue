<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { Service as KanbanService } from '../../../bindings/working/internal/modules/kanban'
import type { Board, Card, Column } from '../../../bindings/working/internal/modules/kanban/types/models'

const boards = ref<Board[]>([])
const selectedBoardId = ref('')
const columns = ref<Column[]>([])
const cards = ref<Card[]>([])
const archivedCards = ref<Card[]>([])
const showArchived = ref(false)
const loading = ref(false)
const error = ref('')
const draggedCard = ref<Card | null>(null)
const draggedColumn = ref<Column | null>(null)
const editingCard = ref<Card | null>(null)
const form = ref({ title: '', description: '', dueDate: '', priority: '', labels: '', assignee: '', checklist: '', attachments: '' })

const selectedBoard = computed(() => boards.value.find(b => b.id === selectedBoardId.value) || null)
const cardsInColumn = (columnID: string) => cards.value.filter(c => c.columnId === columnID).sort((a, b) => a.position - b.position)

async function refresh() {
  if (!selectedBoardId.value) return
  loading.value = true; error.value = ''
  try {
    const view = await KanbanService.BoardGet(selectedBoardId.value)
    columns.value = view?.columns || []
    cards.value = view?.cards || []
    archivedCards.value = await KanbanService.ArchivedCardList(selectedBoardId.value) || []
  } catch (e) { error.value = (e as Error).message } finally { loading.value = false }
}

async function loadBoards() {
  try {
    boards.value = await KanbanService.BoardList() || []
    if (!selectedBoardId.value && boards.value.length) selectedBoardId.value = boards.value[0].id
    await refresh()
  } catch (e) { error.value = (e as Error).message }
}

async function createBoard() {
  const name = window.prompt('새 보드 이름')?.trim(); if (!name) return
  try { const board = await KanbanService.BoardCreate(name); if (board) { boards.value.push(board); selectedBoardId.value = board.id; await refresh() } } catch (e) { error.value = (e as Error).message }
}

async function renameBoard() {
  if (!selectedBoard.value) return
  const name = window.prompt('보드 이름', selectedBoard.value.name)?.trim(); if (!name || name === selectedBoard.value.name) return
  try { await KanbanService.BoardUpdate({ ...selectedBoard.value, name }); await loadBoards() } catch (e) { error.value = (e as Error).message }
}

async function deleteBoard() {
  if (!selectedBoard.value || !window.confirm(`보드 "${selectedBoard.value.name}"과 모든 카드를 삭제할까요?`)) return
  try { await KanbanService.BoardDelete(selectedBoard.value.id); selectedBoardId.value = ''; await loadBoards() } catch (e) { error.value = (e as Error).message }
}

async function createColumn() {
  if (!selectedBoardId.value) return
  const name = window.prompt('새 컬럼 이름')?.trim(); if (!name) return
  try { await KanbanService.ColumnCreate({ id: '', boardId: selectedBoardId.value, name, position: columns.value.length, createdAt: '' }); await refresh() } catch (e) { error.value = (e as Error).message }
}

async function renameColumn(column: Column) {
  const name = window.prompt('컬럼 이름', column.name)?.trim(); if (!name || name === column.name) return
  try { await KanbanService.ColumnUpdate({ ...column, name }); await refresh() } catch (e) { error.value = (e as Error).message }
}

async function deleteColumn(column: Column) {
  if (!window.confirm(`컬럼 "${column.name}"을 삭제할까요?`)) return
  try { await KanbanService.ColumnDelete(column.id); await refresh() } catch (e) { error.value = (e as Error).message }
}

function openNewCard(columnID: string) {
  editingCard.value = null
  form.value = { title: '', description: '', dueDate: '', priority: '', labels: '', assignee: '', checklist: '', attachments: '' }
  form.value.priority = ''
  editingColumnID.value = columnID
}
const editingColumnID = ref('')
function openEditCard(card: Card) {
  editingCard.value = card
  editingColumnID.value = card.columnId
  form.value = {
    title: card.title, description: card.description || '', dueDate: card.dueDate || '', priority: card.priority || '',
    labels: (card.labels || []).join(', '), assignee: card.assignee || '',
    checklist: (card.checklist || []).map(i => `${i.completed ? '[x] ' : ''}${i.title}`).join('\n'),
    attachments: (card.attachments || []).join(', '),
  }
}
function closeCard() { editingCard.value = null; editingColumnID.value = '' }

function parseChecklist(value: string) {
  return value.split('\n').map((line, i) => line.trim()).filter(Boolean).map((line, i) => {
    const completed = line.startsWith('[x]')
    return { id: editingCard.value?.checklist?.[i]?.id || `item-${i}`, title: (completed ? line.slice(3) : line).trim(), completed }
  })
}
async function saveCard() {
  if (!form.value.title.trim() || !selectedBoardId.value || !editingColumnID.value) { error.value = '카드 제목은 필수입니다'; return }
  const old = editingCard.value
  const card: Card = {
    id: old?.id || '', boardId: selectedBoardId.value, columnId: editingColumnID.value, title: form.value.title.trim(),
    description: form.value.description.trim(), dueDate: form.value.dueDate, priority: form.value.priority,
    labels: form.value.labels.split(',').map(v => v.trim()).filter(Boolean), assignee: form.value.assignee.trim(),
    checklist: parseChecklist(form.value.checklist), attachments: form.value.attachments.split(',').map(v => v.trim()).filter(Boolean),
    position: old?.position || 0, archived: old?.archived || false, createdAt: old?.createdAt || '', updatedAt: old?.updatedAt || '',
  }
  try { await KanbanService.CardSave(card); closeCard(); await refresh() } catch (e) { error.value = (e as Error).message }
}

async function archiveCard(card: Card) {
  if (!window.confirm(`카드 "${card.title}"을 아카이브할까요?`)) return
  try { await KanbanService.CardArchive(card.id); await refresh() } catch (e) { error.value = (e as Error).message }
}
async function restoreCard(card: Card) { try { await KanbanService.CardRestore(card.id); await refresh() } catch (e) { error.value = (e as Error).message } }
async function deleteArchived(card: Card) {
  if (!window.confirm(`카드 "${card.title}"을 영구 삭제할까요?`)) return
  try { await KanbanService.CardDelete(card.id); await refresh() } catch (e) { error.value = (e as Error).message }
}

const cardDropTarget = ref({ columnId: '', cardId: '' })
const columnDropTargetId = ref('')
const columnDropSide = ref<'before' | 'after'>('before')

function startDrag(card: Card) {
  draggedCard.value = card
  draggedColumn.value = null
  cardDropTarget.value = { columnId: card.columnId, cardId: card.id }
}

function startColumnDrag(column: Column) {
  draggedColumn.value = column
  draggedCard.value = null
  columnDropTargetId.value = column.id
  columnDropSide.value = 'before'
}

function previewCardDrop(column: Column, beforeCard?: Card) {
  const card = draggedCard.value
  if (!card || draggedColumn.value || beforeCard?.id === card.id) return
  const sourceColumnID = card.columnId
  const sourceCards = cardsInColumn(sourceColumnID).filter(item => item.id !== card.id)
  const targetCards = cardsInColumn(column.id).filter(item => item.id !== card.id)
  const position = beforeCard ? targetCards.findIndex(item => item.id === beforeCard.id) : targetCards.length
  const insertAt = position < 0 ? targetCards.length : position

  if (sourceColumnID === column.id && card.position === insertAt) return
  cardDropTarget.value = { columnId: column.id, cardId: beforeCard?.id || '' }
  card.columnId = column.id
  cards.value = cards.value.filter(item => item.id !== card.id)
  targetCards.splice(insertAt, 0, card)
  targetCards.forEach((item, index) => { item.position = index })
  sourceCards.forEach((item, index) => { item.position = index })
  cards.value.push(card)
}

function previewColumnDrop(targetColumn: Column, event?: DragEvent) {
  const column = draggedColumn.value
  if (!column || draggedCard.value || column.id === targetColumn.id) return
  const targetColumns = columns.value.filter(item => item.id !== column.id)
  const targetIndex = targetColumns.findIndex(item => item.id === targetColumn.id)
  if (targetIndex < 0) return

  const element = event?.currentTarget instanceof HTMLElement
    ? event.currentTarget.closest('.column')
    : null
  const midpoint = element ? element.getBoundingClientRect().left + element.getBoundingClientRect().width / 2 : 0
  const side = event && element && event.clientX >= midpoint ? 'after' : 'before'
  const insertAt = side === 'after' ? targetIndex + 1 : targetIndex
  if (column.position === insertAt && columnDropTargetId.value === targetColumn.id && columnDropSide.value === side) return

  columnDropTargetId.value = targetColumn.id
  columnDropSide.value = side
  targetColumns.splice(insertAt, 0, column)
  targetColumns.forEach((item, index) => { item.position = index })
  columns.value = targetColumns
}

function handleColumnDragOver(column: Column, event: DragEvent) {
  if (draggedColumn.value) previewColumnDrop(column, event)
  else previewCardDrop(column)
}

async function dropColumn() {
  const column = draggedColumn.value
  draggedColumn.value = null
  columnDropTargetId.value = ''
  columnDropSide.value = 'before'
  if (!column) return
  try { await KanbanService.ColumnMove(column.id, column.position); await refresh() } catch (e) { error.value = (e as Error).message; await refresh() }
}

async function dropOnCard(column: Column, card: Card) {
  if (draggedColumn.value) { await dropColumn(); return }
  await dropCard(column, card)
}

async function dropCard(column: Column, beforeCard?: Card) {
  const card = draggedCard.value
  draggedCard.value = null
  cardDropTarget.value = { columnId: '', cardId: '' }
  if (!card) return
  try { await KanbanService.CardMove(card.id, column.id, card.position); await refresh() } catch (e) { error.value = (e as Error).message; await refresh() }
}

function cancelDrag() {
  if (!draggedCard.value && !draggedColumn.value) return
  draggedCard.value = null
  draggedColumn.value = null
  cardDropTarget.value = { columnId: '', cardId: '' }
  columnDropTargetId.value = ''
  columnDropSide.value = 'before'
  void refresh()
}
function isDone(column: Column) { return column.name === '완료' }
function priorityLabel(value?: string) { return value === 'high' ? '높음' : value === 'low' ? '낮음' : value === 'medium' ? '보통' : '' }

onMounted(loadBoards)
</script>

<template>
  <div class="kanban-layout">
    <aside class="kanban-sidebar">
      <div class="side-heading"><h1>칸반</h1><button class="icon-btn" title="보드 추가" @click="createBoard">+</button></div>
      <div class="side-label">보드</div>
      <button v-for="board in boards" :key="board.id" class="board-item" :class="{ active: board.id === selectedBoardId }" @click="selectedBoardId = board.id; refresh()">{{ board.name }}</button>
      <div v-if="!boards.length" class="empty">보드가 없습니다</div>
    </aside>

    <main class="kanban-main">
      <div v-if="error" class="alert error">{{ error }}</div>
      <header class="kanban-header" v-if="selectedBoard">
        <div><h2>{{ selectedBoard.name }}</h2><button class="text-btn" @click="renameBoard">이름 변경</button><button class="text-btn danger" @click="deleteBoard">보드 삭제</button></div>
        <div class="header-actions"><button class="btn" @click="showArchived = !showArchived">{{ showArchived ? '보드 보기' : `아카이브 (${archivedCards.length})` }}</button><button class="btn primary" @click="createColumn">컬럼 추가</button></div>
      </header>
      <div v-if="!selectedBoard" class="empty-state"><p>칸반 보드를 만들어 업무를 관리하세요.</p><button class="btn primary" @click="createBoard">첫 보드 만들기</button></div>

      <section v-else-if="showArchived" class="archive-list">
        <h3>아카이브 카드</h3><div v-if="!archivedCards.length" class="empty-state">아카이브된 카드가 없습니다.</div>
        <article v-for="card in archivedCards" :key="card.id" class="archive-card"><div><strong>{{ card.title }}</strong><span>{{ card.dueDate || '마감일 없음' }}</span></div><div><button class="btn sm" @click="restoreCard(card)">복원</button><button class="btn sm danger" @click="deleteArchived(card)">영구 삭제</button></div></article>
      </section>

      <section v-else class="columns" :class="{ loading }">
        <article v-for="column in columns" :key="column.id" class="column" :class="{ dragging: draggedColumn?.id === column.id }" @dragover.prevent="handleColumnDragOver(column, $event)" @drop="draggedColumn ? dropColumn() : dropCard(column)" @dragend="cancelDrag">
          <header class="column-header" :class="{ 'drop-target': columnDropTargetId === column.id, 'drop-after': columnDropTargetId === column.id && columnDropSide === 'after' }" draggable="true" @dragstart.stop="startColumnDrag(column)" @dragover.prevent.stop="draggedColumn ? previewColumnDrop(column, $event) : previewCardDrop(column)" @drop.stop="draggedColumn ? dropColumn() : dropCard(column)" @dragend="cancelDrag"><h3>{{ column.name }} <small>{{ cardsInColumn(column.id).length }}</small></h3><div><button class="icon-btn sm" title="컬럼 이름 변경" @click="renameColumn(column)">✎</button><button class="icon-btn sm danger" title="컬럼 삭제" @click="deleteColumn(column)">✕</button></div></header>
          <div class="card-list" :class="{ 'drop-target': cardDropTarget.columnId === column.id && !cardDropTarget.cardId }" @dragover.prevent.stop="draggedColumn ? previewColumnDrop(column, $event) : previewCardDrop(column)" @drop.stop="draggedColumn ? dropColumn() : dropCard(column)"><article v-for="card in cardsInColumn(column.id)" :key="card.id" class="task-card" :class="{ done: isDone(column), dragging: draggedCard?.id === card.id, 'drop-target': cardDropTarget.cardId === card.id }" draggable="true" @dragstart.stop="startDrag(card)" @dragover.prevent.stop="draggedColumn ? previewColumnDrop(column, $event) : previewCardDrop(column, card)" @drop.stop="dropOnCard(column, card)" @dragend="cancelDrag" @dblclick="openEditCard(card)">
            <div class="card-top"><strong>{{ card.title }}</strong><button class="icon-btn sm" title="아카이브" @click.stop="archiveCard(card)">⌄</button></div>
            <p v-if="card.description" class="card-description">{{ card.description }}</p>
            <div class="card-meta"><span v-if="card.priority" class="priority" :class="card.priority">{{ priorityLabel(card.priority) }}</span><span v-if="card.dueDate" class="due-date">📅 {{ card.dueDate }}</span></div>
            <div v-if="card.labels?.length" class="labels"><span v-for="label in card.labels" :key="label" class="label">{{ label }}</span></div>
            <div v-if="card.checklist?.length" class="checklist-progress">✓ {{ card.checklist.filter(i => i.completed).length }}/{{ card.checklist.length }}</div>
          </article><button class="add-card" @click="openNewCard(column.id)">+ 카드 추가</button></div>
        </article>
      </section>
    </main>

    <!-- 배경 클릭으로는 닫지 않는다. 작성 중인 카드 내용이 실수로 사라지는 것을 막기 위함. -->
    <div v-if="editingColumnID" class="modal-backdrop"><form class="card-modal" @submit.prevent="saveCard"><header><h3>{{ editingCard ? '카드 편집' : '카드 추가' }}</h3><button type="button" class="icon-btn" @click="closeCard">✕</button></header><div class="form-body">
      <label>제목 *<input v-model="form.title" autofocus /></label><label>설명<textarea v-model="form.description" rows="3" /></label><div class="form-grid"><label>마감일<input v-model="form.dueDate" type="date" /></label><label>우선순위<select v-model="form.priority"><option value="">없음</option><option value="low">낮음</option><option value="medium">보통</option><option value="high">높음</option></select></label></div><label>라벨 <small>쉼표로 구분</small><input v-model="form.labels" /></label><label>담당자<input v-model="form.assignee" /></label><label>체크리스트 <small>한 줄에 하나, 완료 항목은 [x]로 시작</small><textarea v-model="form.checklist" rows="4" /></label><label>첨부파일 경로 <small>쉼표로 구분</small><input v-model="form.attachments" /></label>
    </div><footer><button type="button" class="btn" @click="closeCard">취소</button><button class="btn primary">저장</button></footer></form></div>
  </div>
</template>

<style scoped>
.kanban-layout { display:grid; grid-template-columns:220px 1fr; height:100%; color:var(--text); }
.kanban-sidebar { background:var(--panel); border-right:1px solid var(--border); padding:18px 12px; }
.side-heading,.kanban-header,.column-header,.card-top,.archive-card,.header-actions { display:flex; align-items:center; justify-content:space-between; gap:8px; }
.side-heading h1,.kanban-header h2 { margin:0; font-size:18px; }.side-label { margin:24px 8px 8px; color:var(--muted); font-size:11px; }.board-item { width:100%; text-align:left; padding:9px 10px; border:0; border-radius:6px; background:transparent; color:var(--text); }.board-item:hover,.board-item.active { background:var(--panel-2); }.board-item.active { border-left:3px solid var(--accent); }
.kanban-main { min-width:0; padding:18px; overflow:auto; }.kanban-header { margin-bottom:18px; }.kanban-header h2 { display:inline-block; margin-right:10px; }.text-btn { border:0; background:none; color:var(--muted); font-size:11px; }.text-btn:hover { color:var(--text); }.danger { color:var(--danger)!important; }
.columns { display:flex; align-items:flex-start; gap:14px; min-height:calc(100% - 60px); }.column { width:270px; min-width:270px; background:var(--panel); border:1px solid var(--border); border-radius:8px; transition:opacity .12s, transform .12s; }.column.dragging { opacity:.45; transform:scale(.98); }.column-header { padding:10px 12px; border-bottom:1px solid var(--border); cursor:grab; }.column-header.drop-target { outline:2px solid var(--accent); outline-offset:-2px; background:rgba(91,141,239,.12); }.column-header.drop-target.drop-after { box-shadow:inset -4px 0 0 var(--accent); }.column-header h3 { margin:0; font-size:13px; }.column-header small { color:var(--muted); font-weight:normal; }.card-list { padding:8px; min-height:80px; transition:background .12s, outline .12s; }.card-list.drop-target { outline:2px dashed var(--accent); outline-offset:-3px; background:rgba(91,141,239,.08); }.task-card { margin-bottom:8px; padding:11px; background:var(--panel-2); border:1px solid var(--border); border-radius:7px; cursor:grab; transition:opacity .12s, transform .12s, border-color .12s, box-shadow .12s; }.task-card.dragging { opacity:.35; transform:scale(.97); }.task-card.drop-target { border-color:var(--accent); box-shadow:inset 0 3px 0 var(--accent); }.task-card:hover { border-color:var(--accent); }.task-card.done { opacity:.72; }.task-card.done strong { text-decoration:line-through; }.card-top { align-items:flex-start; }.card-top strong { min-width:0; line-height:1.35; overflow-wrap:anywhere; }/* 공백 없는 긴 문자열(URL 등)이 카드 폭을 넘지 않도록 어디서든 줄바꿈한다. */
.card-description { color:var(--muted); font-size:12px; margin:8px 0; white-space:pre-wrap; overflow-wrap:anywhere; }.card-meta { display:flex; gap:8px; margin-top:9px; font-size:11px; color:var(--muted); }.priority { padding:2px 5px; border-radius:3px; }.priority.high { color:#ff8b98; background:rgba(255,90,106,.12); }.priority.medium { color:#ffc65c; background:rgba(255,198,92,.12); }.priority.low { color:var(--ok); background:rgba(56,211,159,.12); }.labels { display:flex; flex-wrap:wrap; gap:4px; margin-top:8px; }.label { max-width:100%; padding:2px 5px; border-radius:3px; background:var(--border); color:var(--muted); font-size:10px; overflow-wrap:anywhere; }.checklist-progress { margin-top:8px; color:var(--muted); font-size:11px; }.add-card { width:100%; padding:8px; border:1px dashed var(--border); border-radius:6px; background:transparent; color:var(--muted); }.add-card:hover { color:var(--text); border-color:var(--accent); }.icon-btn { background:transparent; border:1px solid var(--border); color:var(--text); border-radius:4px; width:25px; height:25px; }.icon-btn.sm { width:21px; height:21px; font-size:11px; }.btn { padding:7px 12px; border:1px solid var(--border); border-radius:6px; background:var(--panel-2); color:var(--text); }.btn.primary { background:var(--accent); border-color:var(--accent); }.btn.sm { padding:5px 8px; font-size:11px; }.alert { padding:8px 12px; margin-bottom:10px; border-radius:6px; }.alert.error { background:rgba(255,90,106,.12); color:var(--danger); }.empty,.empty-state { color:var(--muted); padding:20px; text-align:center; }.empty-state { margin:80px auto; }.archive-list { max-width:760px; }.archive-card { margin:8px 0; padding:12px; background:var(--panel); border:1px solid var(--border); border-radius:7px; }.archive-card strong,.archive-card span { display:block; }.archive-card span { color:var(--muted); font-size:11px; margin-top:4px; }
.modal-backdrop { position:fixed; inset:0; z-index:30; display:flex; align-items:center; justify-content:center; background:rgba(0,0,0,.55); }.card-modal { width:500px; max-height:90vh; display:flex; flex-direction:column; background:var(--panel); border:1px solid var(--border); border-radius:9px; }.card-modal header,.card-modal footer { display:flex; align-items:center; justify-content:space-between; padding:13px 16px; border-bottom:1px solid var(--border); }.card-modal footer { border-top:1px solid var(--border); border-bottom:0; justify-content:flex-end; }.card-modal h3 { margin:0; }.form-body { display:flex; flex-direction:column; gap:10px; padding:16px; overflow:auto; }.form-body label { display:flex; flex-direction:column; gap:4px; color:var(--muted); font-size:12px; }.form-body small { color:var(--muted); font-size:10px; }.form-body input,.form-body textarea,.form-body select { padding:8px; border:1px solid var(--border); border-radius:5px; background:var(--panel-2); color:var(--text); font:inherit; }.form-grid { display:grid; grid-template-columns:1fr 1fr; gap:10px; }
</style>
