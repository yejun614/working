<script setup lang="ts">
// 자물쇠 아이콘. open을 주면 고리가 열린 모양이라 잠기지 않은 상태를 나타낸다.
// 머리글 버튼과 목록 표시가 크기만 달리 해서 함께 쓴다.
withDefaults(defineProps<{ size?: number; open?: boolean }>(), { size: 18, open: false })
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
    <rect x="4.5" y="10.5" width="15" height="9.5" rx="1.9" />
    <!-- 잠긴 고리는 양쪽 다리가 몸통에 닿고, 열린 고리는 오른쪽 다리를 들어 올린다. -->
    <path v-if="!open" d="M8.3 10.5V7.4a3.7 3.7 0 0 1 7.4 0v3.1" />
    <path v-else d="M8.3 10.5V7.4a3.7 3.7 0 0 1 7.2-1.2" />
    <path d="M12 14.2v2.2" />
  </svg>
</template>
