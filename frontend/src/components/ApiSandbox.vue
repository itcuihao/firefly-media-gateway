<script setup lang="ts">
import { ref, inject } from 'vue'
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
const fileInputRef = ref<HTMLInputElement | null>(null)

// Output states
const statusPillVisible = ref(false)
const statusPillText = ref('')
const statusPillStyle = ref({ background: '', color: '' })
const responseBody = ref('等待发送请求，联调数据将在此实时高亮渲染...')
const apiLoading = ref(false)

function copyConsoleOutput() {
  navigator.clipboard.writeText(responseBody.value).then(() => {
    showToast('已成功复制到剪贴板！')
  }).catch(() => {
    showToast('复制失败，请手动选择复制', 'error')
  })
}

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

      fetchUrl = '/api/v1/media/upload'
      const form = new FormData()
      form.append('file', fileInput.files[0])
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
    <div class="api-sandbox-layout">
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
              <label>文件 (file)</label>
              <div class="input-wrapper">
                <input ref="fileInputRef" type="file" accept="image/*,video/*" />
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
  </div>
</template>
