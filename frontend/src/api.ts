// API client and configuration utilities for Firefly Media Gateway

export class ApiError extends Error {
  status: number
  constructor(message: string, status: number) {
    super(message)
    this.status = status
    this.name = 'ApiError'
  }
}

export interface MediaAsset {
  mediaId: string
  provider: string
  publicUrl: string
  mimeType: string
  sizeBytes: number
  sha256?: string
  project: string
  usage: string
  status: string
  createdAt: string
  updatedAt: string
  deletedAt?: string
  isChunked?: boolean
  chunkCount?: number
}

export interface HealthInfo {
  status: string
  total_files?: number
  total_size?: number
  storage_driver?: string
  is_private?: boolean
  database_driver?: string
  rules_count?: number
  worker_url?: string
  commit?: string
}

// Global configuration helpers
export function getApiBaseUrl(): string {
  return localStorage.getItem('media_gateway_url') || ''
}

export function setApiBaseUrl(url: string) {
  localStorage.setItem('media_gateway_url', url.trim())
}

export function getApiToken(): string {
  return localStorage.getItem('media_gateway_token') || ''
}

export function setApiToken(token: string) {
  localStorage.setItem('media_gateway_token', token.trim())
}

// Fetch wrapper with auth header injection
export async function apiRequest<T = any>(
  path: string,
  options: RequestInit = {}
): Promise<T> {
  const baseUrl = getApiBaseUrl()
  const token = getApiToken()

  // Clean trailing and leading slashes to form absolute URL if baseUrl is set
  let url = path
  if (baseUrl) {
    const cleanBase = baseUrl.endsWith('/') ? baseUrl.slice(0, -1) : baseUrl
    const cleanPath = path.startsWith('/') ? path : '/' + path
    url = cleanBase + cleanPath
  }

  // Prevent dynamic GET API requests from being cached by browser or CDN
  const method = (options.method || 'GET').toUpperCase()
  if (method === 'GET') {
    const separator = url.includes('?') ? '&' : '?'
    url = `${url}${separator}_t=${Date.now()}`
  }

  const headers = new Headers(options.headers || {})
  if (token) {
    headers.set('Authorization', `Bearer ${token}`)
  }

  // Inject active worker credentials for dynamic debugging if configured
  const activeWorkerUrl = localStorage.getItem('active_worker_url') || ''
  const activeWorkerToken = localStorage.getItem('active_worker_token') || ''
  if (activeWorkerUrl) {
    headers.set('X-Worker-Base-URL', activeWorkerUrl)
    headers.set('X-Worker-Auth-Token', activeWorkerToken)
    headers.set('X-Storage-Mode', 'proxy')
  }

  const resp = await fetch(url, {
    ...options,
    headers,
  })

  if (!resp.ok) {
    let errMsg = `Request failed with status ${resp.status}`
    try {
      const errData = await resp.json()
      if (errData.error) {
        errMsg = errData.error
      }
    } catch (_) {}
    throw new ApiError(errMsg, resp.status)
  }

  // Handle empty responses
  if (resp.status === 204) {
    return {} as T
  }

  return await resp.json()
}

// Robust clipboard copy with fallback for non-HTTPS and mobile web views
export async function copyToClipboard(text: string): Promise<boolean> {
  if (navigator.clipboard && window.isSecureContext) {
    try {
      await navigator.clipboard.writeText(text)
      return true
    } catch (_) {}
  }
  try {
    const textArea = document.createElement('textarea')
    textArea.value = text
    textArea.style.position = 'fixed'
    textArea.style.top = '0'
    textArea.style.left = '0'
    textArea.style.width = '2em'
    textArea.style.height = '2em'
    textArea.style.padding = '0'
    textArea.style.border = 'none'
    textArea.style.outline = 'none'
    textArea.style.boxShadow = 'none'
    textArea.style.background = 'transparent'
    document.body.appendChild(textArea)
    textArea.focus()
    textArea.select()
    const successful = document.execCommand('copy')
    document.body.removeChild(textArea)
    return successful
  } catch (_) {
    return false
  }
}

// Helper to open media file URL in new tab injecting headers if necessary
export async function openMediaAsset(publicUrl: string) {
  const activeWorkerUrl = localStorage.getItem('active_worker_url') || ''
  
  // If there are no active worker overrides, we can open the URL directly.
  // The gateway server automatically appends signature tokens for private assets.
  if (!activeWorkerUrl) {
    window.open(publicUrl, '_blank')
    return
  }

  // Pre-open window synchronously to prevent mobile browser popup blockers
  const newWin = window.open('about:blank', '_blank')
  const token = getApiToken()
  try {
    const headers: Record<string, string> = {
      'Authorization': `Bearer ${token}`
    }
    const activeWorkerToken = localStorage.getItem('active_worker_token') || ''
    headers['X-Worker-Base-URL'] = activeWorkerUrl
    headers['X-Worker-Auth-Token'] = activeWorkerToken
    headers['X-Storage-Mode'] = 'proxy'

    const resp = await fetch(publicUrl, { headers })
    if (resp.ok) {
      const blob = await resp.blob()
      const objectURL = URL.createObjectURL(blob)
      if (newWin) {
        newWin.location.href = objectURL
      } else {
        window.location.href = objectURL
      }
    } else {
      if (newWin) newWin.location.href = publicUrl
    }
  } catch (_) {
    if (newWin) newWin.location.href = publicUrl
  }
}
