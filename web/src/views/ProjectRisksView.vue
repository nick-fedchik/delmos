<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'

import { api, ApiError } from '@/api/client'
import type { ProjectDetail, ProjectRisk } from '@/api/types'
import { useSessionStore } from '@/stores/session'

const props = defineProps<{ projectId: string }>()
const session = useSessionStore()

const project = ref<ProjectDetail | null>(null)
const risks = ref<ProjectRisk[]>([])
const loading = ref(true)
const loadError = ref('')

const isCreateOpen = ref(false)
const title = ref('')
const description = ref('')
const impact = ref<'low' | 'medium' | 'high' | 'critical'>('medium')
const likelihood = ref<'low' | 'medium' | 'high'>('medium')
const responseStrategy = ref('')
const ownerRef = ref('')
const creating = ref(false)
const createError = ref('')

const canCreate = computed(() => project.value?.permissions?.includes('wp.create') ?? session.hasPermission('wp.create'))

async function load(): Promise<void> {
  loading.value = true
  loadError.value = ''
  try {
    project.value = await api.get<ProjectDetail>(`/projects/${props.projectId}`)
    risks.value = await api.get<ProjectRisk[]>(`/projects/${props.projectId}/risks`)
  } catch (err) {
    loadError.value = err instanceof ApiError ? err.message : 'Не вдалося прочитати ризики'
  } finally {
    loading.value = false
  }
}

async function handleCreate(): Promise<void> {
  createError.value = ''
  creating.value = true
  try {
    await api.post<ProjectRisk>(`/projects/${props.projectId}/risks`, {
      title: title.value.trim(),
      description: description.value.trim(),
      impact: impact.value,
      likelihood: likelihood.value,
      response_strategy: responseStrategy.value.trim(),
      owner_ref: ownerRef.value.trim(),
    })
    title.value = ''
    description.value = ''
    impact.value = 'medium'
    likelihood.value = 'medium'
    responseStrategy.value = ''
    ownerRef.value = ''
    isCreateOpen.value = false
    await load()
  } catch (err) {
    createError.value = err instanceof ApiError ? err.message : 'Не вдалося зареєструвати ризик'
  } finally {
    creating.value = false
  }
}

onMounted(load)
</script>

