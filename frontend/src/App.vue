<script setup lang="ts">
import EmailView from './components/email/EmailView.vue'
</script>

<template>
  <EmailView />
</template>

<style>
:root {
  --bg: #06070f;
  --panel: #11131f;
  --panel-2: #181b2b;
  --border: #232739;
  --text: #e7eaf3;
  --muted: #9aa3b8;
  --accent: #4f7cff;
  --accent-hover: #6a92ff;
  --danger: #ff5a6a;
  --ok: #38d39f;
}

* {
  box-sizing: border-box;
}

html, body, #app {
  height: 100%;
  margin: 0;
  background: var(--bg);
  color: var(--text);
  font-family: "Inter", system-ui, -apple-system, "Segoe UI", Roboto, sans-serif;
  font-size: 14px;
}

button {
  font-family: inherit;
  cursor: pointer;
}
</style>