import { createPinia, setActivePinia } from 'pinia'
import { createApp, nextTick } from 'vue'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/api/client'
import type { PlanDetailView } from '@/api/types'
import ProjectPlanConfigurationView from '@/views/ProjectPlanConfigurationView.vue'
import ProjectPlanView from '@/views/ProjectPlanView.vue'

const initialPlan: PlanDetailView = {
  work_product_id: 'wp1', code: 'PLAN-001', title: 'План', status: 'draft',
  row_version: 1, revision_id: 'rev1', revision_number: 1, payload_hash: 'hash',
  body: 'Початковий опис', template_key: 'generic', template_version: 1,
  permissions: ['wp.edit'],
  manifest: {
    objectives: [], scope_items: [], assumptions: [], constraints: [],
    responsibility_assignments: [], deliverables: [], phases: [], milestones: [],
    acceptance_rules: [], governance: { change_control_required: true }, extensions: { custom: 1 },
  },
}

async function mountView(component: typeof ProjectPlanView | typeof ProjectPlanConfigurationView) {
  const container = document.createElement('div')
  document.body.append(container)
  const app = createApp(component, { projectId: 'project1' })
  app.use(createPinia())
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/', name: 'projects', component: { template: '<div />' } },
      { path: '/projects/:projectId', name: 'project', component: { template: '<div />' } },
      { path: '/projects/:projectId/plan', name: 'project-plan', component: { template: '<div />' } },
      { path: '/projects/:projectId/plan/configuration', name: 'project-plan-configuration', component: { template: '<div />' } },
      { path: '/projects/:projectId/stakeholders', name: 'project-stakeholders', component: { template: '<div />' } },
      { path: '/projects/:projectId/risks', name: 'project-risks', component: { template: '<div />' } },
    ],
  })
  app.use(router)
  app.mount(container)
  await vi.waitFor(() => expect(container.querySelector('h1')).not.toBeNull())
  return { container, unmount: () => { app.unmount(); container.remove() } }
}

