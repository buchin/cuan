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

export async function apiFetch<T>(
  endpoint: string,
  options: RequestInit = {}
): Promise<T> {
  const token = getAuthToken()
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
    ...(options.headers as Record<string, string>),
  }

  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }

  const url = endpoint.startsWith('http') ? endpoint : endpoint
  const response = await fetch(url, {
    ...options,
    headers,
  })

  if (response.status === 401) {
    // If unauthorized, clear invalid token if not checking login
    if (!endpoint.includes('/auth/login')) {
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

  return response.json() as Promise<T>
}
