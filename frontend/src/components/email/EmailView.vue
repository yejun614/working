<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { Service as EmailService } from '../../../bindings/working/internal/modules/email'
import type { Account } from '../../../bindings/working/internal/modules/account/types/models'
import type { Message } from '../../../bindings/working/internal/modules/email/types/models'
import ComposeForm from './ComposeForm.vue'
import { isDarkMode } from '../../theme'
import { canSendMail } from '../../accounts'

const accounts = ref<Account[]>([])
const selectedAccountId = ref<string>('')
const folders = ref<string[]>([])
const selectedFolder = ref<string>('INBOX')
const messages = ref<Message[]>([])
const selectedMessage = ref<Message | null>(null)

const loadingList = ref(false)
const loadingMore = ref(false)
const deletingMessage = ref(false)
const nextPageToken = ref('')
const error = ref<string>('')
const info = ref<string>('')
const showRaw = ref(false)
const copyingRaw = ref(false)

// 폴더를 빠르게 전환할 때 먼저 시작한 요청이 늦게 도착해
// 현재 폴더의 목록을 덮어쓰지 않도록 요청 순번을 관리한다.
let messageLoadRequest = 0
// 앱 시작 시 캐시가 비어 있는 경우에만 자동 새로고침을 한 번 수행한다.
// 이후 폴더 전환/재조회에서는 사용자가 새로고침 버튼을 눌러야 한다.
const startupAutoRefreshKey = 'app-startup-auto-refresh:email'
let startupAutoRefreshAttempted = sessionStorage.getItem(startupAutoRefreshKey) === '1'

const showCompose = ref(false)
const composeDraft = ref<Message | null>(null)
const replyTarget = ref<Message | null>(null)
const LOCAL_DRAFT_FOLDER = '작성중 메일'
const LOCAL_DRAFTS_KEY = 'working.email.local-drafts'

const selectedAccount = computed(() =>
  accounts.value.find(a => a.id === selectedAccountId.value) || null
)
const canSend = computed(() => accounts.value.some(canSendMail))
// 현재 폴더에서 읽지 않은 메일 수. 목록 헤더에 배지로 표시한다.
const unreadCount = computed(() => messages.value.filter(m => m.unread).length)

const sanitizedBodyHTML = computed(() =>
  sanitizeHTML(selectedMessage.value?.html || '')
)

const renderedBodyHTML = computed(() => {
  if (!sanitizedBodyHTML.value) return ''
  const mediaRecovery = isDarkMode.value
    ? 'img,video,svg{filter:invert(1) hue-rotate(180deg)}'
    : ''
  return `<style>@font-face{font-family:"Pretendard";src:url("/fonts/Pretendard-Regular.woff2") format("woff2");font-weight:400}@font-face{font-family:"Pretendard";src:url("/fonts/Pretendard-Bold.woff2") format("woff2");font-weight:700}@font-face{font-family:"Pretendard JP";src:url("/fonts/PretendardJP-Regular.woff2") format("woff2");font-weight:400}@font-face{font-family:"Pretendard JP";src:url("/fonts/PretendardJP-Bold.woff2") format("woff2");font-weight:700}html,body{margin:0;min-height:100%;background:#fff!important;color:#202633!important;font-family:"Pretendard","Pretendard JP",sans-serif;line-height:1.5}body{padding:16px;overflow-wrap:anywhere}img{max-width:100%;height:auto}${mediaRecovery}</style>${sanitizedBodyHTML.value}`
})

function sanitizeHTML(input: string): string {
  if (!input.trim()) return ''

  const document = new DOMParser().parseFromString(input, 'text/html')
  document.querySelectorAll('script, noscript').forEach(node => node.remove())

  document.querySelectorAll('*').forEach(element => {
    Array.from(element.attributes).forEach(attribute => {
      const name = attribute.name.toLowerCase()
      const value = attribute.value.trim().toLowerCase()
      const isEventHandler = name.startsWith('on')
      const isJavaScriptURL =
        ['href', 'src', 'xlink:href', 'formaction'].includes(name) &&
        value.startsWith('javascript:')

      if (isEventHandler || isJavaScriptURL) {
        element.removeAttribute(attribute.name)
      }
    })
  })

  return document.body.innerHTML
}

