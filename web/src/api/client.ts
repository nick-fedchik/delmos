// Мінімальний типізований клієнт REST API DELMOS. Кожен мутуючий запит
// автоматично додає заголовок X-CSRF-Token (ACCESS_CONTROL.md §2); сесія
// передається браузером через HttpOnly cookie — тут її не видно й не читати.
export class ApiError extends Error {
  status: number
  code: string

  constructor(status: number, code: string, message: string) {
    super(message)
    this.status = status
    this.code = code
  }
}

interface ErrorBody {
  error?: { code?: string; message?: string }
}

function fallbackErrorMessage(status: number): string {
  switch (status) {
    case 400:
    case 422:
      return 'Запит не вдалося обробити. Перевірте введені дані та повторіть дію.'
    case 401:
      return 'Сеанс неавтентифікований або завершився. Увійдіть до системи повторно.'
    case 403:
      return 'Операцію заборонено. Перевірте права доступу або увійдіть до системи повторно.'
    case 404:
      return 'Запитуваний ресурс не знайдено. Оновіть сторінку та повторіть дію.'
    case 409:
      return 'Дані змінив інший користувач. Оновіть сторінку перед повторною спробою.'
    case 429:
      return 'Забагато спроб. Зачекайте трохи перед повторною спробою.'
    default:
      return 'Сервер DELMOS тимчасово недоступний. Повторіть спробу; якщо проблема не зникає, перевірте стан сервісу або зверніться до адміністратора системи.'
  }
}

let csrfToken = ''

export function setCsrfToken(token: string): void {
  csrfToken = token
}

export function clearCsrfToken(): void {
  csrfToken = ''
}

// Оболонка підписується на збої API, щоб показати їх у нижньому статус-барі.
// Передаються лише безпечні дані: метод, шлях, статус, код і текст для
// користувача — без тіла відповіді, заголовків і токенів.
export interface ApiFailure {
  method: string
  path: string
  status: number
  code: string
  message: string
}

let failureReporter: ((failure: ApiFailure) => void) | null = null

export function setApiFailureReporter(reporter: (failure: ApiFailure) => void): void {
  failureReporter = reporter
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = {}
  if (body !== undefined) {
    headers['Content-Type'] = 'application/json'
  }
  if (method !== 'GET' && csrfToken) {
    headers['X-CSRF-Token'] = csrfToken
  }

  let response: Response
  try {
    response = await fetch('/api/v1' + path, {
      method,
      headers,
      credentials: 'include',
      body: body !== undefined ? JSON.stringify(body) : undefined,
    })
  } catch {
    const error = new ApiError(
      0,
      'network_error',
      'Не вдалося з’єднатися із сервером DELMOS. Перевірте мережеве з’єднання або стан сервісу та повторіть спробу.',
    )
    reportFailure(method, path, error)
    throw error
  }

  if (response.status === 204) {
    return undefined as T
  }

  const newCsrf = response.headers.get('X-CSRF-Token')
  if (newCsrf) {
    setCsrfToken(newCsrf)
  }

  const text = await response.text()
  let data: unknown
  try {
    data = text ? (JSON.parse(text) as unknown) : undefined
  } catch {
    data = undefined
  }

  if (!response.ok) {
    const errorBody = data as ErrorBody
    const error = new ApiError(
      response.status,
      errorBody?.error?.code ?? 'unknown_error',
      errorBody?.error?.message ?? fallbackErrorMessage(response.status),
    )
    reportFailure(method, path, error)
    throw error
  }

  return data as T
}

function reportFailure(method: string, path: string, error: ApiError): void {
  failureReporter?.({
    method,
    path: '/api/v1' + path,
    status: error.status,
    code: error.code,
    message: error.message,
  })
}

export const api = {
  get: <T>(path: string) => request<T>('GET', path),
  post: <T>(path: string, body?: unknown) => request<T>('POST', path, body ?? {}),
  delete: <T>(path: string) => request<T>('DELETE', path),
}
