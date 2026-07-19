<script setup lang="ts">
import { ref } from 'vue'
import { Service as EmailService } from '../../../bindings/working/internal/modules/email'
import type { Account } from '../../../bindings/working/internal/modules/email/account/models'
import type { Message } from '../../../bindings/working/internal/modules/email/types/models'

const props = defineProps<{ account: Account | null }>()
const emit = defineEmits<{
  (e: 'close'): void
  (e: 'sent'): void
}>()

const to = ref('')
const cc = ref('')
const subject = ref('')
const body = ref('')
const error = ref('')
const sending = ref(false)

async function send() {
  error.value = ''
  if (!props.account) { error.value = '계정이 선택되지 않았습니다'; return }
  if (!props.account.smtp) { error.value = '이 계정은 SMTP 설정이 없습니다'; return }
  if (!to.value.trim()) { error.value = '받는 사람은 필수입니다'; return }
  if (!subject.value.trim() && !body.value.trim()) { error.value = '제목 또는 본문이 필요합니다'; return }

  const msg: Message = {
    from: props.account.email,
    to: to.value.trim(),
    cc: cc.value.trim(),
    subject: subject.value,
    body: body.value,
  }
  sending.value = true
  try {
    await EmailService.Send(props.account.id, msg)
    emit('sent')
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    sending.value = false
  }
}
</script>

<template>
  <div class="modal-backdrop" @click.self="emit('close')">
    <div class="modal">
      <header class="modal-header">
        <h2>메일 쓰기</h2>
        <button class="icon-btn" @click="emit('close')">✕</button>
      </header>

      <div class="modal-body">
        <div v-if="error" class="alert error">{{ error }}</div>

        <label>
          <span>받는 사람 *</span>
          <input v-model="to" placeholder="recipient@example.com" />
        </label>
        <label>
          <span>참조</span>
          <input v-model="cc" placeholder="cc@example.com" />
        </label>
        <label>
          <span>제목</span>
          <input v-model="subject" />
        </label>
        <label>
          <span>본문</span>
          <textarea v-model="body" rows="10"></textarea>
        </label>
      </div>

      <footer class="modal-footer">
        <button class="btn" @click="emit('close')">취소</button>
        <button class="btn primary" :disabled="sending" @click="send">{{ sending ? '전송 중…' : '전송' }}</button>
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
.modal-body { padding: 16px 18px; overflow: auto; display: flex; flex-direction: column; gap: 10px; }
.modal-footer {
  display: flex; justify-content: flex-end; gap: 8px;
  padding: 12px 18px;
  border-top: 1px solid var(--border);
}
label {
  display: flex; flex-direction: column; gap: 4px;
  font-size: 12px; color: var(--muted);
}
input, textarea {
  background: var(--panel-2);
  border: 1px solid var(--border);
  color: var(--text);
  border-radius: 6px;
  padding: 7px 9px;
  font-size: 13px;
  font-family: inherit;
  resize: vertical;
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
  font-size: 13px;
}
.alert.error { background: rgba(255,90,106,0.12); color: var(--danger); }
</style>