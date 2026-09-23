<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'

import { api, ApiError } from '@/api/client'
import type { WorkProductRevisionView, WorkProductView } from '@/api/types'
import { useSessionStore } from '@/stores/session'

const props = defineProps<{ projectId: string; workProductId: string }>()
const session = useSessionStore()

const workProduct = ref<WorkProductView | null>(null)
const loadError = ref('')
const loading = ref(true)

const body = ref('')
const revising = ref(false)
const reviseError = ref('')

const retiring = ref(false)
const retireError = ref('')

const exporting = ref(false)
const exportError = ref('')
const exportedCommit = ref('')

async function load(): Promise<void> {
  loading.value = true
  loadError.value = ''
  try {
    workProduct.value = await api.get<WorkProductView>(
      `/projects/${props.projectId}/work-products/${props.workProductId}`,
    )
    body.value = workProduct.value.latest_revision.body
  } catch (err) {
    loadError.value = err instanceof ApiError ? err.message : 'Не вдалося прочитати work product'
  } finally {
    loading.value = false
  }
}

async function handleRevise(): Promise<void> {
  if (!workProduct.value) return
  reviseError.value = ''
  revising.value = true
  try {
    await api.post<WorkProductRevisionView>(
      `/projects/${props.projectId}/work-products/${props.workProductId}/revisions`,
      { expected_row_version: workProduct.value.row_version, body: body.value },
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
  <section>
    <RouterLink :to="{ name: 'project', params: { projectId } }">&larr; До проєкту</RouterLink>

    <p v-if="loadError" class="alert-error" role="alert">{{ loadError }}</p>
    <p v-else-if="loading">Завантаження…</p>
    <template v-else-if="workProduct">
      <h1>{{ workProduct.code }} — {{ workProduct.title }}</h1>
      <p>
        <span class="badge">{{ workProduct.type }}</span>
        <span class="badge">{{ workProduct.status }}</span>
        ревізія {{ workProduct.latest_revision.revision_number }}
      </p>
      <p class="muted">payload_hash: {{ workProduct.latest_revision.payload_hash }}</p>

      <div class="card">
        <h2>Зміст</h2>
        <p v-if="reviseError" class="alert-error" role="alert">{{ reviseError }}</p>
        <form v-if="workProduct.status === 'draft' && session.hasPermission('wp.edit')" @submit.prevent="handleRevise">
          <div class="field">
            <textarea v-model="body" rows="6"></textarea>
          </div>
          <button class="btn-primary" type="submit" :disabled="revising">Зберегти нову ревізію</button>
        </form>
        <pre v-else style="white-space: pre-wrap">{{ workProduct.latest_revision.body }}</pre>
      </div>

      <div v-if="workProduct.status === 'draft' && session.hasPermission('wp.retire')" class="card">
        <p v-if="retireError" class="alert-error" role="alert">{{ retireError }}</p>
        <button class="btn-danger" :disabled="retiring" @click="handleRetire">Вивести з експлуатації</button>
      </div>

      <div v-if="session.hasPermission('repository.manage')" class="card">
        <h2>Експорт у сховище</h2>
        <p v-if="exportError" class="alert-error" role="alert">{{ exportError }}</p>
        <p v-if="exportedCommit" class="muted">Комітовано: {{ exportedCommit }}</p>
        <button class="btn-secondary" :disabled="exporting" @click="handleExport">
          Експортувати {{ workProduct.code }}.md
        </button>
      </div>
    </template>
  </section>
</template>
