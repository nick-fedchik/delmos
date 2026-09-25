<!-- Нижній статус-бар оболонки (CORE_SHELL.md §6).
     Компактна діагностика: версії UI/API, стан компонентів, лічильники помилок
     сторінки, копіювання безпечного контексту підтримки та довідка зі скорочень.
     Копіювання ніколи не включає токени, секрети чи сирі логи. -->
<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute } from 'vue-router'

import PIcon from '@/components/PIcon.vue'
import { useLayout } from '@/composables/useLayout'
import { useShortcutsModal } from '@/composables/useShortcutsModal'
import { useSystemHealth } from '@/composables/useSystemHealth'
import { useDiagnosticsStore } from '@/stores/diagnostics'

// UI і API збираються в один бінарник, тому версія завжди спільна.
const appVersion = import.meta.env.VITE_DELMOS_VERSION

const route = useRoute()
const diagnostics = useDiagnosticsStore()
const { openShortcuts } = useShortcutsModal()
const { openRight } = useLayout()
const { status, degraded, summary, lastCheckedAt, checking, refresh } = useSystemHealth()

const copyState = ref<'idle' | 'done' | 'failed'>('idle')

const healthTitle = computed(() => {
  const checkedAt = lastCheckedAt.value
    ? lastCheckedAt.value.toLocaleTimeString()
    : 'ще не перевірено'
  const detail = degraded.value.length
    ? degraded.value.map((item) => `${item.name}: ${item.message}`).join('; ')
    : 'Ядро, база даних, схема даних і сховище репозиторіїв відповідають'
  return `${summary.value} · перевірено ${checkedAt} · ${detail}`
})

// Контекст підтримки: лише те, що безпечно передати каналом підтримки.
function supportContext(): string {
  return [
    `DELMOS ${appVersion}`,
    `Маршрут: ${String(route.name ?? '')} (${route.fullPath})`,
    `Стан системи: ${summary.value}`,
    degraded.value.length
      ? `Деградація: ${degraded.value.map((item) => `${item.id}=${item.status}`).join(', ')}`
      : 'Деградація: немає',
    `Діагностика сторінки: помилок ${diagnostics.errorCount}, попереджень ${diagnostics.warningCount}`,
    diagnostics.latest ? `Останній запис: ${diagnostics.latest.code} ${diagnostics.latest.title}` : '',
  ]
    .filter(Boolean)
    .join('\n')
}

async function copySupportContext(): Promise<void> {
  try {
    await navigator.clipboard.writeText(supportContext())
    copyState.value = 'done'
  } catch {
    copyState.value = 'failed'
  }
  setTimeout(() => {
    copyState.value = 'idle'
  }, 2500)
}
</script>

<template>
  <footer class="app-statusbar" role="contentinfo" aria-label="Стан застосунку">
    <span class="statusbar-product">DELMOS {{ appVersion }}</span>

    <button
      type="button"
      class="statusbar-health"
      :title="healthTitle"
      :disabled="checking"
      @click="refresh"
    >
      <span class="status-circle" :class="'status-' + status"></span>
      <span>{{ summary }}</span>
    </button>

    <button
      v-if="diagnostics.errorCount || diagnostics.warningCount"
      type="button"
      class="statusbar-diagnostics"
      title="Показати перелік помилок і попереджень сторінки"
      @click="openRight('diagnostics')"
    >
      <PIcon :name="diagnostics.errorCount ? 'error' : 'warning'" :size="12" />
      <span v-if="diagnostics.errorCount" class="statusbar-count statusbar-count--error">
        помилок: {{ diagnostics.errorCount }}
      </span>
      <span v-if="diagnostics.warningCount" class="statusbar-count statusbar-count--warning">
        попереджень: {{ diagnostics.warningCount }}
      </span>
    </button>

    <div class="statusbar-spacer"></div>

    <button
      type="button"
      class="statusbar-action"
      title="Скопіювати безпечний контекст підтримки (версії, маршрут, стан) — без токенів і секретів"
      @click="copySupportContext"
    >
      <PIcon name="copy-to-clipboard" :size="12" />
      <span>{{
        copyState === 'done'
          ? 'Скопійовано'
          : copyState === 'failed'
            ? 'Копіювання недоступне'
            : 'Контекст підтримки'
      }}</span>
    </button>

    <button
      type="button"
      class="statusbar-action"
      title="Показати довідку зі скорочень клавіатури (?)"
      @click="openShortcuts"
    >
      <kbd>?</kbd>
      <span>Скорочення</span>
    </button>
  </footer>
</template>
