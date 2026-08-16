import { createApp } from 'vue'
import App from './App.vue'
import { setupIMEFix } from './ime'

setupIMEFix()

createApp(App).mount('#app')
