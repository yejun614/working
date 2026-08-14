import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
import wails from "@wailsio/runtime/plugins/vite";

// https://vitejs.dev/config/
export default defineConfig({
  css: {
    lightningcss: {
      // TOAST UI 플러그인이 함께 가져오는 tui-color-picker CSS에는 옛 IE용
      // 스타 해크(*display 등)가 남아 있다. 최신 브라우저는 무시하는 문법이라
      // 빌드를 멈추는 대신 걷어내고 진행한다.
      errorRecovery: true,
    },
  },
  server: {
    host: "127.0.0.1",
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
  },
  plugins: [vue(), wails("./bindings")],
});
