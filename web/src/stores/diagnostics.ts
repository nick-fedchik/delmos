// Діагностика сторінки для нижнього статус-бара (CORE_SHELL.md §6).
// Зберігаються лише безпечні дані: рівень, заголовок, код помилки, маршрут.
// Тіла відповідей, токени, заголовки запитів і сирі логи сюди не потрапляють.
import { defineStore } from 'pinia'

export type DiagnosticLevel = 'error' | 'warning'

export interface DiagnosticEntry {
  id: number
  level: DiagnosticLevel
  title: string
  detail: string
  code: string
  path: string
  at: string
}

const MAX_ENTRIES = 50

let nextId = 1

export const useDiagnosticsStore = defineStore('diagnostics', {
  state: () => ({
    entries: [] as DiagnosticEntry[],
  }),
  getters: {
    errorCount: (state) => state.entries.filter((entry) => entry.level === 'error').length,
    warningCount: (state) => state.entries.filter((entry) => entry.level === 'warning').length,
    latest: (state): DiagnosticEntry | undefined => state.entries[state.entries.length - 1],
  },
  actions: {
    report(level: DiagnosticLevel, title: string, detail: string, code = ''): void {
      this.entries.push({
        id: nextId++,
        level,
        title,
        detail,
        code,
        path: window.location.pathname + window.location.search,
        at: new Date().toISOString(),
      })
      if (this.entries.length > MAX_ENTRIES) {
        this.entries.splice(0, this.entries.length - MAX_ENTRIES)
      }
    },
    clear(): void {
      this.entries = []
    },
  },
})
