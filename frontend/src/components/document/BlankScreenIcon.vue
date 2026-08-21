<script setup lang="ts">
// 아무것도 없는 화면 아이콘. 문서를 닫고 화면을 비우는 버튼이 쓴다.
// 안을 비워 두어야 "빈 화면"으로 읽히므로 테두리만 그린다.
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
    <rect x="3" y="4.5" width="18" height="15" rx="2.2" />
    <!-- 화면이라는 것이 드러나도록 아래에 받침을 짧게 붙인다. -->
    <path d="M9 22h6" />
  </svg>
</template>
