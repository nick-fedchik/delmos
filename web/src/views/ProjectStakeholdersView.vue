<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'

import { api, ApiError } from '@/api/client'
import type { ProjectDetail, Stakeholder } from '@/api/types'
import { useSessionStore } from '@/stores/session'

const props = defineProps<{ projectId: string }>()
const session = useSessionStore()

const project = ref<ProjectDetail | null>(null)
const stakeholders = ref<Stakeholder[]>([])
const loading = ref(true)
const loadError = ref('')

const isCreateOpen = ref(false)
const kind = ref<'user' | 'organization' | 'external_party'>('user')
const name = ref('')
const contactRef = ref('')
const interest = ref('')
const creating = ref(false)
const createError = ref('')

const canCreate = computed(() => project.value?.permissions?.includes('wp.create') ?? session.hasPermission('wp.create'))

async function load(): Promise<void> {
  loading.value = true
  loadError.value = ''
  try {
    project.value = await api.get<ProjectDetail>(`/projects/${props.projectId}`)
    stakeholders.value = await api.get<Stakeholder[]>(`/projects/${props.projectId}/stakeholders`)
  } catch (err) {
    loadError.value = err instanceof ApiError ? err.message : 'Не вдалося прочитати стейкхолдерів'
  } finally {
    loading.value = false
  }
}

async function handleCreate(): Promise<void> {
  createError.value = ''
  creating.value = true
  try {
    await api.post<Stakeholder>(`/projects/${props.projectId}/stakeholders`, {
      kind: kind.value,
      name: name.value.trim(),
      contact_ref: contactRef.value.trim(),
      interest: interest.value.trim(),
    })
    name.value = ''
    contactRef.value = ''
    interest.value = ''
    isCreateOpen.value = false
    await load()
  } catch (err) {
    createError.value = err instanceof ApiError ? err.message : 'Не вдалося додати стейкхолдера'
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
          Стейкхолдери (RACI)
        </li>
      </ol>
    </nav>

    <header v-if="project" class="detail-header">
      <div class="detail-title-group">
        <div class="detail-title-row">
          <span class="detail-code">{{ project.code }}</span>
          <h1 class="detail-title">Реєстр стейкхолдерів проєкту</h1>
        </div>
        <p class="muted" style="margin: 0">
          Учасники, організації та зовнішні сторони, на яких посилається матриця відповідальності плану (PMBOK Domain).
        </p>
      </div>

      <div class="detail-actions">
        <button
          v-if="canCreate && !isCreateOpen"
          class="btn-primary"
          type="button"
          @click="isCreateOpen = true"
        >
          + Додати стейкхолдера
        </button>
      </div>
    </header>

    <p v-if="loadError" class="alert-error" role="alert">{{ loadError }}</p>
    <div v-else-if="loading" class="loading-state">
      <span class="muted">Завантаження реєстру стейкхолдерів…</span>
    </div>

    <!-- Форма додавання -->
    <div v-if="isCreateOpen && canCreate" class="card" style="margin-bottom: 1.5rem; background: var(--color-surface-subtle)">
      <div class="create-header">
        <h3>Новий стейкхолдер</h3>
        <button class="btn-close" type="button" title="Закрити" @click="isCreateOpen = false">✕</button>
      </div>
      <p v-if="createError" class="alert-error" role="alert">{{ createError }}</p>
      <form @submit.prevent="handleCreate">
        <div class="form-grid">
          <div class="field">
            <label for="sh-kind">Тип стейкхолдера</label>
            <select id="sh-kind" v-model="kind">
              <option value="user">Користувач / інженер (user)</option>
              <option value="organization">Організація / підрозділ (organization)</option>
              <option value="external_party">Зовнішня сторона / клієнт (external_party)</option>
            </select>
          </div>
          <div class="field">
            <label for="sh-name">Ім'я або найменування</label>
            <input id="sh-name" v-model="name" type="text" placeholder="Олександр Петренко або Замовник OEM" required />
          </div>
        </div>
        <div class="form-grid">
          <div class="field">
            <label for="sh-contact">Контактні дані (email / телефон / канал)</label>
            <input id="sh-contact" v-model="contactRef" type="text" placeholder="o.petrenko@company.com" />
          </div>
          <div class="field">
            <label for="sh-interest">Зона інтересу та очікування</label>
            <input id="sh-interest" v-model="interest" type="text" placeholder="Приймання архітектури силового інвертора" />
          </div>
        </div>
        <div class="form-actions">
          <button class="btn-primary" type="submit" :disabled="creating">
            {{ creating ? 'Збереження...' : 'Зареєструвати стейкхолдера' }}
          </button>
          <button class="btn-secondary" type="button" :disabled="creating" @click="isCreateOpen = false">
            Скасувати
          </button>
        </div>
      </form>
    </div>

    <!-- Таблиця стейкхолдерів -->
    <div v-if="stakeholders.length > 0" class="section-box">
      <div class="section-box-body data-table-wrapper" style="padding: 0">
        <table class="data-table">
          <thead>
            <tr>
              <th>Ім'я / Суб'єкт</th>
              <th>Тип</th>
              <th>Контакт</th>
              <th>Зона інтересу</th>
              <th>Дата реєстрації</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="sh in stakeholders" :key="sh.id">
              <td style="font-weight: 600">{{ sh.name }}</td>
              <td>
                <span class="badge badge-info">{{ sh.kind }}</span>
              </td>
              <td>{{ sh.contact_ref || '—' }}</td>
              <td>{{ sh.interest || '—' }}</td>
              <td class="muted" style="font-size: 0.8125rem">{{ new Date(sh.created_at).toLocaleDateString() }}</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <div v-else-if="!isCreateOpen" class="empty-state">
      <h3 class="empty-title">Ще немає зареєстрованих стейкхолдерів</h3>
      <p class="empty-desc">
        Зареєструйте ключових учасників, замовників та експертів для формування матриці відповідальності плану (RACI).
      </p>
      <button v-if="canCreate" class="btn-primary btn-sm" type="button" @click="isCreateOpen = true">
        + Додати першого стейкхолдера
      </button>
    </div>
  </section>
</template>
