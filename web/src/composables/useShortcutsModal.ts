// Довідка зі скорочень клавіатури. Скорочення реєструються централізовано в
// оболонці (CORE_SHELL.md §7): інтерфейс лишається повністю керованим і без них.
import { ref } from 'vue'

export interface ShortcutEntry {
  keys: string
  description: string
}

export const SHORTCUTS: ShortcutEntry[] = [
  { keys: '[', description: 'Згорнути або розгорнути ліву навігаційну панель' },
  { keys: ']', description: 'Відкрити або закрити праву контекстну панель' },
  { keys: '?', description: 'Показати цю довідку зі скорочень' },
  { keys: 'Esc', description: 'Закрити накладну панель або це вікно' },
]

const shortcutsOpen = ref(false)

export function useShortcutsModal() {
  return {
    shortcutsOpen,
    openShortcuts: () => {
      shortcutsOpen.value = true
    },
    closeShortcuts: () => {
      shortcutsOpen.value = false
    },
    toggleShortcuts: () => {
      shortcutsOpen.value = !shortcutsOpen.value
    },
  }
}
