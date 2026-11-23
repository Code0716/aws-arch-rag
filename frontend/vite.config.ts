import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

// 開発時は /api を Go の API サーバー（make server, :8080）へ中継する
export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      "/api": "http://localhost:8080",
    },
  },
});
