/// <reference types="vite/client" />

// code-syntax-highlight 플러그인의 "-all" 번들은 Prism과 지원 언어를 모두 담고
// 있지만 타입 선언이 없다. 기본 진입점의 타입을 그대로 빌려 쓴다.
declare module '@toast-ui/editor-plugin-code-syntax-highlight/dist/toastui-editor-plugin-code-syntax-highlight-all' {
  import codeSyntaxHighlight from '@toast-ui/editor-plugin-code-syntax-highlight'
  export default codeSyntaxHighlight
}
