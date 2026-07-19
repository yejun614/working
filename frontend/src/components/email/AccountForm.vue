<script setup lang="ts">
import { ref, computed } from 'vue'
import { Service as EmailService } from '../../../bindings/working/internal/modules/email'
import type { Account, ServerConfig } from '../../../bindings/working/internal/modules/email/account/models'

const props = defineProps<{ account: Account | null }>()
const emit = defineEmits<{
  (e: 'close'): void
  (e: 'saved'): void
}>()

const isEdit = computed(() => !!props.account)

const name = ref(props.account?.name || '')
const email = ref(props.account?.email || '')
const displayName = ref(props.account?.displayName || '')
const authType = ref(props.account?.authType || 'password')
const credential = ref('')

const smtpHost = ref(props.account?.smtp?.host || '')
const smtpPort = ref(props.account?.smtp?.port || 587)
const smtpEncryption = ref(props.account?.smtp?.encryption || 'starttls')

const imapHost = ref(props.account?.imap?.host || '')
const imapPort = ref(props.account?.imap?.port || 993)
const imapEncryption = ref(props.account?.imap?.encryption || 'tls')

const error = ref('')
const saving = ref(false)

async function save() {
  error.value = ''
  if (!email.value.trim()) { error.value = '이메일 주소는 필수입니다'; return }
  if (!isEdit.value && !credential.value) { error.value = '비밀번호/토큰은 필수입니다'; return }

  const smtp: ServerConfig | null = smtpHost.value
    ? { host: smtpHost.value.trim(), port: Number(smtpPort.value), encryption: smtpEncryption.value }
    : null
  const imap: ServerConfig | null = imapHost.value
    ? { host: imapHost.value.trim(), port: Number(imapPort.value), encryption: imapEncryption.value }
    : null

  const acc: Account = {
    id: props.account?.id || '',
    name: name.value.trim(),
    email: email.value.trim(),
    displayName: displayName.value.trim(),
    smtp,
    imap,
    authType: authType.value,
  }

  saving.value = true
  try {
    if (isEdit.value) {
      await EmailService.AccountUpdate(acc, credential.value)
    } else {
      await EmailService.AccountCreate(acc, credential.value)
    }
    emit('saved')
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <div class="modal-backdrop" @click.self="emit('close')">
    <div class="modal">
      <header class="modal-header">
        <h2>{{ isEdit ? '계정 편집' : '계정 추가' }}</h2>
        <button class="icon-btn" @click="emit('close')">✕</button>
      </header>

      <div class="modal-body">
        <div v-if="error" class="alert error">{{ error }}</div>

        <div class="grid">
          <label>
            <span>표시 이름</span>
            <input v-model="name" placeholder="회사 메일" />
          </label>
          <label>
            <span>이메일 *</span>
            <input v-model="email" type="email" :disabled="isEdit" />
          </label>
          <label>
            <span>발신자 이름</span>
            <input v-model="displayName" placeholder="홍길동" />
          </label>
          <label>
            <span>인증 방식</span>
            <select v-model="authType">
              <option value="password">비밀번호</option>
            </select>
          </label>
          <label class="full">
            <span>{{ isEdit ? '비밀번호/토큰 (변경 시에만 입력)' : '비밀번호/토큰 *' }}</span>
            <input v-model="credential" type="password" autocomplete="new-password" />
          </label>
        </div>

        <h3>SMTP (발송)</h3>
        <div class="grid">
          <label>
            <span>호스트</span>
            <input v-model="smtpHost" placeholder="smtp.example.com" />
          </label>
          <label>
            <span>포트</span>
            <input v-model.number="smtpPort" type="number" />
          </label>
          <label>
            <span>암호화</span>
            <select v-model="smtpEncryption">
              <option value="none">없음</option>
              <option value="starttls">STARTTLS</option>
              <option value="tls">TLS</option>
            </select>
          </label>
        </div>

        <h3>IMAP (수신)</h3>
        <div class="grid">
          <label>
            <span>호스트</span>
            <input v-model="imapHost" placeholder="imap.example.com" />
          </label>
          <label>
            <span>포트</span>
            <input v-model.number="imapPort" type="number" />
          </label>
          <label>
            <span>암호화</span>
            <select v-model="imapEncryption">
              <option value="none">없음</option>
              <option value="starttls">STARTTLS</option>
              <option value="tls">TLS</option>
            </select>
          </label>
        </div>
      </div>

      <footer class="modal-footer">
        <button class="btn" @click="emit('close')">취소</button>
        <button class="btn primary" :disabled="saving" @click="save">{{ saving ? '저장 중…' : '저장' }}</button>
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
label {
  display: flex; flex-direction: column; gap: 4px;
  font-size: 12px; color: var(--muted);
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
</style>