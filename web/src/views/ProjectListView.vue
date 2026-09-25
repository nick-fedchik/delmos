<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'

import { api, ApiError } from '@/api/client'
import type { ProjectDetail, ProjectSummary } from '@/api/types'
import PIcon from '@/components/PIcon.vue'
import { useSessionStore } from '@/stores/session'

const session = useSessionStore()

const projects = ref<ProjectSummary[]>([])
const loading = ref(true)
const loadError = ref('')

const isCreateOpen = ref(false)
const code = ref('')
const name = ref('')
const description = ref('')
const creating = ref(false)
const createError = ref('')

const assigningRole = ref(false)
const assignRoleError = ref('')

async function loadProjects(): Promise<void> {
  loading.value = true
  loadError.value = ''
  try {
    projects.value = await api.get<ProjectSummary[]>('/projects')
  } catch (err) {
    loadError.value = err instanceof ApiError ? err.message : 'Не вдалося прочитати список проєктів'
  } finally {
    loading.value = false
  }
}

async function handleCreate(): Promise<void> {
  createError.value = ''
  creating.value = true
  try {
    await api.post<ProjectDetail>('/projects', {
      code: code.value.trim().toUpperCase(),
      name: name.value.trim(),
      description: description.value.trim(),
    })
    code.value = ''
    name.value = ''
    description.value = ''
    isCreateOpen.value = false
    await loadProjects()
  } catch (err) {
    createError.value = err instanceof ApiError ? err.message : 'Не вдалося створити проєкт'
  } finally {
    creating.value = false
  }
}

async function handleSelfAssignProjectManager(): Promise<void> {
  if (!session.user?.id) return
  assignRoleError.value = ''
  assigningRole.value = true
  try {
    await api.post('/role-bindings', {
      user_id: session.user.id,
      role_key: 'project.manager',
      reason: 'Призначення ролі Project Manager для створення проєктів',
    })
    await session.restore()
    isCreateOpen.value = true
  } catch (err) {
    assignRoleError.value = err instanceof ApiError ? err.message : 'Не вдалося надати роль Project Manager'
  } finally {
    assigningRole.value = false
  }
}

onMounted(loadProjects)
</script>

