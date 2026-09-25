import { describe, expect, it } from 'vitest'

import { ApiError } from '@/api/client'
import { loginErrorMessage } from '@/auth/login-error'

describe('loginErrorMessage', () => {
  it('explains that credentials were not checked when authentication is unavailable', () => {
    expect(loginErrorMessage(new ApiError(500, 'unknown_error', 'ignored'))).toBe(
      'Вхід тимчасово недоступний. DELMOS не може перевірити введені облікові дані. Повторний ввід логіна чи пароля зараз не допоможе. Зверніться до системного адміністратора вашої організації через затверджений канал підтримки.',
    )
  })

  it('does not expose development or hosting implementation details', () => {
    const message = loginErrorMessage(new ApiError(0, 'network_error', 'ignored'))

    expect(message).not.toContain('backend')
    expect(message).not.toContain('make')
    expect(message).toContain('системного адміністратора')
    expect(message).toContain('не допоможе')
  })
})