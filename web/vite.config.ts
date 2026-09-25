import { fileURLToPath, URL } from 'node:url'
import { readFileSync } from 'node:fs'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'

const packageVersion = JSON.parse(readFileSync(new URL('./package.json', import.meta.url), 'utf8')).version as string

// Дев-сервер проксіює /api на бекенд DELMOS (127.0.0.1:10120, SYSTEM_REQUIREMENTS §6.1).
export default defineConfig({
  plugins: [vue()],
  define: {
    'import.meta.env.VITE_DELMOS_VERSION': JSON.stringify(packageVersion),
  },
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url)),
    },
  },
  build: {
    outDir: 'dist',
  },
})
