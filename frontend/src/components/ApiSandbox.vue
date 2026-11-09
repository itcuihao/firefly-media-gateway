<script setup lang="ts">
import { ref, inject, computed, onMounted, watch } from 'vue'
import { getApiBaseUrl, copyToClipboard, apiRequest } from '../api'
import FireflySelect from './FireflySelect.vue'

const showToast = inject<(msg: string, type?: 'success' | 'error') => void>('showToast', () => {})

const selectedApi = ref('health')

const apiOptions = [
  { label: 'GET /api/v1/health (健康检查与系统统计)', value: 'health', icon: 'favorite' },
  { label: 'GET /api/v1/media (文件列表分页与检索)', value: 'list', icon: 'list' },
  { label: 'GET /api/v1/media/projects (已收录项目分类)', value: 'projects', icon: 'folder' },
  { label: 'GET /api/v1/media/usages (已收录用途标签)', value: 'usages', icon: 'label' },
  { label: 'GET /api/v1/media/{mediaId}/meta (媒体元数据详情)', value: 'meta', icon: 'info' },
  { label: 'POST /api/v1/media/upload (上传媒体文件)', value: 'upload', icon: 'cloud_upload' },
  { label: 'DELETE /api/v1/media/{mediaId} (删除媒体文件)', value: 'delete', icon: 'delete' },
  { label: 'POST /api/v1/provider/worker/verify (验证 Worker 节点)', value: 'worker_verify', icon: 'cloud_sync' },
  { label: 'POST /api/v1/provider/telegram/chat-ids (获取 TG 群组)', value: 'telegram_chats', icon: 'forum' },
  { label: 'GET /s3 (S3 ListAllMyBuckets 列举桶)', value: 's3_buckets', icon: 'inventory_2' },
  { label: 'GET /s3/{bucket} (S3 ListObjects 列举对象)', value: 's3_list', icon: 'dataset' }
]

// Parameter states
const sbLimit = ref('20')
const sbOffset = ref('0')
const sbListProject = ref('')
const sbListUsage = ref('')
const sbListSearch = ref('')
const sbMediaId = ref('')
const sbProject = ref('test-project')
const sbUsage = ref('cover')
const sbToken = ref('')
const sbWorkerUrl = ref('')
const sbWorkerToken = ref('')
const sbBucket = ref('default')
const sbPrefix = ref('')
const sbAutoWebp = ref(true)
const fileInputRef = ref<HTMLInputElement | null>(null)
const selectedFileName = ref('')

// Recent assets for quick autofill
const recentAssets = ref<Array<{ mediaId: string; project: string; usage: string; mimeType: string }>>([])
const loadingRecent = ref(false)

async function loadRecentAssets() {
  if (recentAssets.value.length > 0) return
  loadingRecent.value = true
  try {
    const data = await apiRequest<{ assets?: any[]; list?: any[] }>('/api/v1/media?limit=6')
    const items = data.assets || (Array.isArray(data) ? data : [])
    recentAssets.value = items.slice(0, 6).map((a: any) => ({
      mediaId: a.mediaId || a.id || '',
      project: a.project || 'default',
      usage: a.usage || 'default',
      mimeType: a.mimeType || ''
    }))
  } catch (_) {
    // Ignore error if not authorized yet
  } finally {
    loadingRecent.value = false
  }
}

watch(selectedApi, (newVal) => {
  if (newVal === 'meta' || newVal === 'delete') {
    loadRecentAssets()
  }
})

onMounted(() => {
  const activeWorkerUrl = localStorage.getItem('active_worker_url') || ''
  const activeWorkerToken = localStorage.getItem('active_worker_token') || ''
  if (activeWorkerUrl) sbWorkerUrl.value = activeWorkerUrl
  if (activeWorkerToken) sbWorkerToken.value = activeWorkerToken
})

// S3 Sandbox states & computed properties
const sandboxMode = ref<'rest' | 's3'>('rest')
const activeS3Client = ref('aws')

