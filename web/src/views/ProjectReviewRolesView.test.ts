import { createPinia, setActivePinia } from 'pinia'
import { createApp, nextTick } from 'vue'
import { createMemoryHistory, createRouter } from 'vue-router'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/api/client'
import { useSessionStore } from '@/stores/session'
import ProjectReviewRolesView from '@/views/ProjectReviewRolesView.vue'

async function mountView(permissions: string[]) {
  const container = document.createElement('div')
  document.body.append(container)
  const app = createApp(ProjectReviewRolesView)
  app.use(createPinia())
  const router = createRouter({
    history: createMemoryHistory(),
    routes: [{ path: '/', name: 'projects', component: { template: '<div />' } }],
  })
  app.use(router)
  const session = useSessionStore()
  session.$patch({ user: { id: 'admin', login: 'admin', display_name: 'Адміністратор', permissions }, checked: true })
  app.mount(container)
  await nextTick()
  return { container, unmount: () => { app.unmount(); container.remove() } }
}

describe('керування ролями погодження', () => {
  beforeEach(() => setActivePinia(createPinia()))
  afterEach(() => vi.restoreAllMocks())

  it('надає і відкликає роль за обраним користувачем та проєктом', async () => {
    const binding = { id: 'b1', user_id: 'u1', login: 'reviewer', display_name: 'Рецензент', role_key: 'project.reviewer' }
    const get = vi.spyOn(api, 'get')
      .mockResolvedValueOnce([{ id: 'p1', code: 'PLAN-1', name: 'Планування' }])
      .mockResolvedValueOnce([])
      .mockResolvedValueOnce([{ id: 'u1', login: 'reviewer', display_name: 'Рецензент' }])
      .mockResolvedValueOnce([binding])
      .mockResolvedValueOnce([])
    const post = vi.spyOn(api, 'post').mockResolvedValue({ id: 'b1' })
    const remove = vi.spyOn(api, 'delete').mockResolvedValue(undefined)
    const { container, unmount } = await mountView(['access.grant', 'access.revoke'])
    try {
      await vi.waitFor(() => expect(container.querySelector('#review-role-project option[value="p1"]')).not.toBeNull())
      const project = container.querySelector('#review-role-project') as HTMLSelectElement
      project.value = 'p1'
      project.dispatchEvent(new Event('change', { bubbles: true }))
      await vi.waitFor(() => expect(get).toHaveBeenCalledWith('/projects/p1/review-role-bindings'))

      const query = container.querySelector('#review-role-user-query') as HTMLInputElement
      query.value = 'reviewer'
      query.dispatchEvent(new Event('input', { bubbles: true }))
      query.closest('form')!.dispatchEvent(new Event('submit', { bubbles: true, cancelable: true }))
      await vi.waitFor(() => expect(container.querySelector('#review-role-user option[value="u1"]')).not.toBeNull())
      const user = container.querySelector('#review-role-user') as HTMLSelectElement
      user.value = 'u1'
      user.dispatchEvent(new Event('change', { bubbles: true }))
      const reason = container.querySelector('#review-role-reason') as HTMLTextAreaElement
      reason.value = 'Незалежна рецензія'
      reason.dispatchEvent(new Event('input', { bubbles: true }))
      await nextTick()
      Array.from(container.querySelectorAll('button')).find((button) => button.textContent === 'Надати роль')!.click()
      await vi.waitFor(() => expect(post).toHaveBeenCalledWith('/projects/p1/review-role-bindings', {
        user_id: 'u1', role_key: 'project.reviewer', reason: 'Незалежна рецензія',
      }))
      await vi.waitFor(() => expect(container.textContent).toContain('Роль надано'))
      Array.from(container.querySelectorAll('button')).find((button) => button.textContent === 'Відкликати')!.click()
      await nextTick()
      Array.from(container.querySelectorAll('button')).find((button) => button.textContent === 'Підтвердити')!.click()
      await vi.waitFor(() => expect(remove).toHaveBeenCalledWith('/projects/p1/review-role-bindings/b1'))
    } finally {
      unmount()
    }
  })

  it('не викликає API без системного дозволу', async () => {
    const get = vi.spyOn(api, 'get')
    const { container, unmount } = await mountView([])
    try {
      expect(container.textContent).toContain('потрібен дозвіл адміністратора')
      expect(container.querySelector('#review-role-project')).toBeNull()
      expect(get).not.toHaveBeenCalled()
    } finally {
      unmount()
    }
  })
})