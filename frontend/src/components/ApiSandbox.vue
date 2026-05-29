<script setup lang="ts">
import { ref } from 'vue'
import {
  NCard,
  NButton,
  NInput,
  NSelect,
  NCheckbox,
  NForm,
  NFormItem,
  NCode,
  useMessage
} from 'naive-ui'
import {
  Play,
  Terminal,
  Copy
} from 'lucide-vue-next'
import { getApiBaseUrl } from '../api'

const message = useMessage()

// API routing dropdown options
const apiOptions = [
  { label: 'GET /api/v1/health (服务状态与运行环境)', value: 'health' },
  { label: 'GET /api/v1/media (文件列表过滤查询)', value: 'list' },
  { label: 'GET /api/v1/media/{mediaId}/meta (媒体元数据获取)', value: 'meta' },
  { label: 'POST /api/v1/media/upload (上传媒体文件对象)', value: 'upload' },
  { label: 'DELETE /api/v1/media/{mediaId} (删除物理文件资源)', value: 'delete' },
  { label: 'POST /api/v1/provider/telegram/chat-ids (获取 Bot 最新会话)', value: 'telegram_chats' }
]

const selectedApi = ref('health')

// Form parameter states
const params = ref({
  mediaId: '',
  project: '',
  usage: 'scene',
  keyword: '',
  showDeleted: false,
  uploadFile: null as File | null,
  tgToken: '',
  member: false
})

// Loading & Console Output states
const requestLoading = ref(false)
const responseStatus = ref<number | null>(null)
const responseStatusText = ref('')
const responseBody = ref('等待发送请求，联调数据将在此实时高亮渲染...')

function handleFileChange(event: Event) {
  const target = event.target as HTMLInputElement
  if (target.files && target.files.length > 0) {
    params.value.uploadFile = target.files[0]
  }
}

async function triggerApiCall() {
  requestLoading.value = ref(true).value
  responseStatus.value = null
  responseStatusText.value = ''
  responseBody.value = '正在通信中...'

  try {
    let path = ''
    let options: RequestInit = {}

    const token = localStorage.getItem('firefly_api_token') || ''
    const headers = new Headers()
    if (token) {
      headers.set('Authorization', `Bearer ${token}`)
    }

    if (selectedApi.value === 'health') {
      path = '/api/v1/health'
      options = { method: 'GET', headers }
    } else if (selectedApi.value === 'list') {
      const q = new URLSearchParams()
      if (params.value.project) q.append('project', params.value.project.trim())
      if (params.value.usage) q.append('usage', params.value.usage)
      if (params.value.keyword) q.append('q', params.value.keyword.trim())
      if (params.value.showDeleted) q.append('show_deleted', 'true')
      path = `/api/v1/media?${q.toString()}`
      options = { method: 'GET', headers }
    } else if (selectedApi.value === 'meta') {
      if (!params.value.mediaId) {
        throw new Error('请输入 mediaId')
      }
      path = `/api/v1/media/${params.value.mediaId.trim()}/meta`
      options = { method: 'GET', headers }
    } else if (selectedApi.value === 'delete') {
      if (!params.value.mediaId) {
        throw new Error('请输入 mediaId')
      }
      path = `/api/v1/media/${params.value.mediaId.trim()}`
      options = { method: 'DELETE', headers }
    } else if (selectedApi.value === 'telegram_chats') {
      path = '/api/v1/provider/telegram/chat-ids'
      options = {
        method: 'POST',
        headers: {
          ...Object.fromEntries(headers.entries()),
          'Content-Type': 'application/json'
        },
        body: JSON.stringify({ token: params.value.tgToken })
      }
    } else if (selectedApi.value === 'upload') {
      if (!params.value.uploadFile) {
        throw new Error('请选择需要上传的媒体文件')
      }
      const formData = new FormData()
      formData.append('file', params.value.uploadFile)
      formData.append('project', params.value.project.trim())
      formData.append('usage', params.value.usage)
      formData.append('member', params.value.member ? 'true' : 'false')

      path = '/api/v1/media/upload'
      options = {
        method: 'POST',
        headers, // Do NOT set Content-Type header manually for FormData, browser sets boundary
        body: formData
      }
    }

    // Assemble target URL manually to display and invoke correctly
    const baseUrl = getApiBaseUrl()
    let requestUrl = path
    if (baseUrl) {
      const cleanBase = baseUrl.endsWith('/') ? baseUrl.slice(0, -1) : baseUrl
      const cleanPath = path.startsWith('/') ? path : '/' + path
      requestUrl = cleanBase + cleanPath
    }

    const resp = await fetch(requestUrl, options)
    responseStatus.value = resp.status
    responseStatusText.value = resp.statusText

    if (resp.status === 204) {
      responseBody.value = '// 204 No Content (操作执行成功，无返回值)'
      message.success('接口请求执行成功')
      return
    }

    const contentType = resp.headers.get('content-type') || ''
    if (contentType.includes('application/json')) {
      const json = await resp.json()
      responseBody.value = JSON.stringify(json, null, 2)
    } else {
      responseBody.value = await resp.text()
    }

    if (resp.ok) {
      message.success('接口请求成功！')
    } else {
      message.error(`接口请求失败: ${resp.status}`)
    }

  } catch (err: any) {
    responseBody.value = `// 客户端请求异常\nError: ${err.message}`
    message.error(err.message)
  } finally {
    requestLoading.value = false
  }
}

async function copyOutput() {
  try {
    await navigator.clipboard.writeText(responseBody.value)
    message.success('数据控制台响应已复制！')
  } catch (_) {
    message.error('复制失败')
  }
}
</script>

