<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'

import { api, ApiError } from '@/api/client'
import MarkdownEditor from '@/components/MarkdownEditor.vue'
import type { WorkProductRevisionView, WorkProductView } from '@/api/types'
import { useSessionStore } from '@/stores/session'

const props = defineProps<{ projectId: string; workProductId: string }>()
const session = useSessionStore()

const workProduct = ref<WorkProductView | null>(null)
const loadError = ref('')
const loading = ref(true)

const body = ref('')
const metadataText = ref('{}')
const revising = ref(false)
const reviseError = ref('')
const metadataError = ref('')

const retiring = ref(false)
const retireError = ref('')

const exporting = ref(false)
const exportError = ref('')
const exportedCommit = ref('')

const canEdit = computed(() => workProduct.value?.permissions?.includes('wp.edit') ?? session.hasPermission('wp.edit'))
const canRetire = computed(() => workProduct.value?.permissions?.includes('wp.retire') ?? session.hasPermission('wp.retire'))
const canExport = computed(() => workProduct.value?.permissions?.includes('repository.manage') ?? session.hasPermission('repository.manage'))

const isDirty = computed(() => body.value !== workProduct.value?.latest_revision.body || metadataText.value !== JSON.stringify(workProduct.value?.latest_revision.metadata ?? {}, null, 2))

async function load(): Promise<void> {
  loading.value = true
  loadError.value = ''
  try {
    workProduct.value = await api.get<WorkProductView>(
      `/projects/${props.projectId}/work-products/${props.workProductId}`,
    )
    body.value = workProduct.value.latest_revision.body
    metadataText.value = JSON.stringify(workProduct.value.latest_revision.metadata ?? {}, null, 2)
    metadataError.value = ''
  } catch (err) {
    loadError.value = err instanceof ApiError ? err.message : 'Не вдалося прочитати work product'
  } finally {
    loading.value = false
  }
}

async function handleRevise(): Promise<void> {
  if (!workProduct.value) return
  reviseError.value = ''
  metadataError.value = ''
  let metadata: Record<string, unknown>
  try {
    const parsed: unknown = JSON.parse(metadataText.value)
    if (!parsed || Array.isArray(parsed) || typeof parsed !== 'object') throw new Error('metadata must be an object')
    metadata = parsed as Record<string, unknown>
  } catch {
    metadataError.value = 'Metadata має бути коректним JSON-обʼєктом'
    return
  }
  revising.value = true
  try {
    await api.post<WorkProductRevisionView>(
      `/projects/${props.projectId}/work-products/${props.workProductId}/revisions`,
      { expected_row_version: workProduct.value.row_version, body: body.value, metadata },
    )
    await load()
  } catch (err) {
    // 409: паралельний запис змінив work product раніше — потрібно перечитати й повторити.
    reviseError.value = err instanceof ApiError ? err.message : 'Не вдалося зберегти нову ревізію'
  } finally {
    revising.value = false
  }
}

async function handleRetire(): Promise<void> {
  if (!workProduct.value) return
  retireError.value = ''
  retiring.value = true
  try {
    await api.post(`/projects/${props.projectId}/work-products/${props.workProductId}/retire`, {
      expected_row_version: workProduct.value.row_version,
    })
    await load()
  } catch (err) {
    retireError.value = err instanceof ApiError ? err.message : 'Не вдалося вивести з експлуатації'
  } finally {
    retiring.value = false
  }
}

async function handleExport(): Promise<void> {
  exportError.value = ''
  exportedCommit.value = ''
  exporting.value = true
  try {
    const result = await api.post<{ commit_sha: string }>(
      `/projects/${props.projectId}/work-products/${props.workProductId}/export`,
    )
    exportedCommit.value = result.commit_sha
  } catch (err) {
    exportError.value = err instanceof ApiError ? err.message : 'Не вдалося експортувати у сховище'
  } finally {
    exporting.value = false
  }
}

onMounted(load)
</script>

