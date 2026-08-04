<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { Service as AccountService } from '../../../bindings/working/internal/modules/account'
import type { Account, ServerConfig } from '../../../bindings/working/internal/modules/account/types/models'

const props = defineProps<{ account: Account | null }>()
const emit = defineEmits<{ (e: 'close'): void; (e: 'saved'): void }>()

const isEdit = computed(() => !!props.account?.id)

const name = ref('')
const email = ref('')
const displayName = ref('')
const authType = ref<'password' | 'oauth2'>('password')
const credential = ref('')

const useMail = ref(true)
const smtp = ref<ServerConfig>({ host: '', port: 587, encryption: 'starttls' })
const imap = ref<ServerConfig>({ host: '', port: 993, encryption: 'tls' })

const useCalendar = ref(false)
const calendarSource = ref<'local' | 'caldav'>('caldav')
const caldavURL = ref('')
const caldavUsername = ref('')
const color = ref('#4f7cff')
const syncEnabled = ref(true)

const saving = ref(false)
const error = ref('')
const providerNote = ref('')

// OAuth 계정은 브라우저 인증으로 토큰을 받으므로 서버 설정과 비밀번호 입력이 필요 없다.
const isOAuth = computed(() => authType.value === 'oauth2')

watch(
  () => props.account,
  acc => {
    if (!acc) return
    name.value = acc.name || ''
    email.value = acc.email || ''
    displayName.value = acc.displayName || ''
    authType.value = acc.authType === 'oauth2' ? 'oauth2' : 'password'

    useMail.value = !!acc.mail
    if (acc.mail?.smtp) smtp.value = { ...acc.mail.smtp }
    if (acc.mail?.imap) imap.value = { ...acc.mail.imap }

    useCalendar.value = !!acc.calendar
    if (acc.calendar) {
      calendarSource.value = acc.calendar.source === 'local' ? 'local' : 'caldav'
      caldavURL.value = acc.calendar.url || ''
      caldavUsername.value = acc.calendar.username || ''
      color.value = acc.calendar.color || '#4f7cff'
      syncEnabled.value = acc.calendar.syncEnabled !== false
    }
  },
  { immediate: true }
)

// 이메일 도메인으로 메일/캘린더 서버 설정을 한 번에 채운다.
async function autofillFromEmail() {
  const address = email.value.trim()
  if (!address.includes('@')) return
  try {
    const [mailProvider, calendarProvider] = await Promise.all([
      AccountService.MailProviderLookup(address),
      AccountService.CalendarProviderLookup(address),
    ])
    if (mailProvider) {
      if (mailProvider.smtp) smtp.value = { ...mailProvider.smtp }
      if (mailProvider.imap) imap.value = { ...mailProvider.imap }
      providerNote.value = mailProvider.note || ''
      if (!name.value.trim()) name.value = mailProvider.name
    }
    if (calendarProvider?.caldavUrl && !caldavURL.value.trim()) {
      caldavURL.value = calendarProvider.caldavUrl
    }
  } catch (e) {
    error.value = (e as Error).message
  }
}

function buildAccount(): Account {
  return {
    id: props.account?.id || '',
    name: name.value.trim(),
    email: email.value.trim(),
    displayName: displayName.value.trim(),
    authType: authType.value,
    authError: props.account?.authError || '',
    mail: useMail.value
      ? {
          smtp: isOAuth.value ? undefined : { ...smtp.value },
          imap: isOAuth.value ? undefined : { ...imap.value },
        }
      : undefined,
    calendar: useCalendar.value
      ? {
          source: calendarSource.value,
          url: calendarSource.value === 'caldav' ? caldavURL.value.trim() : '',
          username: calendarSource.value === 'caldav' ? caldavUsername.value.trim() : '',
          color: color.value,
          syncEnabled: calendarSource.value === 'caldav' ? syncEnabled.value : false,
          lastSyncAt: props.account?.calendar?.lastSyncAt || '',
        }
      : undefined,
  } as Account
}

async function save() {
  error.value = ''
  saving.value = true
  try {
    const acc = buildAccount()
    if (isEdit.value) {
      await AccountService.Update(acc, credential.value)
    } else {
      await AccountService.Create(acc, credential.value)
    }
    emit('saved')
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    saving.value = false
  }
}

