import { ref } from 'vue'

const STORAGE_KEY = 'working:dark-mode'

function loadDarkMode(): boolean {
  try {
    return localStorage.getItem(STORAGE_KEY) !== 'false'
  } catch {
    return true
  }
}

export const isDarkMode = ref(loadDarkMode())

export function applyTheme(value = isDarkMode.value) {
  isDarkMode.value = value
  document.documentElement.dataset.theme = value ? 'dark' : 'light'
  try {
    localStorage.setItem(STORAGE_KEY, String(value))
  } catch {
    // 저장소를 사용할 수 없는 환경에서도 현재 세션의 테마는 적용한다.
  }
}

applyTheme()
