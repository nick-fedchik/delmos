// Стан оболонки: ліва навігаційна панель і права контекстна панель.
// Вибір користувача зберігається як несекретне вподобання (CORE_SHELL.md §3);
// сесія і права ніколи не потрапляють до localStorage.
import { ref, watch } from 'vue'

const LS_LEFT = 'delmos:shell:left-expanded'
const LS_RIGHT = 'delmos:shell:right-expanded'

function readBool(key: string, fallback: boolean): boolean {
  try {
    const raw = localStorage.getItem(key)
    return raw === null ? fallback : raw === 'true'
  } catch {
    return fallback
  }
}

function writeBool(key: string, value: boolean): void {
  try {
    localStorage.setItem(key, String(value))
  } catch {
    // Приватний режим браузера блокує сховище — оболонка працює й без збереження.
  }
}

const leftExpanded = ref(readBool(LS_LEFT, true))
const rightExpanded = ref(readBool(LS_RIGHT, false))
const rightMode = ref<'help' | 'diagnostics'>('help')

watch(leftExpanded, (value) => writeBool(LS_LEFT, value))
watch(rightExpanded, (value) => writeBool(LS_RIGHT, value))

export function useLayout() {
  return {
    leftExpanded,
    rightExpanded,
    rightMode,
    toggleLeft: () => {
      leftExpanded.value = !leftExpanded.value
    },
    toggleRight: () => {
      rightExpanded.value = !rightExpanded.value
    },
    openRight: (mode?: 'help' | 'diagnostics') => {
      if (mode) {
        rightMode.value = mode
      }
      rightExpanded.value = true
    },
    closeRight: () => {
      rightExpanded.value = false
    },
  }
}
