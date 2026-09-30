<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'

import { api, ApiError } from '@/api/client'
import type { GenericPlanManifest, PlanDetailView, WorkProductSummary } from '@/api/types'
import PlanSectionEditor from '@/components/PlanSectionEditor.vue'
import { useSessionStore } from '@/stores/session'

const props = defineProps<{ projectId: string }>()
const session = useSessionStore()

const plan = ref<PlanDetailView | null>(null)
const manifest = ref<GenericPlanManifest | null>(null)
const initialManifest = ref('')
const workProducts = ref<WorkProductSummary[]>([])
const loading = ref(true)
const saving = ref(false)
const error = ref('')
const saved = ref(false)

type SectionKey = Exclude<keyof GenericPlanManifest, 'governance' | 'extensions'>
type PlanItem = Record<string, unknown>
type PlanField = { key: string; label: string; kind: string; required?: boolean; optionsKey?: string }
type PlanSection = { key: SectionKey; title: string; singular: string; prefix: string; fields: PlanField[] }

const sections: PlanSection[] = [
  { key: 'objectives', title: 'Цілі та критерії успіху', singular: 'ціль', prefix: 'OBJ', fields: [
    { key: 'statement', label: 'Формулювання цілі', kind: 'textarea', required: true },
    { key: 'success_criteria', label: 'Критерії успіху', kind: 'lines' },
  ] },
  { key: 'scope_items', title: 'Межі проєкту', singular: 'елемент меж', prefix: 'SCOPE', fields: [
    { key: 'kind', label: 'Включення', kind: 'select', optionsKey: 'scope', required: true },
    { key: 'statement', label: 'Опис', kind: 'textarea', required: true },
    { key: 'rationale', label: 'Обґрунтування', kind: 'textarea' },
  ] },
  { key: 'assumptions', title: 'Припущення', singular: 'припущення', prefix: 'ASM', fields: [
    { key: 'statement', label: 'Припущення', kind: 'textarea', required: true },
    { key: 'owner_reference', label: 'Відповідальний', kind: 'text' },
    { key: 'validation_date', label: 'Дата перевірки', kind: 'date' },
    { key: 'status', label: 'Стан перевірки', kind: 'text' },
  ] },
  { key: 'constraints', title: 'Обмеження', singular: 'обмеження', prefix: 'CON', fields: [
    { key: 'kind', label: 'Тип обмеження', kind: 'text' },
    { key: 'statement', label: 'Опис', kind: 'textarea', required: true },
    { key: 'source_ref', label: 'Джерело', kind: 'text' },
    { key: 'enforcement', label: 'Спосіб контролю', kind: 'text' },
  ] },
  { key: 'responsibility_assignments', title: 'Розподіл відповідальності', singular: 'призначення', prefix: 'RESP', fields: [
    { key: 'role_key', label: 'Роль', kind: 'text', required: true },
    { key: 'subject', label: 'Особа або підрозділ', kind: 'text', required: true },
    { key: 'responsibility', label: 'За що відповідає', kind: 'textarea', required: true },
  ] },
  { key: 'deliverables', title: 'Очікувані результати', singular: 'результат', prefix: 'DEL', fields: [
    { key: 'name', label: 'Назва результату', kind: 'text', required: true },
    { key: 'work_product_id', label: 'Артефакт проєкту', kind: 'select', optionsKey: 'workProducts' },
    { key: 'required_status', label: 'Потрібний стан', kind: 'select', optionsKey: 'statuses', required: true },
  ] },
  { key: 'phases', title: 'Фази проєкту', singular: 'фазу', prefix: 'PH', fields: [
    { key: 'name', label: 'Назва фази', kind: 'text', required: true },
    { key: 'planned_start', label: 'Плановий початок', kind: 'date', required: true },
    { key: 'planned_finish', label: 'Планове завершення', kind: 'date', required: true },
    { key: 'depends_on', label: 'Після яких фаз', kind: 'multi', optionsKey: 'phases' },
  ] },
  { key: 'milestones', title: 'Контрольні віхи', singular: 'віху', prefix: 'MS', fields: [
    { key: 'name', label: 'Назва віхи', kind: 'text', required: true },
    { key: 'phase_key', label: 'Фаза', kind: 'select', optionsKey: 'phases', required: true },
    { key: 'target_date', label: 'Цільова дата', kind: 'date', required: true },
    { key: 'deliverable_keys', label: 'Очікувані результати', kind: 'multi', optionsKey: 'deliverables' },
    { key: 'acceptance_rule_keys', label: 'Правила приймання', kind: 'multi', optionsKey: 'rules' },
  ] },
]

