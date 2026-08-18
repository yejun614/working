<script setup lang="ts">
// 폴더에 더하기가 붙은 아이콘. 새 폴더와 하위 폴더 추가 버튼이 함께 쓴다.
// 버튼 크기가 달라 size로 변을 받고, 색은 버튼의 글자색을 따른다.
withDefaults(defineProps<{ size?: number }>(), { size: 17 })
</script>

<template>
  <svg
    :width="size"
    :height="size"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    stroke-width="1.9"
    stroke-linecap="round"
    stroke-linejoin="round"
    aria-hidden="true"
    focusable="false"
  >
    <path d="M3 17.5V6.5A1.5 1.5 0 0 1 4.5 5h4l2 2.5h9A1.5 1.5 0 0 1 21 9v8.5a1.5 1.5 0 0 1-1.5 1.5h-15A1.5 1.5 0 0 1 3 17.5z" />
    <path d="M12 10.5v6" />
    <path d="M9 13.5h6" />
  </svg>
</template>
