<script setup lang="ts">
import { ref } from 'vue'

const props = withDefaults(
  defineProps<{
    width: number
    min?: number
    max?: number
    /** 조절 대상 패널이 핸들의 어느 쪽에 있는지. left면 오른쪽으로 끌 때 넓어진다. */
    side?: 'left' | 'right'
    label?: string
  }>(),
  { min: 160, max: 640, side: 'left', label: '패널 너비 조절' },
)

const emit = defineEmits<{ (e: 'update:width', value: number): void }>()

const dragging = ref(false)

function apply(value: number) {
  emit('update:width', Math.min(props.max, Math.max(props.min, Math.round(value))))
}

function onPointerDown(event: PointerEvent) {
  if (event.button !== 0) return
  const handle = event.currentTarget as HTMLElement
  const startX = event.clientX
  const startWidth = props.width

  dragging.value = true
  // 포인터를 캡처해 두면 커서가 다른 패널이나 창 밖으로 나가도 이동 이벤트를 계속 받는다.
  handle.setPointerCapture(event.pointerId)
  // 드래그 중에는 문서 전체의 커서를 고정하고 텍스트 선택을 막는다.
  document.body.style.cursor = 'col-resize'
  document.body.style.userSelect = 'none'

  const onMove = (e: PointerEvent) => {
    const delta = e.clientX - startX
    apply(props.side === 'left' ? startWidth + delta : startWidth - delta)
  }
  const onUp = () => {
    dragging.value = false
    document.body.style.cursor = ''
    document.body.style.userSelect = ''
    handle.removeEventListener('pointermove', onMove)
    handle.removeEventListener('pointerup', onUp)
    handle.removeEventListener('pointercancel', onUp)
  }

  handle.addEventListener('pointermove', onMove)
  handle.addEventListener('pointerup', onUp)
  handle.addEventListener('pointercancel', onUp)
}

function onKeydown(event: KeyboardEvent) {
  const step = event.shiftKey ? 40 : 10
  if (event.key === 'ArrowLeft') apply(props.side === 'left' ? props.width - step : props.width + step)
  else if (event.key === 'ArrowRight') apply(props.side === 'left' ? props.width + step : props.width - step)
  else return
  event.preventDefault()
}
</script>

<template>
  <div
    class="resize-handle"
    :class="{ dragging }"
    role="separator"
    aria-orientation="vertical"
    :aria-label="label"
    :aria-valuenow="width"
    :aria-valuemin="min"
    :aria-valuemax="max"
    tabindex="0"
    @pointerdown="onPointerDown"
    @keydown="onKeydown"
  ></div>
</template>

<style scoped>
.resize-handle {
  width: 5px;
  flex: 0 0 5px;
  align-self: stretch;
  cursor: col-resize;
  background: transparent;
  /* 터치에서 스크롤 제스처에 뺏기지 않도록 한다. */
  touch-action: none;
  transition: background 0.12s ease;
}
.resize-handle:hover,
.resize-handle:focus-visible,
.resize-handle.dragging {
  background: var(--accent);
}
.resize-handle:focus {
  outline: none;
}
</style>