<template>
  <section class="projects-page">
    <div class="page-header">
      <div>
        <h1 class="page-title">Проєкти</h1>
        <p class="page-subtitle muted">
          Інженерні простори життєвого циклу продукту з прив'язкою до плану та сховищ
        </p>
      </div>

      <button
        v-if="session.hasPermission('scopes.manage') && projects.length > 0 && !isCreateOpen"
        class="btn-primary"
        type="button"
        @click="isCreateOpen = true"
      >
        + Новий проєкт
      </button>
    </div>

    <!-- Підказка для адміністратора щодо ролі Project Manager -->
    <div
      v-if="session.hasPermission('access.grant') && !session.hasPermission('scopes.manage')"
      class="notice-box"
    >
      <div class="notice-content">
        <h3 class="notice-title">Налаштування ролей адміністратора</h3>
        <p class="notice-text">
          Ви увійшли як системний адміністратор. Щоб отримати повноваження створювати інженерні проєкти,
          призначте обліковому запису роль <strong>Project Manager</strong>.
        </p>
        <p v-if="assignRoleError" class="alert-error" role="alert">{{ assignRoleError }}</p>
      </div>
      <button
        class="btn-primary"
        type="button"
        :disabled="assigningRole"
        @click="handleSelfAssignProjectManager"
      >
        {{ assigningRole ? 'Призначення...' : 'Призначити роль Project Manager' }}
      </button>
    </div>

    <!-- Форма створення нового проєкту -->
    <div v-if="isCreateOpen && session.hasPermission('scopes.manage')" class="card create-project-card">
      <div class="create-header">
        <div>
          <h2>Новий проєкт</h2>
          <p class="muted">
            Разом із проєктом буде атомарно створено обов'язковий план життєвого циклу (PLAN-001).
          </p>
        </div>
        <button class="btn-close" type="button" title="Закрити форму" @click="isCreateOpen = false">✕</button>
      </div>

      <p v-if="createError" class="alert-error" role="alert">{{ createError }}</p>

      <form @submit.prevent="handleCreate">
        <div class="form-grid">
          <div class="field">
            <label for="code">Код проєкту</label>
            <input
              id="code"
              v-model="code"
              type="text"
              placeholder="INV-001"
              maxlength="32"
              required
              autofocus
            />
            <span class="field-hint muted">Унікальний префікс артефактів (наприклад: INV-001, BMS-02)</span>
          </div>
          <div class="field">
            <label for="name">Назва проєкту</label>
            <input
              id="name"
              v-model="name"
              type="text"
              placeholder="Система керування батареєю (BMS)"
              required
            />
            <span class="field-hint muted">Повна назва інженерної програми або продукту</span>
          </div>
        </div>
        <div class="field">
          <label for="description">Опис</label>
          <textarea
            id="description"
            v-model="description"
            rows="3"
            placeholder="Короткий опис цілей, меж системи та очікуваних результатів..."
          ></textarea>
        </div>
        <div class="form-actions">
          <button class="btn-primary" type="submit" :disabled="creating">
            {{ creating ? 'Створення...' : 'Створити проєкт' }}
          </button>
          <button class="btn-secondary" type="button" :disabled="creating" @click="isCreateOpen = false">
            Скасувати
          </button>
        </div>
      </form>
    </div>

    <!-- Помилка або завантаження -->
    <p v-if="loadError" class="alert-error" role="alert">{{ loadError }}</p>
    <div v-else-if="loading" class="loading-state">
      <span class="muted">Завантаження переліку проєктів…</span>
    </div>

    <!-- Порожній стан за стандартами GitLab Pajamas -->
    <div v-else-if="projects.length === 0 && !isCreateOpen" class="empty-state card">
      <div class="empty-icon-wrapper">
        <svg
          class="empty-icon"
          viewBox="0 0 24 24"
          width="48"
          height="48"
          fill="none"
          stroke="currentColor"
          stroke-width="1.5"
          stroke-linecap="round"
          stroke-linejoin="round"
        >
          <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"></path>
          <line x1="12" y1="11" x2="12" y2="17"></line>
          <line x1="9" y1="14" x2="15" y2="14"></line>
        </svg>
      </div>
      <h2 class="empty-title">Створіть свій перший інженерний проєкт</h2>
      <p class="empty-desc">
        Проєкти в DELMOS координують інженерний життєвий цикл, вимоги, архітектуру та трасованість.
        Кожен проєкт створюється разом із обов'язковим Generic Project Plan (PLAN-001).
      </p>

      <button
        v-if="session.hasPermission('scopes.manage')"
        class="btn-primary empty-cta"
        type="button"
        @click="isCreateOpen = true"
      >
        + Створити перший проєкт
      </button>
      <p v-else-if="!session.hasPermission('access.grant')" class="muted empty-note">
        У вас наразі немає призначених ролей у проєктах. Зверніться до Project Manager або адміністратора організації.
      </p>
    </div>

    <!-- Перелік проєктів -->
    <div v-else-if="projects.length > 0" class="projects-grid">
      <article v-for="proj in projects" :key="proj.id" class="project-card">
        <div class="project-card-header">
          <span class="project-code">{{ proj.code }}</span>
          <span class="badge badge-success">{{ proj.status }}</span>
        </div>
        <h2 class="project-card-title">
          <RouterLink :to="{ name: 'project', params: { projectId: proj.id } }">
            {{ proj.name }}
          </RouterLink>
        </h2>
        <div class="project-card-footer">
          <RouterLink
            :to="{ name: 'project-plan', params: { projectId: proj.id } }"
            class="project-plan-link"
          >
            <PIcon name="planning" :size="14" />
            План PLAN-001
          </RouterLink>
          <RouterLink
            :to="{ name: 'project', params: { projectId: proj.id } }"
            class="project-open-link"
          >
            Відкрити проєкт →
          </RouterLink>
        </div>
      </article>
    </div>
  </section>
</template>