async function refreshAccounts() {
  try {
    const list = await EmailService.AccountList()
    accounts.value = list || []
    if (!selectedAccountId.value && accounts.value.length > 0) {
      selectedAccountId.value = accounts.value[0].id
      await onAccountChanged()
    } else if (selectedAccountId.value && !accounts.value.find(a => a.id === selectedAccountId.value)) {
      selectedAccountId.value = accounts.value[0]?.id || ''
      await onAccountChanged()
    }
  } catch (e) {
    setError((e as Error).message)
  }
}

async function onAccountChanged() {
  folders.value = [LOCAL_DRAFT_FOLDER]
  messages.value = []
  nextPageToken.value = ''
  selectedMessage.value = null
  if (!selectedAccountId.value) return
  try {
    const list = await EmailService.Folders(selectedAccountId.value)
    folders.value = [LOCAL_DRAFT_FOLDER, ...(list || []).filter(folder => folder !== LOCAL_DRAFT_FOLDER)]
    if (selectedFolder.value !== LOCAL_DRAFT_FOLDER && !folders.value.includes(selectedFolder.value)) {
      selectedFolder.value = folders.value[0] || 'INBOX'
    }
    await loadMessages()
  } catch (e) {
    setError((e as Error).message)
  }
}

async function onFolderChanged() {
  selectedMessage.value = null
  await loadMessages()
}

async function refreshMessages() {
  if (!selectedAccountId.value || selectedFolder.value === LOCAL_DRAFT_FOLDER) {
    await loadMessages()
    return
  }
  const requestId = ++messageLoadRequest
  const accountId = selectedAccountId.value
  const folder = selectedFolder.value
  loadingList.value = true
  error.value = ''
  try {
    const page = await EmailService.ListRefreshPage(accountId, folder, '')
    if (requestId === messageLoadRequest && accountId === selectedAccountId.value && folder === selectedFolder.value) {
      messages.value = page?.messages || []
      nextPageToken.value = page?.nextPageToken || ''
    }
  } catch (e) {
    if (requestId === messageLoadRequest) setError((e as Error).message)
  } finally {
    if (requestId === messageLoadRequest) loadingList.value = false
  }
}

async function loadMessages() {
  if (!selectedAccountId.value) return

  if (selectedFolder.value === LOCAL_DRAFT_FOLDER) {
    messages.value = readLocalDrafts()
    nextPageToken.value = ''
    loadingList.value = false
    return
  }

  const requestId = ++messageLoadRequest
  const accountId = selectedAccountId.value
  const folder = selectedFolder.value
  loadingList.value = true
  error.value = ''
  try {
    const page = await EmailService.Page(accountId, folder)

    // 응답이 돌아오는 동안 계정이나 폴더가 바뀌었으면 폐기한다.
    // 요청 순번도 확인해 같은 폴더를 새로고침한 경우의 이전 응답을 막는다.
    if (
      requestId === messageLoadRequest &&
      accountId === selectedAccountId.value &&
      folder === selectedFolder.value
    ) {
      messages.value = page?.messages || []
      nextPageToken.value = page?.nextPageToken || ''
      // 첫 화면의 캐시가 비어 있을 때만 앱 시작 자동 새로고침을 수행한다.
      // refreshMessages가 끝난 뒤에는 결과가 비어 있어도 다시 호출하지 않는다.
      if (messages.value.length === 0 && !startupAutoRefreshAttempted) {
        startupAutoRefreshAttempted = true
        sessionStorage.setItem(startupAutoRefreshKey, '1')
        await refreshMessages()
      }
    }
  } catch (e) {
    if (requestId === messageLoadRequest) {
      setError((e as Error).message)
    }
  } finally {
    if (requestId === messageLoadRequest) {
      loadingList.value = false
    }
  }
}

