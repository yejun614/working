<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { Service as CalendarService } from '../../../bindings/working/internal/modules/calendar'
import type { Account, Source } from '../../../bindings/working/internal/modules/calendar/account/models'
import { AuthType } from '../../../bindings/working/internal/modules/calendar/account/models'
import type { Provider } from '../../../bindings/working/internal/modules/calendar/provider/models'

const props = defineProps<{ account: Account | null }>()
const emit = defineEmits<{
  (e: 'close'): void
  (e: 'saved'): void
}>()

const isEdit = computed(() => !!props.account)

const name = ref(props.account?.name || '')
const source = ref<Source>(props.account?.source || ('local' as Source))
const color = ref(props.account?.color || '#4f7cff')
const caldavUrl = ref(props.account?.caldavUrl || '')
const username = ref(props.account?.username || '')
const credential = ref('')
const syncEnabled = ref(props.account?.syncEnabled ?? true)

const error = ref('')
const saving = ref(false)

const providers = ref<Provider[]>([])
const selectedProviderId = ref<string>('')
const detectedProvider = ref<Provider | null>(null)

const isGoogle = computed(() =>
  selectedProviderId.value === 'google' || caldavUrl.value.includes('apidata.googleusercontent.com/caldav/v2'),
)

async function loadProviders() {
  try {
    const list = await CalendarService.ProviderList()
    providers.value = list || []
  } catch (e) {
    providers.value = []
  }
}
loadProviders()

// 사용자가 수동으로 제공자를 선택했을 때 CalDAV URL 필드를 채운다.
watch(selectedProviderId, (id) => {
  if (!id) return
  const p = providers.value.find(p => p.id === id)
  if (!p) return
  applyProvider(p)
})

// 사용자 이름 입력 시 도메인 기반으로 제공자 자동 감지.
watch(username, async (val) => {
  if (!val || !val.includes('@')) {
    detectedProvider.value = null
    return
  }
  try {
    const p = await CalendarService.ProviderLookupByEmail(val)
    detectedProvider.value = p
    if (p) {
      selectedProviderId.value = p.id
      applyProvider(p)
    }
  } catch (e) {
    detectedProvider.value = null
  }
})

function applyProvider(p: Provider) {
  if (p.caldavUrl) {
    caldavUrl.value = p.caldavUrl
  }
  if (!name.value && p.name) {
    name.value = p.name
  }
}

async function save() {
  error.value = ''
  if (!name.value.trim()) { error.value = '계정 이름은 필수입니다'; return }
  if (source.value === 'caldav') {
    if (!caldavUrl.value.trim()) { error.value = 'CalDAV 서버 URL은 필수입니다'; return }
    if (!username.value.trim()) { error.value = '사용자 이름은 필수입니다'; return }
    if (!isEdit.value && !credential.value) { error.value = '비밀번호/토큰은 필수입니다'; return }
  }

  const acc: Account = {
    id: props.account?.id || '',
    name: name.value.trim(),
    source: source.value,
    color: color.value,
    caldavUrl: source.value === 'caldav' ? caldavUrl.value.trim() : '',
    username: source.value === 'caldav' ? username.value.trim() : '',
    authType: props.account?.authType || (isGoogle.value ? AuthType.AuthOAuth2 : AuthType.AuthBasic),
    syncEnabled: source.value === 'caldav' ? syncEnabled.value : false,
  }

  saving.value = true
  try {
    if (isEdit.value) {
      await CalendarService.AccountUpdate(acc, credential.value)
    } else {
      await CalendarService.AccountCreate(acc, credential.value)
    }
    emit('saved')
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    saving.value = false
  }
}

