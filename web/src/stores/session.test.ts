import { setActivePinia, createPinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useSessionStore } from '@/stores/session'

describe('useSessionStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.restoreAllMocks()
  })

  it('вважає користувача неавтентифікованим до входу', () => {
    const session = useSessionStore()
    expect(session.isAuthenticated).toBe(false)
    expect(session.hasPermission('scopes.manage')).toBe(false)
  })

  it('зберігає дозволи користувача після успішного входу', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        new Response(
          JSON.stringify({
            csrf_token: 'token-123',
            expires_at: '2026-09-24T00:00:00Z',
            user: { id: 'u1', login: 'admin', display_name: 'Admin', permissions: ['scopes.manage'] },
          }),
          { status: 200 },
        ),
      ),
    )

    const session = useSessionStore()
    await session.login('admin', 'pass')

    expect(session.isAuthenticated).toBe(true)
    expect(session.hasPermission('scopes.manage')).toBe(true)
    expect(session.hasPermission('access.grant')).toBe(false)
  })

  it('скидає стан користувача після виходу навіть якщо запит на logout не вдався', async () => {
    const session = useSessionStore()
    session.$patch({ user: { id: 'u1', login: 'admin', display_name: 'Admin', permissions: [] }, checked: true })

    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(null, { status: 500 })))

    await expect(session.logout()).rejects.toThrow()
    expect(session.isAuthenticated).toBe(false)
  })

  it('вважає сесію відсутньою, якщо /auth/session повертає помилку', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(null, { status: 401 })))

    const session = useSessionStore()
    await session.restore()

    expect(session.isAuthenticated).toBe(false)
    expect(session.checked).toBe(true)
  })
})
