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
export function buildApiUrl(path: string): string {
  const baseUrl = getApiBaseUrl()
  if (!baseUrl) return path
  const cleanBase = baseUrl.endsWith('/') ? baseUrl.slice(0, -1) : baseUrl
  const cleanPath = path.startsWith('/') ? path : '/' + path
  return cleanBase + cleanPath
}

export function getAuthAndWorkerHeaders(): Record<string, string> {
  const headers: Record<string, string> = {}
  const token = getApiToken()
  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }
  const activeWorkerUrl = localStorage.getItem('active_worker_url') || ''
  const activeWorkerToken = localStorage.getItem('active_worker_token') || ''
  if (activeWorkerUrl) {
    headers['X-Worker-Base-URL'] = activeWorkerUrl
    headers['X-Worker-Auth-Token'] = activeWorkerToken
    headers['X-Storage-Mode'] = 'proxy'
  }
  return headers
}

export async function apiRequest<T = any>(
  path: string,
  options: RequestInit = {}
): Promise<T> {
  let url = buildApiUrl(path)

  // Prevent dynamic GET API requests from being cached by browser or CDN
  const method = (options.method || 'GET').toUpperCase()
  if (method === 'GET') {
    const separator = url.includes('?') ? '&' : '?'
    url = `${url}${separator}_t=${Date.now()}`
  }

  const headers = new Headers(options.headers || {})
  const extraHeaders = getAuthAndWorkerHeaders()
  for (const [k, v] of Object.entries(extraHeaders)) {
    headers.set(k, v)
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

export interface UploadProgressInfo {
  percent: number
  loadedBytes: number
  totalBytes: number
  speedBps: number
  stage: string
  chunkIndex?: number
  totalChunks?: number
  duplicated?: boolean
}

async function computeFileSHA256(file: File): Promise<string> {
  if (typeof crypto === 'undefined' || !crypto.subtle) return ''
  // Compute client-side SHA-256 for files up to 200MB to enable 0-byte instant deduplication
  if (file.size > 200 * 1024 * 1024) return ''
  try {
    const buffer = await file.arrayBuffer()
    const hashBuffer = await crypto.subtle.digest('SHA-256', buffer)
    const hashArray = Array.from(new Uint8Array(hashBuffer))
    return hashArray.map(b => b.toString(16).padStart(2, '0')).join('')
  } catch (_) {
    return ''
  }
}

function sendXhrWithProgress<T = any>(options: {
  url: string
  method: string
  headers: Record<string, string>
  body: Document | XMLHttpRequestBodyInit | null
  onUploadProgress?: (loaded: number, total: number) => void
  signal?: AbortSignal
}): Promise<T> {
  return new Promise((resolve, reject) => {
    if (options.signal?.aborted) {
      reject(new ApiError('上传已取消', 499))
      return
    }

    const xhr = new XMLHttpRequest()
    xhr.open(options.method, options.url, true)

    for (const [k, v] of Object.entries(options.headers)) {
      xhr.setRequestHeader(k, v)
    }

    const onAbort = () => {
      xhr.abort()
      reject(new ApiError('上传已取消', 499))
    }
    options.signal?.addEventListener('abort', onAbort, { once: true })

    if (xhr.upload && options.onUploadProgress) {
      xhr.upload.onprogress = (e) => {
        if (e.lengthComputable) {
          options.onUploadProgress!(e.loaded, e.total)
        }
      }
    }

    xhr.onload = () => {
      options.signal?.removeEventListener('abort', onAbort)
      if (xhr.status >= 200 && xhr.status < 300) {
        try {
          resolve(xhr.responseText ? JSON.parse(xhr.responseText) : ({} as T))
        } catch (err) {
          reject(new ApiError('解析响应失败', xhr.status))
        }
      } else {
        let errMsg = `请求失败 (HTTP ${xhr.status})`
        try {
          const parsed = JSON.parse(xhr.responseText)
          if (parsed?.error) errMsg = parsed.error
        } catch (_) {}
        reject(new ApiError(errMsg, xhr.status))
      }
    }

    xhr.onerror = () => {
      options.signal?.removeEventListener('abort', onAbort)
      reject(new ApiError('网络传输错误，请检查网络连接', 0))
    }

    xhr.send(options.body)
  })
}

export async function uploadMediaWithProgress(options: {
  file: File
  project: string
  usage: string
  onProgress?: (info: UploadProgressInfo) => void
  signal?: AbortSignal
}): Promise<MediaAsset> {
  const { file, project, usage, onProgress, signal } = options
  const totalBytes = file.size
  const startTime = performance.now()

  const calcSpeed = (loaded: number) => {
    const elapsedSec = Math.max((performance.now() - startTime) / 1000, 0.1)
    return Math.round(loaded / elapsedSec)
  }

  let sha256 = ''
  if (totalBytes <= 200 * 1024 * 1024 && typeof crypto !== 'undefined' && crypto.subtle) {
    onProgress?.({
      percent: 1,
      loadedBytes: 0,
      totalBytes,
      speedBps: 0,
      stage: '正在计算文件指纹 (秒传预检)...',
    })
    sha256 = await computeFileSHA256(file)
  }

  interface InitUploadResponse {
    duplicated: boolean
    asset?: MediaAsset
    uploadId?: string
    chunkSize?: number
    totalChunks?: number
  }

  // Try chunked session API (supports instant 0-byte deduplication & resumable chunk retries)
  let initData: InitUploadResponse | null = null
  try {
    initData = await apiRequest<InitUploadResponse>('/api/v1/media/upload/init', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        project,
        usage,
        fileName: file.name,
        mimeType: file.type || 'application/octet-stream',
        sizeBytes: totalBytes,
        sha256,
      }),
      signal,
    })
  } catch (err: any) {
    // If client aborted or server rejected validation (400/401/413), rethrow immediately
    if (err?.status === 400 || err?.status === 401 || err?.status === 413 || signal?.aborted) {
      throw err
    }
    initData = null
  }

  if (initData?.duplicated && initData.asset) {
    onProgress?.({
      percent: 100,
      loadedBytes: totalBytes,
      totalBytes,
      speedBps: totalBytes,
      stage: '⚡ 秒传成功！已复用相同指纹资源',
      duplicated: true,
    })
    return initData.asset
  }

  if (initData?.uploadId && initData.chunkSize && initData.totalChunks) {
    const { uploadId, chunkSize, totalChunks } = initData
    const chunkLoaded = new Array<number>(totalChunks).fill(0)
    let completedChunks = 0

    const reportProgress = (activeIdx: number) => {
      const loadedBytes = chunkLoaded.reduce((acc, v) => acc + v, 0)
      const rawPercent = totalBytes > 0 ? Math.floor((loadedBytes / totalBytes) * 95) : 95
      const percent = Math.min(95, Math.max(1, rawPercent))
      onProgress?.({
        percent,
        loadedBytes,
        totalBytes,
        speedBps: calcSpeed(loadedBytes),
        stage:
          totalChunks > 1
            ? `正在并发上传分片 (${completedChunks}/${totalChunks})...`
            : '正在上传文件数据...',
        chunkIndex: activeIdx + 1,
        totalChunks,
      })
    }

    const uploadSingleChunkWithRetry = async (index: number) => {
      const start = index * chunkSize
      const end = Math.min(start + chunkSize, totalBytes)
      const blob = file.slice(start, end)
      const maxAttempts = 3

      for (let attempt = 1; attempt <= maxAttempts; attempt++) {
        try {
          await sendXhrWithProgress({
            url: buildApiUrl(`/api/v1/media/upload/${encodeURIComponent(uploadId)}/chunk?index=${index}`),
            method: 'POST',
            headers: {
              ...getAuthAndWorkerHeaders(),
              'Content-Type': 'application/octet-stream',
            },
            body: blob,
            signal,
            onUploadProgress: (loaded) => {
              chunkLoaded[index] = loaded
              reportProgress(index)
            },
          })
          chunkLoaded[index] = end - start
          completedChunks++
          reportProgress(index)
          return
        } catch (err: any) {
          if (signal?.aborted || err?.status === 499 || attempt === maxAttempts) {
            throw err
          }
          chunkLoaded[index] = 0
          await new Promise((r) => setTimeout(r, 800 * attempt))
        }
      }
    }

    // Upload chunks with concurrency = 2 to maximize throughput without saturating bandwidth
    const concurrency = Math.min(2, totalChunks)
    let nextIndex = 0
    const workers = Array.from({ length: concurrency }, async () => {
      while (nextIndex < totalChunks) {
        const current = nextIndex++
        await uploadSingleChunkWithRetry(current)
      }
    })
    await Promise.all(workers)

    onProgress?.({
      percent: 97,
      loadedBytes: totalBytes,
      totalBytes,
      speedBps: calcSpeed(totalBytes),
      stage: totalChunks > 1 ? '正在服务端合成流媒体与同步存储节点...' : '正在同步至存储节点...',
      chunkIndex: totalChunks,
      totalChunks,
    })

    const asset = await apiRequest<MediaAsset>(
      `/api/v1/media/upload/${encodeURIComponent(uploadId)}/complete`,
      {
        method: 'POST',
        signal,
      }
    )

    onProgress?.({
      percent: 100,
      loadedBytes: totalBytes,
      totalBytes,
      speedBps: calcSpeed(totalBytes),
      stage: '上传完成！',
      chunkIndex: totalChunks,
      totalChunks,
    })
    return asset
  }

  // Fallback to single multipart upload with project & usage placed before file for streaming fast-path
  const form = new FormData()
  form.append('project', project)
  form.append('usage', usage)
  form.append('file', file)

  const asset = await sendXhrWithProgress<MediaAsset>({
    url: buildApiUrl('/api/v1/media/upload'),
    method: 'POST',
    headers: getAuthAndWorkerHeaders(),
    body: form,
    signal,
    onUploadProgress: (loaded, total) => {
      const actualTotal = total || totalBytes
      const percent = actualTotal > 0 ? Math.min(96, Math.floor((loaded / actualTotal) * 96)) : 50
      onProgress?.({
        percent,
        loadedBytes: loaded,
        totalBytes: actualTotal,
        speedBps: calcSpeed(loaded),
        stage: loaded >= actualTotal ? '正在同步至存储节点...' : '正在上传文件数据...',
      })
    },
  })

  onProgress?.({
    percent: 100,
    loadedBytes: totalBytes,
    totalBytes,
    speedBps: calcSpeed(totalBytes),
    stage: '上传完成！',
  })
  return asset
}

