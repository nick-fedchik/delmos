<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { RouterLink } from 'vue-router'

import { api, ApiError } from '@/api/client'
import MarkdownEditor from '@/components/MarkdownEditor.vue'
import type { PlanApprovalEvidence, PlanDetailView, PlanReviewCandidate, PlanReviewStatus } from '@/api/types'
import { useSessionStore } from '@/stores/session'

const props = defineProps<{ projectId: string }>()
const session = useSessionStore()

const plan = ref<PlanDetailView | null>(null)
const notes = ref('')
const initialNotes = ref('')
const loading = ref(true)
const saving = ref(false)
const error = ref('')
const saved = ref(false)

const canEdit = computed(() => plan.value?.permissions?.includes('wp.edit') ?? session.hasPermission('wp.edit'))
const canApply = computed(() => plan.value?.permissions?.includes('plan.apply') ?? session.hasPermission('plan.apply'))
const canSubmit = computed(() => plan.value?.permissions?.includes('wp.submit') ?? session.hasPermission('wp.submit'))
const canReview = computed(() => plan.value?.permissions?.includes('wp.review') ?? session.hasPermission('wp.review'))
const canApprove = computed(() => plan.value?.permissions?.includes('wp.approve') ?? session.hasPermission('wp.approve'))
const canRequestChanges = computed(() => plan.value?.permissions?.includes('wp.request_changes') ?? session.hasPermission('wp.request_changes'))

const applying = ref(false)
const applyError = ref('')
const applySuccess = ref('')
const candidates = ref<PlanReviewCandidate[]>([])
const candidatesLoading = ref(false)
const candidatesError = ref('')
const reviewerId = ref('')
const approverId = ref('')
const submitting = ref(false)
const submitError = ref('')
const submitSuccess = ref(false)
const review = ref<PlanReviewStatus | null>(null)
const approvalEvidence = ref<PlanApprovalEvidence[]>([])
const evidenceLoading = ref(false)
const evidenceError = ref('')
const reviewLoading = ref(false)
const reviewError = ref('')
const decisionError = ref('')
const decisionFeedback = ref('')
const deciding = ref(false)
const requestingChanges = ref(false)
const changeReason = ref('')

const reviewers = computed(() => candidates.value.filter((candidate) => candidate.can_review))
const approvers = computed(() => candidates.value.filter((candidate) => candidate.can_approve))
const myReviewPending = computed(() => canReview.value && review.value?.participants.some((person) => person.role === 'reviewer' && person.assigned_to_me && !person.completed))
const myApprovalPending = computed(() => canApprove.value && review.value?.has_positive_review && review.value.participants.some((person) => person.role === 'approver' && person.assigned_to_me && !person.completed))
const myChangeRequestAllowed = computed(() => canRequestChanges.value && review.value?.participants.some((person) => person.assigned_to_me))

const isDirty = computed(() => notes.value !== initialNotes.value)

const readiness = computed(() => {
  const manifest = plan.value?.manifest
  if (!manifest) return []
  return [
    { label: 'Цілі з критеріями успіху', complete: manifest.objectives?.some((item) => item.statement?.trim() && item.success_criteria?.some((criterion) => criterion.trim())) ?? false },
    { label: 'Межі проєкту', complete: manifest.scope_items?.some((item) => item.kind === 'in_scope' && item.statement?.trim()) ?? false },
    { label: 'Відповідальні', complete: manifest.responsibility_assignments?.some((item) => item.subject?.trim() && item.responsibility?.trim()) ?? false },
    { label: 'Результати та артефакти', complete: manifest.deliverables?.some((item) => item.name?.trim() && item.work_product_id) ?? false },
    { label: 'Фази з плановими датами', complete: manifest.phases?.some((item) => item.name?.trim() && item.planned_start && item.planned_finish) ?? false },
    { label: 'Контрольні віхи', complete: manifest.milestones?.some((item) => item.name?.trim() && item.phase_key && item.target_date) ?? false },
  ].map((item) => ({ ...item, complete: Boolean(item.complete) }))
})

