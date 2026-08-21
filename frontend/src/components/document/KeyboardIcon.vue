<script setup lang="ts">
// 자판 아이콘. 단축키 도움말 버튼이 쓴다.
// 자판알은 둥근 끝으로 처리한 점으로 그려 작은 크기에서도 뭉개지지 않는다.
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
    <rect x="2.5" y="6" width="19" height="12" rx="2.2" />
    <path d="M6.6 10h.01M10.2 10h.01M13.8 10h.01M17.4 10h.01" />
    <path d="M8 14h8" />
  </svg>
</template>
