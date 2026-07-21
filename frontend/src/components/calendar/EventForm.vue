<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { Service as CalendarService } from '../../../bindings/working/internal/modules/calendar'
import type { Event } from '../../../bindings/working/internal/modules/calendar/types/models'
import type { Account } from '../../../bindings/working/internal/modules/calendar/account/models'

const props = defineProps<{
  accounts: Account[]
  event: Event | null
  defaultCalendarId: string
  defaultDate: string // YYYY-MM-DD
}>()
const emit = defineEmits<{
  (e: 'close'): void
  (e: 'saved'): void
}>()

const isEdit = computed(() => !!props.event)

const calendarId = ref(props.event?.calendarId || props.defaultCalendarId || props.accounts[0]?.id || '')
const title = ref(props.event?.title || '')
const description = ref(props.event?.description || '')
const location = ref(props.event?.location || '')
const allDay = ref(props.event?.allDay || false)

const startDate = ref(extractDate(props.event?.start || props.defaultDate))
const startTime = ref(extractTime(props.event?.start) || '09:00')
const endDate = ref(extractDate(props.event?.end || props.defaultDate))
const endTime = ref(extractTime(props.event?.end) || '10:00')

const attendeesText = ref((props.event?.attendees || []).join(', '))
const recurrenceRule = ref(props.event?.recurrenceRule || '')

const error = ref('')
const saving = ref(false)

const selectedAccount = computed(() =>
  props.accounts.find(a => a.id === calendarId.value) || null
)

watch(allDay, (v) => {
  if (v) {
    startTime.value = '00:00'
    endTime.value = '23:59'
  }
})

function extractDate(dt: string): string {
  if (!dt) return ''
  const d = new Date(dt)
  if (isNaN(d.getTime())) return dt.slice(0, 10)
  return d.toISOString().slice(0, 10)
}

function extractTime(dt?: string): string {
  if (!dt) return ''
  const d = new Date(dt)
  if (isNaN(d.getTime())) return ''
  return d.toTimeString().slice(0, 5)
}

function toRFC3339(date: string, time: string, endOfDay = false): string {
  if (allDay.value) {
    return new Date(date + 'T00:00:00').toISOString()
  }
  if (endOfDay && time === '23:59') {
    return new Date(date + 'T23:59:59').toISOString()
  }
  return new Date(date + 'T' + time + ':00').toISOString()
}

async function save() {
  error.value = ''
  if (!calendarId.value) { error.value = '캘린더를 선택하세요'; return }
  if (!title.value.trim()) { error.value = '제목은 필수입니다'; return }
  if (!startDate.value || !endDate.value) { error.value = '시작/종료 날짜는 필수입니다'; return }

  const ev: Event = {
    uid: props.event?.uid || '',
    calendarId: calendarId.value,
    title: title.value.trim(),
    description: description.value.trim(),
    location: location.value.trim(),
    start: toRFC3339(startDate.value, startTime.value),
    end: toRFC3339(endDate.value, endTime.value, true),
    allDay: allDay.value,
    attendees: attendeesText.value.trim() ? attendeesText.value.split(',').map(s => s.trim()).filter(Boolean) : [],
    recurrenceRule: recurrenceRule.value.trim(),
    etag: props.event?.etag || '',
    href: props.event?.href || '',
  }

  saving.value = true
  try {
    if (isEdit.value) {
      await CalendarService.EventUpdate(ev)
    } else {
      await CalendarService.EventCreate(ev)
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
        <h2>{{ isEdit ? '일정 편집' : '일정 추가' }}</h2>
        <button class="icon-btn" @click="emit('close')">✕</button>
      </header>

      <div class="modal-body">
        <div v-if="error" class="alert error">{{ error }}</div>

        <label class="field">
          <span>캘린더</span>
          <select v-model="calendarId">
            <option v-for="a in accounts" :key="a.id" :value="a.id">{{ a.name }}</option>
          </select>
        </label>

        <label class="field">
          <span>제목 *</span>
          <input v-model="title" placeholder="일정 제목" />
        </label>

        <div class="datetime-row">
          <label class="checkbox-inline">
            <input type="checkbox" v-model="allDay" />
            <span>종일</span>
          </label>
        </div>

        <div class="grid">
          <div class="field">
            <span>시작 날짜 *</span>
            <input v-model="startDate" type="date" />
          </div>
          <div class="field" v-if="!allDay">
            <span>시작 시간</span>
            <input v-model="startTime" type="time" />
          </div>
          <div class="field">
            <span>종료 날짜 *</span>
            <input v-model="endDate" type="date" />
          </div>
          <div class="field" v-if="!allDay">
            <span>종료 시간</span>
            <input v-model="endTime" type="time" />
          </div>
        </div>

        <label class="field">
          <span>장소</span>
          <input v-model="location" placeholder="회의실 A" />
        </label>

        <label class="field">
          <span>설명</span>
          <textarea v-model="description" rows="4"></textarea>
        </label>

        <label class="field">
          <span>참석자 (콤마로 구분)</span>
          <input v-model="attendeesText" placeholder="a@example.com, b@example.com" />
        </label>

        <label class="field">
          <span>반복 규칙 (RRULE, 선택)</span>
          <input v-model="recurrenceRule" placeholder="FREQ=WEEKLY;BYDAY=MO" />
        </label>
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
  width: 520px;
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
.field {
  display: flex; flex-direction: column; gap: 4px;
  font-size: 12px; color: var(--muted);
}
.grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
}
.datetime-row { display: flex; gap: 8px; }
.checkbox-inline {
  display: flex; align-items: center; gap: 6px;
  color: var(--text);
  font-size: 13px;
  cursor: pointer;
}
input, select, textarea {
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