<template>
  <section class="detail-view">
    <!-- Хлібні крихти (Pajamas Breadcrumbs) -->
    <nav v-if="workProduct" aria-label="Навігаційний ланцюжок">
      <ol class="breadcrumbs">
        <li class="breadcrumb-item">
          <RouterLink :to="{ name: 'projects' }">Проєкти</RouterLink>
        </li>
        <li class="breadcrumb-separator" aria-hidden="true">/</li>
        <li class="breadcrumb-item">
          <RouterLink :to="{ name: 'project', params: { projectId } }">Проєкт</RouterLink>
        </li>
        <li class="breadcrumb-separator" aria-hidden="true">/</li>
        <li class="breadcrumb-item breadcrumb-current" aria-current="page">
          {{ workProduct.code }}
        </li>
      </ol>
    </nav>

    <p v-if="loadError" class="alert-error" role="alert">{{ loadError }}</p>
    <div v-else-if="loading" class="loading-state">
      <span class="muted">Завантаження артефакту…</span>
    </div>

    <template v-else-if="workProduct">
      <!-- Заголовок артефакту з метаданими та діями -->
      <header class="detail-header">
        <div class="detail-title-group">
          <div class="detail-title-row">
            <span class="detail-code">{{ workProduct.code }}</span>
            <h1 class="detail-title">{{ workProduct.title }}</h1>
            <span class="badge badge-info">{{ workProduct.type }}</span>
            <span :class="['badge', 'badge-' + workProduct.status]">{{ workProduct.status }}</span>
            <span class="muted" style="font-size: 0.8125rem">Ревізія {{ workProduct.latest_revision.revision_number }}</span>
          </div>
          <p class="muted" style="margin: 0; font-size: 0.75rem; font-family: ui-monospace, SFMono-Regular, monospace">
            SHA-256 payload: {{ workProduct.latest_revision.payload_hash }}
          </p>
        </div>

        <div class="detail-actions">
          <span v-if="isDirty" class="dirty-pill">Є незбережені зміни</span>
          <button
            v-if="workProduct.status === 'draft' && canEdit"
            :class="['btn-primary', { 'btn-unsaved': isDirty }]"
            type="button"
            :disabled="revising || !isDirty"
            @click="handleRevise"
          >
            {{ revising ? 'Збереження...' : 'Зберегти нову ревізію' }}
          </button>
        </div>
      </header>

      <!-- Markdown редактор ревізії -->
      <div class="section-box">
        <div class="section-box-header">
          <div>
            <h2 class="section-box-title">Зміст документа (Docs-as-Code Markdown)</h2>
            <p class="section-box-desc">
              Будь-яка зміна фіксується як нова незмінна ревізія з аудитом авторства
            </p>
          </div>
          <span v-if="isDirty" class="dirty-pill">Незбережені правки</span>
        </div>

        <div class="section-box-body">
          <p v-if="reviseError" class="alert-error" role="alert">{{ reviseError }}</p>
          <p v-if="metadataError" class="alert-error" role="alert">{{ metadataError }}</p>

          <form v-if="workProduct.status === 'draft' && canEdit" @submit.prevent="handleRevise">
            <div class="field">
              <MarkdownEditor v-model="body" />
            </div>

            <div class="field" style="margin-top: 1.25rem">
              <label for="work-product-metadata">Структуровані метадані (JSON)</label>
              <span class="field-hint muted">Машиночитані атрибути стандарту, рівні вимог або трасованість</span>
              <textarea
                id="work-product-metadata"
                v-model="metadataText"
                rows="4"
                spellcheck="false"
                class="monospace-cell"
                style="margin-top: 0.25rem"
              ></textarea>
            </div>

            <div style="display: flex; justify-content: flex-end; margin-top: 1rem">
              <button
                :class="['btn-primary', { 'btn-unsaved': isDirty }]"
                type="submit"
                :disabled="revising || !isDirty"
              >
                {{ revising ? 'Збереження...' : 'Зберегти нову ревізію' }}
              </button>
            </div>
          </form>

          <MarkdownEditor v-else v-model="body" />
        </div>
      </div>

      <!-- Секція операцій життєвого циклу (Експорт та виведення з експлуатації) -->
      <div style="display: grid; grid-template-columns: repeat(auto-fit, minmax(280px, 1fr)); gap: 1rem">
        <!-- Експорт у Git -->
        <div v-if="canExport" class="section-box">
          <div class="section-box-header">
            <div>
              <h3 class="section-box-title" style="font-size: 0.9375rem">Експорт у Git-сховище</h3>
              <p class="section-box-desc">Фіксація ревізії у Docs-as-Code репозиторії</p>
            </div>
          </div>
          <div class="section-box-body">
            <p v-if="exportError" class="alert-error" role="alert">{{ exportError }}</p>
            <p v-if="exportedCommit" class="muted" style="font-size: 0.8125rem">
              Останній коміт: <code>{{ exportedCommit }}</code>
            </p>
            <button class="btn-secondary" type="button" :disabled="exporting" @click="handleExport">
              {{ exporting ? 'Експорт...' : `Експортувати ${workProduct.code}.md` }}
            </button>
          </div>
        </div>

        <!-- Виведення з експлуатації -->
        <div v-if="workProduct.status === 'draft' && canRetire" class="section-box">
          <div class="section-box-header">
            <div>
              <h3 class="section-box-title" style="font-size: 0.9375rem">Життєвий цикл</h3>
              <p class="section-box-desc">Завершення активної роботи над чернеткою</p>
            </div>
          </div>
          <div class="section-box-body">
            <p v-if="retireError" class="alert-error" role="alert">{{ retireError }}</p>
            <p class="muted" style="margin: 0 0 1rem; font-size: 0.8125rem">
              Переводить артефакт у стан <code>obsolete</code> без можливості подальших правок.
            </p>
            <button class="btn-danger" type="button" :disabled="retiring" @click="handleRetire">
              {{ retiring ? 'Виведення...' : 'Вивести з експлуатації' }}
            </button>
          </div>
        </div>
      </div>
    </template>
  </section>
</template>