async function connectGoogle() {
  error.value = ''
  if (!name.value.trim()) { error.value = '계정 이름은 필수입니다'; return }
  if (!username.value.trim()) { error.value = 'Google 계정 이메일은 필수입니다'; return }

  const acc: Account = {
    id: props.account?.id || '',
    name: name.value.trim(),
    source: 'caldav' as Source,
    color: color.value,
    caldavUrl: caldavUrl.value.trim(),
    username: username.value.trim(),
    authType: AuthType.AuthOAuth2,
    syncEnabled: syncEnabled.value,
  }

  saving.value = true
  try {
    await CalendarService.GoogleOAuthConnect(acc)
    emit('saved')
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="modal-backdrop">
    <div class="modal">
      <header class="modal-header">
        <h2>{{ isEdit ? '캘린더 계정 편집' : '캘린더 계정 추가' }}</h2>
        <button class="icon-btn" @click="emit('close')">✕</button>
      </header>

      <div class="modal-body">
        <div v-if="error" class="alert error">{{ error }}</div>

        <div class="grid">
          <div class="field">
            <span>표시 이름 *</span>
            <input v-model="name" placeholder="회사 캘린더" />
          </div>
          <div class="field">
            <span>종류</span>
            <select v-model="source" :disabled="isEdit">
              <option value="local">로컬 (오프라인)</option>
              <option value="caldav">CalDAV 서버</option>
            </select>
          </div>
          <div class="field">
            <span>색상</span>
            <input v-model="color" type="color" class="color-input" />
          </div>
        </div>

        <template v-if="source === 'caldav'">
          <h3>CalDAV 서버</h3>
          <div class="provider-row">
            <div class="field provider-select">
              <span>제공자 (자동 입력)</span>
              <select v-model="selectedProviderId">
                <option value="">— 직접 입력 —</option>
                <option v-for="p in providers" :key="p.id" :value="p.id">{{ p.name }}</option>
              </select>
            </div>
            <div v-if="detectedProvider?.note" class="provider-note">
              <div class="note-text">{{ detectedProvider.note }}</div>
              <a v-if="detectedProvider.helpUrl" :href="detectedProvider.helpUrl" target="_blank" rel="noopener" class="note-link">도움말 ↗</a>
            </div>
          </div>

          <div class="grid">
            <div class="field full">
              <span>CalDAV URL *</span>
              <input v-model="caldavUrl" placeholder="https://caldav.example.com/principals/users/me/" />
            </div>
            <div class="field">
              <span>사용자 이름 *</span>
              <input v-model="username" placeholder="user@example.com" />
            </div>
            <div v-if="!isGoogle" class="field">
              <span>{{ isEdit ? '비밀번호/토큰 (변경 시)' : '비밀번호/토큰 *' }}</span>
              <input v-model="credential" type="password" autocomplete="new-password" />
            </div>
            <div v-else class="oauth-note">
              Google 계정은 비밀번호 대신 Google OAuth 인증을 사용합니다.
            </div>
            <div class="field checkbox-field">
              <label class="checkbox">
                <input type="checkbox" v-model="syncEnabled" />
                <span>동기화 활성화</span>
              </label>
            </div>
          </div>
        </template>
      </div>

      <footer class="modal-footer">
        <button class="btn" @click="emit('close')">취소</button>
        <button v-if="isGoogle && !isEdit" class="btn primary" :disabled="saving" @click="connectGoogle">{{ saving ? 'Google 인증 중…' : 'Google로 연결' }}</button>
        <button v-else class="btn primary" :disabled="saving" @click="save">{{ saving ? '저장 중…' : '저장' }}</button>
      </footer>
    </div>
  </div>
</template>

<style scoped>
.modal-backdrop {
  position: fixed; inset: 0;
  background: rgba(0,0,0,0.5);
  display: flex; align-items: center; justify-content: center;
  z-index: 100;
}
.modal {
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 10px;
  width: 560px;
  max-height: 90vh;
  display: flex;
  flex-direction: column;
}
.modal-header {
  display: flex; justify-content: space-between; align-items: center;
  padding: 14px 18px;
  border-bottom: 1px solid var(--border);
}
.modal-header h2 { margin: 0; font-size: 16px; }
.modal-body { padding: 16px 18px; overflow: auto; }
.modal-footer {
  display: flex; justify-content: flex-end; gap: 8px;
  padding: 12px 18px;
  border-top: 1px solid var(--border);
}
.grid {
  display: grid;
  grid-template-columns: 1fr 1fr 1fr;
  gap: 10px;
  margin-bottom: 12px;
}
.grid .full { grid-column: 1 / -1; }
.field {
  display: flex; flex-direction: column; gap: 4px;
  font-size: 12px; color: var(--muted);
}
.checkbox-field { justify-content: flex-end; }
.checkbox {
  display: flex; align-items: center; gap: 6px;
  cursor: pointer;
  color: var(--text);
  font-size: 13px;
}
input, select {
  background: var(--panel-2);
  border: 1px solid var(--border);
  color: var(--text);
  border-radius: 6px;
  padding: 7px 9px;
  font-size: 13px;
  font-family: inherit;
}
input:disabled { opacity: 0.6; }
.color-input { padding: 4px; height: 32px; }
h3 {
  margin: 12px 0 6px;
  font-size: 13px;
  color: var(--muted);
  text-transform: uppercase;
  letter-spacing: 0.5px;
}
.icon-btn {
  background: transparent;
  border: 1px solid var(--border);
  color: var(--text);
  border-radius: 4px;
  width: 24px; height: 24px;
}
.btn {
  background: var(--panel-2);
  color: var(--text);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 7px 14px;
}
.btn.primary { background: var(--accent); border-color: var(--accent); }
.btn.primary:hover { background: var(--accent-hover); }
.btn:disabled { opacity: 0.5; cursor: not-allowed; }
.alert {
  padding: 8px 12px;
  border-radius: 6px;
  margin-bottom: 10px;
  font-size: 13px;
}
.alert.error { background: rgba(255,90,106,0.12); color: var(--danger); }
.oauth-note {
  align-self: end;
  grid-column: span 2;
  padding: 8px 10px;
  border-radius: 6px;
  background: rgba(79,124,255,0.10);
  color: var(--muted);
  font-size: 12px;
}

.provider-row {
  display: flex;
  gap: 10px;
  margin-bottom: 12px;
  align-items: stretch;
}
.provider-select { flex: 0 0 220px; }
.provider-note {
  flex: 1;
  background: rgba(79,124,255,0.10);
  border: 1px solid rgba(79,124,255,0.35);
  border-radius: 6px;
  padding: 8px 10px;
  font-size: 12px;
  color: var(--text);
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.note-text { line-height: 1.4; }
.note-link {
  color: var(--accent);
  text-decoration: none;
  font-size: 11px;
  align-self: flex-start;
}
.note-link:hover { text-decoration: underline; }
</style>