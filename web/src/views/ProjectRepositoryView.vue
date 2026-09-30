<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'

import { api, ApiError } from '@/api/client'
import GitSetupHelp from '@/components/GitSetupHelp.vue'
import PIcon from '@/components/PIcon.vue'
import type { ProjectDetail, RepositoryBindingView } from '@/api/types'
import { useSessionStore } from '@/stores/session'

const props = defineProps<{ projectId: string }>()
const session = useSessionStore()

const project = ref<ProjectDetail | null>(null)
const repositoryBinding = ref<RepositoryBindingView | null>(null)
const loading = ref(true)
const loadError = ref('')
const remoteUrl = ref('')
const bindingError = ref('')
const binding = ref(false)

const canManageRepo = computed(() => project.value?.permissions?.includes('repository.manage') ?? session.hasPermission('repository.manage'))

async function load(): Promise<void> {
  loading.value = true
  loadError.value = ''
  try {
    project.value = await api.get<ProjectDetail>(`/projects/${props.projectId}`)
    try {
      repositoryBinding.value = await api.get<RepositoryBindingView>(`/projects/${props.projectId}/repository`)
    } catch {
      repositoryBinding.value = null
    }
  } catch (err) {
    loadError.value = err instanceof ApiError ? err.message : 'Не вдалося прочитати проєкт'
  } finally {
    loading.value = false
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
  <section class="detail-view">
    <nav v-if="project" aria-label="Навігаційний ланцюжок">
      <ol class="breadcrumbs">
        <li class="breadcrumb-item">
          <RouterLink :to="{ name: 'projects' }">Проєкти</RouterLink>
        </li>
        <li class="breadcrumb-separator" aria-hidden="true">/</li>
        <li class="breadcrumb-item">
          <RouterLink :to="{ name: 'project', params: { projectId: project.id } }">{{ project.code }}</RouterLink>
        </li>
        <li class="breadcrumb-separator" aria-hidden="true">/</li>
        <li class="breadcrumb-item breadcrumb-current" aria-current="page">Сховище</li>
      </ol>
    </nav>

    <p v-if="loadError" class="alert-error" role="alert">{{ loadError }}</p>
    <div v-else-if="loading" class="loading-state">
      <span class="muted">Завантаження сховища…</span>
    </div>

    <template v-else-if="project">
      <header class="detail-header">
        <div class="detail-title-group">
          <div class="detail-title-row">
            <span class="detail-code">{{ project.code }}</span>
            <h1 class="detail-title">Сховище</h1>
          </div>
          <p class="muted" style="margin: 0">Git-підключення проєкту {{ project.name }}</p>
        </div>
        <RouterLink :to="{ name: 'project', params: { projectId: project.id } }" class="btn-secondary">
          <PIcon name="overview" :size="14" />
          До огляду проєкту
        </RouterLink>
      </header>

      <div class="section-box">
        <div class="section-box-header">
          <div>
            <h2 class="section-box-title">Підключення Git-сховища</h2>
            <p class="section-box-desc">
              Джерело істини для артефактів — база даних DELMOS. Git потрібен лише для експорту ревізій у форматі Docs-as-Code.
            </p>
          </div>
        </div>
        <div class="section-box-body">
          <template v-if="repositoryBinding">
            <div class="repository-summary">
              <div>
                <p class="repository-summary-line">
                  <span :class="['badge', repositoryBinding.status === 'active' ? 'badge-success' : 'badge-danger']">
                    {{ repositoryBinding.status }}
                  </span>
                  <code>{{ repositoryBinding.remote_url }}</code>
                  <span class="muted">(гілка: {{ repositoryBinding.default_branch }})</span>
                </p>
                <p v-if="repositoryBinding.last_error" class="alert-error" style="margin: 0.5rem 0 0">
                  {{ repositoryBinding.last_error }}
                </p>
              </div>
            </div>
            <GitSetupHelp
              :remote-url="repositoryBinding.remote_url"
              :default-branch="repositoryBinding.default_branch"
              :project-code="project.code"
            />
          </template>

          <template v-else>
            <p class="muted repository-empty-copy">
              Проєкт не прив'язаний до Git-сховища — це нічого не блокує. План, вимоги та решта артефактів уже зберігаються в базі даних DELMOS із незмінною історією ревізій. Прив'язка потрібна лише тоді, коли ви хочете експортувати артефакти в Git.
            </p>

            <template v-if="canManageRepo">
              <GitSetupHelp :remote-url="null" default-branch="main" :project-code="project.code" />

              <form class="repository-form" @submit.prevent="handleBindRepository">
                <p v-if="bindingError" class="alert-error" role="alert">{{ bindingError }}</p>
                <div class="field">
                  <label for="remote-url">URL або локальний шлях bare-сховища</label>
                  <input
                    id="remote-url"
                    v-model="remoteUrl"
                    type="text"
                    placeholder="/var/lib/delmos/repos/inv-001.git або https://git.company.com/repo.git"
                    required
                  />
                  <span class="field-hint muted">Локальний Unix bare репозиторій або HTTPS Git endpoint</span>
                </div>
                <button class="btn-secondary" type="submit" :disabled="binding">
                  {{ binding ? 'Прив’язка...' : 'Прив’язати сховище' }}
                </button>
              </form>
            </template>
            <p v-else class="muted repository-permission-note">У вас немає права керувати Git-підключенням цього проєкту.</p>
          </template>
        </div>
      </div>
    </template>
  </section>
</template>
