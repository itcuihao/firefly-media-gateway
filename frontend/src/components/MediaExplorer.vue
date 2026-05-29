<script setup lang="ts">
import { ref, onMounted, computed, watch, inject, type Ref } from 'vue'
import {
  NCard,
  NGrid,
  NGi,
  NButton,
  NInput,
  NSelect,
  NCheckbox,
  NDrawer,
  NDrawerContent,
  NModal,
  NForm,
  NFormItem,
  NTag,
  useMessage,
  useDialog
} from 'naive-ui'
import {
  Search,
  Grid3X3,
  List,
  RefreshCw,
  Plus,
  Trash2,
  ExternalLink,
  Copy,
  FileText,
  Video,
  X
} from 'lucide-vue-next'
import { apiRequest, openMediaAsset } from '../api'
import type { MediaAsset } from '../api'

const props = defineProps({
  triggerUpload: Boolean
})

const emit = defineEmits(['uploadHandled'])

const message = useMessage()
const dialog = useDialog()
const loading = ref(false)
const assets = ref<MediaAsset[]>([])

// Query Filters
const searchKeyword = ref('')
const selectedProject = ref<string | null>(null)
const selectedUsage = ref<string | null>(null)
const showDeleted = ref(false)
const layoutMode = ref<'grid' | 'list'>('grid')

// Details Drawer state
const drawerActive = ref(false)
const activeAsset = ref<MediaAsset | null>(null)

// Upload Modal Dialog state
const uploadModalActive = ref(false)
const uploadForm = ref({
  project: 'interactive-video',
  usage: 'scene',
  member: false,
  file: null as File | null
})

// Global refresh dispatcher
const refreshStats = inject<Ref<number>>('refreshStats')

// Unique options computed from current items database for filters dropdown
const projectOptions = computed(() => {
  const projects = new Set<string>()
  assets.value.forEach(a => {
    if (a.project) projects.add(a.project)
  })
  return Array.from(projects).map(p => ({ label: p, value: p }))
})

const usageOptions = computed(() => {
  const usages = new Set<string>()
  assets.value.forEach(a => {
    if (a.usage) usages.add(a.usage)
  })
  return Array.from(usages).map(u => ({ label: u, value: u }))
})

// Core filter query logic matching backend list responses
const filteredAssets = computed(() => {
  return assets.value.filter(asset => {
    // Deleted filter
    if (!showDeleted.value && asset.status === 'deleted') {
      return false
    }

    // Keyword query
    if (searchKeyword.value) {
      const q = searchKeyword.value.toLowerCase()
      const matchId = asset.id.toLowerCase().includes(q)
      const matchMime = asset.mimeType.toLowerCase().includes(q)
      if (!matchId && !matchMime) return false
    }

    // Project selection
    if (selectedProject.value && asset.project !== selectedProject.value) {
      return false
    }

    // Usage selection
    if (selectedUsage.value && asset.usage !== selectedUsage.value) {
      return false
    }

    return true
  })
})

async function fetchAssets() {
  loading.value = true
  try {
    const data = await apiRequest<MediaAsset[]>('/api/v1/media?show_deleted=true')
    assets.value = data
  } catch (err: any) {
    message.error(`无法加载媒体资源: ${err.message}`)
  } finally {
    loading.value = false
  }
}

function handleFileChange(event: Event) {
  const target = event.target as HTMLInputElement
  if (target.files && target.files.length > 0) {
    uploadForm.value.file = target.files[0]
  }
}

async function submitUpload() {
  if (!uploadForm.value.file) {
    message.error('请选择需要上传的文件')
    return
  }

  const formData = new FormData()
  formData.append('file', uploadForm.value.file)
  formData.append('project', uploadForm.value.project.trim())
  formData.append('usage', uploadForm.value.usage)
  formData.append('member', uploadForm.value.member ? 'true' : 'false')

  message.info('正在向网关上传媒体资源，请稍候...')
  uploadModalActive.value = false

  try {
    await apiRequest('/api/v1/media/upload', {
      method: 'POST',
      body: formData
    })
    message.success('文件上传成功！')
    
    // Clear and reload
    uploadForm.value.file = null
    fetchAssets()
    if (refreshStats) refreshStats.value++
  } catch (err: any) {
    message.error(`文件上传失败: ${err.message}`)
  }
}