describe('редактори плану', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.spyOn(api, 'get').mockResolvedValue(structuredClone(initialPlan))
  })

  afterEach(() => vi.restoreAllMocks())

  it('показує цілі, критерії та межі в огляді плану', async () => {
    vi.spyOn(api, 'get').mockResolvedValue({
      ...initialPlan,
      manifest: {
        ...initialPlan.manifest,
        objectives: [{ key: 'OBJ-001', statement: 'Підготувати систему', success_criteria: ['Пройдено приймання'] }],
        scope_items: [
          { key: 'SCOPE-001', kind: 'in_scope', statement: 'Прошивка', rationale: '' },
          { key: 'SCOPE-002', kind: 'out_of_scope', statement: 'Серійне виробництво', rationale: '' },
        ],
      },
    })
    const { container, unmount } = await mountView(ProjectPlanView)
    try {
      expect(container.textContent).toContain('Підготувати систему')
      expect(container.textContent).toContain('Пройдено приймання')
      expect(container.textContent).toContain('Прошивка')
      expect(container.textContent).toContain('Серійне виробництво')
    } finally {
      unmount()
    }
  })

  it('не пропонує вводити в дію непогоджену чернетку', async () => {
    vi.spyOn(api, 'get').mockResolvedValue({ ...initialPlan, permissions: ['wp.edit', 'plan.apply'] })
    const { container, unmount } = await mountView(ProjectPlanView)
    try {
      expect(container.textContent).toContain('незалежно погодити')
      expect([...container.querySelectorAll('button')].some((button) => button.textContent?.includes('Ввести в дію'))).toBe(false)
    } finally {
      unmount()
    }
  })

  it('показує, чого бракує чернетці перед погодженням', async () => {
    const { container, unmount } = await mountView(ProjectPlanView)
    try {
      const items = container.querySelectorAll('.plan-readiness-list li')
      expect(items).toHaveLength(6)
      expect([...items].every((item) => item.textContent?.includes('Доповнити'))).toBe(true)
      expect(container.textContent).toContain('Заповнити розділи')
    } finally {
      unmount()
    }
  })

  it('подає ревізію з незалежними призначеними учасниками', async () => {
    vi.spyOn(api, 'get')
      .mockResolvedValueOnce({ ...initialPlan, permissions: ['wp.submit'] })
      .mockResolvedValueOnce([
        { id: 'reviewer-id', login: 'reviewer', display_name: 'Рецензент', can_review: true, can_approve: false },
        { id: 'approver-id', login: 'approver', display_name: 'Погоджувач', can_review: false, can_approve: true },
      ])
      .mockResolvedValueOnce({ ...initialPlan, status: 'in_review', permissions: ['wp.submit'] })
    const post = vi.spyOn(api, 'post').mockResolvedValue({ id: 'request-id' })
    const { container, unmount } = await mountView(ProjectPlanView)
    try {
      await vi.waitFor(() => expect(container.querySelector('#plan-reviewer option[value="reviewer-id"]')).not.toBeNull())
      const reviewer = container.querySelector('#plan-reviewer') as HTMLSelectElement
      const approver = container.querySelector('#plan-approver') as HTMLSelectElement
      reviewer.value = 'reviewer-id'
      reviewer.dispatchEvent(new Event('change', { bubbles: true }))
      approver.value = 'approver-id'
      approver.dispatchEvent(new Event('change', { bubbles: true }))
      await nextTick()
      ;(container.querySelector('button[type="submit"]') as HTMLButtonElement).click()

      await vi.waitFor(() => expect(post).toHaveBeenCalledWith('/projects/project1/work-products/wp1/submit', {
        assignments: [
          { user_id: 'reviewer-id', assignment_role: 'reviewer' },
          { user_id: 'approver-id', assignment_role: 'approver' },
        ],
      }))
      await vi.waitFor(() => expect(container.textContent).toContain('План подано на незалежне погодження'))
      expect(container.querySelector('#plan-reviewer')).toBeNull()
    } finally {
      unmount()
    }
  })

  it('відкриває погодження тільки після позитивної рецензії призначеної особи', async () => {
    const inReview = { ...initialPlan, status: 'in_review', permissions: ['wp.review', 'wp.approve'] }
    const assigned = (completed: boolean) => ({
      revision_id: 'rev1', has_positive_review: completed,
      participants: [
        { display_name: 'Незалежний фахівець', role: 'reviewer', completed, assigned_to_me: true },
        { display_name: 'Незалежний фахівець', role: 'approver', completed: false, assigned_to_me: true },
      ],
    })
    vi.spyOn(api, 'get')
      .mockResolvedValueOnce(inReview).mockResolvedValueOnce(assigned(false))
      .mockResolvedValueOnce(inReview).mockResolvedValueOnce(assigned(true))
      .mockResolvedValueOnce({ ...inReview, status: 'approved' })
    const post = vi.spyOn(api, 'post').mockResolvedValue({ id: 'decision-id' })
    const { container, unmount } = await mountView(ProjectPlanView)
    try {
      await vi.waitFor(() => expect(container.textContent).toContain('Підтвердити рецензію'))
      expect(container.textContent).not.toContain('Погодити план')
      Array.from(container.querySelectorAll('button')).find((button) => button.textContent === 'Підтвердити рецензію')!.click()
      await vi.waitFor(() => expect(container.textContent).toContain('Погодити план'))
      expect(post).toHaveBeenCalledWith('/projects/project1/work-products/wp1/reviews', {})
      Array.from(container.querySelectorAll('button')).find((button) => button.textContent === 'Погодити план')!.click()
      await vi.waitFor(() => expect(container.textContent).toContain('План погоджено'))
      expect(post).toHaveBeenCalledWith('/projects/project1/work-products/wp1/approvals', {})
      expect(container.querySelector('button[type="button"]')?.textContent).not.toContain('Підтвердити рецензію')
    } finally {
      unmount()
    }
  })

  it('показує підписи лише поточної погодженої ревізії', async () => {
    vi.spyOn(api, 'get')
      .mockResolvedValueOnce({ ...initialPlan, status: 'approved' })
      .mockResolvedValueOnce([
        { revision_id: 'rev1', decision_kind: 'review', display_name: 'Рецензент', decided_at: '2026-09-28T09:00:00Z' },
        { revision_id: 'rev1', decision_kind: 'approval', display_name: 'Погоджувач', decided_at: '2026-09-28T10:00:00Z' },
        { revision_id: 'old-revision', decision_kind: 'review', display_name: 'Старий підпис', decided_at: '2026-09-27T09:00:00Z' },
      ])
    const { container, unmount } = await mountView(ProjectPlanView)
    try {
      await vi.waitFor(() => expect(container.textContent).toContain('Погоджувач'))
      expect(container.textContent).toContain('Підтвердження ревізії 1')
      expect(container.textContent).toContain('Рецензент')
      expect(container.textContent).not.toContain('Старий підпис')
    } finally {
      unmount()
    }
  })

  it('не показує дії погодження непризначеному користувачу', async () => {
    vi.spyOn(api, 'get')
      .mockResolvedValueOnce({ ...initialPlan, status: 'in_review', permissions: ['wp.review', 'wp.approve'] })
      .mockResolvedValueOnce({
        revision_id: 'rev1', has_positive_review: true,
        participants: [
          { display_name: 'Рецензент', role: 'reviewer', completed: true, assigned_to_me: false },
          { display_name: 'Погоджувач', role: 'approver', completed: false, assigned_to_me: false },
        ],
      })
    const { container, unmount } = await mountView(ProjectPlanView)
    try {
      await vi.waitFor(() => expect(container.textContent).toContain('Погоджувач · Погодження'))
      expect(container.textContent).not.toContain('Погодити план')
      expect(container.textContent).not.toContain('Потрібні зміни')
    } finally {
      unmount()
    }
  })

  it('не дозволяє ухвалити рішення із незбереженими нотатками', async () => {
    vi.spyOn(api, 'get')
      .mockResolvedValueOnce({ ...initialPlan, status: 'in_review', permissions: ['wp.edit', 'wp.review'] })
      .mockResolvedValueOnce({
        revision_id: 'rev1', has_positive_review: false,
        participants: [{ display_name: 'Рецензент', role: 'reviewer', completed: false, assigned_to_me: true }],
      })
    const post = vi.spyOn(api, 'post')
    const { container, unmount } = await mountView(ProjectPlanView)
    try {
      await vi.waitFor(() => expect(container.textContent).toContain('Підтвердити рецензію'))
      const editor = container.querySelector('.markdown-editor') as HTMLTextAreaElement
      editor.value = 'Змінені нотатки'
      editor.dispatchEvent(new Event('input', { bubbles: true }))
      await nextTick()
      const decision = Array.from(container.querySelectorAll('button')).find((button) => button.textContent === 'Підтвердити рецензію') as HTMLButtonElement
      expect(decision.disabled).toBe(true)
      expect(container.textContent).toContain('Спершу збережіть зміни нотаток')
      expect(post).not.toHaveBeenCalled()
    } finally {
      unmount()
    }
  })

  it('повертає план на доопрацювання лише з причиною', async () => {
    vi.spyOn(api, 'get')
      .mockResolvedValueOnce({ ...initialPlan, status: 'in_review', permissions: ['wp.review', 'wp.request_changes'] })
      .mockResolvedValueOnce({
        revision_id: 'rev1', has_positive_review: false,
        participants: [{ display_name: 'Рецензент', role: 'reviewer', completed: false, assigned_to_me: true }],
      })
      .mockResolvedValueOnce({ ...initialPlan, status: 'draft', permissions: ['wp.review'] })
    const post = vi.spyOn(api, 'post').mockResolvedValue({ id: 'decision-id' })
    const { container, unmount } = await mountView(ProjectPlanView)
    try {
      await vi.waitFor(() => expect(container.textContent).toContain('Потрібні зміни'))
      Array.from(container.querySelectorAll('button')).find((button) => button.textContent === 'Потрібні зміни')!.click()
      await nextTick()
      const submit = Array.from(container.querySelectorAll('button')).find((button) => button.textContent === 'Повернути на доопрацювання') as HTMLButtonElement
      expect(submit.disabled).toBe(true)
      const reason = container.querySelector('#plan-change-reason') as HTMLTextAreaElement
      reason.value = 'Уточнити критерії приймання'
      reason.dispatchEvent(new Event('input', { bubbles: true }))
      await nextTick()
      submit.click()
      await vi.waitFor(() => expect(post).toHaveBeenCalledWith('/projects/project1/work-products/wp1/request-changes', {
        reason: 'Уточнити критерії приймання',
      }))
      await vi.waitFor(() => expect(container.textContent).toContain('План повернуто на доопрацювання'))
    } finally {
      unmount()
    }
  })

  it('показує графік чернетки окремо від діючої проєкції', async () => {
    vi.spyOn(api, 'get').mockResolvedValue({
      ...initialPlan,
      phases: [],
      milestones: [],
      manifest: {
        ...initialPlan.manifest,
        deliverables: [{ key: 'DEL-001', name: 'Специфікація', required_status: 'approved' }],
        phases: [{ key: 'PH-001', name: 'Розробка', planned_start: '2026-10-01', planned_finish: '2026-11-01' }],
        milestones: [{ key: 'MS-001', name: 'Приймання', target_date: '2026-11-01' }],
      },
    })
    const { container, unmount } = await mountView(ProjectPlanView)
    try {
      expect(container.textContent).toContain('Специфікація')
      expect(container.textContent).toContain('Розробка')
      expect(container.textContent).toContain('Приймання')
      expect(container.textContent).not.toContain('Проєкція фаз інженерного життєвого циклу')
    } finally {
      unmount()
    }
  })

  it('показує відповідальність і правила змін без дублювання реєстрів', async () => {
    vi.spyOn(api, 'get').mockResolvedValue({
      ...initialPlan,
      manifest: {
        ...initialPlan.manifest,
        responsibility_assignments: [{ key: 'RESP-001', role_key: 'PM', subject: 'Олена', responsibility: 'Приймання' }],
        assumptions: [{ key: 'ASM-001', statement: 'Доступність обладнання', owner_reference: '', validation_date: '', status: '' }],
        constraints: [{ key: 'CON-001', kind: 'date', statement: 'До листопада', source_ref: '', enforcement: '' }],
      },
    })
    const { container, unmount } = await mountView(ProjectPlanView)
    try {
      expect(container.textContent).toContain('Олена · Приймання')
      expect(container.textContent).toContain('Доступність обладнання')
      expect(container.textContent).toContain('До листопада')
      expect(container.textContent).toContain('формального контролю')
      expect(container.textContent).toContain('Реєстр ризиків')
    } finally {
      unmount()
    }
  })

  it('зберігає нотатки з незміненим маніфестом', async () => {
    const post = vi.spyOn(api, 'post').mockResolvedValue({ ...initialPlan, body: 'Новий опис', row_version: 2 })
    const { container, unmount } = await mountView(ProjectPlanView)
    try {
      const notes = container.querySelector('textarea')!
      notes.value = 'Новий опис'
      notes.dispatchEvent(new Event('input', { bubbles: true }))
      await nextTick()
      ;(container.querySelector('button[type="submit"]') as HTMLButtonElement).click()
      await vi.waitFor(() => expect(post).toHaveBeenCalled())
      expect(post).toHaveBeenCalledWith('/projects/project1/plan/revisions', {
        expected_row_version: 1, body: 'Новий опис', manifest: initialPlan.manifest,
      })
    } finally {
      unmount()
    }
  })

  it('зберігає маніфест із незміненими нотатками та розширеннями', async () => {
    const post = vi.spyOn(api, 'post').mockResolvedValue({ ...initialPlan, row_version: 2 })
    const { container, unmount } = await mountView(ProjectPlanConfigurationView)
    try {
      const addObjective = [...container.querySelectorAll('button')].find((button) => button.textContent?.includes('Додати ціль'))!
      addObjective.click()
      await nextTick()
      const objective = container.querySelector('#objectives-OBJ-001-statement') as HTMLTextAreaElement
      objective.value = 'Мета проєкту'
      objective.dispatchEvent(new Event('input', { bubbles: true }))
      await nextTick()
      ;(container.querySelector('button[type="submit"]') as HTMLButtonElement).click()
      await vi.waitFor(() => expect(post).toHaveBeenCalled())
      expect(post).toHaveBeenCalledWith('/projects/project1/plan/revisions', {
        expected_row_version: 1,
        body: 'Початковий опис',
        manifest: { ...initialPlan.manifest, objectives: [{ key: 'OBJ-001', statement: 'Мета проєкту', success_criteria: [] }] },
      })
    } finally {
      unmount()
    }
  })

  it('пов’язує віху з фазою через форму і не видаляє пов’язану фазу', async () => {
    vi.spyOn(api, 'get').mockResolvedValueOnce(structuredClone(initialPlan)).mockResolvedValueOnce([])
    const post = vi.spyOn(api, 'post').mockResolvedValue({ ...initialPlan, row_version: 2 })
    const { container, unmount } = await mountView(ProjectPlanConfigurationView)
    const enter = (selector: string, value: string, event = 'input') => {
      const field = container.querySelector(selector) as HTMLInputElement
      expect(field).not.toBeNull()
      field.value = value
      field.dispatchEvent(new Event(event, { bubbles: true }))
    }
    try {
      Array.from(container.querySelectorAll('button')).find((button) => button.textContent?.includes('Додати фазу'))!.click()
      await nextTick()
      enter('#phases-PH-001-name', 'Проєктування')
      enter('#phases-PH-001-planned_start', '2026-10-01')
      enter('#phases-PH-001-planned_finish', '2026-11-30')

      Array.from(container.querySelectorAll('button')).find((button) => button.textContent?.includes('Додати віху'))!.click()
      await nextTick()
      enter('#milestones-MS-001-name', 'Приймання проєкту')
      enter('#milestones-MS-001-phase_key', 'PH-001', 'change')
      enter('#milestones-MS-001-target_date', '2026-11-30')
      await nextTick()

      ;(container.querySelector('[aria-label="Видалити фазу 1"]') as HTMLButtonElement).click()
      await nextTick()
      expect(container.textContent).toContain('Спочатку приберіть зв’язки з цією фазою')
      expect(container.querySelector('#phases-PH-001-name')).not.toBeNull()

      ;(container.querySelector('button[type="submit"]') as HTMLButtonElement).click()
      await vi.waitFor(() => expect(post).toHaveBeenCalled())
      expect(post).toHaveBeenCalledWith('/projects/project1/plan/revisions', expect.objectContaining({
        body: 'Початковий опис',
        manifest: expect.objectContaining({
          phases: [{ key: 'PH-001', name: 'Проєктування', planned_start: '2026-10-01', planned_finish: '2026-11-30', depends_on: [] }],
          milestones: [{ key: 'MS-001', name: 'Приймання проєкту', phase_key: 'PH-001', target_date: '2026-11-30', deliverable_keys: [], acceptance_rule_keys: [] }],
        }),
      }))
    } finally {
      unmount()
    }
  })

  it('відкриває старий план із порожніми nullable-секціями', async () => {
    vi.spyOn(api, 'get').mockResolvedValueOnce({
      ...initialPlan,
      manifest: { ...initialPlan.manifest, phases: null, milestones: null, acceptance_rules: null },
    }).mockResolvedValueOnce([])
    const { container, unmount } = await mountView(ProjectPlanConfigurationView)
    try {
      expect(container.querySelector('h1')?.textContent).toBe('Розділи плану')
      expect(container.textContent).toContain('Правил ще немає')
      expect(container.querySelector('[role="alert"]')).toBeNull()
    } finally {
      unmount()
    }
  })
})