async function load(): Promise<void> {
  loading.value = true
  error.value = ''
  try {
    plan.value = await api.get<PlanDetailView>(`/projects/${props.projectId}/plan`)
    review.value = null
    approvalEvidence.value = []
    notes.value = plan.value.body
    initialNotes.value = plan.value.body
    if (plan.value.status === 'draft' && canSubmit.value) {
      candidatesLoading.value = true
      candidatesError.value = ''
      try {
        const available = await api.get<PlanReviewCandidate[]>(`/projects/${props.projectId}/plan/review-candidates`)
        candidates.value = Array.isArray(available) ? available : []
      } catch (err) {
        candidatesError.value = err instanceof ApiError ? err.message : 'Не вдалося завантажити учасників погодження'
      } finally {
        candidatesLoading.value = false
      }
    } else if (plan.value.status === 'in_review') {
      reviewLoading.value = true
      reviewError.value = ''
      try {
        review.value = await api.get<PlanReviewStatus>(`/projects/${props.projectId}/plan/review`)
      } catch (err) {
        reviewError.value = err instanceof ApiError ? err.message : 'Не вдалося прочитати стан погодження'
      } finally {
        reviewLoading.value = false
      }
    } else if (plan.value.status === 'approved') {
      evidenceLoading.value = true
      evidenceError.value = ''
      try {
        const result = await api.get<PlanApprovalEvidence[]>(`/projects/${props.projectId}/plan/approval-evidence`)
        approvalEvidence.value = Array.isArray(result) ? result.filter((item) => item.revision_id === plan.value?.revision_id) : []
      } catch (err) {
        evidenceError.value = err instanceof ApiError ? err.message : 'Не вдалося прочитати підтвердження плану'
      } finally {
        evidenceLoading.value = false
      }
    }
  } catch (err) {
    error.value = err instanceof ApiError ? err.message : 'Не вдалося прочитати план проєкту'
  } finally {
    loading.value = false
  }
}

async function recordDecision(kind: 'reviews' | 'approvals' | 'request-changes'): Promise<void> {
  if (deciding.value || isDirty.value || !plan.value || plan.value.status !== 'in_review' || !review.value || review.value.revision_id !== plan.value.revision_id) return
  if (kind === 'reviews' && !myReviewPending.value) return
  if (kind === 'approvals' && !myApprovalPending.value) return
  if (kind === 'request-changes' && (!myChangeRequestAllowed.value || !changeReason.value.trim())) return
  deciding.value = true
  decisionError.value = ''
  decisionFeedback.value = ''
  submitSuccess.value = false
  try {
    await api.post(`/projects/${props.projectId}/work-products/${plan.value.work_product_id}/${kind}`,
      kind === 'request-changes' ? { reason: changeReason.value.trim() } : {})
    decisionFeedback.value = kind === 'reviews' ? 'Рецензію зафіксовано.' : kind === 'approvals' ? 'План погоджено.' : 'План повернуто на доопрацювання.'
    requestingChanges.value = false
    changeReason.value = ''
    await load()
  } catch (err) {
    decisionError.value = err instanceof ApiError ? err.message : 'Не вдалося зафіксувати рішення'
  } finally {
    deciding.value = false
  }
}

async function submitForReview(): Promise<void> {
  if (submitting.value || !plan.value || plan.value.status !== 'draft' || !canSubmit.value || isDirty.value || !reviewerId.value || !approverId.value) return
  submitError.value = ''
  submitSuccess.value = false
  decisionFeedback.value = ''
  submitting.value = true
  try {
    await api.post(`/projects/${props.projectId}/work-products/${plan.value.work_product_id}/submit`, {
      assignments: [
        { user_id: reviewerId.value, assignment_role: 'reviewer' },
        { user_id: approverId.value, assignment_role: 'approver' },
      ],
    })
    submitSuccess.value = true
    reviewerId.value = ''
    approverId.value = ''
    await load()
  } catch (err) {
    submitError.value = err instanceof ApiError ? err.message : 'Не вдалося подати план на погодження'
  } finally {
    submitting.value = false
  }
}