function openDetail(asset: MediaAsset) {
  activeAsset.value = asset
  drawerActive.value = true
}

async function copyLink(url: string) {
  try {
    await navigator.clipboard.writeText(url)
    message.success('链接已复制到剪贴板')
  } catch (_) {
    message.error('复制失败，请手动选择复制')
  }
}

function handleDeleteAsset(asset: MediaAsset) {
  dialog.warning({
    title: '确认删除资源',
    content: `您确定要删除该资源吗? ID: ${asset.id}`,
    positiveText: '确认删除',
    negativeText: '取消',
    onPositiveClick: async () => {
      try {
        await apiRequest(`/api/v1/media/${asset.id}`, { method: 'DELETE' })
        message.success('资源已标记为删除')
        drawerActive.value = false
        fetchAssets()
        if (refreshStats) refreshStats.value++
      } catch (err: any) {
        message.error(`删除失败: ${err.message}`)
      }
    }
  })
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

// Watch props trigger to load upload panel from App component
watch(() => props.triggerUpload, (val) => {
  if (val) {
    uploadModalActive.value = true
    emit('uploadHandled')
  }
})

onMounted(() => {
  fetchAssets()
})
</script>

<template>
  <div class="flex flex-col gap-6 select-none relative min-h-[calc(100vh-140px)]">
    
    <!-- Filter search card panel -->
    <n-card class="border border-white/5 shadow-md">
      <div class="flex flex-col lg:flex-row gap-4 justify-between items-start lg:items-center">
        
        <!-- Input fields -->
        <div class="flex flex-wrap items-center gap-3 w-full lg:w-auto">
          <n-input 
            v-model:value="searchKeyword" 
            placeholder="搜索资源 ID 或 MIME 格式..." 
            clearable
            class="max-w-[260px] w-full"
          >
            <template #prefix>
              <Search class="w-4 h-4 text-gray-500" />
            </template>
          </n-input>
          
          <n-select 
            v-model:value="selectedProject" 
            placeholder="全部项目 (Project)" 
            clearable 
            :options="projectOptions"
            class="max-w-[180px] w-full"
          />

          <n-select 
            v-model:value="selectedUsage" 
            placeholder="全部用途 (Usage)" 
            clearable 
            :options="usageOptions"
            class="max-w-[180px] w-full"
          />

          <n-checkbox v-model:checked="showDeleted">
            显示已删除资源
          </n-checkbox>
        </div>

        <!-- Layout triggers and Refresh buttons -->
        <div class="flex items-center gap-3 ml-auto lg:ml-0">
          <div class="flex items-center bg-white/5 rounded-xl p-0.5 border border-white/5">
            <button 
              @click="layoutMode = 'grid'"
              class="p-2 rounded-lg cursor-pointer transition"
              :class="layoutMode === 'grid' ? 'bg-[#1a2126] text-cyan-400' : 'text-gray-500 hover:text-white'"
            >
              <Grid3X3 class="w-4 h-4" />
            </button>
            <button 
              @click="layoutMode = 'list'"
              class="p-2 rounded-lg cursor-pointer transition"
              :class="layoutMode === 'list' ? 'bg-[#1a2126] text-cyan-400' : 'text-gray-500 hover:text-white'"
            >
              <List class="w-4 h-4" />
            </button>
          </div>

          <n-button @click="fetchAssets" :disabled="loading" secondary>
            <template #icon>
              <RefreshCw class="w-4 h-4" :class="loading ? 'animate-spin' : ''" />
            </template>
            <span>刷新</span>
          </n-button>
        </div>

      </div>
    </n-card>

    <!-- Empty state loader -->
    <div v-if="filteredAssets.length === 0" class="flex flex-col items-center justify-center py-20 text-gray-500">
      <FolderOpen class="w-16 h-16 text-white/5 mb-4" />
      <span class="text-sm">未检索到符合条件的媒体资源文件</span>
    </div>

    <!-- Grid Layout render -->
    <n-grid v-else-if="layoutMode === 'grid'" cols="2 m:3 l:4 xl:5" responsive="screen" :x-gap="16" :y-gap="16" class="select-text">
      <n-gi v-for="asset in filteredAssets" :key="asset.id">
        <n-card 
          @click="openDetail(asset)"
          class="border border-white/5 hover:border-cyan-500/30 transition-all duration-300 group cursor-pointer h-full relative"
          content-class="p-0"
        >
          <!-- Asset Preview -->
          <div class="aspect-square bg-black/40 flex items-center justify-center overflow-hidden relative">
            <img 
              v-if="asset.mimeType.startsWith('image/') && asset.status === 'active'"
              :src="asset.publicUrl"
              class="w-full h-full object-cover group-hover:scale-105 transition duration-300"
            />
            <div v-else-if="asset.mimeType.startsWith('video/')" class="flex flex-col items-center gap-2 text-cyan-500/70">
              <Video class="w-8 h-8" />
              <span class="text-[10px] uppercase font-bold bg-cyan-950/40 border border-cyan-500/20 px-2 py-0.5 rounded-full">Video</span>
            </div>
            <div v-else class="text-gray-600">
              <FileText class="w-8 h-8" />
            </div>

            <!-- Deleted Badge -->
            <div v-if="asset.status === 'deleted'" class="absolute inset-0 bg-red-950/70 flex items-center justify-center border border-red-500/25">
              <span class="text-xs text-red-200 font-extrabold uppercase tracking-widest bg-red-950 px-3 py-1 rounded-full border border-red-500/40">已删除</span>
            </div>
          </div>

          <!-- Description Footer info -->
          <div class="p-3.5 flex flex-col gap-1">
            <div class="text-xs font-bold text-white truncate max-w-full font-mono">{{ asset.id }}</div>
            <div class="flex items-center justify-between text-[10px] text-gray-500">
              <span>{{ formatBytes(asset.fileSize) }}</span>
              <span class="truncate max-w-[80px] font-semibold text-cyan-500/70">{{ asset.project }}</span>
            </div>
          </div>
        </n-card>
      </n-gi>
    </n-grid>

    <!-- List Layout render -->
    <div v-else class="w-full bg-[#11181c] border border-white/5 rounded-2xl overflow-hidden shadow-lg select-text">
      <table class="w-full text-xs text-left border-collapse">
        <thead>
          <tr class="bg-white/5 border-b border-white/5 text-gray-400 font-semibold uppercase tracking-wider">
            <th class="py-4 px-5 w-[80px]">预览</th>
            <th class="py-4 px-5">资源 ID</th>
            <th class="py-4 px-5">类型 (MIME)</th>
            <th class="py-4 px-5">大小</th>
            <th class="py-4 px-5">项目/用途</th>
            <th class="py-4 px-5">状态</th>
            <th class="py-4 px-5 text-right">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr 
            v-for="asset in filteredAssets" 
            :key="asset.id"
            @click="openDetail(asset)"
            class="border-b border-white/5 hover:bg-white/5 cursor-pointer transition duration-150"
          >
            <!-- Preview -->
            <td class="py-3 px-5">
              <div class="w-10 h-10 bg-black/40 rounded-lg overflow-hidden flex items-center justify-center">
                <img 
                  v-if="asset.mimeType.startsWith('image/') && asset.status === 'active'"
                  :src="asset.publicUrl"
                  class="w-full h-full object-cover"
                />
                <Video v-else-if="asset.mimeType.startsWith('video/')" class="w-4 h-4 text-cyan-400" />
                <FileText v-else class="w-4 h-4 text-gray-600" />
              </div>
            </td>
            
            <!-- ID -->
            <td class="py-3 px-5 font-mono text-white font-semibold">{{ asset.id }}</td>
            
            <!-- Mime -->
            <td class="py-3 px-5 text-gray-400 font-mono">{{ asset.mimeType }}</td>
            
            <!-- Size -->
            <td class="py-3 px-5 text-gray-300">{{ formatBytes(asset.fileSize) }}</td>
            
            <!-- Project/Usage -->
            <td class="py-3 px-5">
              <div class="flex items-center gap-1.5 flex-wrap">
                <n-tag size="small" type="info" :bordered="false">{{ asset.project }}</n-tag>
                <n-tag size="small" :bordered="false">{{ asset.usage }}</n-tag>
              </div>
            </td>

            <!-- Status -->
            <td class="py-3 px-5">
              <n-tag size="small" :type="asset.status === 'active' ? 'success' : 'error'">
                {{ asset.status === 'active' ? '已激活' : '已删除' }}
              </n-tag>
            </td>

            <!-- Actions -->
            <td class="py-3 px-5 text-right" @click.stop>
              <div class="flex justify-end gap-2">
                <n-button size="tiny" type="primary" secondary @click="openMediaAsset(asset.publicUrl)">
                  <template #icon><ExternalLink class="w-3.5 h-3.5" /></template>
                  <span>访问</span>
                </n-button>
                <n-button size="tiny" type="default" secondary @click="copyLink(asset.publicUrl)">
                  <template #icon><Copy class="w-3.5 h-3.5" /></template>
                  <span>复制</span>
                </n-button>
                <n-button 
                  v-if="asset.status === 'active'"
                  size="tiny" 
                  type="error" 
                  secondary 
                  @click="handleDeleteAsset(asset)"
                >
                  <template #icon><Trash2 class="w-3.5 h-3.5" /></template>
                  <span>删除</span>
                </n-button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Floating Action Button for uploading files -->
    <button 
      @click="uploadModalActive = true"
      class="fab fixed bottom-8 right-8 w-14 h-14 bg-cyan-400 hover:bg-cyan-500 text-gray-900 rounded-full flex items-center justify-center shadow-lg hover:shadow-cyan-400/20 hover:scale-105 active:scale-95 transition-all duration-200 cursor-pointer z-40"
    >
      <Plus class="w-8 h-8 font-bold" />
    </button>

    <!-- Metadata Details Sheet (Drawer) -->
    <n-drawer v-model:show="drawerActive" :width="400" placement="right">
      <n-drawer-content closable>
        <template #header>
          <div class="text-base font-bold text-white">📁 媒体资源元数据</div>
        </template>

        <div v-if="activeAsset" class="flex flex-col gap-6 select-text">
          
          <!-- Large image/video preview in side pane -->
          <div class="w-full aspect-video bg-black/60 border border-white/5 rounded-2xl overflow-hidden flex items-center justify-center relative">
            <img 
              v-if="activeAsset.mimeType.startsWith('image/') && activeAsset.status === 'active'"
              :src="activeAsset.publicUrl"
              class="w-full h-full object-contain"
            />
            <div v-else-if="activeAsset.mimeType.startsWith('video/')" class="flex flex-col items-center gap-2 text-cyan-400">
              <Video class="w-12 h-12" />
              <span class="text-xs uppercase font-extrabold tracking-wider bg-cyan-950/60 border border-cyan-500/20 px-3 py-1 rounded-full">Video File</span>
            </div>
            <div v-else class="text-gray-500">
              <FileText class="w-12 h-12" />
            </div>
          </div>

          <!-- Metadata parameters Table list -->
          <div class="flex flex-col gap-4">
            <h4 class="text-xs font-semibold uppercase tracking-wider text-gray-500 border-b border-white/5 pb-1">核心信息</h4>
            <div class="grid grid-cols-3 text-xs py-1">
              <span class="text-gray-400">资源 ID</span>
              <span class="col-span-2 text-white font-mono font-bold break-all">{{ activeAsset.id }}</span>
            </div>
            <div class="grid grid-cols-3 text-xs py-1">
              <span class="text-gray-400">MIME 类型</span>
              <span class="col-span-2 text-white font-mono">{{ activeAsset.mimeType }}</span>
            </div>
            <div class="grid grid-cols-3 text-xs py-1">
              <span class="text-gray-400">资源大小</span>
              <span class="col-span-2 text-white font-semibold">{{ formatBytes(activeAsset.fileSize) }}</span>
            </div>
            <div class="grid grid-cols-3 text-xs py-1">
              <span class="text-gray-400">项目目录</span>
              <span class="col-span-2 text-cyan-400 font-bold">{{ activeAsset.project }}</span>
            </div>
            <div class="grid grid-cols-3 text-xs py-1">
              <span class="text-gray-400">使用场景</span>
              <span class="col-span-2 text-white font-semibold">{{ activeAsset.usage }}</span>
            </div>
            <div class="grid grid-cols-3 text-xs py-1">
              <span class="text-gray-400">状态</span>
              <span class="col-span-2">
                <n-tag size="small" :type="activeAsset.status === 'active' ? 'success' : 'error'">
                  {{ activeAsset.status === 'active' ? '已激活 (Active)' : '已删除 (Deleted)' }}
                </n-tag>
              </span>
            </div>
          </div>

          <!-- BOT / Storage engine credentials -->
          <div class="flex flex-col gap-4">
            <h4 class="text-xs font-semibold uppercase tracking-wider text-gray-500 border-b border-white/5 pb-1">底层接口参数</h4>
            <div v-if="activeAsset.tgFileId" class="grid grid-cols-3 text-xs py-1">
              <span class="text-gray-400">TG File ID</span>
              <span class="col-span-2 text-white font-mono break-all">{{ activeAsset.tgFileId }}</span>
            </div>
            <div v-if="activeAsset.tgMessageId" class="grid grid-cols-3 text-xs py-1">
              <span class="text-gray-400">TG Message ID</span>
              <span class="col-span-2 text-white font-mono">{{ activeAsset.tgMessageId }}</span>
            </div>
            <div v-if="activeAsset.tgChatId" class="grid grid-cols-3 text-xs py-1">
              <span class="text-gray-400">TG Chat ID</span>
              <span class="col-span-2 text-white font-mono">{{ activeAsset.tgChatId }}</span>
            </div>
            <div v-if="activeAsset.discordMessageId" class="grid grid-cols-3 text-xs py-1">
              <span class="text-gray-400">Discord Msg ID</span>
              <span class="col-span-2 text-white font-mono">{{ activeAsset.discordMessageId }}</span>
            </div>
            <div v-if="activeAsset.discordChannelId" class="grid grid-cols-3 text-xs py-1">
              <span class="text-gray-400">Discord Chl ID</span>
              <span class="col-span-2 text-white font-mono">{{ activeAsset.discordChannelId }}</span>
            </div>
            <div v-if="activeAsset.s3Key" class="grid grid-cols-3 text-xs py-1">
              <span class="text-gray-400">S3 Key</span>
              <span class="col-span-2 text-white font-mono break-all">{{ activeAsset.s3Key }}</span>
            </div>
          </div>

          <div class="flex flex-col gap-4">
            <h4 class="text-xs font-semibold uppercase tracking-wider text-gray-500 border-b border-white/5 pb-1">时间戳</h4>
            <div class="grid grid-cols-3 text-xs py-1">
              <span class="text-gray-400">创建时间</span>
              <span class="col-span-2 text-white font-semibold">{{ new Date(activeAsset.createdAt).toLocaleString() }}</span>
            </div>
            <div class="grid grid-cols-3 text-xs py-1">
              <span class="text-gray-400">更新时间</span>
              <span class="col-span-2 text-white font-semibold">{{ new Date(activeAsset.updatedAt).toLocaleString() }}</span>
            </div>
          </div>

          <!-- Bottom Trigger Buttons -->
          <div class="flex gap-3 border-t border-white/5 pt-4 mt-2">
            <n-button class="flex-1 cursor-pointer" type="primary" @click="openMediaAsset(activeAsset.publicUrl)">
              <template #icon><ExternalLink class="w-4 h-4" /></template>
              在新标签页打开
            </n-button>
            <n-button class="flex-1 cursor-pointer" type="default" @click="copyLink(activeAsset.publicUrl)">
              <template #icon><Copy class="w-4 h-4" /></template>
              复制直链
            </n-button>
            <n-button 
              v-if="activeAsset.status === 'active'"
              class="flex-1 cursor-pointer" 
              type="error" 
              secondary 
              @click="handleDeleteAsset(activeAsset)"
            >
              <template #icon><Trash2 class="w-4 h-4" /></template>
              删除文件
            </n-button>
          </div>

        </div>
      </n-drawer-content>
    </n-drawer>

    <!-- Upload dialog overlay form modal -->
    <n-modal v-model:show="uploadModalActive">
      <n-card
        style="width: 500px; border-radius: 20px;"
        title="📤 上传新媒体文件"
        :bordered="false"
        size="huge"
        role="dialog"
        aria-modal="true"
        class="border border-white/10 select-text"
      >
        <template #header-extra>
          <button @click="uploadModalActive = false" class="text-gray-400 hover:text-white cursor-pointer">
            <X class="w-5 h-5" />
          </button>
        </template>

        <p class="text-xs text-gray-400 mb-6">
          文件最大支持限制：图片类最大 10MB (jpg/png/webp)，视频类最大 120MB (mp4/webm/mov)。
        </p>

        <n-form :model="uploadForm" class="flex flex-col gap-5">
          <n-form-item label="所属项目 (Project)">
            <n-input v-model:value="uploadForm.project" placeholder="如 interactive-video" />
          </n-form-item>

          <n-form-item label="使用场景 (Usage)">
            <n-select 
              v-model:value="uploadForm.usage" 
              :options="[
                { label: 'scene (正片场景)', value: 'scene' },
                { label: 'cover (封面缩略)', value: 'cover' },
                { label: 'avatar (头像标志)', value: 'avatar' }
              ]" 
            />
          </n-form-item>

          <n-checkbox v-model:checked="uploadForm.member">
            是否启用大文件分片上传 (需要会员身份)
          </n-checkbox>

          <n-form-item label="选择媒体文件" class="mt-2">
            <div class="w-full flex flex-col gap-2">
              <input 
                type="file" 
                accept="image/*,video/*" 
                @change="handleFileChange"
                class="block w-full text-xs text-slate-500 file:mr-4 file:py-2.5 file:px-4 file:rounded-full file:border-0 file:text-xs file:font-semibold file:bg-cyan-500/10 file:text-cyan-400 hover:file:bg-cyan-500/20 file:cursor-pointer"
              />
              <span v-if="uploadForm.file" class="text-xs text-cyan-400 font-semibold font-mono">
                已选中: {{ uploadForm.file.name }} ({{ formatBytes(uploadForm.file.size) }})
              </span>
            </div>
          </n-form-item>
        </n-form>

        <template #footer>
          <div class="flex justify-end gap-3">
            <n-button @click="uploadModalActive = false" class="cursor-pointer">取消</n-button>
            <n-button type="primary" @click="submitUpload" class="cursor-pointer">确认上传</n-button>
          </div>
        </template>
      </n-card>
    </n-modal>

  </div>
</template>
