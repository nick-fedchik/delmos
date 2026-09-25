import { ApiError } from '@/api/client'

export function loginErrorMessage(error: unknown): string {
  if (error instanceof ApiError && (error.status === 0 || error.status >= 500)) {
    return 'Вхід тимчасово недоступний. DELMOS не може перевірити введені облікові дані. Повторний ввід логіна чи пароля зараз не допоможе. Зверніться до системного адміністратора вашої організації через затверджений канал підтримки.'
  }

  if (error instanceof ApiError) {
    return error.message
  }

  return 'Вхід тимчасово недоступний через помилку інтерфейсу. Оновіть сторінку; якщо проблема повторюється, зверніться до системного адміністратора вашої організації через затверджений канал підтримки.'
}