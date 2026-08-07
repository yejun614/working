<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Service as AccountService } from '../../../bindings/working/internal/modules/account'
import type { Account } from '../../../bindings/working/internal/modules/account/types/models'
import AccountForm from './AccountForm.vue'

const accounts = ref<Account[]>([])
const loading = ref(false)
const error = ref('')
const info = ref('')
const showForm = ref(false)
const editingAccount = ref<Account | null>(null)
const reconnectingAccountId = ref('')

async function refresh() {
  loading.value = true
  try {
    accounts.value = (await AccountService.List()) || []
  } catch (e) {
    setError((e as Error).message)
  } finally {
    loading.value = false
  }
}

function openNew() {
  editingAccount.value = null
  showForm.value = true
}

function openEdit(acc: Account) {
  editingAccount.value = acc
  showForm.value = true
}

async function onSaved() {
  showForm.value = false
  editingAccount.value = null
  await refresh()
  setInfo('계정이 저장되었습니다.')
}

async function remove(acc: Account) {
  if (!confirm(`계정 "${acc.name}" 을(를) 삭제할까요? 키체인 자격증명과 이 계정의 일정 캐시도 함께 삭제됩니다.`)) return
  try {
    await AccountService.Delete(acc.id)
    await refresh()
    setInfo('계정이 삭제되었습니다.')
  } catch (e) {
    setError((e as Error).message)
  }
}

// Google 재인증. 새 토큰 하나로 메일과 캘린더의 인증이 함께 복구된다.
async function reconnect(acc: Account) {
  if (reconnectingAccountId.value) return
  reconnectingAccountId.value = acc.id
  try {
    await AccountService.GoogleReconnect(acc.id)
    await refresh()
    setInfo(`"${acc.name}" 계정을 다시 인증했습니다.`)
  } catch (e) {
    setError((e as Error).message)
  } finally {
    reconnectingAccountId.value = ''
  }
}

function setError(message: string) {
  error.value = message
  info.value = ''
}

function setInfo(message: string) {
  info.value = message
  error.value = ''
  setTimeout(() => { if (info.value === message) info.value = '' }, 3500)
}

function authLabel(acc: Account): string {
  return acc.authType === 'oauth2' ? 'Google OAuth' : '비밀번호'
}

function calendarLabel(acc: Account): string {
  return acc.calendar?.source === 'caldav' ? 'CalDAV' : '로컬'
}

function formatDate(value?: string): string {
  if (!value) return ''
  const date = new Date(value)
  return isNaN(date.getTime()) ? value : date.toLocaleString()
}

onMounted(refresh)
</script>

<template>
  <div class="accounts-layout">
    <header class="accounts-header">
      <div>
        <h1>계정</h1>
        <p class="subtitle">메일과 캘린더가 함께 사용하는 계정을 한곳에서 관리합니다.</p>
      </div>
      <button class="btn primary" @click="openNew">계정 추가</button>
    </header>

    <div v-if="error" class="alert error">{{ error }}</div>
    <div v-if="info" class="alert info">{{ info }}</div>

    <div class="accounts-body">
      <div v-if="loading" class="state">불러오는 중…</div>
      <div v-else-if="accounts.length === 0" class="state">
        <p>등록된 계정이 없습니다.</p>
        <button class="btn primary" @click="openNew">첫 계정 추가</button>
      </div>

      <ul v-else class="account-cards">
        <li v-for="acc in accounts" :key="acc.id" class="account-card">
          <div class="card-main">
            <div class="card-title">
              <span
                class="color-dot"
                :style="{ background: acc.calendar?.color || 'var(--border)' }"
                aria-hidden="true"
              ></span>
              <strong>{{ acc.name }}</strong>
              <span class="badge">{{ authLabel(acc) }}</span>
              <span v-if="acc.mail" class="badge feature">메일</span>
              <span v-if="acc.calendar" class="badge feature">캘린더 · {{ calendarLabel(acc) }}</span>
            </div>
            <div class="card-meta">{{ acc.email || '이메일 주소 없음' }}</div>
            <div v-if="acc.calendar?.lastSyncAt" class="card-meta">
              마지막 동기화 {{ formatDate(acc.calendar.lastSyncAt) }}
            </div>
            <div v-if="acc.authError" class="card-auth-error">
              ⚠ 인증이 만료되었습니다 — 재인증이 필요합니다
              <span class="detail">{{ acc.authError }}</span>
            </div>
          </div>

          <div class="card-actions">
            <button
              v-if="acc.authError && acc.authType === 'oauth2'"
              class="btn sm warning"
              :disabled="!!reconnectingAccountId"
              @click="reconnect(acc)"
            >{{ reconnectingAccountId === acc.id ? '인증 중…' : '재인증' }}</button>
            <button class="btn sm" @click="openEdit(acc)">편집</button>
            <button class="btn sm danger" @click="remove(acc)">삭제</button>
          </div>
        </li>
      </ul>
    </div>

    <AccountForm
      v-if="showForm"
      :account="editingAccount"
      @close="showForm = false"
      @saved="onSaved"
    />
  </div>