async function loadMoreMessages() {
  if (!selectedAccountId.value || selectedFolder.value === LOCAL_DRAFT_FOLDER || !nextPageToken.value || loadingMore.value) return

  const requestId = messageLoadRequest
  const accountId = selectedAccountId.value
  const folder = selectedFolder.value
  loadingMore.value = true
  error.value = ''
  try {
    const page = await EmailService.ListMore(accountId, folder, nextPageToken.value)
    if (requestId === messageLoadRequest && accountId === selectedAccountId.value && folder === selectedFolder.value) {
      messages.value = page?.messages || messages.value
      nextPageToken.value = page?.nextPageToken || ''
    }
  } catch (e) {
    setError((e as Error).message)
  } finally {
    loadingMore.value = false
  }
}

function selectMessage(m: Message) {
  selectedMessage.value = m
  showRaw.value = false
  replyTarget.value = null
  composeDraft.value = isDraftFolder(selectedFolder.value) ? m : null
  showCompose.value = composeDraft.value !== null
  // 메일을 열면 메일 서버에도 읽음으로 반영한다. 실패해도 본문 열람은 막지 않는다.
  if (m.unread) void markRead(m, true)
}

// 캐시된 메시지를 식별하는 키. Gmail은 원격 ID를, IMAP은 UID를 사용한다.
function messageKey(m: Message): string {
  return m.id || `uid:${m.uid}`
}

// 읽음 상태를 메일 서버(Gmail UNREAD 라벨 / IMAP \Seen 플래그)에 반영한다.
// 화면은 먼저 바꾸고, 실패하면 이전 상태로 되돌린다.
async function markRead(m: Message, read: boolean) {
  if (!selectedAccountId.value || selectedFolder.value === LOCAL_DRAFT_FOLDER) return
  const previous = m.unread
  m.unread = !read
  try {
    await EmailService.MessageMarkRead(selectedAccountId.value, selectedFolder.value, m.id || '', m.uid || 0, read)
  } catch (e) {
    m.unread = previous
    setError((e as Error).message)
  }
}

function toggleRead(m: Message) {
  void markRead(m, m.unread === true)
}

// 메일을 서버에서 삭제한다. Gmail은 휴지통으로, IMAP은 폴더에서 제거된다.
async function deleteMessage(m: Message) {
  if (selectedFolder.value === LOCAL_DRAFT_FOLDER) {
    deleteLocalDraft(m)
    if (selectedMessage.value && messageKey(selectedMessage.value) === messageKey(m)) selectedMessage.value = null
    return
  }
  if (!selectedAccountId.value || deletingMessage.value) return
  if (!confirm(`메일 "${m.subject || '(제목 없음)'}" 을(를) 삭제할까요?`)) return

  deletingMessage.value = true
  try {
    await EmailService.MessageDelete(selectedAccountId.value, selectedFolder.value, m.id || '', m.uid || 0)
    const key = messageKey(m)
    messages.value = messages.value.filter(item => messageKey(item) !== key)
    if (selectedMessage.value && messageKey(selectedMessage.value) === key) selectedMessage.value = null
    setInfo('메일이 삭제되었습니다.')
  } catch (e) {
    setError((e as Error).message)
  } finally {
    deletingMessage.value = false
  }
}

function isDraftFolder(folder: string): boolean {
  const normalized = folder.toLowerCase()
  return normalized.includes('draft') || normalized.includes('임시') || normalized.includes('초안') || normalized.includes('작성중')
}

function readLocalDrafts(): Message[] {
  try {
    const raw = localStorage.getItem(LOCAL_DRAFTS_KEY)
    if (!raw) return []
    const drafts = JSON.parse(raw) as Message[]
    return Array.isArray(drafts) ? drafts : []
  } catch {
    return []
  }
}

function writeLocalDrafts(drafts: Message[]) {
  localStorage.setItem(LOCAL_DRAFTS_KEY, JSON.stringify(drafts))
}