<template>
  <section class="detail-view">
    <nav v-if="project" aria-label="Навігаційний ланцюжок">
      <ol class="breadcrumbs">
        <li class="breadcrumb-item">
          <RouterLink :to="{ name: 'projects' }">Проєкти</RouterLink>
        </li>
        <li class="breadcrumb-separator" aria-hidden="true">/</li>
        <li class="breadcrumb-item">
          <RouterLink :to="{ name: 'project', params: { projectId } }">{{ project.code }}</RouterLink>
        </li>
        <li class="breadcrumb-separator" aria-hidden="true">/</li>
        <li class="breadcrumb-item breadcrumb-current" aria-current="page">
          Реєстр ризиків
        </li>
      </ol>
    </nav>

    <header v-if="project" class="detail-header">
      <div class="detail-title-group">
        <div class="detail-title-row">
          <span class="detail-code">{{ project.code }}</span>
          <h1 class="detail-title">Оперативний реєстр ризиків проєкту</h1>
        </div>
        <p class="muted" style="margin: 0">
          Ідентифікація невизначеностей, оцінка впливу та стратегії пом'якшення (PMBOK Risk Performance Domain).
        </p>
      </div>

      <div class="detail-actions">
        <button
          v-if="canCreate && !isCreateOpen"
          class="btn-primary"
          type="button"
          @click="isCreateOpen = true"
        >
          + Зареєструвати ризик
        </button>
      </div>
    </header>

    <p v-if="loadError" class="alert-error" role="alert">{{ loadError }}</p>
    <div v-else-if="loading" class="loading-state">
      <span class="muted">Завантаження реєстру ризиків…</span>
    </div>

    <!-- Форма створення ризику -->
    <div v-if="isCreateOpen && canCreate" class="card" style="margin-bottom: 1.5rem; background: var(--color-surface-subtle)">
      <div class="create-header">
        <h3>Новий проєктний ризик</h3>
        <button class="btn-close" type="button" title="Закрити" @click="isCreateOpen = false">✕</button>
      </div>
      <p v-if="createError" class="alert-error" role="alert">{{ createError }}</p>
      <form @submit.prevent="handleCreate">
        <div class="field">
          <label for="risk-title">Заголовок ризику</label>
          <input id="risk-title" v-model="title" type="text" placeholder="Затримка постачання SiC MOSFET ключів з боку постачальника" required />
        </div>
        <div class="form-grid">
          <div class="field">
            <label for="risk-impact">Рівень впливу (Impact)</label>
            <select id="risk-impact" v-model="impact">
              <option value="low">Низький (low)</option>
              <option value="medium">Середній (medium)</option>
              <option value="high">Високий (high)</option>
              <option value="critical">Критичний (critical)</option>
            </select>
          </div>
          <div class="field">
            <label for="risk-likelihood">Ймовірність (Likelihood)</label>
            <select id="risk-likelihood" v-model="likelihood">
              <option value="low">Низька (low)</option>
              <option value="medium">Середня (medium)</option>
              <option value="high">Висока (high)</option>
            </select>
          </div>
        </div>
        <div class="form-grid">
          <div class="field">
            <label for="risk-strategy">Стратегія реагування (Response strategy)</label>
            <input id="risk-strategy" v-model="responseStrategy" type="text" placeholder="Попереднє замовлення зразків другого джерела (second source)" />
          </div>
          <div class="field">
            <label for="risk-owner">Відповідальний за моніторинг (Owner)</label>
            <input id="risk-owner" v-model="ownerRef" type="text" placeholder="Головний апаратний архітектор" />
          </div>
        </div>
        <div class="field">
          <label for="risk-desc">Опис та наслідки</label>
          <textarea id="risk-desc" v-model="description" rows="2" placeholder="Детальний опис потенційного впливу на розклад та технічні характеристики..."></textarea>
        </div>
        <div class="form-actions">
          <button class="btn-primary" type="submit" :disabled="creating">
            {{ creating ? 'Збереження...' : 'Зафіксувати ризик' }}
          </button>
          <button class="btn-secondary" type="button" :disabled="creating" @click="isCreateOpen = false">
            Скасувати
          </button>
        </div>
      </form>
    </div>

    <!-- Таблиця ризиків -->
    <div v-if="risks.length > 0" class="section-box">
      <div class="section-box-body data-table-wrapper" style="padding: 0">
        <table class="data-table">
          <thead>
            <tr>
              <th>Заголовок ризику</th>
              <th>Вплив</th>
              <th>Ймовірність</th>
              <th>Стратегія реагування</th>
              <th>Відповідальний</th>
              <th>Статус</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="r in risks" :key="r.id">
              <td style="font-weight: 600">
                {{ r.title }}
                <p v-if="r.description" class="muted" style="margin: 0.15rem 0 0; font-size: 0.75rem; font-weight: normal">
                  {{ r.description }}
                </p>
              </td>
              <td>
                <span :class="['badge', r.impact === 'critical' ? 'badge-danger' : r.impact === 'high' ? 'badge-warning' : 'badge-neutral']">
                  {{ r.impact }}
                </span>
              </td>
              <td>
                <span :class="['badge', r.likelihood === 'high' ? 'badge-warning' : 'badge-neutral']">
                  {{ r.likelihood }}
                </span>
              </td>
              <td>{{ r.response_strategy || '—' }}</td>
              <td>{{ r.owner_ref || '—' }}</td>
              <td>
                <span class="badge badge-info">{{ r.status }}</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div v-else-if="!isCreateOpen" class="empty-state">
      <h3 class="empty-title">Ще немає зареєстрованих ризиків</h3>
      <p class="empty-desc">
        Фіксуйте технічні, часові та постачальницькі ризики для контролю життєвого циклу без зміни маніфесту плану.
      </p>
      <button v-if="canCreate" class="btn-primary btn-sm" type="button" @click="isCreateOpen = true">
        + Зареєструвати перший ризик
      </button>
    </div>
  </section>
</template>