const options = computed(() => ({
  scope: [{ value: 'in_scope', label: 'У межах проєкту' }, { value: 'out_of_scope', label: 'Поза межами проєкту' }],
  statuses: [
    { value: 'draft', label: 'Чернетка' }, { value: 'in_review', label: 'На погодженні' },
    { value: 'approved', label: 'Погоджено' }, { value: 'obsolete', label: 'Застарілий' },
  ],
  workProducts: workProducts.value.filter((item) => item.id !== plan.value?.work_product_id)
    .map((item) => ({ value: item.id, label: `${item.code} · ${item.title}` })),
  phases: (manifest.value?.phases ?? []).map((item) => ({ value: (item as PlanItem).key as string, label: (item as PlanItem).name as string })),
  deliverables: (manifest.value?.deliverables ?? []).map((item) => ({ value: (item as PlanItem).key as string, label: (item as PlanItem).name as string })),
  rules: (manifest.value?.acceptance_rules ?? []).map((item) => ({ value: (item as PlanItem).key as string, label: (item as PlanItem).key as string })),
}))

const canEdit = computed(() => plan.value?.permissions?.includes('wp.edit') ?? session.hasPermission('wp.edit'))
const isDirty = computed(() => manifest.value !== null && JSON.stringify(manifest.value) !== initialManifest.value)

function itemsFor(key: SectionKey): PlanItem[] {
  return (manifest.value?.[key] ?? []) as PlanItem[]
}

function loadManifest(data: GenericPlanManifest): void {
  const copy = JSON.parse(JSON.stringify(data)) as GenericPlanManifest
  for (const section of sections) {
    if (!Array.isArray(copy[section.key])) copy[section.key] = []
  }
  if (!Array.isArray(copy.acceptance_rules)) copy.acceptance_rules = []
  manifest.value = copy
  initialManifest.value = JSON.stringify(manifest.value)
}

function addItem(section: PlanSection): void {
  if (!manifest.value) return
  const items = itemsFor(section.key)
  const keys = new Set(items.map((item) => item.key))
  let number = 1
  while (keys.has(`${section.prefix}-${String(number).padStart(3, '0')}`)) number += 1
  const item: PlanItem = { key: `${section.prefix}-${String(number).padStart(3, '0')}` }
  for (const field of section.fields) {
    item[field.key] = field.kind === 'lines' || field.kind === 'multi' ? [] :
      field.key === 'kind' && section.key === 'scope_items' ? 'in_scope' :
      field.key === 'required_status' ? 'approved' : ''
  }
  items.push(item)
}

function removeItem(section: PlanSection, index: number): void {
  const items = itemsFor(section.key)
  const key = items[index]?.key
  if (section.key === 'phases' && manifest.value && (
    manifest.value.phases.some((item) => Array.isArray((item as PlanItem).depends_on) && ((item as PlanItem).depends_on as string[]).includes(key as string)) ||
    manifest.value.milestones.some((item) => (item as PlanItem).phase_key === key)
  )) {
    error.value = 'Спочатку приберіть зв’язки з цією фазою.'
    return
  }
  if (section.key === 'deliverables' && manifest.value?.milestones.some((item) =>
    Array.isArray((item as PlanItem).deliverable_keys) && ((item as PlanItem).deliverable_keys as string[]).includes(key as string))) {
    error.value = 'Спочатку приберіть результат з контрольних віх.'
    return
  }
  error.value = ''
  items.splice(index, 1)
}

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    plan.value = await api.get<PlanDetailView>(`/projects/${props.projectId}/plan`)
    loadManifest(plan.value.manifest)
    try {
      const result = await api.get<WorkProductSummary[]>(`/projects/${props.projectId}/work-products`)
      workProducts.value = Array.isArray(result) ? result : []
    } catch {
      workProducts.value = []
    }
  } catch (err) {
    error.value = err instanceof ApiError ? err.message : 'Не вдалося прочитати план проєкту'
  } finally {
    loading.value = false
  }
}

