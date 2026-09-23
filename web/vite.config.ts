import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

// Дев-сервер проксіює /api на бекенд DELMOS (127.0.0.1:10020, SYSTEM_REQUIREMENTS §6.1).
export default defineConfig({
  plugins: [vue()],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  server: {
    // Дозволяє доступ через тунелі переспрямування портів (VS Code remote/codespaces)
    // під час розробки; продакшн-збірка (vite build) цього не стосується.
    allowedHosts: true,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:10020',
        changeOrigin: false,
      },
    },
  },
  build: {
    outDir: 'dist',
  },
})
