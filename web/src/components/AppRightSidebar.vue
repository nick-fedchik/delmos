<!-- Права контекстна панель оболонки (CORE_SHELL.md §5).
     Режим «Довідка» працює офлайн і не залежить від зовнішнього сервісу.
     Режим «Діагностика» показує невирішені збої поточного сеансу.
     Єдиний орган керування — шеврон унизу, видимий в обох станах. -->
<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useRoute } from 'vue-router'

import PIcon from '@/components/PIcon.vue'
import { useLayout } from '@/composables/useLayout'
import { findTopic, searchTopics } from '@/help/content'
import { useDiagnosticsStore } from '@/stores/diagnostics'

const props = defineProps<{ overlay: boolean }>()

const route = useRoute()
const { rightExpanded, rightMode, toggleRight } = useLayout()
const diagnostics = useDiagnosticsStore()

const query = ref('')
const panelRef = ref<HTMLElement | null>(null)

const topic = computed(() => findTopic(String(route.name ?? '')))
const matches = computed(() => searchTopics(query.value))

const diagnosticsBadge = computed(() => diagnostics.errorCount + diagnostics.warningCount)

// Зміна маршруту оновлює тему довідки, але не скидає введений пошуковий запит
// і не закриває панель.
watch(
  () => route.name,
  () => {
    if (rightMode.value === 'help') {
      query.value = ''
    }
  },
)

// У накладному режимі фокус переходить усередину панелі, а Escape повертає
// його на кнопку-тригер у верхньому барі.
watch(
  () => props.overlay && rightExpanded.value,
  async (active) => {
    if (!active) return
    await nextTick()
    panelRef.value?.focus()
  },
)

function formatTime(iso: string): string {
  return new Date(iso).toLocaleTimeString()
}
</script>

<template>
  <aside
    class="app-rightbar"
    :class="{ 'app-rightbar--open': rightExpanded, 'app-rightbar--overlay': overlay }"
    aria-label="Контекстна панель"
  >
    <div v-if="rightExpanded" ref="panelRef" class="rightbar-panel" tabindex="-1">
      <div class="rightbar-header">
        <div class="rightbar-tabs" role="tablist" aria-label="Режим контекстної панелі">
          <button
            type="button"
            role="tab"
            class="rightbar-tab"
            :class="{ active: rightMode === 'help' }"
            :aria-selected="rightMode === 'help'"
            @click="rightMode = 'help'"
          >
            Довідка
          </button>
          <button
            type="button"
            role="tab"
            class="rightbar-tab"
            :class="{ active: rightMode === 'diagnostics' }"
            :aria-selected="rightMode === 'diagnostics'"
            @click="rightMode = 'diagnostics'"
          >
            Діагностика
            <span v-if="diagnosticsBadge > 0" class="rightbar-tab-badge">{{ diagnosticsBadge }}</span>
          </button>
        </div>
      </div>

      <div v-if="rightMode === 'help'" class="rightbar-body">
        <label class="rightbar-search">
          <PIcon name="search" :size="14" />
          <input
            v-model="query"
            type="search"
            placeholder="Пошук у довідці"
            aria-label="Пошук у довідці"
          />
        </label>

        <template v-if="query.trim()">
          <p class="rightbar-note">
            Знайдено розділів: {{ matches.length }}
          </p>
          <article v-for="found in matches" :key="found.routeName" class="help-block">
            <h3 class="help-title">{{ found.title }}</h3>
            <p class="help-text">{{ found.purpose }}</p>
          </article>
          <p v-if="matches.length === 0" class="rightbar-note">
            Нічого не знайдено. Спробуйте інше слово або очистіть запит, щоб побачити довідку для
            поточного екрана.
          </p>
        </template>

        <template v-else>
          <article class="help-block">
            <h3 class="help-title">{{ topic.title }}</h3>
            <p class="help-text">{{ topic.purpose }}</p>
          </article>
          <article class="help-block">
            <h4 class="help-subtitle">Що потрібно до початку</h4>
            <ul class="help-list">
              <li v-for="item in topic.prerequisites" :key="item">{{ item }}</li>
            </ul>
          </article>
          <article class="help-block">
            <h4 class="help-subtitle">Що зробити далі</h4>
            <ul class="help-list">
              <li v-for="item in topic.nextSteps" :key="item">{{ item }}</li>
            </ul>
          </article>
        </template>
      </div>

      <div v-else class="rightbar-body">
        <p v-if="diagnostics.entries.length === 0" class="rightbar-note">
          Невирішених помилок і попереджень немає.
        </p>
        <template v-else>
          <button type="button" class="btn-secondary btn-sm" @click="diagnostics.clear()">
            Очистити перелік
          </button>
          <article
            v-for="entry in [...diagnostics.entries].reverse()"
            :key="entry.id"
            class="diag-entry"
            :class="'diag-entry--' + entry.level"
          >
            <div class="diag-entry-head">
              <PIcon :name="entry.level === 'error' ? 'error' : 'warning'" :size="14" />
              <span class="diag-entry-title">{{ entry.title }}</span>
            </div>
            <p class="diag-entry-text">{{ entry.detail }}</p>
            <p class="diag-entry-meta">
              <span v-if="entry.code">{{ entry.code }} · </span>{{ entry.path }} ·
              {{ formatTime(entry.at) }}
            </p>
          </article>
        </template>
      </div>
    </div>

    <div class="panel-footer">
      <button
        type="button"
        class="panel-toggle-btn"
        :title="rightExpanded ? 'Згорнути контекстну панель (])' : 'Розгорнути контекстну панель (])'"
        :aria-label="rightExpanded ? 'Згорнути контекстну панель' : 'Розгорнути контекстну панель'"
        :aria-expanded="rightExpanded"
        @click="toggleRight"
      >
        <span v-if="rightExpanded">Згорнути</span>
        <PIcon
          :name="rightExpanded ? 'chevron-double-lg-right' : 'chevron-double-lg-left'"
          :size="14"
        />
        <span v-if="!rightExpanded && diagnosticsBadge > 0" class="rightbar-strip-badge">
          {{ diagnosticsBadge }}
        </span>
      </button>
    </div>
  </aside>
</template>
