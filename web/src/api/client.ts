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

let csrfToken = ''

export function setCsrfToken(token: string): void {
  csrfToken = token
}

export function clearCsrfToken(): void {
  csrfToken = ''
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = {}
  if (body !== undefined) {
    headers['Content-Type'] = 'application/json'
  }
  if (method !== 'GET' && csrfToken) {
    headers['X-CSRF-Token'] = csrfToken
  }

  const response = await fetch('/api/v1' + path, {
    method,
    headers,
    credentials: 'include',
    body: body !== undefined ? JSON.stringify(body) : undefined,
  })

  if (response.status === 204) {
    return undefined as T
  }

  const text = await response.text()
  const data = text ? (JSON.parse(text) as unknown) : undefined

  if (!response.ok) {
    const errorBody = data as ErrorBody
    throw new ApiError(
      response.status,
      errorBody?.error?.code ?? 'unknown_error',
      errorBody?.error?.message ?? `HTTP ${response.status}`,
    )
  }

  return data as T
}

export const api = {
  get: <T>(path: string) => request<T>('GET', path),
  post: <T>(path: string, body?: unknown) => request<T>('POST', path, body ?? {}),
  delete: <T>(path: string) => request<T>('DELETE', path),
}
