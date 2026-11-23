// Unified Fetch Client with automatic Auth Bearer Injection

const TOKEN_KEY = 'cuan_token'

export function getAuthToken(): string {
  if (typeof window !== 'undefined') {
    return localStorage.getItem(TOKEN_KEY) || ''
  }
  return ''
}

export function setAuthToken(token: string) {
  if (typeof window !== 'undefined') {
    if (token) {
      localStorage.setItem(TOKEN_KEY, token)
    } else {
      localStorage.removeItem(TOKEN_KEY)
    }
  }
}

export class ApiError extends Error {
  status: number
  data?: any

  constructor(message: string, status: number, data?: any) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.data = data
  }
}

const pendingReads = new Map<string, Promise<unknown>>()

export async function apiFetch<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
  const method = (options.method || 'GET').toUpperCase()
  if (method !== 'GET' || options.signal || options.body || options.cache === 'no-store') return performApiFetch<T>(endpoint, options)
  const key = getAuthToken() + '\0' + endpoint + '\0' + JSON.stringify(options)
  const previous = pendingReads.get(key)
  if (previous) return previous as Promise<T>
  const request = performApiFetch<T>(endpoint, options).finally(() => pendingReads.delete(key))
  pendingReads.set(key, request)
  return request
}

async function performApiFetch<T>(
  endpoint: string,
  options: RequestInit = {}
): Promise<T> {
  const token = getAuthToken()
  const langPref = typeof window !== 'undefined' ? localStorage.getItem('cuan-language') : ''
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(langPref ? { 'Accept-Language': langPref } : {}),
    ...(options.headers as Record<string, string>),
  }

  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }

  const url = endpoint.startsWith('http') ? endpoint : endpoint
  const readController = !options.signal && (options.method || 'GET').toUpperCase() === 'GET' ? new AbortController() : null
  const deadline = readController ? setTimeout(() => readController.abort(), 20000) : null
  try {
    const response = await fetch(url, {
      ...options,
      headers,
      signal: options.signal || readController?.signal,
    })

    if (response.status === 401) {
      // If unauthorized, clear invalid token if not checking login
      if (!endpoint.includes('/auth/login') && getAuthToken() === token) {
        setAuthToken('')
      }
    }

    if (!response.ok) {
      let errorMsg = `HTTP Error ${response.status}: ${response.statusText}`
      let errData: any = null
      try {
        errData = await response.json()
        if (errData.error) errorMsg = errData.error
        else if (errData.message) errorMsg = errData.message
      } catch {
        // not JSON
      }
      throw new ApiError(errorMsg, response.status, errData)
    }

    // Handle empty 204 No Content
    if (response.status === 204) {
      return {} as T
    }

    return await response.json() as T
  } finally { if (deadline) clearTimeout(deadline) }
}
