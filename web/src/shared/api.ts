export type ApiError = Error & { status?: number }

export async function request<T>(url: string, options: RequestInit = {}): Promise<T> {
  const headers = new Headers(options.headers)
  if (options.body && !(options.body instanceof FormData)) headers.set('Content-Type', 'application/json')
  const response = await fetch(url, { ...options, headers, credentials: 'same-origin' })
  const payload = response.status === 204 ? {} : await response.json().catch(() => ({}))
  if (!response.ok) {
    const error = new Error(payload.error || '请求处理失败') as ApiError
    error.status = response.status
    throw error
  }
  return payload as T
}