<template>
  <div class="flex flex-col gap-6 select-none h-full">
    <div>
      <h2 class="text-xl font-bold text-white mb-1">API 联调沙盒</h2>
      <p class="text-xs text-gray-400">在此面板中，您可以快速发起网关核心 API 的连通性校验，直接填充参数并实时高亮输出接口返回的元数据内容。</p>
    </div>

    <div class="grid grid-cols-1 lg:grid-cols-5 gap-6 items-start">
      
      <!-- Params Form card panel -->
      <n-card class="border border-white/5 shadow-md lg:col-span-2 select-text">
        <h3 class="text-sm font-bold text-white mb-4">测试参数构建</h3>
        
        <n-form :show-feedback="false" class="flex flex-col gap-4">
          <n-form-item label="选择要联调的 API">
            <n-select v-model:value="selectedApi" :options="apiOptions" />
          </n-form-item>

          <!-- GET /api/v1/media list parameters -->
          <template v-if="selectedApi === 'list'">
            <n-form-item label="项目 (Project)">
              <n-input v-model:value="params.project" placeholder="如 interactive-video" />
            </n-form-item>
            <n-form-item label="用途 (Usage)">
              <n-select 
                v-model:value="params.usage" 
                :options="[
                  { label: '全部', value: '' },
                  { label: 'scene (正片场景)', value: 'scene' },
                  { label: 'cover (封面缩略)', value: 'cover' },
                  { label: 'avatar (头像标志)', value: 'avatar' }
                ]" 
              />
            </n-form-item>
            <n-form-item label="搜索关键词">
              <n-input v-model:value="params.keyword" placeholder="搜索资源 ID 或 MIME" />
            </n-form-item>
            <n-checkbox v-model:checked="params.showDeleted">
              显示已删除资源
            </n-checkbox>
          </template>

          <!-- GET /meta or DELETE assets parameters -->
          <template v-else-if="selectedApi === 'meta' || selectedApi === 'delete'">
            <n-form-item label="媒体资源 ID (mediaId)">
              <n-input v-model:value="params.mediaId" placeholder="输入已存储的资源 ID" />
            </n-form-item>
          </template>

          <!-- POST /provider/telegram/chat-ids parameter -->
          <template v-else-if="selectedApi === 'telegram_chats'">
            <n-form-item label="Telegram Bot Token">
              <n-input v-model:value="params.tgToken" type="password" placeholder="为空则自动加载后端 env 配置" />
            </n-form-item>
          </template>

          <!-- POST /media/upload parameters -->
          <template v-else-if="selectedApi === 'upload'">
            <n-form-item label="所属项目 (Project)">
              <n-input v-model:value="params.project" placeholder="如 interactive-video" />
            </n-form-item>
            <n-form-item label="使用场景 (Usage)">
              <n-select 
                v-model:value="params.usage" 
                :options="[
                  { label: 'scene (正片场景)', value: 'scene' },
                  { label: 'cover (封面缩略)', value: 'cover' },
                  { label: 'avatar (头像标志)', value: 'avatar' }
                ]" 
              />
            </n-form-item>
            <n-checkbox v-model:checked="params.member" class="mb-2">
              启用大文件分片上传 (需要会员身份)
            </n-checkbox>
            <n-form-item label="选择上传文件">
              <input 
                type="file" 
                accept="image/*,video/*" 
                @change="handleFileChange"
                class="block w-full text-xs text-slate-500 file:mr-4 file:py-2 file:px-4 file:rounded-full file:border-0 file:text-xs file:font-semibold file:bg-cyan-500/10 file:text-cyan-400 hover:file:bg-cyan-500/20 file:cursor-pointer"
              />
            </n-form-item>
          </template>
        </n-form>

        <div class="mt-6">
          <n-button 
            type="primary" 
            @click="triggerApiCall" 
            :loading="requestLoading" 
            class="w-full cursor-pointer"
          >
            <template #icon>
              <Play class="w-4 h-4" />
            </template>
            <span>发起 API 请求</span>
          </n-button>
        </div>
      </n-card>

      <!-- Console output logger block -->
      <div class="lg:col-span-3 flex flex-col border border-white/5 rounded-2xl overflow-hidden bg-[#0a0e12] h-[550px] shadow-lg">
        
        <!-- Header -->
        <div class="bg-white/5 px-5 py-3.5 border-b border-white/5 flex items-center justify-between">
          <div class="flex items-center gap-2">
            <Terminal class="w-4 h-4 text-purple-400" />
            <h3 class="text-xs font-bold text-white uppercase tracking-wider">控制台调试输出 (Console Response)</h3>
          </div>

          <div class="flex items-center gap-3">
            <span 
              v-if="responseStatus !== null"
              class="text-[10px] font-bold px-2 py-0.5 rounded-full font-mono uppercase"
              :class="responseStatus >= 200 && responseStatus < 300 
                ? 'bg-green-950/40 border border-green-500/25 text-green-400' 
                : 'bg-red-950/40 border border-red-500/25 text-red-400'"
            >
              HTTP {{ responseStatus }} {{ responseStatusText }}
            </span>
            
            <n-button size="tiny" type="default" secondary @click="copyOutput">
              <template #icon>
                <Copy class="w-3.5 h-3.5" />
              </template>
              <span>复制响应</span>
            </n-button>
          </div>
        </div>

        <!-- Render text console -->
        <div class="flex-1 p-5 overflow-auto font-mono text-xs select-text leading-relaxed">
          <n-code 
            :code="responseBody" 
            language="json" 
            :word-wrap="true" 
          />
        </div>

      </div>

    </div>
  </div>
</template>
