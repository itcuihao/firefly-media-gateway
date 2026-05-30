<script setup lang="ts">
import { ref, inject, computed } from 'vue'
import { getApiBaseUrl } from '../api'

const showToast = inject<(msg: string, type?: 'success' | 'error') => void>('showToast', () => {})

const selectedApi = ref('health')

// Parameter states
const sbLimit = ref('20')
const sbOffset = ref('0')
const sbMediaId = ref('')
const sbProject = ref('test-project')
const sbUsage = ref('cover')
const sbIsMember = ref('false')
const sbToken = ref('')
const sbAutoWebp = ref(true)
const fileInputRef = ref<HTMLInputElement | null>(null)
const selectedFileName = ref('')

// S3 Sandbox states & computed properties
const sandboxMode = ref('rest')
const activeS3Client = ref('aws')

const s3Endpoint = computed(() => {
  const baseUrl = getApiBaseUrl() || 'http://localhost:8088'
  const cleanBase = baseUrl.endsWith('/') ? baseUrl.slice(0, -1) : baseUrl
  return `${cleanBase}/s3`
})

const gatewayToken = computed(() => {
  return localStorage.getItem('media_gateway_token') || 'firefly'
})

const activeS3Code = computed(() => {
  const endpoint = s3Endpoint.value
  const token = gatewayToken.value || 'firefly'
  
  if (activeS3Client.value === 'aws') {
    return `# 1. 配置 AWS CLI 连接凭证
aws configure --profile firefly
# 提示时输入：
# AWS Access Key ID: ${token}
# AWS Secret Access Key: dummy
# Default region name: us-east-1
# Default output format: json

# 2. 上传文件 (PutObject)
aws --endpoint-url ${endpoint} s3 cp ./image.jpg s3://my-bucket/myproject/cover/avatar.jpg --profile firefly

# 3. 列出文件 (ListObjects)
aws --endpoint-url ${endpoint} s3 ls s3://my-bucket/ --profile firefly

# 4. 删除文件 (DeleteObject)
# 注意: 需要附带 ?asset_id=xxx 标识 (这里以 UUID 12345678... 为例)
aws --endpoint-url ${endpoint} s3 rm s3://my-bucket/myproject/cover/avatar.jpg?asset_id=12345678123456781234567812345678 --profile firefly`
  } else if (activeS3Client.value === 's3cmd') {
    const hostPort = endpoint.replace('http://', '').replace('https://', '').replace('/s3', '')
    return `# 1. 创建 s3cmd 配置文件 (~/.s3cfg)
[default]
access_key = ${token}
secret_key = dummy
host_base = ${hostPort}
host_bucket = ${hostPort}/s3/%(bucket)s
use_https = False
signature_v2 = False

# 2. 上传文件 (PutObject)
s3cmd put ./image.jpg s3://my-bucket/myproject/cover/avatar.jpg

# 3. 列出文件 (ListObjects)
s3cmd ls s3://my-bucket/

# 4. 删除文件 (DeleteObject)
# 注意: 需要附带 ?asset_id=xxx 标识 (这里以 UUID 12345678... 为例)
s3cmd del s3://my-bucket/myproject/cover/avatar.jpg?asset_id=12345678123456781234567812345678`
  } else if (activeS3Client.value === 'mc') {
    return `# 1. 配置 MinIO 客户端别名
mc alias set firefly-gateway ${s3Endpoint.value.replace('/s3', '')} ${token} dummy --api S3v4

# 2. 上传文件
mc cp ./image.jpg firefly-gateway/s3/my-bucket/myproject/cover/avatar.jpg

# 3. 列出文件
mc ls firefly-gateway/s3/my-bucket/`
  } else if (activeS3Client.value === 'rclone') {
    return `# 1. 添加 Rclone 配置 (~/.config/rclone/rclone.conf)
[firefly]
type = s3
provider = Other
access_key_id = ${token}
secret_access_key = dummy
endpoint = ${s3Endpoint.value}
force_path_style = true

# 2. 拷贝上传文件
rclone copy ./local-folder/ firefly:my-bucket/myproject/cover/

# 3. 列出桶内资源
rclone ls firefly:my-bucket/`
  } else if (activeS3Client.value === 'node') {
    return `// Node.js 使用 AWS SDK v3 访问 S3 兼容网关
const { S3Client, PutObjectCommand } = require("@aws-sdk/client-s3");
const fs = require("fs");

const s3 = new S3Client({
  endpoint: "${s3Endpoint.value}",
  region: "us-east-1",
  credentials: {
    accessKeyId: "${token}",
    secretAccessKey: "dummy"
  },
  forcePathStyle: true // 必须启用 path-style 访问
});

async function upload() {
  const fileStream = fs.createReadStream("./image.jpg");
  await s3.send(new PutObjectCommand({
    Bucket: "my-bucket",
    Key: "myproject/cover/avatar.jpg",
    Body: fileStream,
    ContentType: "image/jpeg"
  }));
  console.log("上传成功！");
}

upload();`
  }
  return ''
})

