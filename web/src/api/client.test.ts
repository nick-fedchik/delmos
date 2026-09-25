import { beforeEach, describe, expect, it, vi } from 'vitest'

import { ApiError, api, clearCsrfToken } from '@/api/client'

describe('api client', () => {
  beforeEach(() => {
    clearCsrfToken()
    vi.restoreAllMocks()
  })

  it('explains an unstructured server failure without exposing a raw HTTP status', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('upstream unavailable', { status: 500 })))

    await expect(api.post('/auth/login', { login: 'admin', password: 'admin' })).rejects.toMatchObject({
      status: 500,
      code: 'unknown_error',
      message:
        'Сервер DELMOS тимчасово недоступний. Повторіть спробу; якщо проблема не зникає, перевірте стан сервісу або зверніться до адміністратора системи.',
    } satisfies Partial<ApiError>)
  })

  it('explains a network failure with the required next action', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('Failed to fetch')))

    await expect(api.get('/auth/session')).rejects.toMatchObject({
      status: 0,
      code: 'network_error',
      message:
        'Не вдалося з’єднатися із сервером DELMOS. Перевірте мережеве з’єднання або стан сервісу та повторіть спробу.',
    } satisfies Partial<ApiError>)
  })
})