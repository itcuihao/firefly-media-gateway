// API client and configuration utilities for Firefly Media Gateway

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
    throw new Error(errMsg)
  }

  // Handle empty responses
  if (resp.status === 204) {
    return {} as T
  }

  return await resp.json()
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
      window.open(objectURL, '_blank')
    } else {
      window.open(publicUrl, '_blank')
    }
  } catch (_) {
    window.open(publicUrl, '_blank')
  }
}