const s3Endpoint = computed(() => {
  const baseUrl = getApiBaseUrl() || (typeof window !== 'undefined' ? window.location.origin : 'https://firefly.haocit.com')
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
# 交互提示时输入：
# AWS Access Key ID: ${token}
# AWS Secret Access Key: dummy
# Default region name: us-east-1
# Default output format: json

# 2. 列举所有存储桶 (ListBuckets)
aws --endpoint-url ${endpoint} s3 ls --profile firefly

# 3. 上传文件 (PutObject: 支持 1段、2段、3段前缀)
# 格式规范: s3://{bucket}/{project}/{usage}/{filename}
aws --endpoint-url ${endpoint} s3 cp ./image.jpg s3://default/myproject/cover/avatar.jpg --profile firefly

# 4. 列出桶内文件 (ListObjects)
aws --endpoint-url ${endpoint} s3 ls s3://default/ --profile firefly

# 5. 下载/获取文件 (GetObject)
aws --endpoint-url ${endpoint} s3 cp s3://default/myproject/cover/avatar.jpg ./downloaded.jpg --profile firefly

# 6. 删除文件 (DeleteObject: 直接支持按 S3 Key 原生删除，无需 hack 参数)
aws --endpoint-url ${endpoint} s3 rm s3://default/myproject/cover/avatar.jpg --profile firefly

# 7. 目录同步 (Sync)
aws --endpoint-url ${endpoint} s3 sync ./dist/ s3://default/webapp/file/ --profile firefly`
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
s3cmd put ./image.jpg s3://default/myproject/cover/avatar.jpg

# 3. 列出文件 (ListObjects)
s3cmd ls s3://default/

# 4. 删除文件 (DeleteObject: 原生路径直接删除)
s3cmd del s3://default/myproject/cover/avatar.jpg`
  } else if (activeS3Client.value === 'mc') {
    return `# 1. 配置 MinIO 客户端别名
mc alias set firefly ${s3Endpoint.value} ${token} dummy --api S3v4

# 2. 查看存储桶
mc ls firefly/

# 3. 上传文件
mc cp ./image.jpg firefly/default/myproject/cover/avatar.jpg

# 4. 列出桶内文件
mc ls firefly/default/

# 5. 删除文件
mc rm firefly/default/myproject/cover/avatar.jpg`
  } else if (activeS3Client.value === 'rclone') {
    return `# 1. 添加 Rclone 配置 (~/.config/rclone/rclone.conf)
[firefly]
type = s3
provider = Other
access_key_id = ${token}
secret_access_key = dummy
endpoint = ${s3Endpoint.value}
force_path_style = true

# 2. 列出存储桶
rclone lsd firefly:

# 3. 拷贝上传文件
rclone copy ./local-folder/ firefly:default/myproject/cover/

# 4. 列出桶内资源
rclone ls firefly:default/

# 5. 删除文件
rclone delete firefly:default/myproject/cover/avatar.jpg`
  } else if (activeS3Client.value === 'node') {
    return `// Node.js 使用 AWS SDK v3 访问 Firefly S3 兼容网关
const { 
  S3Client, 
  PutObjectCommand, 
  ListObjectsV2Command,
  DeleteObjectCommand 
} = require("@aws-sdk/client-s3");
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

async function main() {
  // 1. 上传文件
  const stream = fs.createReadStream("./image.jpg");
  await s3.send(new PutObjectCommand({
    Bucket: "default",
    Key: "myproject/cover/avatar.jpg",
    Body: stream,
    ContentType: "image/jpeg"
  }));
  console.log("上传成功！");

  // 2. 列举文件
  const list = await s3.send(new ListObjectsV2Command({
    Bucket: "default",
    MaxKeys: 10
  }));
  console.log("文件列表:", list.Contents?.map(c => c.Key));

  // 3. 删除文件
  await s3.send(new DeleteObjectCommand({
    Bucket: "default",
    Key: "myproject/cover/avatar.jpg"
  }));
  console.log("删除完成！");
}

main().catch(console.error);`
  }
  return ''
})

