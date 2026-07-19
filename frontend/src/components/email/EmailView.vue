<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { Service as EmailService } from '../../../bindings/working/internal/modules/email'
import type { Account } from '../../../bindings/working/internal/modules/email/account/models'
import type { Message } from '../../../bindings/working/internal/modules/email/types/models'
import AccountForm from './AccountForm.vue'
import ComposeForm from './ComposeForm.vue'

const accounts = ref<Account[]>([])
const selectedAccountId = ref<string>('')
const folders = ref<string[]>([])
const selectedFolder = ref<string>('INBOX')
const messages = ref<Message[]>([])
const selectedMessage = ref<Message | null>(null)

const loadingList = ref(false)
const error = ref<string>('')
const info = ref<string>('')

const showAccountForm = ref(false)
const editingAccount = ref<Account | null>(null)
const showCompose = ref(false)

const selectedAccount = computed(() =>
  accounts.value.find(a => a.id === selectedAccountId.value) || null
)

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
  folders.value = []
  messages.value = []
  selectedMessage.value = null
  if (!selectedAccountId.value) return
  try {
    const list = await EmailService.Folders(selectedAccountId.value)
    folders.value = list || []
    if (!folders.value.includes(selectedFolder.value)) {
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

async function loadMessages() {
  if (!selectedAccountId.value) return
  loadingList.value = true
  error.value = ''
  try {
    const list = await EmailService.List(selectedAccountId.value, selectedFolder.value)
    messages.value = list || []
  } catch (e) {
    setError((e as Error).message)
  } finally {
    loadingList.value = false
  }
}

function selectMessage(m: Message) {
  selectedMessage.value = m
}

function openNewAccount() {
  editingAccount.value = null
  showAccountForm.value = true
}

function openEditAccount(acc: Account) {
  editingAccount.value = acc
  showAccountForm.value = true
}

async function deleteAccount(acc: Account) {
  if (!confirm(`계정 "${acc.name || acc.email}" 을(를) 삭제할까요? 키체인 자격증명도 함께 삭제됩니다.`)) return
  try {
    await EmailService.AccountDelete(acc.id)
    setInfo('계정이 삭제되었습니다.')
    await refreshAccounts()
  } catch (e) {
    setError((e as Error).message)
  }
}

async function onAccountSaved() {
  showAccountForm.value = false
  await refreshAccounts()
  setInfo('계정이 저장되었습니다.')
}

function openCompose() {
  showCompose.value = true
}

async function onSent() {
  showCompose.value = false
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

onMounted(() => {
  refreshAccounts()
})
</script>

<template>
  <div class="layout">
    <aside class="sidebar">
      <div class="sidebar-header">
        <h1>working</h1>
        <span class="module-tag">이메일 모듈</span>
      </div>

      <div class="account-section">
        <div class="section-title">
          <span>계정</span>
          <button class="icon-btn" title="계정 추가" @click="openNewAccount">+</button>
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
              <div class="account-actions">
                <button class="icon-btn sm" title="편집" @click.stop="openEditAccount(a)">✎</button>
                <button class="icon-btn sm danger" title="삭제" @click.stop="deleteAccount(a)">✕</button>
              </div>
            </div>
            <div class="account-email">{{ a.email }}</div>
          </li>
          <li v-if="accounts.length === 0" class="empty">등록된 계정이 없습니다</li>
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
        <div class="list-title">{{ selectedFolder }}</div>
        <div class="list-actions">
          <button class="btn" @click="loadMessages" :disabled="!selectedAccountId">새로고침</button>
          <button class="btn primary" @click="openCompose" :disabled="!selectedAccount?.smtp">메일 쓰기</button>
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
            <div class="msg-from">{{ m.from }}</div>
            <div class="msg-subject">{{ m.subject }}</div>
            <div class="msg-date">{{ formatDate(m.date) }}</div>
          </li>
        </ul>
      </div>
    </section>

    <section class="detail-pane">
      <div v-if="!selectedMessage" class="state">메시지를 선택하세요</div>
      <article v-else class="message-detail">
        <h2>{{ selectedMessage.subject }}</h2>
        <div class="meta"><span>From:</span> {{ selectedMessage.from }}</div>
        <div class="meta"><span>To:</span> {{ selectedMessage.to }}</div>
        <div class="meta" v-if="selectedMessage.cc"><span>Cc:</span> {{ selectedMessage.cc }}</div>
        <div class="meta"><span>Date:</span> {{ formatDate(selectedMessage.date) }}</div>
        <pre class="body">{{ selectedMessage.body }}</pre>
      </article>
    </section>

    <AccountForm
      v-if="showAccountForm"
      :account="editingAccount"
      @close="showAccountForm = false"
      @saved="onAccountSaved"
    />
    <ComposeForm
      v-if="showCompose"
      :account="selectedAccount"
      @close="showCompose = false"
      @sent="onSent"
    />
  </div>
</template>

<style scoped>
.layout {
  display: grid;
  grid-template-columns: 240px 320px 1fr;
  height: 100vh;
}
.sidebar {
  background: var(--panel);
  border-right: 1px solid var(--border);
  display: flex;
  flex-direction: column;
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
.module-tag {
  font-size: 11px;
  color: var(--muted);
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
.account-actions {
  display: none;
  gap: 4px;
}
.account-list li:hover .account-actions {
  display: flex;
}
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
.list-title { font-weight: 600; }
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
.message-list { list-style: none; margin: 0; padding: 0; }
.message-list li {
  padding: 10px 16px;
  border-bottom: 1px solid var(--border);
  cursor: pointer;
}
.message-list li:hover { background: var(--panel-2); }
.message-list li.selected { background: var(--panel-2); border-left: 3px solid var(--accent); padding-left: 13px; }
.message-list li.unread .msg-subject { font-weight: 700; }
.msg-from { font-size: 13px; }
.msg-subject { color: var(--text); margin: 2px 0; }
.msg-date { font-size: 11px; color: var(--muted); }

.message-detail { padding: 18px 24px; }
.message-detail h2 { margin: 0 0 12px; }
.meta { font-size: 13px; margin: 2px 0; color: var(--muted); }
.meta span { color: var(--text); margin-right: 6px; }
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
</style>