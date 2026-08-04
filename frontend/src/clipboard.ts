// copyText는 클립보드에 텍스트를 복사한다.
// WebView 환경에 따라 navigator.clipboard를 쓸 수 없는 경우가 있어
// 화면 밖 textarea와 execCommand로 대체한다.
export async function copyText(text: string): Promise<void> {
  if (navigator.clipboard?.writeText) {
    await navigator.clipboard.writeText(text)
    return
  }

  const textarea = document.createElement('textarea')
  textarea.value = text
  textarea.setAttribute('readonly', '')
  textarea.style.position = 'fixed'
  textarea.style.opacity = '0'
  document.body.appendChild(textarea)
  textarea.focus()
  textarea.select()
  try {
    if (!document.execCommand('copy')) {
      throw new Error('클립보드 복사 명령이 거부되었습니다.')
    }
  } finally {
    textarea.remove()
  }
}
