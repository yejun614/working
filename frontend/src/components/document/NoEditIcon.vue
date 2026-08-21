<script setup lang="ts">
// 연필에 사선을 그은 아이콘. 읽기 전용(수정 금지)을 나타낸다.
// 사선은 연필과 직각으로 그어야 작은 크기에서도 두 모양이 겹쳐 보이지 않는다.
withDefaults(defineProps<{ size?: number }>(), { size: 18 })
</script>

<template>
  <svg
    :width="size"
    :height="size"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="2.2"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
    focusable="false"
  >
    <path d="M16.4 4.2a2.55 2.55 0 0 1 3.6 3.6L8.6 19.2l-4.4 1.1 1.1-4.4L16.4 4.2z" />
    <path d="M4.4 4.4l15.4 15.4" />
  </svg>
</template>
