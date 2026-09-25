<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'

import { api, ApiError } from '@/api/client'
import GitSetupHelp from '@/components/GitSetupHelp.vue'
import MarkdownEditor from '@/components/MarkdownEditor.vue'
import PIcon from '@/components/PIcon.vue'
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

const canCreateWp = computed(() => project.value?.permissions?.includes('wp.create') ?? session.hasPermission('wp.create'))
const canManageRepo = computed(() => project.value?.permissions?.includes('repository.manage') ?? session.hasPermission('repository.manage'))

const wpCode = ref('')
const wpType = ref<string>(CORE_WORK_PRODUCT_TYPES[0])
const wpTitle = ref('')
const wpBody = ref('')
const wpMetadataText = ref('{}')
const creatingWp = ref(false)
const wpError = ref('')
const isCreateWpOpen = ref(false)

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
    let metadata: Record<string, unknown>
    try {
      const parsed: unknown = JSON.parse(wpMetadataText.value)
      if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') throw new Error('metadata')
      metadata = parsed as Record<string, unknown>
    } catch {
      wpError.value = 'Metadata має бути коректним JSON-обʼєктом'
      return
    }
    await api.post<WorkProductView>(`/projects/${props.projectId}/work-products`, {
      code: wpCode.value,
      type: wpType.value,
      title: wpTitle.value,
      body: wpBody.value,
      metadata,
    })
    wpCode.value = ''
    wpTitle.value = ''
    wpBody.value = ''
    wpMetadataText.value = '{}'
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
  <section class="detail-view">
    <!-- Хлібні крихти навігації (Pajamas Breadcrumbs) -->
    <nav aria-label="Навігаційний ланцюжок">
      <ol class="breadcrumbs">
        <li class="breadcrumb-item">
          <RouterLink :to="{ name: 'projects' }">Проєкти</RouterLink>
        </li>
        <li class="breadcrumb-separator" aria-hidden="true">/</li>
        <li class="breadcrumb-item breadcrumb-current" aria-current="page">
          {{ project?.code || '...' }}
        </li>
      </ol>
    </nav>

    <p v-if="loadError" class="alert-error" role="alert">{{ loadError }}</p>
    <div v-else-if="loading" class="loading-state">
      <span class="muted">Завантаження інформації про проєкт…</span>
    </div>

    <template v-else-if="project">
      <!-- Головний заголовок проєкту з метаданими та діями -->
      <header class="detail-header">
        <div class="detail-title-group">
          <div class="detail-title-row">
            <span class="detail-code">{{ project.code }}</span>
            <h1 class="detail-title">{{ project.name }}</h1>
            <span :class="['badge', 'badge-' + project.status]">{{ project.status }}</span>
          </div>
          <p v-if="project.description" class="muted" style="margin: 0">
            {{ project.description }}
          </p>
        </div>

        <div class="detail-actions">
          <RouterLink
            :to="{ name: 'project-plan', params: { projectId: project.id } }"
            class="btn-primary"
          >
            <PIcon name="planning" :size="14" />
            Редагувати план проєкту
          </RouterLink>
        </div>
      </header>

      <!-- Секція плану проєкту (Generic Project Plan PLAN-001) -->
      <div class="section-box">
        <div class="section-box-header">
          <div>
            <h2 class="section-box-title">Обов'язковий план проєкту ({{ project.plan.code }})</h2>
            <p class="section-box-desc">
              Ревізія {{ project.plan.revision_number }} · статус:
              <span :class="['badge', 'badge-' + project.plan.status]" style="margin-left: 0.25rem">
                {{ project.plan.status }}
              </span>
            </p>
          </div>
          <RouterLink
            :to="{ name: 'project-plan', params: { projectId: project.id } }"
            class="btn-secondary btn-sm"
          >
            Відкрити повний план
          </RouterLink>
        </div>
        <div class="section-box-body">
          <div v-if="project.plan.body" class="markdown-preview" style="min-height: auto; max-height: 180px; overflow-y: auto">
            <p style="white-space: pre-wrap; margin: 0">{{ project.plan.body }}</p>
          </div>
          <p v-else class="muted" style="margin: 0">
            План створено у базовій конфігурації. Натисніть «Відкрити повний план» для визначення цілей, меж і фаз.
          </p>
        </div>
      </div>

      <!-- Секція Git-сховища (RepositoryProvider) -->
      <div class="section-box">
        <div class="section-box-header">
          <div>
            <h2 class="section-box-title">Підключення Git-сховища — необов'язкове</h2>
            <p class="section-box-desc">
              Джерело істини для артефактів — база даних DELMOS. Git потрібен лише для
              експорту ревізій у форматі Docs-as-Code
            </p>
          </div>
        </div>
        <div class="section-box-body">
          <template v-if="repositoryBinding">
            <div style="display: flex; align-items: center; justify-content: space-between; gap: 1rem">
              <div>
                <p style="margin: 0 0 0.25rem; font-weight: 500">
                  <span :class="['badge', repositoryBinding.status === 'active' ? 'badge-success' : 'badge-danger']" style="margin-right: 0.5rem">
                    {{ repositoryBinding.status }}
                  </span>
                  <code>{{ repositoryBinding.remote_url }}</code>
                  <span class="muted" style="margin-left: 0.5rem">(гілка: {{ repositoryBinding.default_branch }})</span>
                </p>
                <p v-if="repositoryBinding.last_error" class="alert-error" style="margin: 0.5rem 0 0">{{ repositoryBinding.last_error }}</p>
              </div>
            </div>

            <GitSetupHelp
              :remote-url="repositoryBinding.remote_url"
              :default-branch="repositoryBinding.default_branch"
              :project-code="project.code"
            />
          </template>

          <template v-else>
            <p class="muted" style="margin: 0 0 1rem">
              Проєкт не прив'язаний до Git-сховища — це нічого не блокує. План, вимоги та решта
              артефактів уже зберігаються в базі даних DELMOS із незмінною історією ревізій.
              Прив'язка потрібна лише тоді, коли ви хочете експортувати артефакти в Git.
            </p>

            <template v-if="canManageRepo">
              <GitSetupHelp :remote-url="null" default-branch="main" :project-code="project.code" />

              <form style="max-width: 560px" @submit.prevent="handleBindRepository">
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
          </template>
        </div>
      </div>

      <!-- Секція Work Products -->
      <div class="section-box">
        <div class="section-box-header">
          <div>
            <h2 class="section-box-title">Інженерні артефакти (Work Products)</h2>
            <p class="section-box-desc">Вимоги, архітектурні рішення, тестові специфікації, звіти та записи</p>
          </div>
          <button
            v-if="canCreateWp && !isCreateWpOpen"
            class="btn-primary btn-sm"
            type="button"
            @click="isCreateWpOpen = true"
          >
            + Додати Work Product
          </button>
        </div>

        <div class="section-box-body">
          <!-- Форма створення Work Product -->
          <div v-if="isCreateWpOpen && canCreateWp" class="card" style="margin-bottom: 1.5rem; background: var(--color-surface-subtle)">
            <div class="create-header">
              <h3>Новий Work Product</h3>
              <button class="btn-close" type="button" title="Закрити" @click="isCreateWpOpen = false">✕</button>
            </div>
            <p v-if="wpError" class="alert-error" role="alert">{{ wpError }}</p>
            <form @submit.prevent="handleCreateWorkProduct">
              <div class="form-grid">
                <div class="field">
                  <label for="wp-code">Код артефакту</label>
                  <input id="wp-code" v-model="wpCode" type="text" placeholder="REQ-001" required />
                  <span class="field-hint muted">Префікс типу і порядковий номер (REQ-001, ARCH-001)</span>
                </div>
                <div class="field">
                  <label for="wp-type">Тип артефакту</label>
                  <select id="wp-type" v-model="wpType">
                    <option v-for="type in CORE_WORK_PRODUCT_TYPES" :key="type" :value="type">{{ type }}</option>
                  </select>
                  <span class="field-hint muted">Один із 6 канонічних типів ядра DELMOS</span>
                </div>
              </div>
              <div class="field">
                <label for="wp-title">Заголовок</label>
                <input id="wp-title" v-model="wpTitle" type="text" placeholder="Вимоги до теплового режиму інвертора" required />
              </div>
              <div class="field">
                <label>Зміст (початкова чернетка)</label>
                <MarkdownEditor v-model="wpBody" :rows="8" />
              </div>
              <div class="field">
                <label for="wp-metadata">Metadata JSON</label>
                <textarea id="wp-metadata" v-model="wpMetadataText" rows="3" spellcheck="false"></textarea>
              </div>
              <div class="form-actions">
                <button class="btn-primary" type="submit" :disabled="creatingWp">
                  {{ creatingWp ? 'Створення...' : 'Створити Work Product' }}
                </button>
                <button class="btn-secondary" type="button" :disabled="creatingWp" @click="isCreateWpOpen = false">
                  Скасувати
                </button>
              </div>
            </form>
          </div>

          <!-- Таблиця артефактів або Empty state -->
          <div v-if="workProducts.length > 0" class="data-table-wrapper">
            <table class="data-table">
              <thead>
                <tr>
                  <th>Код</th>
                  <th>Назва</th>
                  <th>Тип</th>
                  <th>Статус</th>
                  <th>Дії</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="wp in workProducts" :key="wp.id">
                  <td class="monospace-cell">
                    <RouterLink :to="{ name: 'work-product', params: { projectId: project.id, workProductId: wp.id } }">
                      {{ wp.code }}
                    </RouterLink>
                  </td>
                  <td>
                    <RouterLink :to="{ name: 'work-product', params: { projectId: project.id, workProductId: wp.id } }" style="color: var(--color-text); font-weight: 500">
                      {{ wp.title }}
                    </RouterLink>
                  </td>
                  <td>
                    <span class="badge badge-info">{{ wp.type }}</span>
                  </td>
                  <td>
                    <span :class="['badge', 'badge-' + wp.status]">{{ wp.status }}</span>
                  </td>
                  <td>
                    <RouterLink :to="{ name: 'work-product', params: { projectId: project.id, workProductId: wp.id } }" class="btn-secondary btn-sm">
                      Відкрити →
                    </RouterLink>
                  </td>
                </tr>
              </tbody>
            </table>
          </div>

          <!-- Empty State для Work Products -->
          <div v-else-if="!isCreateWpOpen" class="empty-state" style="padding: 2rem 1rem; margin: 0 auto">
            <h3 class="empty-title">Ще немає створених артефактів</h3>
            <p class="empty-desc">
              Додайте першу вимогу, специфікацію тестування чи архітектурний опис для старту наскрізної трасованості.
            </p>
            <button
              v-if="canCreateWp"
              class="btn-primary btn-sm"
              type="button"
              @click="isCreateWpOpen = true"
            >
              + Створити перший Work Product
            </button>
          </div>
        </div>
      </div>
    </template>
  </section>
</template>