async function save(): Promise<void> {
  if (!plan.value) return
  error.value = ''
  saved.value = false
  submitSuccess.value = false
  decisionFeedback.value = ''
  saving.value = true
  try {
    const revised = await api.post<PlanDetailView>(`/projects/${props.projectId}/plan/revisions`, {
      expected_row_version: plan.value.row_version,
      body: notes.value,
      manifest: plan.value.manifest,
    })
    plan.value = { ...revised, permissions: plan.value.permissions }
    notes.value = revised.body
    initialNotes.value = revised.body
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
            v-if="canApply && plan.status === 'approved' && plan.effective_plan_revision_id !== plan.revision_id"
            class="btn-secondary"
            type="button"
            :disabled="applying || isDirty"
            title="Ввести погоджену ревізію плану в дію"
            @click="handleApply"
          >
            {{ applying ? 'Застосування...' : 'Ввести в дію' }}
          </button>
        </div>
      </header>

      <p v-if="plan.status === 'draft' && canApply && plan.effective_plan_revision_id !== plan.revision_id" class="muted">
        Перш ніж вводити цей план у дію, його ревізію має незалежно погодити уповноважена особа.
      </p>

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

      <section v-if="plan.status === 'draft' && canEdit" class="section-box plan-overview-section">
        <div class="section-box-header">
          <h2 class="section-box-title">Підготовка до погодження</h2>
          <RouterLink :to="{ name: 'project-plan-configuration', params: { projectId } }" class="btn-secondary btn-sm">Заповнити розділи</RouterLink>
        </div>
        <div class="section-box-body">
          <ul class="plan-readiness-list">
            <li v-for="item in readiness" :key="item.label">
              <span :class="['badge', item.complete ? 'badge-success' : 'badge-warning']">{{ item.complete ? 'Готово' : 'Доповнити' }}</span>
              {{ item.label }}
            </li>
          </ul>
        </div>
      </section>

      <section v-if="plan.status === 'draft' && canSubmit" class="section-box plan-overview-section">
        <div class="section-box-header"><h2 class="section-box-title">Подати на погодження</h2></div>
        <div class="section-box-body">
          <p v-if="candidatesLoading" class="muted">Завантаження учасників…</p>
          <p v-else-if="candidatesError" class="alert-error" role="alert">{{ candidatesError }}</p>
          <p v-else-if="!reviewers.length || !approvers.length" class="muted">
            Для погодження потрібні незалежні рецензент і погоджувач із правами в цьому проєкті.
            <RouterLink v-if="session.hasPermission('access.grant')" :to="{ name: 'admin-review-roles' }">Налаштувати ролі</RouterLink>
            <template v-else>Зверніться до адміністратора ролей.</template>
          </p>
          <form v-else @submit.prevent="submitForReview">
            <div class="plan-entry-fields">
              <div class="field">
                <label for="plan-reviewer">Рецензент</label>
                <select id="plan-reviewer" v-model="reviewerId" required>
                  <option value="">Оберіть рецензента</option>
                  <option v-for="candidate in reviewers" :key="candidate.id" :value="candidate.id">
                    {{ candidate.display_name || candidate.login }} ({{ candidate.login }})
                  </option>
                </select>
              </div>
              <div class="field">
                <label for="plan-approver">Погоджувач</label>
                <select id="plan-approver" v-model="approverId" required>
                  <option value="">Оберіть погоджувача</option>
                  <option v-for="candidate in approvers" :key="candidate.id" :value="candidate.id">
                    {{ candidate.display_name || candidate.login }} ({{ candidate.login }})
                  </option>
                </select>
              </div>
            </div>
            <p v-if="isDirty" class="muted">Спершу збережіть зміни нотаток.</p>
            <p v-if="submitError" class="alert-error" role="alert">{{ submitError }}</p>
            <div class="form-actions">
              <button class="btn-primary" type="submit" :disabled="submitting || isDirty || !reviewerId || !approverId">
                {{ submitting ? 'Подання…' : 'Подати ревізію' }}
              </button>
            </div>
          </form>
        </div>
      </section>

      <p v-if="submitSuccess" class="notice-box" role="status">План подано на незалежне погодження.</p>

      <section v-if="plan.status === 'in_review'" class="section-box plan-overview-section">
        <div class="section-box-header"><h2 class="section-box-title">Погодження ревізії</h2></div>
        <div class="section-box-body">
          <p v-if="reviewLoading" class="muted">Завантаження стану погодження…</p>
          <p v-else-if="reviewError" class="alert-error" role="alert">{{ reviewError }}</p>
          <template v-else-if="review">
            <ul class="plan-readiness-list">
              <li v-for="(person, index) in review.participants" :key="index">
                <span :class="['badge', person.completed ? 'badge-success' : 'badge-warning']">
                  {{ person.completed ? 'Готово' : 'Очікується' }}
                </span>
                {{ person.display_name }} · {{ person.role === 'reviewer' ? 'Рецензія' : 'Погодження' }}
              </li>
            </ul>
            <p v-if="isDirty && (myReviewPending || myApprovalPending || myChangeRequestAllowed)" class="muted">Спершу збережіть зміни нотаток.</p>
            <div v-if="review.revision_id === plan.revision_id" class="form-actions plan-review-actions">
              <button v-if="myReviewPending" class="btn-primary" type="button" :disabled="deciding || isDirty" @click="recordDecision('reviews')">Підтвердити рецензію</button>
              <button v-if="myApprovalPending" class="btn-primary" type="button" :disabled="deciding || isDirty" @click="recordDecision('approvals')">Погодити план</button>
              <button v-if="myChangeRequestAllowed && !requestingChanges" class="btn-secondary" type="button" :disabled="deciding || isDirty" @click="requestingChanges = true">Потрібні зміни</button>
            </div>
            <form v-if="myChangeRequestAllowed && requestingChanges" @submit.prevent="recordDecision('request-changes')">
              <div class="field">
                <label for="plan-change-reason">Причина повернення на доопрацювання</label>
                <textarea id="plan-change-reason" v-model="changeReason" rows="3" required></textarea>
              </div>
              <div class="form-actions">
                <button class="btn-primary" type="submit" :disabled="deciding || isDirty || !changeReason.trim()">Повернути на доопрацювання</button>
                <button class="btn-secondary" type="button" :disabled="deciding" @click="requestingChanges = false">Скасувати</button>
              </div>
            </form>
            <p v-if="decisionError" class="alert-error" role="alert">{{ decisionError }}</p>
          </template>
        </div>
      </section>

      <p v-if="decisionFeedback" class="notice-box" role="status">{{ decisionFeedback }}</p>

      <section v-if="plan.status === 'approved'" class="section-box plan-overview-section">
        <div class="section-box-header"><h2 class="section-box-title">Підтвердження ревізії {{ plan.revision_number }}</h2></div>
        <div class="section-box-body">
          <p v-if="evidenceLoading" class="muted">Завантаження підтверджень…</p>
          <p v-else-if="evidenceError" class="alert-error" role="alert">{{ evidenceError }}</p>
          <p v-else-if="!approvalEvidence.length" class="alert-error" role="alert">Не знайдено підтверджень поточної ревізії.</p>
          <ul v-else class="plan-readiness-list">
            <li v-for="(item, index) in approvalEvidence" :key="index">
              <span class="badge badge-success">{{ item.decision_kind === 'review' ? 'Рецензія' : 'Погодження' }}</span>
              {{ item.display_name }} · {{ new Intl.DateTimeFormat('uk-UA', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(item.decided_at)) }}
            </li>
          </ul>
        </div>
      </section>

      <section class="section-box plan-overview-section">
        <div class="section-box-header">
          <h2 class="section-box-title">Мета й межі проєкту</h2>
        </div>
        <div class="section-box-body plan-overview">
          <div>
            <h3>Цілі та критерії успіху</h3>
            <p v-if="!plan.manifest.objectives?.length" class="muted">Цілі ще не визначено.</p>
            <ul v-else>
              <li v-for="objective in plan.manifest.objectives" :key="String(objective.key)">
                {{ objective.statement }}
                <ul v-if="objective.success_criteria?.length">
                  <li v-for="criterion in objective.success_criteria" :key="String(criterion)">{{ criterion }}</li>
                </ul>
              </li>
            </ul>
          </div>
          <div>
            <h3>У межах проєкту</h3>
            <ul v-if="plan.manifest.scope_items?.some((item) => item.kind === 'in_scope')">
              <li v-for="item in plan.manifest.scope_items.filter((entry) => entry.kind === 'in_scope')" :key="String(item.key)">
                {{ item.statement }}
              </li>
            </ul>
            <p v-else class="muted">Межі ще не визначено.</p>
          </div>
          <div v-if="plan.manifest.scope_items?.some((item) => item.kind === 'out_of_scope')">
            <h3>Поза межами проєкту</h3>
            <ul>
              <li v-for="item in plan.manifest.scope_items.filter((entry) => entry.kind === 'out_of_scope')" :key="String(item.key)">
                {{ item.statement }}
              </li>
            </ul>
          </div>
        </div>
      </section>

      <section class="section-box plan-overview-section">
        <div class="section-box-header">
          <h2 class="section-box-title">Результати й графік поточної ревізії</h2>
        </div>
        <div class="section-box-body plan-overview">
          <div>
            <h3>Очікувані результати</h3>
            <ul v-if="plan.manifest.deliverables?.length">
              <li v-for="item in plan.manifest.deliverables" :key="String(item.key)">
                {{ item.name }} · {{ item.required_status }}
              </li>
            </ul>
            <p v-else class="muted">Результати ще не визначено.</p>
          </div>
          <div>
            <h3>Заплановані фази</h3>
            <ul v-if="plan.manifest.phases?.length">
              <li v-for="phase in plan.manifest.phases" :key="String(phase.key)">
                {{ phase.name }} · {{ phase.planned_start }} – {{ phase.planned_finish }}
              </li>
            </ul>
            <p v-else class="muted">Фази ще не визначено.</p>
          </div>
          <div>
            <h3>Контрольні віхи</h3>
            <ul v-if="plan.manifest.milestones?.length">
              <li v-for="milestone in plan.manifest.milestones" :key="String(milestone.key)">
                {{ milestone.name }} · {{ milestone.target_date }}
              </li>
            </ul>
            <p v-else class="muted">Віхи ще не визначено.</p>
          </div>
        </div>
      </section>

      <section class="section-box plan-overview-section">
        <div class="section-box-header">
          <h2 class="section-box-title">Відповідальність і умови виконання</h2>
        </div>
        <div class="section-box-body plan-overview">
          <div>
            <h3>Розподіл відповідальності</h3>
            <ul v-if="plan.manifest.responsibility_assignments?.length">
              <li v-for="assignment in plan.manifest.responsibility_assignments" :key="assignment.key">
                {{ assignment.subject }} · {{ assignment.responsibility }}
              </li>
            </ul>
            <p v-else class="muted">Відповідальних ще не визначено.</p>
            <RouterLink :to="{ name: 'project-stakeholders', params: { projectId } }">Стейкхолдери проєкту</RouterLink>
          </div>
          <div>
            <h3>Припущення й обмеження</h3>
            <ul v-if="plan.manifest.assumptions?.length || plan.manifest.constraints?.length">
              <li v-for="assumption in plan.manifest.assumptions" :key="assumption.key">{{ assumption.statement }}</li>
              <li v-for="constraint in plan.manifest.constraints" :key="constraint.key">{{ constraint.statement }}</li>
            </ul>
            <p v-else class="muted">Припущень і обмежень ще не визначено.</p>
            <RouterLink :to="{ name: 'project-risks', params: { projectId } }">Реєстр ризиків</RouterLink>
          </div>
          <div>
            <h3>Керування змінами</h3>
            <p>{{ plan.manifest.governance.change_control_required ? 'Після погодження зміни плану потребують формального контролю.' : 'Формальний контроль змін не вимагається.' }}</p>
          </div>
        </div>
      </section>

      <div class="section-box">
        <div class="section-box-header">
          <div>
            <h2 class="section-box-title">Розділи плану</h2>
            <p class="section-box-desc">Цілі, межі, відповідальність, результати та графік</p>
          </div>
          <RouterLink :to="{ name: 'project-plan-configuration', params: { projectId } }" class="btn-secondary btn-sm">
            Редагувати розділи
          </RouterLink>
        </div>
      </div>

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