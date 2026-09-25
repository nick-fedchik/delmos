<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'

import { api, ApiError } from '@/api/client'
import MarkdownEditor from '@/components/MarkdownEditor.vue'
import type { GenericPlanManifest, PlanDetailView } from '@/api/types'
import { useSessionStore } from '@/stores/session'

const props = defineProps<{ projectId: string }>()
const session = useSessionStore()

const plan = ref<PlanDetailView | null>(null)
const notes = ref('')
const initialNotes = ref('')
const sectionText = ref<Record<string, string>>({})
const initialSectionText = ref<Record<string, string>>({})
const changeControlRequired = ref(true)
const initialChangeControl = ref(true)
const loading = ref(true)
const saving = ref(false)
const error = ref('')
const saved = ref(false)

const sections = [
  ['objectives', 'Цілі проєкту (Objectives)', 'Ключові вимірювані цілі та критерії успіху'],
  ['scope_items', 'Межі системи (Scope items)', 'Включені (in_scope) та виключені (out_of_scope) елементи'],
  ['assumptions', 'Припущення (Assumptions)', 'Фактори, що вважаються істинними без остаточних доказів'],
  ['constraints', 'Обмеження (Constraints)', 'Технічні, нормативні, часові або фінансові ліміти'],
  ['responsibility_assignments', 'Розподіл відповідальності (RACI)', 'Призначення ролей та суб’єктів відповідальності'],
  ['deliverables', 'Очікувані результати (Deliverables)', 'Зобов’язання та артефакти життєвого циклу'],
  ['phases', 'Фази життєвого циклу (Phases)', 'Послідовні етапи інженерного проєкту та залежності'],
  ['milestones', 'Контрольні віхи (Milestones)', 'Ключові точки перевірки, приймання та звітності'],
  ['acceptance_rules', 'Правила приймання (Acceptance rules)', 'Декларативні перевірки та предикати завершення'],
] as const

const canEdit = computed(() => plan.value?.permissions?.includes('wp.edit') ?? session.hasPermission('wp.edit'))
const canApply = computed(() => plan.value?.permissions?.includes('plan.apply') ?? session.hasPermission('plan.apply'))

const applying = ref(false)
const applyError = ref('')
const applySuccess = ref('')

const isDirty = computed(() => {
  if (notes.value !== initialNotes.value) return true
  if (changeControlRequired.value !== initialChangeControl.value) return true
  for (const [key] of sections) {
    if (sectionText.value[key] !== initialSectionText.value[key]) return true
  }
  return false
})

function loadSections(manifest: GenericPlanManifest): void {
  sectionText.value = {}
  initialSectionText.value = {}
  for (const [key] of sections) {
    const formatted = JSON.stringify(manifest[key], null, 2)
    sectionText.value[key] = formatted
    initialSectionText.value[key] = formatted
  }
  changeControlRequired.value = manifest.governance.change_control_required
  initialChangeControl.value = manifest.governance.change_control_required
}

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    plan.value = await api.get<PlanDetailView>(`/projects/${props.projectId}/plan`)
    notes.value = plan.value.body
    initialNotes.value = plan.value.body
    loadSections(plan.value.manifest)
  } catch (err) {
    error.value = err instanceof ApiError ? err.message : 'Не вдалося прочитати план проєкту'
  } finally {
    loading.value = false
  }
}

function parseSection(key: string): unknown[] {
  const parsed: unknown = JSON.parse(sectionText.value[key])
  if (!Array.isArray(parsed)) throw new Error(`Секція «${key}» має бути JSON-масивом`)
  return parsed
}

function buildManifest(): GenericPlanManifest {
  const manifest = {
    objectives: parseSection('objectives'),
    scope_items: parseSection('scope_items'),
    assumptions: parseSection('assumptions'),
    constraints: parseSection('constraints'),
    responsibility_assignments: parseSection('responsibility_assignments'),
    deliverables: parseSection('deliverables'),
    phases: parseSection('phases'),
    milestones: parseSection('milestones'),
    acceptance_rules: parseSection('acceptance_rules'),
    governance: { change_control_required: changeControlRequired.value },
    extensions: plan.value?.manifest.extensions ?? {},
  }
  return manifest
}

async function save(): Promise<void> {
  if (!plan.value) return
  error.value = ''
  saved.value = false
  saving.value = true
  try {
    const revised = await api.post<PlanDetailView>(`/projects/${props.projectId}/plan/revisions`, {
      expected_row_version: plan.value.row_version,
      body: notes.value,
      manifest: buildManifest(),
    })
    plan.value = revised
    notes.value = revised.body
    loadSections(revised.manifest)
    saved.value = true
  } catch (err) {
    error.value = err instanceof ApiError ? err.message : err instanceof Error ? err.message : 'Не вдалося зберегти план'
  } finally {
    saving.value = false
  }
}

