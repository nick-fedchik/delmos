<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'

import { api, ApiError } from '@/api/client'
import type { ProjectReviewRoleBinding, ReviewRoleProject, ReviewRoleUser } from '@/api/types'
import { useSessionStore } from '@/stores/session'

const session = useSessionStore()
const canGrant = computed(() => session.hasPermission('access.grant'))
const canRevoke = computed(() => session.hasPermission('access.revoke'))

const projectQuery = ref('')
const projects = ref<ReviewRoleProject[]>([])
const projectId = ref('')
const projectsLoading = ref(false)
const projectsError = ref('')

const userQuery = ref('')
const users = ref<ReviewRoleUser[]>([])
const userId = ref('')
const usersLoading = ref(false)
const usersError = ref('')

const bindings = ref<ProjectReviewRoleBinding[]>([])
const bindingsLoading = ref(false)
const bindingsError = ref('')
const roleKey = ref<'project.reviewer' | 'project.approver'>('project.reviewer')
const reason = ref('')
const granting = ref(false)
const grantError = ref('')
const grantSuccess = ref('')
const pendingRevokeId = ref('')
const revoking = ref(false)
const revokeError = ref('')

async function searchProjects(): Promise<void> {
  projectsLoading.value = true
  projectsError.value = ''
  try {
    const result = await api.get<ReviewRoleProject[]>(`/admin/review-role-projects?query=${encodeURIComponent(projectQuery.value.trim())}`)
    projects.value = Array.isArray(result) ? result : []
    if (projectId.value && !projects.value.some((project) => project.id === projectId.value)) {
      projectId.value = ''
      bindings.value = []
      users.value = []
      userId.value = ''
    }
  } catch (err) {
    projectsError.value = err instanceof ApiError ? err.message : 'Не вдалося знайти проєкти'
  } finally {
    projectsLoading.value = false
  }
}

async function loadBindings(): Promise<void> {
  bindings.value = []
  bindingsError.value = ''
  pendingRevokeId.value = ''
  if (!projectId.value) return
  bindingsLoading.value = true
  try {
    const result = await api.get<ProjectReviewRoleBinding[]>(`/projects/${projectId.value}/review-role-bindings`)
    bindings.value = Array.isArray(result) ? result : []
  } catch (err) {
    bindingsError.value = err instanceof ApiError ? err.message : 'Не вдалося прочитати ролі проєкту'
  } finally {
    bindingsLoading.value = false
  }
}

function selectProject(): void {
  userId.value = ''
  users.value = []
  userQuery.value = ''
  reason.value = ''
  grantSuccess.value = ''
  void loadBindings()
}

async function searchUsers(): Promise<void> {
  usersError.value = ''
  users.value = []
  userId.value = ''
  if (userQuery.value.trim().length < 2) {
    usersError.value = 'Введіть щонайменше два символи'
    return
  }
  usersLoading.value = true
  try {
    const result = await api.get<ReviewRoleUser[]>(`/admin/review-role-users?query=${encodeURIComponent(userQuery.value.trim())}`)
    users.value = Array.isArray(result) ? result : []
  } catch (err) {
    usersError.value = err instanceof ApiError ? err.message : 'Не вдалося знайти користувачів'
  } finally {
    usersLoading.value = false
  }
}

async function grant(): Promise<void> {
  if (!canGrant.value || !projectId.value || !userId.value || !reason.value.trim()) return
  granting.value = true
  grantError.value = ''
  grantSuccess.value = ''
  try {
    await api.post(`/projects/${projectId.value}/review-role-bindings`, {
      user_id: userId.value,
      role_key: roleKey.value,
      reason: reason.value.trim(),
    })
    userId.value = ''
    reason.value = ''
    await loadBindings()
    grantSuccess.value = 'Роль надано.'
  } catch (err) {
    grantError.value = err instanceof ApiError ? err.message : 'Не вдалося надати роль'
  } finally {
    granting.value = false
  }
}

async function revoke(bindingId: string): Promise<void> {
  if (!canRevoke.value || !projectId.value || revoking.value || pendingRevokeId.value !== bindingId) return
  revoking.value = true
  revokeError.value = ''
  try {
    await api.delete(`/projects/${projectId.value}/review-role-bindings/${bindingId}`)
    await loadBindings()
  } catch (err) {
    revokeError.value = err instanceof ApiError ? err.message : 'Не вдалося відкликати роль'
  } finally {
    revoking.value = false
  }
}

onMounted(() => {
  if (canGrant.value) void searchProjects()
})
</script>