</template>

<style scoped>
.accounts-layout {
  display: flex;
  flex-direction: column;
  width: 100%;
  height: 100%;
  min-width: 0;
  overflow: hidden;
  color: var(--text);
}
.accounts-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  padding: 18px 24px 14px;
  border-bottom: 1px solid var(--border);
}
.accounts-header h1 { margin: 0; font-size: 18px; }
.subtitle { margin: 4px 0 0; color: var(--muted); font-size: 12px; }
.accounts-body { flex: 1; min-height: 0; overflow: auto; padding: 16px 24px 24px; }
.state { padding: 60px 24px; text-align: center; color: var(--muted); }
.state .btn { margin-top: 12px; }

.account-cards { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 10px; }
.account-card {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 14px;
  padding: 14px 16px;
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 8px;
}
.card-main { min-width: 0; }
.card-title { display: flex; align-items: center; flex-wrap: wrap; gap: 7px; }
.color-dot { width: 10px; height: 10px; border-radius: 50%; flex-shrink: 0; }
.card-meta { margin-top: 5px; color: var(--muted); font-size: 12px; overflow-wrap: anywhere; }
.card-auth-error {
  display: flex;
  flex-direction: column;
  gap: 2px;
  margin-top: 8px;
  padding: 7px 10px;
  border-radius: 6px;
  background: rgba(255, 198, 92, 0.14);
  color: #ffc65c;
  font-size: 12px;
}
.card-auth-error .detail { color: var(--muted); font-size: 11px; overflow-wrap: anywhere; }
.card-actions { display: flex; flex-shrink: 0; gap: 6px; }

.badge {
  padding: 1px 6px;
  border-radius: 3px;
  background: var(--border);
  color: var(--muted);
  font-size: 10px;
}
.badge.feature { background: rgba(79, 124, 255, 0.16); color: var(--accent); }

.btn {
  padding: 6px 12px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--panel-2);
  color: var(--text);
}
.btn:hover:not(:disabled) { background: var(--border); }
.btn.sm { padding: 4px 9px; font-size: 12px; }
.btn.primary { background: var(--accent); border-color: var(--accent); color: var(--on-accent); }
.btn.danger { color: var(--danger); }
.btn.warning { color: #ffc65c; border-color: rgba(255, 198, 92, 0.5); }
.btn:disabled { opacity: 0.5; cursor: not-allowed; }

.alert { margin: 10px 24px 0; padding: 8px 12px; border-radius: 6px; font-size: 13px; }
.alert.error { background: rgba(255, 90, 106, 0.12); color: var(--danger); }
.alert.info { background: rgba(56, 211, 159, 0.12); color: var(--ok); }
</style>
