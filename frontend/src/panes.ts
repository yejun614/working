import { ref, watch, type Ref } from 'vue'

const PREFIX = 'working:pane-width:'

function load(key: string, fallback: number): number {
  try {
    const raw = localStorage.getItem(PREFIX + key)
    if (raw === null) return fallback
    const parsed = Number(raw)
    return Number.isFinite(parsed) && parsed > 0 ? parsed : fallback
  } catch {
    return fallback
  }
}

/**
 * 드래그로 조절한 패널 너비(px)를 기억한다.
 * 모듈을 오갈 때마다 컴포넌트가 새로 만들어지므로 localStorage에 남겨 둔다.
 */
export function usePaneWidth(key: string, fallback: number): Ref<number> {
  const width = ref(load(key, fallback))
  watch(width, (value) => {
    try {
      localStorage.setItem(PREFIX + key, String(value))
    } catch {
      // 저장소를 쓸 수 없어도 현재 세션의 너비는 그대로 유지된다.
    }
  })
  return width
}
