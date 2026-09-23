<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'

import { api, ApiError } from '@/api/client'
import type { ProjectDetail, ProjectSummary } from '@/api/types'
import { useSessionStore } from '@/stores/session'

const session = useSessionStore()

const projects = ref<ProjectSummary[]>([])
const loading = ref(true)
const loadError = ref('')

const code = ref('')
const name = ref('')
const description = ref('')
const creating = ref(false)
const createError = ref('')

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
      code: code.value,
      name: name.value,
      description: description.value,
    })
    code.value = ''
    name.value = ''
    description.value = ''
    await loadProjects()
  } catch (err) {
    createError.value = err instanceof ApiError ? err.message : 'Не вдалося створити проєкт'
  } finally {
    creating.value = false
  }
}

onMounted(loadProjects)
</script>

<template>
  <section>
    <h1>Проєкти</h1>
    <p class="muted">
      Список містить лише проєкти, де у вас є чинна роль (SWR-42 §3): доступ в одному проєкті
      не дає видимості в іншому.
    </p>

    <p v-if="loadError" class="alert-error" role="alert">{{ loadError }}</p>
    <p v-else-if="loading">Завантаження…</p>
    <p v-else-if="projects.length === 0" class="muted">Ще немає доступних проєктів.</p>
    <ul v-else class="card-list">
      <li v-for="project in projects" :key="project.id">
        <RouterLink :to="{ name: 'project', params: { projectId: project.id } }">{{ project.code }}</RouterLink>
        — {{ project.name }}
        <span class="badge">{{ project.status }}</span>
      </li>
    </ul>

    <div v-if="session.hasPermission('scopes.manage')" class="card">
      <h2>Створити проєкт</h2>
      <p class="muted">Разом із проєктом буде атомарно створено обов'язковий план (PLAN-001).</p>
      <p v-if="createError" class="alert-error" role="alert">{{ createError }}</p>
      <form @submit.prevent="handleCreate">
        <div class="field">
          <label for="code">Код проєкту</label>
          <input id="code" v-model="code" type="text" placeholder="INV-001" required />
        </div>
        <div class="field">
          <label for="name">Назва</label>
          <input id="name" v-model="name" type="text" required />
        </div>
        <div class="field">
          <label for="description">Опис</label>
          <textarea id="description" v-model="description" rows="2"></textarea>
        </div>
        <button class="btn-primary" type="submit" :disabled="creating">Створити</button>
      </form>
    </div>
  </section>
</template>