<template>
  <section class="detail-view">
    <nav aria-label="Навігаційний ланцюжок">
      <ol class="breadcrumbs">
        <li class="breadcrumb-item"><RouterLink :to="{ name: 'projects' }">Проєкти</RouterLink></li>
        <li class="breadcrumb-separator" aria-hidden="true">/</li>
        <li class="breadcrumb-item breadcrumb-current" aria-current="page">Ролі погодження</li>
      </ol>
    </nav>
    <header class="detail-header">
      <h1 class="detail-title">Ролі погодження</h1>
    </header>

    <p v-if="!canGrant" class="alert-error" role="alert">Для керування ролями потрібен дозвіл адміністратора.</p>
    <template v-else>
      <section class="section-box">
        <div class="section-box-header"><h2 class="section-box-title">Проєкт</h2></div>
        <div class="section-box-body">
          <form class="form-actions" @submit.prevent="searchProjects">
            <div class="field">
              <label for="review-role-project-query">Код або назва проєкту</label>
              <input id="review-role-project-query" v-model="projectQuery" type="search" />
            </div>
            <button class="btn-secondary" type="submit" :disabled="projectsLoading">Знайти</button>
          </form>
          <p v-if="projectsError" class="alert-error" role="alert">{{ projectsError }}</p>
          <div class="field">
            <label for="review-role-project">Проєкт</label>
            <select id="review-role-project" v-model="projectId" :disabled="projectsLoading" @change="selectProject">
              <option value="">Оберіть проєкт</option>
              <option v-for="project in projects" :key="project.id" :value="project.id">{{ project.code }} · {{ project.name }}</option>
            </select>
          </div>
        </div>
      </section>

      <template v-if="projectId">
        <section class="section-box">
          <div class="section-box-header"><h2 class="section-box-title">Чинні призначення</h2></div>
          <div class="section-box-body">
            <p v-if="bindingsLoading" class="muted">Завантаження ролей…</p>
            <p v-else-if="bindingsError" class="alert-error" role="alert">{{ bindingsError }}</p>
            <p v-else-if="!bindings.length" class="muted">Рецензентів і погоджувачів ще не призначено.</p>
            <div v-else class="data-table-wrapper">
              <table class="data-table">
                <thead><tr><th>Користувач</th><th>Роль</th><th>Дія</th></tr></thead>
                <tbody>
                  <tr v-for="binding in bindings" :key="binding.id">
                    <td>{{ binding.display_name || binding.login }} ({{ binding.login }})</td>
                    <td>{{ binding.role_key === 'project.reviewer' ? 'Рецензент' : 'Погоджувач' }}</td>
                    <td>
                      <template v-if="canRevoke && pendingRevokeId === binding.id">
                        <span>Відкликати роль?</span>
                        <button class="btn-secondary btn-sm" type="button" :disabled="revoking" @click="revoke(binding.id)">Підтвердити</button>
                        <button class="btn-secondary btn-sm" type="button" :disabled="revoking" @click="pendingRevokeId = ''">Скасувати</button>
                      </template>
                      <button v-else-if="canRevoke" class="btn-secondary btn-sm" type="button" @click="pendingRevokeId = binding.id">Відкликати</button>
                    </td>
                  </tr>
                </tbody>
              </table>
            </div>
            <p v-if="revokeError" class="alert-error" role="alert">{{ revokeError }}</p>
          </div>
        </section>

        <section class="section-box">
          <div class="section-box-header"><h2 class="section-box-title">Надати роль</h2></div>
          <div class="section-box-body">
            <form class="form-actions" @submit.prevent="searchUsers">
              <div class="field">
                <label for="review-role-user-query">Ім’я або логін користувача</label>
                <input id="review-role-user-query" v-model="userQuery" type="search" minlength="2" />
              </div>
              <button class="btn-secondary" type="submit" :disabled="usersLoading">Знайти користувача</button>
            </form>
            <p v-if="usersError" class="alert-error" role="alert">{{ usersError }}</p>
            <form @submit.prevent="grant">
              <div class="plan-entry-fields">
                <div class="field">
                  <label for="review-role-user">Користувач</label>
                  <select id="review-role-user" v-model="userId" required>
                    <option value="">Оберіть користувача</option>
                    <option v-for="user in users" :key="user.id" :value="user.id">{{ user.display_name || user.login }} ({{ user.login }})</option>
                  </select>
                </div>
                <div class="field">
                  <label for="review-role-key">Роль</label>
                  <select id="review-role-key" v-model="roleKey">
                    <option value="project.reviewer">Рецензент</option>
                    <option value="project.approver">Погоджувач</option>
                  </select>
                </div>
              </div>
              <div class="field">
                <label for="review-role-reason">Підстава призначення</label>
                <textarea id="review-role-reason" v-model="reason" rows="2" required></textarea>
              </div>
              <p v-if="grantError" class="alert-error" role="alert">{{ grantError }}</p>
              <p v-if="grantSuccess" class="notice-box" role="status">{{ grantSuccess }}</p>
              <div class="form-actions">
                <button class="btn-primary" type="submit" :disabled="granting || !userId || !reason.trim()">{{ granting ? 'Надання…' : 'Надати роль' }}</button>
              </div>
            </form>
          </div>
        </section>
      </template>
    </template>
  </section>
</template>