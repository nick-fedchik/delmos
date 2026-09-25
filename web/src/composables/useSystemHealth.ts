// Єдине джерело стану компонентів системи для оболонки. Опитування виконується
// один раз на застосунок (CORE_SHELL.md §2: захист від паралельних повторів),
// а верхній бар і нижній статус-бар лише читають готовий стан.
import { computed, onMounted, onUnmounted, readonly, ref } from 'vue'

import type { BootStatusResponse, ComponentHealth, ComponentHealthStatus } from '@/api/types'

const POLL_INTERVAL_MS = 15000

const status = ref<ComponentHealthStatus>('yellow')
const components = ref<ComponentHealth[]>([])
const backendVersion = ref<string>('')
const lastCheckedAt = ref<Date | null>(null)
const checking = ref(false)

const degraded = computed(() => components.value.filter((item) => item.status !== 'green'))

// Текст пояснює, ЩО саме в цьому стані, а не абстрактне «Готово»
// (CORE_SHELL.md §6: колір і значок доповнюються текстом).
const summary = computed(() => {
  if (status.value === 'red') {
    return 'Немає зв’язку з ядром системи'
  }
  if (degraded.value.length > 0) {
    return `Компоненти системи: деградація (${degraded.value.length})`
  }
  if (status.value === 'yellow') {
    return 'Компоненти системи: перевірка'
  }
  return 'Компоненти системи справні'
})

let subscribers = 0
let timer: ReturnType<typeof setInterval> | null = null
let inFlight: Promise<void> | null = null

function markUnreachable(reason: string): void {
  status.value = 'red'
  backendVersion.value = ''
  components.value = [
    { id: 'core', name: 'Ядро DELMOS', status: 'red', message: reason },
  ]
}

async function refresh(): Promise<void> {
  if (inFlight) {
    return inFlight
  }
  checking.value = true
  inFlight = (async () => {
    try {
      const res = await fetch('/api/v1/system/boot-status', { credentials: 'include' })
      const data = (await res.json()) as BootStatusResponse
      status.value = data.status
      backendVersion.value = data.version
      components.value = data.components ?? []
    } catch {
      markUnreachable('Немає з’єднання з сервером DELMOS')
    } finally {
      lastCheckedAt.value = new Date()
      checking.value = false
      inFlight = null
    }
  })()
  return inFlight
}

export function useSystemHealth() {
  onMounted(() => {
    subscribers += 1
    void refresh()
    if (!timer) {
      timer = setInterval(() => {
        void refresh()
      }, POLL_INTERVAL_MS)
    }
  })

  onUnmounted(() => {
    subscribers = Math.max(0, subscribers - 1)
    if (subscribers === 0 && timer) {
      clearInterval(timer)
      timer = null
    }
  })

  return {
    status: readonly(status),
    components: readonly(components),
    degraded,
    summary,
    backendVersion: readonly(backendVersion),
    lastCheckedAt: readonly(lastCheckedAt),
    checking: readonly(checking),
    refresh,
  }
}
