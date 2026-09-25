<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'

import type { BootStatusResponse, ComponentHealth, ComponentHealthStatus } from '@/api/types'

const defaultComponents: ComponentHealth[] = [
  { id: 'core', name: 'Ядро DELMOS', status: 'yellow', message: 'Перевірка запуску...' },
  { id: 'database', name: 'База даних (PostgreSQL)', status: 'yellow', message: 'Очікування...' },
  { id: 'schema', name: 'Схема даних', status: 'yellow', message: 'Очікування...' },
  { id: 'git_storage', name: 'Сховище репозиторіїв', status: 'yellow', message: 'Очікування...' },
]

const components = ref<ComponentHealth[]>(defaultComponents)
const overallStatus = ref<ComponentHealthStatus>('yellow')
const systemVersion = ref<string>('')
const lastChecked = ref<string>('')
const checking = ref(false)
let timer: ReturnType<typeof setInterval> | null = null

async function checkBootHealth(): Promise<void> {
  checking.value = true
  try {
    const res = await fetch('/api/v1/system/boot-status')
    if (res.ok) {
      const data = (await res.json()) as BootStatusResponse
      components.value = data.components
      overallStatus.value = data.status
      systemVersion.value = data.version
    } else {
      let parsed: BootStatusResponse | null = null
      try {
        parsed = (await res.json()) as BootStatusResponse
      } catch {
        parsed = null
      }
      if (parsed?.components) {
        components.value = parsed.components
        overallStatus.value = parsed.status
        systemVersion.value = parsed.version
      } else {
        markAllUnreachable(`Помилка сервера HTTP ${res.status}`)
      }
    }
  } catch {
    markAllUnreachable('Немає з’єднання з сервером DELMOS')
  } finally {
    checking.value = false
    const now = new Date()
    lastChecked.value = now.toLocaleTimeString()
  }
}

function markAllUnreachable(reason: string): void {
  overallStatus.value = 'red'
  components.value = [
    { id: 'core', name: 'Ядро DELMOS', status: 'red', message: reason },
    { id: 'database', name: 'База даних (PostgreSQL)', status: 'red', message: 'Недоступно' },
    { id: 'schema', name: 'Схема даних', status: 'red', message: 'Не перевірено' },
    { id: 'git_storage', name: 'Сховище репозиторіїв', status: 'red', message: 'Недоступно' },
  ]
}

function statusCircleLabel(status: ComponentHealthStatus): string {
  switch (status) {
    case 'green':
      return 'Справно'
    case 'yellow':
      return 'Попередження або завантаження'
    case 'red':
      return 'Помилка або недоступно'
    default:
      return 'Невідомо'
  }
}

onMounted(() => {
  void checkBootHealth()
  timer = setInterval(() => {
    void checkBootHealth()
  }, 10000)
})

onUnmounted(() => {
  if (timer) {
    clearInterval(timer)
  }
})
</script>

<template>
  <footer class="boot-status-bar" role="status" aria-label="Стан завантаження та інтеграцій системи">
    <div class="boot-status-header">
      <div class="boot-status-title">
        <span
          class="status-circle"
          :class="'status-' + overallStatus"
          :aria-label="statusCircleLabel(overallStatus)"
        ></span>
        <span class="boot-title-text">Boot Health · Діагностика інтеграцій</span>
        <span v-if="systemVersion" class="boot-version-text muted">({{ systemVersion }})</span>
      </div>
      <div class="boot-status-actions">
        <span v-if="lastChecked" class="muted boot-last-checked">Оновлено: {{ lastChecked }}</span>
        <button
          class="btn-secondary boot-refresh-btn"
          type="button"
          :disabled="checking"
          title="Перевірити стан компонентів"
          @click="checkBootHealth"
        >
          {{ checking ? 'Перевірка...' : 'Оновити' }}
        </button>
      </div>
    </div>

    <ul class="boot-status-list">
      <li v-for="comp in components" :key="comp.id" class="boot-status-item">
        <span
          class="status-circle"
          :class="'status-' + comp.status"
          :aria-label="statusCircleLabel(comp.status)"
          :title="statusCircleLabel(comp.status)"
        ></span>
        <span class="boot-comp-name">{{ comp.name }}</span>
        <span class="boot-comp-msg muted">({{ comp.message }})</span>
      </li>
    </ul>
  </footer>
</template>