function saveLocalDraft(message: Message) {
  const draft: Message = {
    ...message,
    uid: message.uid || Date.now(),
  }
  const drafts = readLocalDrafts()
  const index = drafts.findIndex(item => item.uid === draft.uid)
  if (index >= 0) drafts[index] = draft
  else drafts.unshift(draft)
  writeLocalDrafts(drafts)
  composeDraft.value = draft
  selectedMessage.value = draft
  if (selectedFolder.value === LOCAL_DRAFT_FOLDER) messages.value = drafts
}

function deleteLocalDraft(message?: Message | null) {
  if (!message?.uid) return
  const drafts = readLocalDrafts().filter(item => item.uid !== message.uid)
  writeLocalDrafts(drafts)
  if (selectedFolder.value === LOCAL_DRAFT_FOLDER) messages.value = drafts
}

async function copyRaw() {
  const raw = selectedMessage.value?.raw
  if (!raw) {
    setError('복사할 이메일 원문이 없습니다.')
    return
  }

  copyingRaw.value = true
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(raw)
    } else {
      const textarea = document.createElement('textarea')
      textarea.value = raw
      textarea.style.position = 'fixed'
      textarea.style.opacity = '0'
      document.body.appendChild(textarea)
      textarea.focus()
      textarea.select()
      if (!document.execCommand('copy')) throw new Error('클립보드 복사 명령이 거부되었습니다.')
      textarea.remove()
    }
    setInfo('이메일 원문을 클립보드에 복사했습니다.')
  } catch (e) {
    setError(`이메일 원문 복사에 실패했습니다: ${(e as Error).message}`)
  } finally {
    copyingRaw.value = false
  }
}

function openCompose() {
  composeDraft.value = null
  replyTarget.value = null
  showCompose.value = true
}

// 현재 보고 있는 메일의 발신자와 제목을 답장 양식에 미리 채운다.
// 실제 발송은 새 메일 작성과 동일한 ComposeForm/Send 경로를 사용한다.
function openReply() {
  if (!selectedMessage.value) return
  composeDraft.value = null
  replyTarget.value = selectedMessage.value
  showCompose.value = true
}

function closeCompose() {
  deleteLocalDraft(composeDraft.value)
  showCompose.value = false
  composeDraft.value = null
  replyTarget.value = null
}

async function onSent() {
  deleteLocalDraft(composeDraft.value)
  closeCompose()
  setInfo('메일이 전송되었습니다.')
  await loadMessages()
}

function setError(msg: string) {
  error.value = msg
  info.value = ''
}
function setInfo(msg: string) {
  info.value = msg
  error.value = ''
  setTimeout(() => { if (info.value === msg) info.value = '' }, 3500)
}

function formatDate(s?: string) {
  if (!s) return ''
  const d = new Date(s)
  if (isNaN(d.getTime())) return s
  return d.toLocaleString()
}