export function buildDownloadUrl(publicUrl: string): string {
  if (!publicUrl) return ''
  const separator = publicUrl.includes('?') ? '&' : '?'
  return `${publicUrl}${separator}download=1`
}

export function extByMIME(mime: string): string {
  const map: Record<string, string> = {
    'image/jpeg': '.jpg',
    'image/png': '.png',
    'image/webp': '.webp',
    'image/gif': '.gif',
    'image/svg+xml': '.svg',
    'image/avif': '.avif',
    'image/heic': '.heic',
    'image/x-icon': '.ico',
    'image/vnd.microsoft.icon': '.ico',
    'video/mp4': '.mp4',
    'video/webm': '.webm',
    'video/quicktime': '.mov',
    'video/x-matroska': '.mkv',
    'audio/mpeg': '.mp3',
    'audio/ogg': '.ogg',
    'audio/wav': '.wav',
    'audio/aac': '.aac',
    'audio/flac': '.flac',
    'audio/mp4': '.m4a',
    'application/pdf': '.pdf',
    'application/vnd.openxmlformats-officedocument.wordprocessingml.document': '.docx',
    'application/msword': '.doc',
    'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet': '.xlsx',
    'application/vnd.ms-excel': '.xls',
    'application/vnd.openxmlformats-officedocument.presentationml.presentation': '.pptx',
    'application/vnd.ms-powerpoint': '.ppt',
    'text/csv': '.csv',
    'text/plain': '.txt',
    'application/zip': '.zip',
    'application/x-rar-compressed': '.rar',
    'application/x-7z-compressed': '.7z',
    'application/x-tar': '.tar',
    'application/gzip': '.gz',
    'application/json': '.json',
    'text/yaml': '.yaml',
  }
  return map[mime] || ''
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
