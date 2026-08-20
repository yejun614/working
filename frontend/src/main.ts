import { createApp } from 'vue'
import App from './App.vue'
// @wailsio/runtime을 로드하면 창으로 끌어다 놓은 파일을 감지하는 전역 드랍
// 리스너가 붙는다. 문서 모듈이 lazy라 이 import를 그쪽에만 두면 문서 탭을
// 열기 전에는 드랍이 동작하지 않으므로, 앱 시작 시점에 여기서 로드한다.
import '@wailsio/runtime'

createApp(App).mount('#app')