function formatBytes(size?: number) {
  if (!size || size < 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB']
  const unitIndex = Math.min(Math.floor(Math.log(size) / Math.log(1024)), units.length - 1)
  const value = size / Math.pow(1024, unitIndex)
  return `${value >= 10 || unitIndex === 0 ? value.toFixed(0) : value.toFixed(2)} ${units[unitIndex]}`
}

function attachmentMimeType(name: string): string {
  const extension = name.split('.').pop()?.toLowerCase() || ''
  const types: Record<string, string> = {
    jpg: 'image/jpeg', jpeg: 'image/jpeg', png: 'image/png', gif: 'image/gif', webp: 'image/webp',
    pdf: 'application/pdf', txt: 'text/plain', csv: 'text/csv', md: 'text/markdown',
  }
  return types[extension] || 'application/octet-stream'
}

function canPreview(name: string): boolean {
  return attachmentMimeType(name) !== 'application/octet-stream'
}

function decodeBase64(encoded: string): Uint8Array {
  const binary = atob(encoded)
  return Uint8Array.from(binary, character => character.charCodeAt(0))
}

async function attachmentBlob(index: number, name: string): Promise<Blob> {
  if (!selectedMessage.value?.raw) {
    throw new Error('첨부파일 원문을 찾을 수 없습니다.')
  }
  const encoded = await EmailService.AttachmentData(selectedMessage.value.raw, index)
  return new Blob([decodeBase64(encoded)], { type: attachmentMimeType(name) })
}

async function downloadAttachment(index: number, name: string) {
  try {
    const url = URL.createObjectURL(await attachmentBlob(index, name))
    const link = document.createElement('a')
    link.href = url
    link.download = name
    link.click()
    setTimeout(() => URL.revokeObjectURL(url), 1000)
  } catch (e) {
    setError((e as Error).message)
  }
}

async function previewAttachment(index: number, name: string) {
  if (!canPreview(name)) {
    alert('이 파일 형식은 미리보기를 지원하지 않습니다. 다운로드 후 해당 앱에서 열어주세요.')
    return
  }
  try {
    const previewWindow = window.open('', '_blank')
    if (!previewWindow) {
      throw new Error('미리보기 창을 열 수 없습니다.')
    }
    const url = URL.createObjectURL(await attachmentBlob(index, name))
    previewWindow.location.href = url
    setTimeout(() => URL.revokeObjectURL(url), 60_000)
  } catch (e) {
    setError((e as Error).message)
  }
}

onMounted(() => {
  refreshAccounts()
})
</script>

<template>
  <div class="layout">
    <aside class="sidebar">
      <div class="sidebar-header">
        <h1>이메일</h1>
      </div>

      <div class="account-section">
        <div class="section-title">
          <span>계정</span>
        </div>
        <ul class="account-list">
          <li
            v-for="a in accounts"
            :key="a.id"
            :class="{ active: a.id === selectedAccountId }"
            @click="selectedAccountId = a.id; onAccountChanged()"
          >
            <div class="account-row">
              <div class="account-name">{{ a.name || a.email }}</div>
              <span v-if="a.authError" class="auth-warning" :title="a.authError">⚠</span>
            </div>
            <div class="account-email">{{ a.email }}</div>
          </li>
          <li v-if="accounts.length === 0" class="empty">계정 탭에서 계정을 추가하세요</li>
        </ul>
      </div>

      <div class="folder-section" v-if="folders.length > 0">
        <div class="section-title"><span>폴더</span></div>
        <ul class="folder-list">
          <li
            v-for="f in folders"
            :key="f"
            :class="{ active: f === selectedFolder }"
            @click="selectedFolder = f; onFolderChanged()"
          >{{ f }}</li>
        </ul>
      </div>
    </aside>

    <section class="list-pane">
      <div class="list-header">
        <div class="list-title">
          {{ selectedFolder }}
          <span v-if="unreadCount" class="unread-badge" :title="`읽지 않은 메일 ${unreadCount}개`">{{ unreadCount }}</span>
        </div>
        <div class="list-actions">
          <button class="btn" @click="refreshMessages" :disabled="!selectedAccountId || loadingList">새로고침</button>
          <button class="btn primary" @click="openCompose" :disabled="!canSend">메일 쓰기</button>
        </div>
      </div>

      <div v-if="error" class="alert error">{{ error }}</div>
      <div v-if="info" class="alert info">{{ info }}</div>

      <div class="list-body">
        <div v-if="loadingList" class="state">불러오는 중…</div>
        <div v-else-if="!selectedAccountId" class="state">계정을 선택하세요</div>
        <div v-else-if="messages.length === 0" class="state">메일이 없습니다</div>
        <ul v-else class="message-list">
          <li
            v-for="m in messages"
            :key="m.uid"
            :class="{ unread: m.unread, selected: selectedMessage?.uid === m.uid }"
            @click="selectMessage(m)"
          >
            <div class="msg-row">
              <span class="unread-dot" :class="{ hidden: !m.unread }" aria-hidden="true"></span>
              <div class="msg-from">{{ m.from }}</div>
              <button
                class="icon-btn sm danger msg-delete"
                title="메일 삭제"
                :disabled="deletingMessage"
                @click.stop="deleteMessage(m)"
              >✕</button>
            </div>
            <div class="msg-subject">{{ m.subject }}</div>
            <div class="msg-date">{{ formatDate(m.date) }}</div>
            <span class="sr-only">{{ m.unread ? '읽지 않음' : '읽음' }}</span>
          </li>
        </ul>
        <div v-if="nextPageToken" class="load-more">
          <button class="btn" type="button" @click="loadMoreMessages" :disabled="loadingMore">
            {{ loadingMore ? '불러오는 중…' : '더 불러오기' }}
          </button>
        </div>
      </div>
    </section>

    <section class="detail-pane">
      <ComposeForm
        v-if="showCompose"
        :account="selectedAccount"
        :accounts="accounts"
        :initial-account-id="selectedAccountId"
        :draft="composeDraft"
        :reply-to="replyTarget"
        embedded
        @close="closeCompose"
        @changed="saveLocalDraft"
        @sent="onSent"
      />
      <div v-else-if="!selectedMessage" class="state">메시지를 선택하세요</div>
      <article v-else class="message-detail">
        <h2>{{ selectedMessage.subject }}</h2>
        <div class="meta"><span>From:</span> {{ selectedMessage.from }}</div>
        <div class="meta"><span>To:</span> {{ selectedMessage.to }}</div>
        <div class="meta" v-if="selectedMessage.cc"><span>Cc:</span> {{ selectedMessage.cc }}</div>
        <div class="meta"><span>Date:</span> {{ formatDate(selectedMessage.date) }}</div>
        <div class="detail-actions">
          <button
            class="btn primary"
            :disabled="!canSendMail(selectedAccount)"
            @click="openReply"
          >답장 작성</button>
          <button class="btn" @click="showRaw = !showRaw">
            {{ showRaw ? '본문 보기' : '원문 보기' }}
          </button>
          <button
            class="btn"
            v-if="selectedFolder !== LOCAL_DRAFT_FOLDER"
            @click="toggleRead(selectedMessage)"
          >{{ selectedMessage.unread ? '읽음으로 표시' : '안 읽음으로 표시' }}</button>
          <button
            class="btn danger-btn"
            :disabled="deletingMessage"
            @click="deleteMessage(selectedMessage)"
          >{{ deletingMessage ? '삭제 중…' : '삭제' }}</button>
        </div>
        <section v-if="selectedMessage.attachments?.length" class="attachments" aria-label="첨부파일">
          <h3>첨부파일 ({{ selectedMessage.attachments.length }})</h3>
          <ul>
            <li v-for="(attachment, index) in selectedMessage.attachments" :key="`${attachment.name}-${attachment.size}`">
              <div class="attachment-info">
                <span class="attachment-name">{{ attachment.name }}</span>
                <span class="attachment-size">{{ formatBytes(attachment.size) }}</span>
              </div>
              <div class="attachment-actions">
                <button class="btn attachment-btn" type="button" @click="downloadAttachment(index, attachment.name)">다운로드</button>
                <button class="btn attachment-btn" type="button" @click="previewAttachment(index, attachment.name)">미리보기</button>
              </div>
            </li>
          </ul>
        </section>
        <div v-if="showRaw" class="raw-view">
          <pre class="body raw-body">
            <button
              class="raw-copy-btn"
              type="button"
              title="원문 복사"
              aria-label="원문 복사"
              @click="copyRaw"
              :disabled="copyingRaw"
            >
              <svg viewBox="0 0 24 24" aria-hidden="true">
                <rect x="8" y="8" width="11" height="11" rx="2" fill="none" stroke="currentColor" stroke-width="1.8" />
                <path d="M16 8V6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h2" fill="none" stroke="currentColor" stroke-width="1.8" />
              </svg>
            </button>
            <span class="raw-content">{{ selectedMessage.raw }}</span>
          </pre>
        </div>
        <div v-else-if="renderedBodyHTML" class="body html-body">
          <iframe
            class="email-frame"
            :class="{ 'dark-mode': isDarkMode }"
            :srcdoc="renderedBodyHTML"
            sandbox=""
            title="이메일 본문"
          ></iframe>
        </div>
        <pre v-else class="body">{{ selectedMessage.body }}</pre>
      </article>
    </section>
  </div>
</template>

<style scoped>
.layout {
  display: grid;
  grid-template-columns: minmax(0, 240px) minmax(0, 320px) minmax(0, 1fr);
  width: 100%;
  min-width: 0;
  height: 100%;
}
.sidebar {
  background: var(--panel);
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
  min-width: 0;
  overflow: auto;
}
.sidebar-header {
  padding: 18px 16px 12px;
  border-bottom: 1px solid var(--border);
}
.sidebar-header h1 {
  margin: 0;
  font-size: 18px;
  letter-spacing: 0.5px;
}
.section-title {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px 6px;
  color: var(--muted);
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.6px;
}
.account-list, .folder-list {
  list-style: none;
  margin: 0;
  padding: 0;
}
.account-list li, .folder-list li {
  padding: 8px 16px;
  cursor: pointer;
  border-bottom: 1px solid transparent;
}
.account-list li:hover, .folder-list li:hover {
  background: var(--panel-2);
}
.account-list li.active, .folder-list li.active {
  background: var(--panel-2);
  border-left: 3px solid var(--accent);
  padding-left: 13px;
}
.account-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.account-name {
  font-weight: 600;
}
.account-email {
  font-size: 11px;
  color: var(--muted);
}
.auth-warning { flex-shrink: 0; color: #ffc65c; font-size: 12px; cursor: help; }
.empty {
  color: var(--muted);
  font-style: italic;
  cursor: default;
}
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

.list-pane, .detail-pane {
  display: flex;
  flex-direction: column;
  min-width: 0;
  overflow: hidden;
}
.list-pane { border-right: 1px solid var(--border); }
.list-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 12px 16px;
  border-bottom: 1px solid var(--border);
}
.list-title { display: flex; align-items: center; gap: 8px; font-weight: 600; }
.unread-badge {
  min-width: 20px;
  padding: 1px 6px;
  border-radius: 10px;
  background: var(--accent);
  color: #fff;
  font-size: 11px;
  font-weight: 700;
  text-align: center;
}
.list-actions { display: flex; gap: 6px; }
.btn {
  background: var(--panel-2);
  color: var(--text);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 6px 12px;
}
.btn:hover { background: var(--border); }
.btn.primary {
  background: var(--accent);
  border-color: var(--accent);
}
.btn.primary:hover { background: var(--accent-hover); }
.btn:disabled { opacity: 0.5; cursor: not-allowed; }
.btn.danger-btn { color: var(--danger); }
.btn.danger-btn:hover:not(:disabled) { background: rgba(255,90,106,0.12); border-color: var(--danger); }

.alert {
  margin: 8px 16px;
  padding: 8px 12px;
  border-radius: 6px;
  font-size: 13px;
}
.alert.error { background: rgba(255,90,106,0.12); color: var(--danger); }
.alert.info { background: rgba(56,211,159,0.12); color: var(--ok); }

.list-body, .detail-pane { flex: 1; overflow: auto; }
.state {
  padding: 24px;
  text-align: center;
  color: var(--muted);
}
.message-list { list-style: none; margin: 0; padding: 0; min-width: 0; width: 100%; overflow: hidden; }
.message-list li {
  min-width: 0;
  overflow: hidden;
  padding: 10px 16px;
  border-bottom: 1px solid var(--border);
  cursor: pointer;
}
.message-list li:hover { background: var(--panel-2); }
.message-list li.selected { background: var(--panel-2); border-left: 3px solid var(--accent); padding-left: 13px; }
.message-list li.unread .msg-subject { font-weight: 700; }
.message-list li.unread .msg-from { color: var(--text); font-weight: 600; }
.msg-row { display: flex; align-items: center; gap: 6px; min-width: 0; }
.unread-dot {
  flex: 0 0 auto;
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--accent);
}
/* 읽은 메일도 같은 자리를 차지하게 해 목록의 좌측 정렬이 흔들리지 않도록 한다. */
.unread-dot.hidden { visibility: hidden; }
.msg-delete { display: none; flex: 0 0 auto; }
.message-list li:hover .msg-delete { display: inline-flex; }
.sr-only {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0 0 0 0);
  white-space: nowrap;
}
.load-more {
  display: flex;
  justify-content: center;
  padding: 12px 16px 18px;
}
.load-more .btn { width: fit-content; }
.msg-from, .msg-subject, .msg-date {
  -webkit-user-select: text;
  user-select: text;
  --wails-draggable: no-drag;
}
.msg-from { flex: 1; min-width: 0; font-size: 13px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.msg-subject { color: var(--text); margin: 2px 0; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.msg-date { font-size: 11px; color: var(--muted); }

.message-detail {
  display: flex;
  flex-direction: column;
  min-height: 100%;
  padding: 18px 24px;
  -webkit-user-select: text;
  user-select: text;
  --wails-draggable: no-drag;
}
.message-detail h2 { margin: 0 0 12px; }
.meta { font-size: 13px; margin: 2px 0; color: var(--muted); }
.meta span { color: var(--text); margin-right: 6px; }
.detail-actions { display: flex; gap: 6px; margin-top: 12px; }
.attachments {
  margin-top: 14px;
  padding: 12px 14px;
  background: var(--panel-2);
  border: 1px solid var(--border);
  border-radius: 8px;
}
.attachments h3 { margin: 0 0 8px; font-size: 13px; }
.attachments ul { display: flex; flex-direction: column; gap: 6px; margin: 0; padding: 0; list-style: none; }
.attachments li { display: flex; justify-content: space-between; align-items: center; gap: 12px; min-width: 0; font-size: 13px; }
.attachment-info { display: flex; align-items: baseline; gap: 8px; min-width: 0; }
.attachment-name { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; color: var(--text); }
.attachment-size { flex: 0 0 auto; color: var(--muted); }
.attachment-actions { display: flex; flex: 0 0 auto; gap: 5px; }
.attachment-btn { padding: 4px 8px; font-size: 12px; }
.raw-view { display: flex; flex-direction: column; min-height: 0; margin-top: 14px; }
.raw-body {
  position: relative;
  margin-top: 0;
  padding-top: 48px;
  overflow: auto;
}
.raw-copy-btn {
  position: absolute;
  top: 10px;
  right: 10px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 30px;
  height: 30px;
  padding: 6px;
  color: var(--muted);
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 6px;
  cursor: pointer;
}
.raw-copy-btn:hover { color: var(--text); background: var(--border); }
.raw-copy-btn:disabled { opacity: 0.5; cursor: not-allowed; }
.raw-copy-btn svg { width: 16px; height: 16px; }
.raw-content { display: block; }
.body {
  margin-top: 14px;
  white-space: pre-wrap;
  word-wrap: break-word;
  font-family: inherit;
  font-size: 14px;
  line-height: 1.5;
  background: var(--panel-2);
  padding: 14px;
  border-radius: 8px;
  border: 1px solid var(--border);
}
.html-body {
  display: flex;
  flex: 1;
  min-height: 0;
  padding: 0;
  overflow: hidden;
}
.html-body iframe {
  display: block;
  flex: 1;
  width: 100%;
  min-height: 0;
  height: auto;
  border: 0;
  background: #fff;
  transition: filter 0.2s ease;
}
.html-body iframe.dark-mode {
  filter: invert(0.92) hue-rotate(180deg);
}
</style>