function copyS3Command() {
  navigator.clipboard.writeText(activeS3Code.value).then(() => {
    showToast('示例代码已复制到剪贴板！')
  }).catch(() => {
    showToast('复制失败，请手动选择复制', 'error')
  })
}

function convertImageToWebp(file: File, quality = 0.85): Promise<Blob> {
  return new Promise((resolve, reject) => {
    if (file.type === 'image/webp') {
      resolve(file)
      return
    }

    const reader = new FileReader()
    reader.readAsDataURL(file)
    reader.onload = (event) => {
      const img = new Image()
      img.src = event.target?.result as string
      img.onload = () => {
        const canvas = document.createElement('canvas')
        canvas.width = img.naturalWidth
        canvas.height = img.naturalHeight
        
        const ctx = canvas.getContext('2d')
        if (!ctx) {
          reject(new Error('Failed to get 2D context'))
          return
        }
        ctx.drawImage(img, 0, 0)
        
        canvas.toBlob((blob) => {
          if (blob) {
            resolve(blob)
          } else {
            reject(new Error('Canvas conversion to webp blob failed'))
          }
        }, 'image/webp', quality)
      }
      img.onerror = (err) => {
        reject(err)
      }
    }
    reader.onerror = (err) => {
      reject(err)
    }
  })
}

// Output states
const statusPillVisible = ref(false)
const statusPillText = ref('')
const statusPillStyle = ref({ background: '', color: '' })
const responseBody = ref('等待发送请求，联调数据将在此实时高亮渲染...')
const apiLoading = ref(false)

function onFileChange(e: Event) {
  const target = e.target as HTMLInputElement
  if (target.files && target.files.length > 0) {
    selectedFileName.value = target.files[0].name
  } else {
    selectedFileName.value = ''
  }
}

function copyConsoleOutput() {
  navigator.clipboard.writeText(responseBody.value).then(() => {
    showToast('已成功复制到剪贴板！')
  }).catch(() => {
    showToast('复制失败，请手动选择复制', 'error')
  })
}

function copyCurlCommand() {
  navigator.clipboard.writeText(generatedCurl.value).then(() => {
    showToast('Curl 命令已复制到剪贴板！')
  }).catch(() => {
    showToast('复制失败，请手动选择复制', 'error')
  })
}