async function copyS3Command() {
  const ok = await copyToClipboard(activeS3Code.value)
  if (ok) {
    showToast('示例代码已复制到剪贴板！')
  } else {
    showToast('复制失败，请手动选择复制', 'error')
  }
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

async function copyConsoleOutput() {
  const ok = await copyToClipboard(responseBody.value)
  if (ok) {
    showToast('已成功复制到剪贴板！')
  } else {
    showToast('复制失败，请手动选择复制', 'error')
  }
}

async function copyCurlCommand() {
  const ok = await copyToClipboard(generatedCurl.value)
  if (ok) {
    showToast('Curl 命令已复制到剪贴板！')
  } else {
    showToast('复制失败，请手动选择复制', 'error')
  }
}

// Compute curl dynamically based on the current form state
const generatedCurl = computed(() => {
  const baseUrl = getApiBaseUrl() || (typeof window !== 'undefined' ? window.location.origin : '')
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

  // Active Worker Overrides
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
    const params = new URLSearchParams()
    if (sbLimit.value) params.set('limit', sbLimit.value)
    if (sbOffset.value) params.set('offset', sbOffset.value)
    if (sbListProject.value.trim()) params.set('project', sbListProject.value.trim())
    if (sbListUsage.value.trim()) params.set('usage', sbListUsage.value.trim())
    if (sbListSearch.value.trim()) params.set('search', sbListSearch.value.trim())
    const q = params.toString()
    queryParams = q ? `?${q}` : ''
  } else if (selectedApi.value === 'projects') {
    path = '/api/v1/media/projects'
    method = 'GET'
  } else if (selectedApi.value === 'usages') {
    path = '/api/v1/media/usages'
    method = 'GET'
  } else if (selectedApi.value === 'meta') {
    const mediaId = sbMediaId.value.trim() || '<mediaId>'
    path = `/api/v1/media/${encodeURIComponent(mediaId)}/meta`
    method = 'GET'
  } else if (selectedApi.value === 'delete') {
    const mediaId = sbMediaId.value.trim() || '<mediaId>'
    path = `/api/v1/media/${encodeURIComponent(mediaId)}`
    method = 'DELETE'
  } else if (selectedApi.value === 'worker_verify') {
    path = '/api/v1/provider/worker/verify'
    method = 'POST'
    headers['Content-Type'] = 'application/json'
    bodyStr = JSON.stringify({
      worker_url: sbWorkerUrl.value.trim(),
      worker_token: sbWorkerToken.value.trim()
    })
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
    multipartParts.push({ key: 'project', value: sbProject.value.trim() })
    multipartParts.push({ key: 'usage', value: sbUsage.value.trim() })
    multipartParts.push({ key: 'file', value: `@${fileName}`, isFile: true })
  } else if (selectedApi.value === 's3_buckets') {
    path = '/s3'
    method = 'GET'
  } else if (selectedApi.value === 's3_list') {
    const bucket = sbBucket.value.trim() || 'default'
    path = `/s3/${encodeURIComponent(bucket)}`
    method = 'GET'
    if (sbPrefix.value.trim()) {
      queryParams = `?prefix=${encodeURIComponent(sbPrefix.value.trim())}`
    }
  }

  // Constructing the curl command string
  let curl = `curl -X ${method} "${cleanBase}${path}${queryParams}"`

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
      const params = new URLSearchParams()
      if (sbLimit.value) params.set('limit', sbLimit.value)
      if (sbOffset.value) params.set('offset', sbOffset.value)
      if (sbListProject.value.trim()) params.set('project', sbListProject.value.trim())
      if (sbListUsage.value.trim()) params.set('usage', sbListUsage.value.trim())
      if (sbListSearch.value.trim()) params.set('search', sbListSearch.value.trim())
      fetchUrl = `/api/v1/media?${params.toString()}`
      options = { method: 'GET', headers }
    } else if (selectedApi.value === 'projects') {
      fetchUrl = '/api/v1/media/projects'
      options = { method: 'GET', headers }
    } else if (selectedApi.value === 'usages') {
      fetchUrl = '/api/v1/media/usages'
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
    } else if (selectedApi.value === 'worker_verify') {
      fetchUrl = '/api/v1/provider/worker/verify'
      options = {
        method: 'POST',
        headers: Object.assign({ 'Content-Type': 'application/json' }, headers),
        body: JSON.stringify({
          worker_url: sbWorkerUrl.value.trim(),
          worker_token: sbWorkerToken.value.trim()
        })
      }
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
      form.append('project', project)
      form.append('usage', usage)
      form.append('file', fileToUpload)

      options = {
        method: 'POST',
        headers, // Browser sets multipart boundary automatically
        body: form
      }
    } else if (selectedApi.value === 's3_buckets') {
      fetchUrl = '/s3'
      options = { method: 'GET', headers }
    } else if (selectedApi.value === 's3_list') {
      const bucket = sbBucket.value.trim() || 'default'
      fetchUrl = `/s3/${encodeURIComponent(bucket)}`
      if (sbPrefix.value.trim()) {
        fetchUrl += `?prefix=${encodeURIComponent(sbPrefix.value.trim())}`
      }
      options = { method: 'GET', headers }
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
        background: 'rgba(0, 229, 255, 0.15)',
        color: '#00e5ff'
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

// Quick trigger S3 direct test from the S3 reference tab
async function runS3Test(target: 'buckets' | 'objects') {
  if (target === 'buckets') {
    selectedApi.value = 's3_buckets'
  } else {
    selectedApi.value = 's3_list'
    sbBucket.value = 'default'
  }
  sandboxMode.value = 'rest'
  await runSandboxApi()
}
</script>

<template>
  <div class="panel-view active" id="panel_sandbox">
    <!-- Sandbox Mode Tabs Toggle -->
    <div class="mode-tabs-bar">
      <button :class="['m3-btn', sandboxMode === 'rest' ? 'm3-btn-primary' : 'm3-btn-secondary']" @click="sandboxMode = 'rest'">
        <span class="material-symbols-rounded" style="font-size: 18px;">api</span>
        <span>REST API 接口联调</span>
      </button>
      <button :class="['m3-btn', sandboxMode === 's3' ? 'm3-btn-primary' : 'm3-btn-secondary']" @click="sandboxMode = 's3'">
        <span class="material-symbols-rounded" style="font-size: 18px;">cloud</span>
        <span>S3 兼容协议与客户端接入</span>
      </button>
    </div>

    <!-- REST API Sandbox View -->
    <div v-if="sandboxMode === 'rest'" class="api-sandbox-layout">
      <!-- Left panel: Form Controls -->
      <div class="m3-card api-params-col">
        <h2 class="section-title">
          <span class="material-symbols-rounded" style="color: #00e5ff;">network_check</span>
          API 测试参数选择
        </h2>
        <p style="font-size: 13px; color: rgba(255,255,255,0.6); margin-bottom: 20px;">
          选择 API 端点，系统将自动生成对应的 Curl 命令并支持在线真实发起调用。
        </p>
        
        <div class="form-field">
          <label>选择调试 API</label>
          <FireflySelect v-model="selectedApi" :options="apiOptions" min-width="100%" />
        </div>

        <!-- Dynamic parameters builder container -->
        <div id="sandboxParamsContainer" style="margin-top: 20px;">
          <!-- GET /api/v1/media list parameters -->
          <div v-if="selectedApi === 'list'">
            <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 12px;">
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
            <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 12px;">
              <div class="form-field">
                <label>过滤项目 (project)</label>
                <div class="input-wrapper">
                  <input v-model="sbListProject" type="text" placeholder="可选，如 sheep" />
                </div>
              </div>
              <div class="form-field">
                <label>过滤用途 (usage)</label>
                <div class="input-wrapper">
                  <input v-model="sbListUsage" type="text" placeholder="可选，如 cover" />
                </div>
              </div>
            </div>
            <div class="form-field">
              <label>关键词搜索 (search)</label>
              <div class="input-wrapper">
                <input v-model="sbListSearch" type="text" placeholder="ID 或标签前缀" />
              </div>
            </div>
          </div>

          <!-- GET /meta or DELETE assets parameters -->
          <div v-else-if="selectedApi === 'meta' || selectedApi === 'delete'">
            <div class="form-field">
              <label>媒体 ID (mediaId)</label>
              <div class="input-wrapper">
                <input v-model="sbMediaId" type="text" placeholder="32 位十六进制媒体 UUID" />
              </div>
            </div>

            <!-- Quick autofill chips -->
            <div style="margin-top: 12px;">
              <div style="font-size: 11px; color: rgba(255,255,255,0.5); margin-bottom: 6px; display: flex; align-items: center; justify-content: space-between;">
                <span>从最近收录媒体快速填入 ID:</span>
                <span v-if="loadingRecent" style="color: #00e5ff; font-size: 11px;">加载中...</span>
              </div>
              <div class="quick-id-chips">
                <button
                  v-for="item in recentAssets"
                  :key="item.mediaId"
                  type="button"
                  class="id-chip-btn"
                  :class="{ active: sbMediaId === item.mediaId }"
                  @click="sbMediaId = item.mediaId"
                  :title="`${item.project}/${item.usage} (${item.mimeType})`"
                >
                  <span class="chip-dot"></span>
                  <span class="chip-id">{{ item.mediaId.slice(0, 8) }}...</span>
                  <span class="chip-tag">{{ item.project }}</span>
                </button>
                <span v-if="!loadingRecent && recentAssets.length === 0" style="font-size: 12px; color: rgba(255,255,255,0.4);">
                  暂无活跃媒体或尚未登录
                </span>
              </div>
            </div>
          </div>

          <!-- POST /media/upload parameters -->
          <div v-else-if="selectedApi === 'upload'">
            <div class="form-field">
              <label>所属项目 (project)</label>
              <div class="input-wrapper">
                <input v-model="sbProject" type="text" placeholder="例如 sheep 或 my-project" />
              </div>
            </div>
            <div class="form-field">
              <label>使用用途 (usage: cover/scene/avatar/audio/file)</label>
              <div class="input-wrapper">
                <input v-model="sbUsage" type="text" placeholder="例如 cover / avatar / scene" />
              </div>
            </div>
            <div class="form-field">
              <label style="display: flex; align-items: center; gap: 8px; cursor: pointer; user-select: none; color: rgba(255,255,255,0.8);">
                <input type="checkbox" v-model="sbAutoWebp" style="accent-color: #00e5ff;" />
                <span>自动优化图片并转换为 WebP 格式</span>
              </label>
            </div>
            <div class="form-field">
              <label>选择媒体文件 (file)</label>
              <div class="input-wrapper">
                <input ref="fileInputRef" type="file" accept="image/*,video/*,audio/*,.pdf,.zip" @change="onFileChange" />
              </div>
            </div>
          </div>

          <!-- POST /provider/worker/verify parameter -->
          <div v-else-if="selectedApi === 'worker_verify'">
            <div class="form-field">
              <label>Worker 节点地址 (worker_url)</label>
              <div class="input-wrapper">
                <input v-model="sbWorkerUrl" type="text" placeholder="https://xxx.workers.dev" />
              </div>
            </div>
            <div class="form-field">
              <label>Worker 访问密钥 (worker_token)</label>
              <div class="input-wrapper">
                <input v-model="sbWorkerToken" type="password" placeholder="CF Worker 环境变量配置的 Token" />
              </div>
            </div>
          </div>

          <!-- POST /provider/telegram/chat-ids parameter -->
          <div v-else-if="selectedApi === 'telegram_chats'">
            <div class="form-field">
              <label>Telegram Bot Token</label>
              <div class="input-wrapper">
                <input v-model="sbToken" type="password" placeholder="留空则使用网关环境变量默认配置" />
              </div>
            </div>
          </div>

          <!-- GET /s3/list parameter -->
          <div v-else-if="selectedApi === 's3_list'">
            <div class="form-field">
              <label>存储桶名称 (bucket)</label>
              <div class="input-wrapper">
                <input v-model="sbBucket" type="text" placeholder="例如 default 或 media-assets" />
              </div>
            </div>
            <div class="form-field">
              <label>对象前缀 (prefix)</label>
              <div class="input-wrapper">
                <input v-model="sbPrefix" type="text" placeholder="例如 sheep/ 或 sheep/cover/" />
              </div>
            </div>
          </div>

          <!-- No parameters needed -->
          <p v-else style="font-size: 13px; color: rgba(255,255,255,0.5);">此接口无需配置额外参数，点击下方按钮直接发起请求。</p>
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
            <span class="material-symbols-rounded" style="color: #00e5ff;">code</span>
            请求 Curl 命令
          </h2>
          <button class="m3-btn m3-btn-secondary m3-btn-sm" @click="copyCurlCommand">
            <span class="material-symbols-rounded" style="font-size: 16px;">content_copy</span>
            <span>复制 Curl</span>
          </button>
        </div>
        <div class="console-output font-mono" style="min-height: auto; max-height: 180px; margin-bottom: 24px; color: #a5d6ff; background: #06090d; overflow-y: auto; border: 1px solid rgba(255,255,255,0.06); padding: 14px; border-radius: 8px;">{{ generatedCurl }}</div>

        <!-- Response Console -->
        <div class="console-header">
          <h2 class="section-title" style="margin-bottom: 0;">
            <span class="material-symbols-rounded" style="color: #fbbf24;">terminal</span>
            联调响应控制台
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

        <div class="console-output font-mono" id="sandboxOutput" style="background: #06090d; border: 1px solid rgba(255,255,255,0.06); border-radius: 8px; padding: 16px; min-height: 280px; max-height: 480px; overflow-y: auto; color: #7ee787;">{{ responseBody }}</div>
      </div>
    </div>

    <!-- S3 Reference Panel Layout -->
    <div v-else class="s3-reference-layout">
      <!-- Overview & Live Test Card -->
      <div class="m3-card" style="margin-bottom: 24px; padding: 24px; background: rgba(255,255,255,0.02); border: 1px solid rgba(255,255,255,0.06);">
        <div style="display: flex; justify-content: space-between; align-items: flex-start; flex-wrap: wrap; gap: 16px;">
          <div>
            <h2 class="section-title" style="margin-bottom: 8px; color: #00e5ff; display: flex; align-items: center; gap: 8px;">
              <span class="material-symbols-rounded">cloud_sync</span>
              S3 兼容存储网关参考
            </h2>
            <p style="font-size: 13.5px; color: rgba(255,255,255,0.65); line-height: 1.6; max-width: 820px;">
              Firefly Media Gateway 原生支持标准 Amazon S3 协议访问。底层媒体数据自动对接分布式存储（Telegram / Cloudflare R2）。支持标准的 AWS CLI、MinIO Client (mc)、Rclone、S3cmd 以及任意主流 S3 SDK。
            </p>
          </div>
          
          <!-- Live S3 Test Action Buttons -->
          <div style="display: flex; gap: 10px; flex-wrap: wrap;">
            <button class="m3-btn m3-btn-secondary m3-btn-sm" @click="runS3Test('buckets')">
              <span class="material-symbols-rounded" style="font-size: 16px; color: #00e5ff;">play_circle</span>
              <span>测试 ListBuckets (GET /s3)</span>
            </button>
            <button class="m3-btn m3-btn-secondary m3-btn-sm" @click="runS3Test('objects')">
              <span class="material-symbols-rounded" style="font-size: 16px; color: #fbbf24;">play_circle</span>
              <span>测试 ListObjects (GET /s3/default)</span>
            </button>
          </div>
        </div>
        
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
            <div class="s3-config-label">对象键名结构 (Key Format)</div>
            <div class="s3-config-value font-mono">{project}/{usage}/{filename}</div>
          </div>
        </div>
        
        <!-- Highlight info notice -->
        <div class="m3-alert" style="background: rgba(0, 229, 255, 0.05); border: 1px solid rgba(0, 229, 255, 0.2); border-radius: 8px; padding: 14px 18px; margin-top: 20px; display: flex; gap: 12px; align-items: flex-start;">
          <span class="material-symbols-rounded" style="color: #00e5ff; font-size: 20px; flex-shrink: 0; margin-top: 1px;">verified</span>
          <div style="font-size: 13px; color: rgba(255,255,255,0.75); line-height: 1.6;">
            <strong>✨ 协议支持与路径自动解析：</strong><br />
            • <strong>标准 S3 原生删除与下载：</strong> 支持直接使用 S3 Key (如 <code>s3://default/sheep/cover/avatar.jpg</code>) 执行 <code>aws s3 rm</code> 与 <code>aws s3 cp</code>，无需附加额外参数。<br />
            • <strong>灵活的 Key 结构：</strong> 支持 3 段式 (<code>{project}/{usage}/{filename}</code>)、2 段式 (<code>{project}/{filename}</code>) 与 1 段式 (<code>{filename}</code>)，系统根据扩展名自动分派用途。<br />
            • <strong>认证方式：</strong> 支持 AWS SigV4 签名、Authorization Bearer Token 以及 URL 参数 <code>?auth_token=...</code>。
          </div>
        </div>
      </div>

      <!-- Client Tabs & Code Snippet Display -->
      <div class="m3-card" style="padding: 24px; background: rgba(255,255,255,0.02); border: 1px solid rgba(255,255,255,0.06);">
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
              Node.js (AWS SDK v3)
            </button>
          </div>
          <button class="m3-btn m3-btn-secondary m3-btn-sm" @click="copyS3Command">
            <span class="material-symbols-rounded" style="font-size: 16px;">content_copy</span>
            <span>复制示例代码</span>
          </button>
        </div>

        <div class="console-output font-mono" style="margin-top: 16px; color: #a5d6ff; background: #06090d; min-height: 260px; max-height: 420px; overflow-y: auto; white-space: pre-wrap; font-size: 13px; line-height: 1.6; border: 1px solid rgba(255,255,255,0.06); padding: 16px; border-radius: 8px;">{{ activeS3Code }}</div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.mode-tabs-bar {
  display: flex;
  gap: 10px;
  margin-bottom: 24px;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  padding-bottom: 14px;
}

.s3-config-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 12px;
  margin-top: 18px;
}

.s3-config-card {
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.06);
  border-radius: 8px;
  padding: 12px 16px;
}

.s3-config-label {
  font-size: 11px;
  color: rgba(255, 255, 255, 0.5);
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
  font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace;
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
  color: rgba(255, 255, 255, 0.6);
  font-size: 13px;
  padding: 7px 14px;
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.2s cubic-bezier(0.4, 0, 0.2, 1);
}

.s3-tab-btn:hover {
  background: rgba(255, 255, 255, 0.06);
  color: #fff;
}

.s3-tab-btn.active {
  background: rgba(0, 229, 255, 0.12);
  color: #00e5ff;
  font-weight: 600;
  border: 1px solid rgba(0, 229, 255, 0.25);
}

.quick-id-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
}

.id-chip-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.08);
  padding: 4px 10px;
  border-radius: 6px;
  cursor: pointer;
  font-size: 12px;
  color: rgba(255, 255, 255, 0.8);
  font-family: monospace;
  transition: all 0.2s ease;
}

.id-chip-btn:hover {
  background: rgba(0, 229, 255, 0.08);
  border-color: rgba(0, 229, 255, 0.3);
  color: #00e5ff;
}

.id-chip-btn.active {
  background: rgba(0, 229, 255, 0.15);
  border-color: #00e5ff;
  color: #00e5ff;
}

.chip-dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #00e5ff;
  box-shadow: 0 0 6px #00e5ff;
}

.chip-tag {
  font-size: 10px;
  padding: 1px 4px;
  border-radius: 3px;
  background: rgba(255, 255, 255, 0.08);
  color: rgba(255, 255, 255, 0.6);
}

@media (max-width: 640px) {
  .s3-tabs-header {
    flex-direction: column;
    align-items: stretch;
    gap: 12px;
  }
  .s3-tabs {
    overflow-x: auto;
    white-space: nowrap;
    -webkit-overflow-scrolling: touch;
    padding-bottom: 4px;
  }
  .s3-tab-btn {
    flex-shrink: 0;
  }
}
</style>
