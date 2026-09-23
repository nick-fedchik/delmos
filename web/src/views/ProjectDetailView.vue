<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'

import { api, ApiError } from '@/api/client'
import type { ProjectDetail, RepositoryBindingView, WorkProductSummary, WorkProductView } from '@/api/types'
import { CORE_WORK_PRODUCT_TYPES } from '@/api/types'
import { useSessionStore } from '@/stores/session'

const props = defineProps<{ projectId: string }>()
const session = useSessionStore()

const project = ref<ProjectDetail | null>(null)
const workProducts = ref<WorkProductSummary[]>([])
const repositoryBinding = ref<RepositoryBindingView | null>(null)
const loadError = ref('')
const loading = ref(true)

const wpCode = ref('')
const wpType = ref<string>(CORE_WORK_PRODUCT_TYPES[0])
const wpTitle = ref('')
const wpBody = ref('')
const creatingWp = ref(false)
const wpError = ref('')

const remoteUrl = ref('')
const bindingError = ref('')
const binding = ref(false)

async function load(): Promise<void> {
  loading.value = true
  loadError.value = ''
  try {
    project.value = await api.get<ProjectDetail>(`/projects/${props.projectId}`)
    workProducts.value = await api.get<WorkProductSummary[]>(`/projects/${props.projectId}/work-products`)
    try {
      repositoryBinding.value = await api.get<RepositoryBindingView>(`/projects/${props.projectId}/repository`)
    } catch {
      repositoryBinding.value = null // сховище ще не прив'язано — не помилка сторінки
    }
  } catch (err) {
    loadError.value = err instanceof ApiError ? err.message : 'Не вдалося прочитати проєкт'
  } finally {
    loading.value = false
  }
}

async function handleCreateWorkProduct(): Promise<void> {
  wpError.value = ''
  creatingWp.value = true
  try {
    await api.post<WorkProductView>(`/projects/${props.projectId}/work-products`, {
      code: wpCode.value,
      type: wpType.value,
      title: wpTitle.value,
      body: wpBody.value,
    })
    wpCode.value = ''
    wpTitle.value = ''
    wpBody.value = ''
    await load()
  } catch (err) {
    wpError.value = err instanceof ApiError ? err.message : 'Не вдалося створити work product'
  } finally {
    creatingWp.value = false
  }
}

async function handleBindRepository(): Promise<void> {
  bindingError.value = ''
  binding.value = true
  try {
    repositoryBinding.value = await api.post<RepositoryBindingView>(`/projects/${props.projectId}/repository`, {
      remote_url: remoteUrl.value,
    })
    remoteUrl.value = ''
  } catch (err) {
    bindingError.value = err instanceof ApiError ? err.message : 'Не вдалося прив\u2019язати сховище'
  } finally {
    binding.value = false
  }
}

onMounted(load)
</script>

<template>
  <section>
    <p v-if="loadError" class="alert-error" role="alert">{{ loadError }}</p>
    <p v-else-if="loading">Завантаження…</p>
    <template v-else-if="project">
      <h1>{{ project.code }} — {{ project.name }}</h1>
      <p class="muted">{{ project.description }}</p>

      <div class="card">
        <h2>План проєкту ({{ project.plan.code }})</h2>
        <p>
          <span class="badge">{{ project.plan.status }}</span>
          ревізія {{ project.plan.revision_number }}
        </p>
        <pre style="white-space: pre-wrap">{{ project.plan.body }}</pre>
      </div>

      <div class="card">
        <h2>Сховище</h2>
        <template v-if="repositoryBinding">
          <p>
            <span class="badge">{{ repositoryBinding.status }}</span>
            {{ repositoryBinding.remote_url }} ({{ repositoryBinding.default_branch }})
          </p>
          <p v-if="repositoryBinding.last_error" class="alert-error">{{ repositoryBinding.last_error }}</p>
        </template>
        <p v-else class="muted">Проєкт ще не прив'язаний до Git-сховища.</p>

        <form v-if="session.hasPermission('repository.manage')" @submit.prevent="handleBindRepository">
          <p v-if="bindingError" class="alert-error" role="alert">{{ bindingError }}</p>
          <div class="field">
            <label for="remote-url">URL або локальний шлях сховища</label>
            <input id="remote-url" v-model="remoteUrl" type="text" placeholder="/var/lib/delmos/repos/inv-001.git" required />
          </div>
          <button class="btn-secondary" type="submit" :disabled="binding">Прив'язати</button>
        </form>
      </div>

      <h2>Work Products</h2>
      <ul v-if="workProducts.length" class="card-list">
        <li v-for="wp in workProducts" :key="wp.id">
          <RouterLink :to="{ name: 'work-product', params: { projectId: project.id, workProductId: wp.id } }">
            {{ wp.code }}
          </RouterLink>
          — {{ wp.title }}
          <span class="badge">{{ wp.type }}</span>
          <span class="badge">{{ wp.status }}</span>
        </li>
      </ul>
      <p v-else class="muted">Ще немає жодного work product, окрім плану.</p>

      <div v-if="session.hasPermission('wp.create')" class="card">
        <h2>Створити Work Product</h2>
        <p v-if="wpError" class="alert-error" role="alert">{{ wpError }}</p>
        <form @submit.prevent="handleCreateWorkProduct">
          <div class="field">
            <label for="wp-code">Код</label>
            <input id="wp-code" v-model="wpCode" type="text" placeholder="REQ-001" required />
          </div>
          <div class="field">
            <label for="wp-type">Тип</label>
            <select id="wp-type" v-model="wpType">
              <option v-for="type in CORE_WORK_PRODUCT_TYPES" :key="type" :value="type">{{ type }}</option>
            </select>
          </div>
          <div class="field">
            <label for="wp-title">Заголовок</label>
            <input id="wp-title" v-model="wpTitle" type="text" required />
          </div>
          <div class="field">
            <label for="wp-body">Зміст (перша ревізія)</label>
            <textarea id="wp-body" v-model="wpBody" rows="4"></textarea>
          </div>
          <button class="btn-primary" type="submit" :disabled="creatingWp">Створити</button>
        </form>
      </div>
    </template>
  </section>
</template>
