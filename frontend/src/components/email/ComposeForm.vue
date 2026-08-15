<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import Editor from '@toast-ui/editor'
import '@toast-ui/editor/dist/toastui-editor.css'
import '@toast-ui/editor/dist/theme/toastui-editor-dark.css'
import { Service as EmailService } from '../../../bindings/working/internal/modules/email'
import type { Account } from '../../../bindings/working/internal/modules/account/types/models'
import { canSendMail } from '../../accounts'
import { undoSendEnabled } from '../../mail'
import type { Message } from '../../../bindings/working/internal/modules/email/types/models'

const props = defineProps<{
  account: Account | null
  accounts?: Account[]
  initialAccountId?: string
  replyTo?: Message | null
  draft?: Message | null
  embedded?: boolean
}>()
const emit = defineEmits<{
  (e: 'close'): void
  (e: 'sent'): void
  // 전송 취소 시간을 쓰는 경우, 실제 발송은 부모가 시간을 두고 처리한다.
  (e: 'schedule', payload: { accountId: string; message: Message }): void
  (e: 'changed', message: Message): void
}>()

const initialSenderAccountId = (props.accounts || []).find(account =>
  account.id === props.draft?.from || account.email === props.draft?.from
)?.id || props.initialAccountId || props.account?.id || ''
const senderAccountId = ref(initialSenderAccountId)
const senderAccount = () =>
  (props.accounts || []).find(account => account.id === senderAccountId.value) || props.account
const to = ref(props.draft?.to || props.replyTo?.from || '')
const cc = ref(props.draft?.cc || '')
const subject = ref(props.draft?.subject || replySubject(props.replyTo?.subject || ''))
const body = ref(props.draft?.body || replyBody(props.replyTo?.body || ''))
const error = ref('')
const sending = ref(false)
const editorElement = ref<HTMLElement | null>(null)
let editor: Editor | null = null

function emitChanged() {
  const account = senderAccount()
  emit('changed', {
    uid: props.draft?.uid,
    from: account?.email || '',
    to: to.value,
    cc: cc.value,
    subject: subject.value,
    body: body.value,
  })
}

function onEditorChanged() {
  body.value = editor?.getMarkdown() || ''
  emitChanged()
}

function replySubject(original: string): string {
  return /^re:/i.test(original.trim()) ? original : `Re: ${original}`
}

function replyBody(original: string): string {
  if (!original.trim()) return ''
  const quoted = original
    .split('\n')
    .map(line => `> ${line}`)
    .join('\n')
  return `\n\n--- 원본 메시지 ---\n${quoted}`
}

onMounted(() => {
  if (!editorElement.value) return
  editor = new Editor({
    el: editorElement.value,
    height: '420px',
    initialEditType: 'markdown',
    previewStyle: 'vertical',
    theme: 'dark',
    initialValue: body.value,
  })
  editor.on('change', onEditorChanged)
})

onBeforeUnmount(() => {
  editor?.destroy()
  editor = null
})

async function send() {
  error.value = ''
  const account = senderAccount()
  if (!account) { error.value = '보내는 계정을 선택하세요'; return }
  if (!canSendMail(account)) { error.value = '선택한 계정은 메일 발송 설정이 없습니다'; return }
  if (!to.value.trim()) { error.value = '받는 사람은 필수입니다'; return }
  if (!subject.value.trim() && !body.value.trim()) { error.value = '제목 또는 본문이 필요합니다'; return }

  const msg: Message = {
    from: account.email || '',
    to: to.value.trim(),
    cc: cc.value.trim(),
    subject: subject.value,
    body: body.value,
  }
  // 전송 취소 시간을 쓰면 여기서 보내지 않고 부모에게 넘긴다.
  // 부모가 알림을 띄우고 시간이 지난 뒤에 실제로 보낸다.
  if (undoSendEnabled.value) {
    emit('schedule', { accountId: account.id, message: msg })
    return
  }

  sending.value = true
  try {
    await EmailService.Send(account.id, msg)
    emit('sent')
  } catch (e) {
    error.value = (e as Error).message
  } finally {
    sending.value = false
  }
}
</script>

<template>
  <div :class="props.embedded ? 'compose-detail' : 'modal-backdrop'" @click.self="emit('close')">
    <div :class="props.embedded ? 'compose-editor' : 'modal'">
      <header class="modal-header">
        <h2>{{ draft ? '임시 메일 이어쓰기' : replyTo ? '답장 작성' : '메일 쓰기' }}</h2>
        <button class="icon-btn" @click="emit('close')">✕</button>
      </header>

      <div class="modal-body">
        <div v-if="error" class="alert error">{{ error }}</div>

        <label>
          <span>보내는 사람 *</span>
          <select v-model="senderAccountId" @change="emitChanged">
            <option
              v-for="account in (accounts || []).filter(canSendMail)"
              :key="account.id"
              :value="account.id"
            >{{ account.name || account.email }} ({{ account.email }})</option>
          </select>
        </label>

        <label>
          <span>받는 사람 *</span>
          <input v-model="to" placeholder="recipient@example.com" @input="emitChanged" />
        </label>
        <label>
          <span>참조</span>
          <input v-model="cc" placeholder="cc@example.com" @input="emitChanged" />
        </label>
        <label>
          <span>제목</span>
          <input v-model="subject" @input="emitChanged" />
        </label>
        <div class="field">
          <span>본문</span>
          <div ref="editorElement" class="toast-editor"></div>
        </div>
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
.compose-detail {
  display: flex;
  flex: 1;
  min-height: 0;
  overflow: auto;
}
.compose-editor {
  display: flex;
  flex: 1;
  min-height: 100%;
  flex-direction: column;
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
.field, label {
  display: flex; flex-direction: column; gap: 4px;
  font-size: 12px; color: var(--muted);
}
input, textarea, select {
  background: var(--panel-2);
  border: 1px solid var(--border);
  color: var(--text);
  border-radius: 6px;
  padding: 7px 9px;
  font-size: 13px;
  font-family: inherit;
  resize: vertical;
}
.toast-editor {
  width: 100%;
  min-height: 420px;
}
.toast-editor :deep(.toastui-editor-defaultUI) {
  border-color: var(--border);
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
.btn.primary { background: var(--accent); border-color: var(--accent); color: var(--on-accent); }
.btn.primary:hover { background: var(--accent-hover); }
.btn:disabled { opacity: 0.5; cursor: not-allowed; }
.alert {
  padding: 8px 12px;
  border-radius: 6px;
  font-size: 13px;
}
.alert.error { background: rgba(255,90,106,0.12); color: var(--danger); }
</style>