// Compute curl dynamically based on the current form state
const generatedCurl = computed(() => {
  const baseUrl = getApiBaseUrl() || ''
  const cleanBase = baseUrl.endsWith('/') ? baseUrl.slice(0, -1) : baseUrl
  
  let path = ''
  let method = 'GET'
  let queryParams = ''
  const headers: Record<string, string> = {}
  let bodyStr = ''
  let isMultipart = false
  const multipartParts: { key: string; value: string; isFile?: boolean }[] = []

  // Auth Header
  const token = localStorage.getItem('media_gateway_token') || ''
  if (token) {
    headers['Authorization'] = `Bearer ${token}`
  }

  // Active Worker Overrides (e.g. X-Worker-Base-URL)
  const activeWorkerUrl = localStorage.getItem('active_worker_url') || ''
  const activeWorkerToken = localStorage.getItem('active_worker_token') || ''
  if (activeWorkerUrl) {
    headers['X-Worker-Base-URL'] = activeWorkerUrl
    headers['X-Worker-Auth-Token'] = activeWorkerToken
    headers['X-Storage-Mode'] = 'proxy'
  }

  if (selectedApi.value === 'health') {
    path = '/api/v1/health'
    method = 'GET'
  } else if (selectedApi.value === 'list') {
    path = '/api/v1/media'
    method = 'GET'
    const limit = sbLimit.value || '20'
    const offset = sbOffset.value || '0'
    queryParams = `?limit=${encodeURIComponent(limit)}&offset=${encodeURIComponent(offset)}`
  } else if (selectedApi.value === 'meta') {
    const mediaId = sbMediaId.value.trim() || '<mediaId>'
    path = `/api/v1/media/${encodeURIComponent(mediaId)}/meta`
    method = 'GET'
  } else if (selectedApi.value === 'delete') {
    const mediaId = sbMediaId.value.trim() || '<mediaId>'
    path = `/api/v1/media/${encodeURIComponent(mediaId)}`
    method = 'DELETE'
  } else if (selectedApi.value === 'telegram_chats') {
    path = '/api/v1/provider/telegram/chat-ids'
    method = 'POST'
    headers['Content-Type'] = 'application/json'
    bodyStr = JSON.stringify({ token: sbToken.value.trim() })
  } else if (selectedApi.value === 'upload') {
    path = '/api/v1/media/upload'
    method = 'POST'
    isMultipart = true
    let fileName = selectedFileName.value || 'file.jpg'
    const isJpgOrPng = fileName.endsWith('.jpg') || fileName.endsWith('.jpeg') || fileName.endsWith('.png')
    if (isJpgOrPng && sbAutoWebp.value) {
      const lastDot = fileName.lastIndexOf('.')
      if (lastDot !== -1) {
        fileName = fileName.substring(0, lastDot) + '.webp'
      } else {
        fileName = fileName + '.webp'
      }
    }
    multipartParts.push({ key: 'file', value: `@${fileName}`, isFile: true })
    multipartParts.push({ key: 'project', value: sbProject.value.trim() })
    multipartParts.push({ key: 'usage', value: sbUsage.value.trim() })
    multipartParts.push({ key: 'member', value: sbIsMember.value })
  }

  // Constructing the curl command string
  let curl = `curl -X ${method} "${cleanBase || 'http://localhost:8080'}${path}${queryParams}"`

  // Add headers
  for (const [key, val] of Object.entries(headers)) {
    const escapedVal = val.replace(/'/g, "'\\''")
    curl += ` \\\n  -H "${key}: ${escapedVal}"`
  }

  // Add body
  if (isMultipart) {
    for (const part of multipartParts) {
      const escapedKey = part.key.replace(/'/g, "'\\''")
      const escapedVal = part.value.replace(/'/g, "'\\''")
      curl += ` \\\n  -F "${escapedKey}=${escapedVal}"`
    }
  } else if (bodyStr) {
    const escapedBody = bodyStr.replace(/'/g, "'\\''")
    curl += ` \\\n  -d '${escapedBody}'`
  }

  return curl
})

async function runSandboxApi() {
  responseBody.value = '发送请求中，请稍后...'
  statusPillVisible.value = false
  apiLoading.value = true

  const startTime = performance.now()
  let fetchUrl = ''
  let options: RequestInit = {
    method: 'GET'
  }

  try {
    const token = localStorage.getItem('media_gateway_token') || ''
    const headers: Record<string, string> = {}
    if (token) {
      headers['Authorization'] = `Bearer ${token}`
    }

    // Active worker headers override
    const activeWorkerUrl = localStorage.getItem('active_worker_url') || ''
    const activeWorkerToken = localStorage.getItem('active_worker_token') || ''
    if (activeWorkerUrl) {
      headers['X-Worker-Base-URL'] = activeWorkerUrl
      headers['X-Worker-Auth-Token'] = activeWorkerToken
      headers['X-Storage-Mode'] = 'proxy'
    }

    if (selectedApi.value === 'health') {
      fetchUrl = '/api/v1/health'
      options = { method: 'GET', headers }
    } else if (selectedApi.value === 'list') {
      const limit = sbLimit.value || '20'
      const offset = sbOffset.value || '0'
      fetchUrl = `/api/v1/media?limit=${limit}&offset=${offset}`
      options = { method: 'GET', headers }
    } else if (selectedApi.value === 'meta') {
      const mediaId = sbMediaId.value.trim()
      if (!mediaId) {
        showToast('请先输入 mediaId 参数！', 'error')
        responseBody.value = '错误: 缺少 mediaId'
        apiLoading.value = false
        return
      }
      fetchUrl = `/api/v1/media/${encodeURIComponent(mediaId)}/meta`
      options = { method: 'GET', headers }
    } else if (selectedApi.value === 'delete') {
      const mediaId = sbMediaId.value.trim()
      if (!mediaId) {
        showToast('请先输入 mediaId 参数！', 'error')
        responseBody.value = '错误: 缺少 mediaId'
        apiLoading.value = false
        return
      }
      fetchUrl = `/api/v1/media/${encodeURIComponent(mediaId)}`
      options = { method: 'DELETE', headers }
    } else if (selectedApi.value === 'telegram_chats') {
      const tgToken = sbToken.value.trim()
      fetchUrl = '/api/v1/provider/telegram/chat-ids'
      options = {
        method: 'POST',
        headers: Object.assign({ 'Content-Type': 'application/json' }, headers),
        body: JSON.stringify({ token: tgToken })
      }
    } else if (selectedApi.value === 'upload') {
      const project = sbProject.value.trim()
      const usage = sbUsage.value.trim()
      const isMember = sbIsMember.value
      const fileInput = fileInputRef.value

      if (!fileInput || !fileInput.files || !fileInput.files[0]) {
        showToast('请在参数中选择一个文件！', 'error')
        responseBody.value = '错误: 请选择文件'
        apiLoading.value = false
        return
      }

      let fileToUpload = fileInput.files[0]
      const isJpgOrPng = fileToUpload.type === 'image/jpeg' || fileToUpload.type === 'image/png'

      if (isJpgOrPng && sbAutoWebp.value) {
        showToast('正在本地优化压缩并转换为 WebP 格式...', 'success')
        try {
          const webpBlob = await convertImageToWebp(fileToUpload)
          let newName = fileToUpload.name
          const lastDot = newName.lastIndexOf('.')
          if (lastDot !== -1) {
            newName = newName.substring(0, lastDot) + '.webp'
          } else {
            newName = newName + '.webp'
          }
          fileToUpload = new File([webpBlob], newName, { type: 'image/webp' })
        } catch (err: any) {
          console.warn('WebP conversion failed, fallback to original:', err)
        }
      }

      fetchUrl = '/api/v1/media/upload'
      const form = new FormData()
      form.append('file', fileToUpload)
      form.append('project', project)
      form.append('usage', usage)
      form.append('member', isMember)

      options = {
        method: 'POST',
        headers, // Browser sets multipart boundary automatically
        body: form
      }
    }

    // Call fetch directly to get raw response and custom timing metrics
    const baseUrl = getApiBaseUrl()
    const cleanBase = baseUrl.endsWith('/') ? baseUrl.slice(0, -1) : baseUrl
    const cleanPath = fetchUrl.startsWith('/') ? fetchUrl : '/' + fetchUrl
    const targetUrl = cleanBase + cleanPath

    const response = await fetch(targetUrl, options)
    const duration = (performance.now() - startTime).toFixed(0)

    // Update status badge
    statusPillVisible.value = true
    statusPillText.value = `HTTP ${response.status} • ${duration}ms`
    
    if (response.status >= 200 && response.status < 300) {
      statusPillStyle.value = {
        background: 'rgba(133, 247, 176, 0.15)',
        color: 'hsl(var(--md-sys-color-success))'
      }
      showToast('API 请求执行成功！')
    } else {
      statusPillStyle.value = {
        background: 'rgba(255, 180, 171, 0.15)',
        color: '#ffb4ab'
      }
      showToast(`请求失败: HTTP ${response.status}`, 'error')
    }

    if (response.status === 204) {
      responseBody.value = '// 204 No Content (操作执行成功，无返回值)'
      return
    }

    const contentType = response.headers.get('Content-Type')
    if (contentType && contentType.indexOf('application/json') !== -1) {
      const jsonVal = await response.json()
      responseBody.value = JSON.stringify(jsonVal, null, 2)
    } else {
      responseBody.value = await response.text()
    }
  } catch (err: any) {
    const duration = (performance.now() - startTime).toFixed(0)
    statusPillVisible.value = true
    statusPillText.value = `ERROR • ${duration}ms`
    statusPillStyle.value = {
      background: 'rgba(255, 180, 171, 0.15)',
      color: '#ffb4ab'
    }
    responseBody.value = `请求捕获异常: ${err.message}\n检查控制台报错或服务器网络配置。`
    showToast('请求链接错误', 'error')
  } finally {
    apiLoading.value = false
  }
}
</script>

<template>
  <div class="panel-view active" id="panel_sandbox">
    <!-- Sandbox Mode Tabs Toggle -->
    <div style="display: flex; gap: 8px; margin-bottom: 20px; border-bottom: 1px solid rgba(255,255,255,0.06); padding-bottom: 12px;">
      <button :class="['m3-btn', sandboxMode === 'rest' ? 'm3-btn-primary' : 'm3-btn-secondary']" @click="sandboxMode = 'rest'">
        <span class="material-symbols-rounded" style="font-size: 18px;">api</span>
        <span>REST API 接口联调</span>
      </button>
      <button :class="['m3-btn', sandboxMode === 's3' ? 'm3-btn-primary' : 'm3-btn-secondary']" @click="sandboxMode = 's3'">
        <span class="material-symbols-rounded" style="font-size: 18px;">cloud</span>
        <span>S3 兼容协议命令参考</span>
      </button>
    </div>

    <div v-if="sandboxMode === 'rest'" class="api-sandbox-layout">
      <!-- Left panel: Form Controls -->
      <div class="m3-card api-params-col">
        <h2 class="section-title">
          <span class="material-symbols-rounded" style="color: hsl(var(--md-sys-color-primary));">network_check</span>
          API 测试参数选择
        </h2>
        <p style="font-size: 13px; color: hsl(var(--md-sys-color-on-surface-variant)); margin-bottom: 20px;">选择一个接口端点，填充参数进行实时响应联调。</p>
        
        <div class="form-field">
          <label>选择调试 API</label>
          <div class="input-wrapper">
            <select id="sandboxApiSelector" v-model="selectedApi">
              <option value="health">GET /api/v1/health (健康检查)</option>
              <option value="list">GET /api/v1/media (文件列表)</option>
              <option value="meta">GET /api/v1/media/{mediaId}/meta (媒体元数据)</option>
              <option value="upload">POST /api/v1/media/upload (上传媒体文件)</option>
              <option value="delete">DELETE /api/v1/media/{mediaId} (删除媒体文件)</option>
              <option value="telegram_chats">POST /api/v1/provider/telegram/chat-ids (获取TG群组)</option>
            </select>
          </div>
        </div>

        <!-- Dynamic parameters builder container -->
        <div id="sandboxParamsContainer" style="margin-top: 20px;">
          <!-- GET /api/v1/media list parameters -->
          <div v-if="selectedApi === 'list'">
            <div class="form-field">
              <label>每页条数 (limit)</label>
              <div class="input-wrapper">
                <input v-model="sbLimit" type="number" placeholder="默认 20" />
              </div>
            </div>
            <div class="form-field">
              <label>偏移起始 (offset)</label>
              <div class="input-wrapper">
                <input v-model="sbOffset" type="number" placeholder="默认 0" />
              </div>
            </div>
          </div>

          <!-- GET /meta or DELETE assets parameters -->
          <div v-else-if="selectedApi === 'meta' || selectedApi === 'delete'">
            <div class="form-field">
              <label>媒体 ID (mediaId)</label>
              <div class="input-wrapper">
                <input v-model="sbMediaId" type="text" placeholder="请输入已存在的 mediaId" />
              </div>
            </div>
          </div>

          <!-- POST /media/upload parameters -->
          <div v-else-if="selectedApi === 'upload'">
            <div class="form-field">
              <label>所属项目 (project)</label>
              <div class="input-wrapper">
                <input v-model="sbProject" type="text" placeholder="例如 default-proj" />
              </div>
            </div>
            <div class="form-field">
              <label>使用用途 (usage)</label>
              <div class="input-wrapper">
                <input v-model="sbUsage" type="text" placeholder="例如 cover / avatar" />
              </div>
            </div>
            <div class="form-field">
              <label>会员专享 (member)</label>
              <div class="input-wrapper">
                <select v-model="sbIsMember">
                  <option value="false">否 (false)</option>
                  <option value="true">是 (true)</option>
                </select>
              </div>
            </div>
            <div class="form-field">
              <label style="display: flex; align-items: center; gap: 8px; cursor: pointer; user-select: none; color: hsl(var(--md-sys-color-on-surface-variant));">
                <input type="checkbox" v-model="sbAutoWebp" style="accent-color: hsl(var(--md-sys-color-primary));" />
                <span>自动优化图片并转换为 WebP 格式</span>
              </label>
            </div>
            <div class="form-field">
              <label>文件 (file)</label>
              <div class="input-wrapper">
                <input ref="fileInputRef" type="file" accept="image/*,video/*" @change="onFileChange" />
              </div>
            </div>
          </div>

          <!-- POST /provider/telegram/chat-ids parameter -->
          <div v-else-if="selectedApi === 'telegram_chats'">
            <div class="form-field">
              <label>Telegram Bot Token</label>
              <div class="input-wrapper">
                <input v-model="sbToken" type="password" placeholder="若留空则使用默认配置" />
              </div>
            </div>
          </div>

          <!-- No parameters needed -->
          <p v-else style="font-size: 13px; color:hsl(var(--md-sys-color-on-surface-variant));">此接口无需配置额外参数</p>
        </div>

        <div style="margin-top: 28px;">
          <button class="m3-btn m3-btn-primary" style="width: 100%;" @click="runSandboxApi" :disabled="apiLoading">
            <span class="material-symbols-rounded">play_arrow</span>
            <span>{{ apiLoading ? '请求中...' : '发起 API 请求' }}</span>
          </button>
        </div>
      </div>

      <!-- Right panel: Code Output console -->
      <div class="api-response-col">
        <!-- Request Curl Block -->
        <div class="console-header" style="margin-bottom: 8px;">
          <h2 class="section-title" style="margin-bottom: 0;">
            <span class="material-symbols-rounded" style="color: hsl(var(--md-sys-color-primary));">code</span>
            请求 Curl 命令
          </h2>
          <button class="m3-btn m3-btn-secondary m3-btn-sm" @click="copyCurlCommand">
            <span class="material-symbols-rounded" style="font-size: 16px;">content_copy</span>
            <span>复制 Curl</span>
          </button>
        </div>
        <div class="console-output" style="min-height: auto; max-height: 180px; margin-bottom: 24px; color: #a5d6ff; background: #070b0e; overflow-y: auto;">{{ generatedCurl }}</div>

        <!-- Response Console -->
        <div class="console-header">
          <h2 class="section-title" style="margin-bottom: 0;">
            <span class="material-symbols-rounded" style="color: hsl(var(--md-sys-color-secondary));">terminal</span>
            调试响应控制台
          </h2>
          
          <div style="display: flex; gap: 10px; align-items: center;">
            <div class="status-pill" id="sandboxStatusPill" v-if="statusPillVisible" :style="statusPillStyle">
              {{ statusPillText }}
            </div>
            <button class="m3-btn m3-btn-secondary m3-btn-sm" @click="copyConsoleOutput">
              <span class="material-symbols-rounded" style="font-size: 16px;">content_copy</span>
              <span>复制响应</span>
            </button>
          </div>
        </div>

        <div class="console-output" id="sandboxOutput">{{ responseBody }}</div>
      </div>
    </div>

    <!-- S3 Reference Panel Layout -->
    <div v-else class="s3-reference-layout">
      <div class="m3-card" style="margin-bottom: 24px; padding: 24px; background: rgba(255,255,255,0.015); border: 1px solid rgba(255,255,255,0.04);">
        <h2 class="section-title" style="margin-bottom: 12px; color: hsl(var(--md-sys-color-primary)); display: flex; align-items: center; gap: 8px;">
          <span class="material-symbols-rounded">cloud_sync</span>
          S3 兼容存储网关参考
        </h2>
        <p style="font-size: 13.5px; color: hsl(var(--md-sys-color-on-surface-variant)); line-height: 1.6; margin-bottom: 16px;">
          本网关支持标准的 Amazon S3 协议访问，底层的存储与上传逻辑自动映射到 Telegram 存储系统中。您可以使用任何兼容 S3 的客户端（如 AWS CLI、MinIO Client、Rclone 等）或主流编程语言 SDK 进行集成。
        </p>
        
        <!-- Configuration Info Table -->
        <div class="s3-config-grid">
          <div class="s3-config-card">
            <div class="s3-config-label">服务终结点 (Endpoint)</div>
            <div class="s3-config-value font-mono">{{ s3Endpoint }}</div>
          </div>
          <div class="s3-config-card">
            <div class="s3-config-label">访问密钥 (Access Key)</div>
            <div class="s3-config-value font-mono">{{ gatewayToken || 'firefly' }}</div>
          </div>
          <div class="s3-config-card">
            <div class="s3-config-label">安全密钥 (Secret Key)</div>
            <div class="s3-config-value font-mono">dummy (任意非空字符串)</div>
          </div>
          <div class="s3-config-card">
            <div class="s3-config-label">存储路径映射格式</div>
            <div class="s3-config-value font-mono">s3://{bucket}/{project}/{usage}/{filename}</div>
          </div>
        </div>
        
        <div class="m3-alert" style="background: rgba(33,150,243,0.06); border: 1px dashed rgba(33,150,243,0.2); border-radius: 8px; padding: 12px 16px; margin-top: 20px; display: flex; gap: 10px; align-items: flex-start;">
          <span class="material-symbols-rounded" style="color: #64b5f6; font-size: 20px; flex-shrink: 0; margin-top: 2px;">info</span>
          <div style="font-size: 13px; color: hsl(var(--md-sys-color-on-surface-variant)); line-height: 1.5;">
            <strong>💡 目录与用途要求：</strong> S3 键名 (Key) 必须包含至少两层前缀文件夹。首层为<strong>项目名称 (Project)</strong>，第二层为<strong>文件用途 (Usage)</strong>，目前用途只支持 <code>cover</code> (封面) 或 <code>scene</code> (正片/场景)。例如：<code>my-project/cover/banner.webp</code>。
          </div>
        </div>
      </div>

      <!-- Client Tabs -->
      <div class="m3-card" style="padding: 24px; background: rgba(255,255,255,0.015); border: 1px solid rgba(255,255,255,0.04);">
        <div class="s3-tabs-header">
          <div class="s3-tabs">
            <button :class="['s3-tab-btn', { active: activeS3Client === 'aws' }]" @click="activeS3Client = 'aws'">
              AWS CLI
            </button>
            <button :class="['s3-tab-btn', { active: activeS3Client === 's3cmd' }]" @click="activeS3Client = 's3cmd'">
              s3cmd
            </button>
            <button :class="['s3-tab-btn', { active: activeS3Client === 'mc' }]" @click="activeS3Client = 'mc'">
              MinIO Client (mc)
            </button>
            <button :class="['s3-tab-btn', { active: activeS3Client === 'rclone' }]" @click="activeS3Client = 'rclone'">
              Rclone
            </button>
            <button :class="['s3-tab-btn', { active: activeS3Client === 'node' }]" @click="activeS3Client = 'node'">
              Node.js (AWS SDK)
            </button>
          </div>
          <button class="m3-btn m3-btn-secondary m3-btn-sm" @click="copyS3Command">
            <span class="material-symbols-rounded" style="font-size: 16px;">content_copy</span>
            <span>复制示例代码</span>
          </button>
        </div>

        <div class="console-output" style="margin-top: 16px; font-family: monospace; color: #a5d6ff; background: #070b0e; min-height: 240px; max-height: 380px; overflow-y: auto; white-space: pre-wrap; font-size: 13px; line-height: 1.6; border: 1px solid rgba(255,255,255,0.05); padding: 16px; border-radius: 8px;">{{ activeS3Code }}</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.s3-config-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 12px;
  margin-top: 16px;
}
.s3-config-card {
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.05);
  border-radius: 8px;
  padding: 12px 16px;
}
.s3-config-label {
  font-size: 11px;
  color: hsl(var(--md-sys-color-on-surface-variant));
  text-transform: uppercase;
  letter-spacing: 0.5px;
  margin-bottom: 4px;
}
.s3-config-value {
  font-size: 13px;
  color: #fff;
  word-break: break-all;
}
.font-mono {
  font-family: monospace;
}
.s3-tabs-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  padding-bottom: 10px;
}
.s3-tabs {
  display: flex;
  gap: 6px;
}
.s3-tab-btn {
  background: transparent;
  border: none;
  color: hsl(var(--md-sys-color-on-surface-variant));
  font-size: 13px;
  padding: 6px 12px;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s ease;
}
.s3-tab-btn:hover {
  background: rgba(255, 255, 255, 0.05);
  color: #fff;
}
.s3-tab-btn.active {
  background: rgba(255, 255, 255, 0.08);
  color: hsl(var(--md-sys-color-primary));
  font-weight: 600;
}
</style>