async function handleApply(): Promise<void> {
  if (!plan.value) return
  applyError.value = ''
  applySuccess.value = ''
  applying.value = true
  try {
    const res = await api.post<{ config_generation: number }>(`/projects/${props.projectId}/plan/apply`, {
      expected_row_version: plan.value.row_version,
      revision_id: plan.value.revision_id,
    })
    applySuccess.value = `План успішно введено в дію (покоління конфігурації: ${res.config_generation}).`
    await load()
  } catch (err) {
    applyError.value = err instanceof ApiError ? err.message : 'Не вдалося ввести план у дію'
  } finally {
    applying.value = false
  }
}

onMounted(load)
</script>

<template>
  <section class="detail-view">
    <!-- Хлібні крихти (Pajamas Breadcrumbs) -->
    <nav v-if="plan" aria-label="Навігаційний ланцюжок">
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
          {{ plan.title }}
        </li>
      </ol>
    </nav>

    <p v-if="error" class="alert-error" role="alert">{{ error }}</p>
    <div v-else-if="loading" class="loading-state">
      <span class="muted">Завантаження плану проєкту…</span>
    </div>

    <template v-else-if="plan">
      <!-- Заголовок плану з діями та індикацією незбережених змін -->
      <header class="detail-header">
        <div class="detail-title-group">
          <div class="detail-title-row">
            <span class="detail-code">PLAN-001</span>
            <h1 class="detail-title">{{ plan.title }}</h1>
            <span :class="['badge', 'badge-' + plan.status]">{{ plan.status }}</span>
            <span class="badge badge-info">{{ plan.template_key }}@{{ plan.template_version }}</span>
            <span class="muted" style="font-size: 0.8125rem">Ревізія {{ plan.revision_number }}</span>
            <span v-if="plan.effective_plan_revision_id === plan.revision_id" class="badge badge-success">
              ✓ Діюча конфігурація (Gen {{ plan.config_generation }})
            </span>
            <span v-else-if="(plan.config_generation ?? 0) > 0" class="badge badge-warning">
              Чернетка (діє попередня Gen {{ plan.config_generation }})
            </span>
            <span v-else class="badge badge-warning">
              Не введено в дію
            </span>
          </div>
          <p class="muted" style="margin: 0">
            Обов'язковий нейтральний план інженерного життєвого циклу. Будь-які модифікації створюють нову незмінну ревізію.
          </p>
        </div>

        <div class="detail-actions">
          <span v-if="isDirty" class="dirty-pill">Є незбережені зміни</span>
          <button
            v-if="canEdit"
            :class="['btn-primary', { 'btn-unsaved': isDirty }]"
            type="button"
            :disabled="saving || !isDirty"
            @click="save"
          >
            {{ saving ? 'Збереження...' : 'Зберегти нову ревізію' }}
          </button>
          <button
            v-if="canApply && plan.effective_plan_revision_id !== plan.revision_id"
            class="btn-secondary"
            type="button"
            :disabled="applying || isDirty"
            title="Застосувати поточну ревізію плану до конфігурації проєкту (plan.apply)"
            @click="handleApply"
          >
            {{ applying ? 'Застосування...' : 'Ввести в дію (plan.apply)' }}
          </button>
        </div>
      </header>

      <p v-if="applySuccess" class="notice-box" role="status" style="margin: 0">
        <span style="color: var(--color-success); font-weight: 500">
          ✓ {{ applySuccess }}
        </span>
      </p>
      <p v-if="applyError" class="alert-error" role="alert" style="margin: 0">{{ applyError }}</p>

      <p v-if="saved" class="notice-box" role="status" style="margin: 0">
        <span style="color: var(--color-success); font-weight: 500">
          ✓ Нову ревізію плану успішно зафіксовано.
        </span>
      </p>

      <form @submit.prevent="save">
        <!-- Пояснювальні нотатки -->
        <div class="section-box">
          <div class="section-box-header">
            <div>
              <h2 class="section-box-title">Пояснювальні нотатки (Markdown)</h2>
              <p class="section-box-desc">Опис контексту для людини-інженера (не замінює системних правил)</p>
            </div>
          </div>
          <div class="section-box-body">
            <MarkdownEditor v-model="notes" :rows="8" />
          </div>
        </div>

        <!-- Структуровані секції маніфесту плану -->
        <div class="section-box" style="margin-top: 1.5rem">
          <div class="section-box-header">
            <div>
              <h2 class="section-box-title">Маніфест конфігурації плану</h2>
              <p class="section-box-desc">
                Машиночитані розділи планування згідно з шаблоном {{ plan.template_key }}
              </p>
            </div>
          </div>

          <div class="section-box-body" style="display: flex; flex-direction: column; gap: 1.25rem">
            <div v-for="([key, label, description]) in sections" :key="key" class="field">
              <label :for="`plan-${key}`">{{ label }}</label>
              <span class="field-hint muted">{{ description }}</span>
              <textarea
                :id="`plan-${key}`"
                v-model="sectionText[key]"
                rows="6"
                spellcheck="false"
                :readonly="!canEdit"
                class="monospace-cell"
                style="margin-top: 0.25rem"
              ></textarea>
            </div>

            <div class="field" style="padding-top: 0.5rem; border-top: 1px solid var(--color-border-subtle)">
              <label class="plan-checkbox">
                <input v-model="changeControlRequired" type="checkbox" :disabled="!canEdit" />
                <strong>Вимога Change Control:</strong> будь-які зміни конфігурації проєкту після затвердження вимагають формального протоколу змін.
              </label>
            </div>
          </div>
        </div>

        <!-- Нижня панель збереження -->
        <div v-if="canEdit" style="display: flex; align-items: center; justify-content: flex-end; gap: 1rem; margin-top: 1.5rem">
          <span v-if="isDirty" class="dirty-pill">Є незбережені зміни</span>
          <button
            :class="['btn-primary', { 'btn-unsaved': isDirty }]"
            type="submit"
            :disabled="saving || !isDirty"
          >
            {{ saving ? 'Збереження...' : 'Зберегти нову ревізію' }}
          </button>
        </div>
      </form>

      <!-- Проєкції виконання діючого плану (Phases & Milestones) -->
      <div v-if="plan.phases && plan.phases.length > 0" class="section-box" style="margin-top: 1.5rem">
        <div class="section-box-header">
          <div>
            <h2 class="section-box-title">Проєкція фаз інженерного життєвого циклу</h2>
            <p class="section-box-desc">Діючий граф етапів за застосованою конфігурацією проєкту</p>
          </div>
        </div>
        <div class="section-box-body data-table-wrapper" style="padding: 0">
          <table class="data-table">
            <thead>
              <tr>
                <th>Код фази</th>
                <th>Назва фази</th>
                <th>Запланований старт</th>
                <th>Запланований фініш</th>
                <th>Залежності</th>
                <th>Статус</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="ph in plan.phases" :key="ph.phase_key">
                <td class="monospace-cell">{{ ph.phase_key }}</td>
                <td style="font-weight: 500">{{ ph.name }}</td>
                <td>{{ ph.planned_start || '—' }}</td>
                <td>{{ ph.planned_finish || '—' }}</td>
                <td class="monospace-cell">{{ ph.depends_on?.join(', ') || '—' }}</td>
                <td>
                  <span :class="['badge', ph.status === 'completed' ? 'badge-success' : ph.status === 'active' ? 'badge-info' : 'badge-neutral']">
                    {{ ph.status }}
                  </span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div v-if="plan.milestones && plan.milestones.length > 0" class="section-box" style="margin-top: 1.5rem">
        <div class="section-box-header">
          <div>
            <h2 class="section-box-title">Проєкція контрольних віх (Milestones)</h2>
            <p class="section-box-desc">Точки приймання та обов'язкові артефакти</p>
          </div>
        </div>
        <div class="section-box-body data-table-wrapper" style="padding: 0">
          <table class="data-table">
            <thead>
              <tr>
                <th>Код віхи</th>
                <th>Фаза</th>
                <th>Назва віхи</th>
                <th>Цільова дата</th>
                <th>Очікувані результати</th>
                <th>Статус</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="ms in plan.milestones" :key="ms.milestone_key">
                <td class="monospace-cell">{{ ms.milestone_key }}</td>
                <td class="monospace-cell">{{ ms.phase_key }}</td>
                <td style="font-weight: 500">{{ ms.name }}</td>
                <td>{{ ms.target_date || '—' }}</td>
                <td>{{ ms.deliverable_keys?.join(', ') || '—' }}</td>
                <td>
                  <span :class="['badge', ms.status === 'passed' ? 'badge-success' : ms.status === 'failed' ? 'badge-danger' : 'badge-warning']">
                    {{ ms.status }}
                  </span>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </template>
  </section>
</template>