// Google 계정은 브라우저에서 인증한 뒤 토큰 하나로 Gmail과 캘린더를 모두 사용한다.
async function connectGoogle() {
  error.value = ''
  saving.value = true
  try {
    authType.value = 'oauth2'
    await AccountService.GoogleConnect(buildAccount())
    emit('saved')
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <!-- 배경 클릭으로는 닫지 않는다. 입력 중인 계정 정보가 실수로 사라지는 것을 막기 위함. -->
  <div class="modal-backdrop">
    <form class="account-modal" @submit.prevent="save">
      <header>
        <h3>{{ isEdit ? '계정 편집' : '계정 추가' }}</h3>
        <button type="button" class="icon-btn" @click="emit('close')">✕</button>
      </header>

      <div class="form-body">
        <div v-if="error" class="alert error">{{ error }}</div>

        <label>계정 이름 *
          <input v-model="name" placeholder="회사 계정" />
        </label>
        <label>이메일 주소
          <input v-model="email" type="email" placeholder="me@example.com" @blur="autofillFromEmail" />
        </label>
        <label>발신자 표시 이름
          <input v-model="displayName" placeholder="홍길동" />
        </label>
        <label>인증 방식
          <select v-model="authType" :disabled="isEdit">
            <option value="password">비밀번호 / 앱 비밀번호</option>
            <option value="oauth2">Google OAuth</option>
          </select>
        </label>
        <p v-if="providerNote" class="note">{{ providerNote }}</p>

        <section class="feature">
          <label class="toggle">
            <input type="checkbox" v-model="useMail" />
            <span>메일 사용</span>
          </label>
          <div v-if="useMail && !isOAuth" class="feature-body">
            <div class="grid">
              <label>SMTP 호스트<input v-model="smtp.host" /></label>
              <label>포트<input v-model.number="smtp.port" type="number" /></label>
              <label>암호화
                <select v-model="smtp.encryption">
                  <option value="none">none</option>
                  <option value="starttls">STARTTLS</option>
                  <option value="tls">TLS</option>
                </select>
              </label>
            </div>
            <div class="grid">
              <label>IMAP 호스트<input v-model="imap.host" /></label>
              <label>포트<input v-model.number="imap.port" type="number" /></label>
              <label>암호화
                <select v-model="imap.encryption">
                  <option value="none">none</option>
                  <option value="starttls">STARTTLS</option>
                  <option value="tls">TLS</option>
                </select>
              </label>
            </div>
          </div>
          <p v-else-if="useMail" class="note">Gmail API로 송수신하므로 SMTP/IMAP 설정이 필요 없습니다.</p>
        </section>

        <section class="feature">
          <label class="toggle">
            <input type="checkbox" v-model="useCalendar" />
            <span>캘린더 사용</span>
          </label>
          <div v-if="useCalendar" class="feature-body">
            <label>저장소
              <select v-model="calendarSource" :disabled="isOAuth">
                <option value="caldav">CalDAV 서버</option>
                <option value="local">로컬 전용</option>
              </select>
            </label>
            <template v-if="calendarSource === 'caldav' && !isOAuth">
              <label>CalDAV URL<input v-model="caldavURL" placeholder="https://caldav.example.com" /></label>
              <label>CalDAV 사용자 이름 <small>비우면 이메일 주소를 사용합니다</small>
                <input v-model="caldavUsername" />
              </label>
            </template>
            <div class="grid two">
              <label>표시 색상<input v-model="color" type="color" /></label>
              <label v-if="calendarSource === 'caldav'" class="toggle inline">
                <input type="checkbox" v-model="syncEnabled" />
                <span>동기화 사용</span>
              </label>
            </div>
          </div>
        </section>

        <label v-if="!isOAuth">
          비밀번호 / 앱 비밀번호 <small v-if="isEdit">비워 두면 기존 값을 유지합니다</small>
          <input v-model="credential" type="password" autocomplete="new-password" />
        </label>
        <p v-else class="note">
          Google 계정은 아래 버튼으로 인증합니다. 발급된 토큰 하나를 Gmail과 Google 캘린더가 함께 사용합니다.
        </p>
      </div>

      <footer>
        <button type="button" class="btn" @click="emit('close')">취소</button>
        <button v-if="isOAuth" type="button" class="btn primary" :disabled="saving" @click="connectGoogle">
          {{ saving ? '인증 중…' : 'Google 인증' }}
        </button>
        <button v-else class="btn primary" :disabled="saving">{{ saving ? '저장 중…' : '저장' }}</button>
      </footer>
    </form>
  </div>
</template>

<style scoped>
.modal-backdrop {
  position: fixed;
  inset: 0;
  z-index: 30;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.55);
}
.account-modal {
  width: 560px;
  max-height: 90vh;
  display: flex;
  flex-direction: column;
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 9px;
  color: var(--text);
}
.account-modal header,
.account-modal footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 13px 16px;
  border-bottom: 1px solid var(--border);
}
.account-modal footer {
  border-top: 1px solid var(--border);
  border-bottom: 0;
  justify-content: flex-end;
  gap: 6px;
}
.account-modal h3 { margin: 0; font-size: 15px; }
.form-body {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 16px;
  overflow: auto;
}
.form-body label {
  display: flex;
  flex-direction: column;
  gap: 4px;
  color: var(--muted);
  font-size: 12px;
}
.form-body small { color: var(--muted); font-size: 10px; }
.form-body input,
.form-body select {
  padding: 8px;
  border: 1px solid var(--border);
  border-radius: 5px;
  background: var(--panel-2);
  color: var(--text);
  font: inherit;
}
.form-body input[type='color'] { padding: 2px; height: 32px; }
.grid { display: grid; grid-template-columns: 2fr 1fr 1fr; gap: 8px; }
.grid.two { grid-template-columns: 1fr 1fr; align-items: end; }
.feature {
  padding: 10px 12px;
  border: 1px solid var(--border);
  border-radius: 7px;
  background: var(--panel-2);
}
.feature-body { display: flex; flex-direction: column; gap: 8px; margin-top: 10px; }
.toggle { flex-direction: row !important; align-items: center; gap: 7px; color: var(--text); font-size: 13px; }
.toggle input { width: 14px; height: 14px; accent-color: var(--accent); }
.toggle.inline { margin-bottom: 8px; }
.note { margin: 0; color: var(--muted); font-size: 11px; line-height: 1.5; }
.btn {
  padding: 7px 12px;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--panel-2);
  color: var(--text);
}
.btn.primary { background: var(--accent); border-color: var(--accent); }
.btn:disabled { opacity: 0.5; cursor: not-allowed; }
.icon-btn {
  background: transparent;
  border: 1px solid var(--border);
  color: var(--text);
  border-radius: 4px;
  width: 25px;
  height: 25px;
}
.alert.error {
  padding: 8px 12px;
  border-radius: 6px;
  background: rgba(255, 90, 106, 0.12);
  color: var(--danger);
  font-size: 12px;
}
</style>