async function save(): Promise<void> {
  if (!plan.value || !manifest.value || !canEdit.value || !isDirty.value) return
  error.value = ''
  saved.value = false
  saving.value = true
  try {
    const submitted = JSON.parse(JSON.stringify(manifest.value)) as GenericPlanManifest
    for (const objective of submitted.objectives) {
      objective.success_criteria = (objective.success_criteria ?? []).map((item) => item.trim()).filter(Boolean)
    }
    const revised = await api.post<PlanDetailView>(`/projects/${props.projectId}/plan/revisions`, {
      expected_row_version: plan.value.row_version,
      body: plan.value.body,
      manifest: submitted,
    })
    plan.value = { ...revised, permissions: plan.value.permissions }
    loadManifest(revised.manifest)
    saved.value = true
  } catch (err) {
    error.value = err instanceof ApiError ? err.message : 'Не вдалося зберегти план проєкту'
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>

<template>
  <section class="detail-view">
    <nav aria-label="Навігаційний ланцюжок">
      <ol class="breadcrumbs">
        <li class="breadcrumb-item"><RouterLink :to="{ name: 'projects' }">Проєкти</RouterLink></li>
        <li class="breadcrumb-separator" aria-hidden="true">/</li>
        <li class="breadcrumb-item"><RouterLink :to="{ name: 'project', params: { projectId } }">Проєкт</RouterLink></li>
        <li class="breadcrumb-separator" aria-hidden="true">/</li>
        <li class="breadcrumb-item"><RouterLink :to="{ name: 'project-plan', params: { projectId } }">План проєкту</RouterLink></li>
        <li class="breadcrumb-separator" aria-hidden="true">/</li>
        <li class="breadcrumb-item breadcrumb-current" aria-current="page">Розділи плану</li>
      </ol>
    </nav>

    <div v-if="loading" class="loading-state"><span class="muted">Завантаження конфігурації…</span></div>
    <template v-else-if="plan && manifest">
      <header class="detail-header">
        <div class="detail-title-group">
          <h1 class="detail-title">Розділи плану</h1>
          <p class="muted" style="margin: 0">Ревізія {{ plan.revision_number }}</p>
        </div>
        <div class="detail-actions">
          <span v-if="isDirty" class="dirty-pill">Є незбережені зміни</span>
          <button v-if="canEdit" class="btn-primary" type="submit" form="plan-configuration-form" :disabled="saving || !isDirty">
            {{ saving ? 'Збереження...' : 'Зберегти нову ревізію' }}
          </button>
        </div>
      </header>

      <p v-if="error" class="alert-error" role="alert">{{ error }}</p>
      <p v-if="saved" class="notice-box" role="status">Нову ревізію плану збережено.</p>

      <form id="plan-configuration-form" class="plan-configuration-form" @submit.prevent="save">
        <PlanSectionEditor
          v-for="section in sections"
          :key="section.key"
          :section-key="section.key"
          :title="section.title"
          :singular="section.singular"
          :items="itemsFor(section.key)"
          :fields="section.fields"
          :options="options"
          :editable="canEdit"
          @add="addItem(section)"
          @remove="removeItem(section, $event)"
        />

        <section class="section-box">
          <div class="section-box-header"><h2 class="section-box-title">Правила приймання</h2></div>
          <div class="section-box-body plan-section-list">
            <p v-if="!manifest.acceptance_rules.length" class="muted">Правил ще немає.</p>
            <div v-for="rule in manifest.acceptance_rules" :key="String((rule as PlanItem).key)" class="plan-entry">
              <strong>{{ (rule as PlanItem).key }}</strong>
              <span class="muted">{{ (rule as PlanItem).predicate_key }}</span>
            </div>
          </div>
        </section>

        <section class="section-box">
          <div class="section-box-header"><h2 class="section-box-title">Керування змінами</h2></div>
          <div class="section-box-body">
            <label class="plan-checkbox">
              <input v-model="manifest.governance.change_control_required" type="checkbox" :disabled="!canEdit" />
              Вимагати формального контролю змін після затвердження плану
            </label>
          </div>
        </section>

        <div v-if="canEdit" class="form-actions">
          <button class="btn-primary" type="submit" :disabled="saving || !isDirty">
            {{ saving ? 'Збереження...' : 'Зберегти нову ревізію' }}
          </button>
        </div>
      </form>
    </template>
    <p v-else-if="error" class="alert-error" role="alert">{{ error }}</p>
  </section>
</template>