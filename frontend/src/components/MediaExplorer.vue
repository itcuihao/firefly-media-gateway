<script setup lang="ts">
import { ref, onMounted, computed, watch, inject } from 'vue'
import { apiRequest, openMediaAsset } from '../api'
import type { MediaAsset } from '../api'

const props = defineProps({
  triggerUpload: Boolean
})

const emit = defineEmits(['uploadHandled'])

const showToast = inject<(msg: string, type?: 'success' | 'error') => void>('showToast', () => {})

const loading = ref(false)
const assets = ref<MediaAsset[]>([])
const layoutMode = ref<'grid' | 'list'>('grid')

// Query Filters & Search Debounce
const searchQueryInput = ref('')
const searchKeyword = ref('')
const selectedProject = ref('')
const selectedUsage = ref('')
const showDeleted = ref(false)

const projectsList = ref<string[]>([])
const usagesList = ref<string[]>([])

// Pagination
const currentPage = ref(1)
const pageSize = ref(20)
const totalCount = ref(0)
const totalPages = computed(() => Math.ceil(totalCount.value / pageSize.value))

// Details Drawer state
const sheetActive = ref(false)
const activeAsset = ref<MediaAsset | null>(null)

// Upload Modal state
const uploadDialogOpen = ref(false)
const uploadProject = ref('interactive-video')
const uploadUsage = ref('cover')
const uploadAutoWebp = ref(true)
const fileInputRef = ref<HTMLInputElement | null>(null)
const selectedFileKind = ref<'image' | 'video' | 'audio' | 'file' | null>(null)
const selectedFileName = ref('')
const dragOver = ref(false)

const sizeLimitHint = computed(() => {
  if (selectedFileKind.value === 'image') return '图片最大 10MB (jpg/png/webp/gif/svg/avif)'
  if (selectedFileKind.value === 'video') return '视频最大 2GB (mp4/webm/mov/mkv，超过 15MB 自动分片加速)'
  if (selectedFileKind.value === 'audio') return '音频最大 50MB (mp3/ogg/wav/aac/flac/m4a)'
  if (selectedFileKind.value === 'file') return '文档/压缩包最大 50MB (pdf/doc/docx/xls/xlsx/ppt/pptx/csv/txt/zip/rar/7z)'
  return '支持图片 (最大10MB)、视频 (最大2GB)、音频与文档/压缩包 (最大50MB)'
})

function onFileSelected() {
  const file = fileInputRef.value?.files?.[0]
  if (!file) {
    selectedFileKind.value = null
    selectedFileName.value = ''
    return
  }
  selectedFileName.value = file.name
  const ext = file.name.substring(file.name.lastIndexOf('.')).toLowerCase()
  if (file.type.startsWith('image/')) {
    selectedFileKind.value = 'image'
  } else if (file.type.startsWith('video/')) {
    selectedFileKind.value = 'video'
    uploadUsage.value = 'scene'
  } else if (file.type.startsWith('audio/') || ext === '.mp3' || ext === '.wav' || ext === '.flac' || ext === '.ogg' || ext === '.aac' || ext === '.m4a') {
    selectedFileKind.value = 'audio'
    uploadUsage.value = 'audio'
  } else {
    selectedFileKind.value = 'file'
    uploadUsage.value = 'file'
  }
}

function applyFile(file: File) {
  const dt = new DataTransfer()
  dt.items.add(file)
  if (fileInputRef.value) {
    fileInputRef.value.files = dt.files
  }
  selectedFileName.value = file.name
  const ext = file.name.substring(file.name.lastIndexOf('.')).toLowerCase()
  if (file.type.startsWith('image/')) {
    selectedFileKind.value = 'image'
  } else if (file.type.startsWith('video/')) {
    selectedFileKind.value = 'video'
    uploadUsage.value = 'scene'
  } else if (file.type.startsWith('audio/') || ext === '.mp3' || ext === '.wav' || ext === '.flac' || ext === '.ogg' || ext === '.aac' || ext === '.m4a') {
    selectedFileKind.value = 'audio'
    uploadUsage.value = 'audio'
  } else {
    selectedFileKind.value = 'file'
    uploadUsage.value = 'file'
  }
}

function onDrop(e: DragEvent) {
  dragOver.value = false
  const file = e.dataTransfer?.files?.[0]
  if (file && (file.type.startsWith('image/') || file.type.startsWith('video/') || file.type.startsWith('audio/'))) {
    applyFile(file)
  }
}

function clearSelectedFile() {
  selectedFileKind.value = null
  selectedFileName.value = ''
  if (fileInputRef.value) fileInputRef.value.value = ''
}

// Debounce search input
let debounceTimer: any = null
watch(searchQueryInput, (newVal) => {
  clearTimeout(debounceTimer)
  debounceTimer = setTimeout(() => {
    searchKeyword.value = newVal.trim()
  }, 300)
})

async function fetchMetadata() {
  try {
    const [projs, usgs] = await Promise.all([
      apiRequest<string[]>('/api/v1/media/projects'),
      apiRequest<string[]>('/api/v1/media/usages')
    ])
    projectsList.value = projs || []
    usagesList.value = usgs || []
  } catch (err: any) {
    console.error('获取过滤标签失败:', err)
  }
}

// Filter Logic - since server filters, filteredAssets is just the current page assets
const filteredAssets = computed(() => assets.value)

async function fetchAssets() {
  loading.value = true
  try {
    const params = new URLSearchParams()
    params.set('limit', String(pageSize.value))
    params.set('offset', String((currentPage.value - 1) * pageSize.value))
    if (selectedProject.value) params.set('project', selectedProject.value)
    if (selectedUsage.value) params.set('usage', selectedUsage.value)
    if (searchKeyword.value) params.set('search', searchKeyword.value)
    if (showDeleted.value) {
      params.set('status', 'all')
    } else {
      params.set('status', 'active')
    }

    interface MediaResponse {
      total: number
      limit: number
      offset: number
      items: MediaAsset[]
    }

    const data = await apiRequest<MediaResponse>(`/api/v1/media?${params.toString()}`)
    if (data) {
      assets.value = data.items || []
      totalCount.value = data.total || 0
    }
  } catch (err: any) {
    showToast(err.message || '拉取资源失败', 'error')
  } finally {
    loading.value = false
  }
}

// Watch filters - reset to page 1
watch([selectedProject, selectedUsage, showDeleted, searchKeyword, pageSize], () => {
  currentPage.value = 1
  fetchAssets()
})

// Watch page change
watch(currentPage, () => {
  fetchAssets()
})

function prevPage() {
  if (currentPage.value > 1) currentPage.value--
}

function nextPage() {
  if (currentPage.value < totalPages.value) currentPage.value++
}

function firstPage() {
  currentPage.value = 1
}

function lastPage() {
  if (totalPages.value > 0) currentPage.value = totalPages.value
}

function setExplorerLayout(layout: 'grid' | 'list') {
  layoutMode.value = layout
}

function openDetailSheet(mediaId: string) {
  const asset = assets.value.find(a => a.mediaId === mediaId)
  if (!asset) return
  activeAsset.value = asset
  sheetActive.value = true
}

function closeDetailSheet() {
  sheetActive.value = false
  activeAsset.value = null
}

function openUploadDialog() {
  uploadDialogOpen.value = true
}

function closeUploadDialog() {
  uploadDialogOpen.value = false
  selectedFileKind.value = null
  selectedFileName.value = ''
  dragOver.value = false
  if (fileInputRef.value) {
    fileInputRef.value.value = ''
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

async function submitUploadFile() {
  const fileInput = fileInputRef.value
  if (!fileInput || !fileInput.files || !fileInput.files[0]) {
    showToast('请选择需要上传的文件！', 'error')
    return
  }

  let fileToUpload = fileInput.files[0]
  const isJpgOrPng = fileToUpload.type === 'image/jpeg' || fileToUpload.type === 'image/png'

  if (isJpgOrPng && uploadAutoWebp.value) {
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

  const form = new FormData()
  form.append('file', fileToUpload)
  form.append('project', uploadProject.value.trim())
  form.append('usage', uploadUsage.value)

  showToast('正在上传，请耐心等待...', 'success')
  closeUploadDialog()

  try {
    await apiRequest('/api/v1/media/upload', {
      method: 'POST',
      body: form
    })
    showToast('媒体文件上传成功！')
    fetchAssets()
    fetchMetadata()
  } catch (e: any) {
    showToast(e.message || '上传失败', 'error')
  }
}

async function deleteAsset(mediaId: string) {
  if (!confirm('确定要删除此媒体资源吗？\nID: ' + mediaId)) return

  try {
    await apiRequest(`/api/v1/media/${encodeURIComponent(mediaId)}`, {
      method: 'DELETE'
    })
    showToast('媒体资源已成功标记删除！')
    fetchAssets()
    fetchMetadata()
    closeDetailSheet()
  } catch (e: any) {
    showToast(e.message || '删除失败', 'error')
  }
}

function copyText(txt: string) {
  navigator.clipboard.writeText(txt).then(() => {
    showToast('已成功复制到剪贴板！')
  }).catch(() => {
    showToast('复制失败，请手动选择复制', 'error')
  })
}

function extByMIME(mime: string) {
  const map: Record<string, string> = {
    'image/jpeg': '.jpg', 'image/png': '.png', 'image/webp': '.webp', 'image/gif': '.gif',
    'image/svg+xml': '.svg', 'image/avif': '.avif', 'image/heic': '.heic', 'image/x-icon': '.ico', 'image/vnd.microsoft.icon': '.ico',
    'video/mp4': '.mp4', 'video/webm': '.webm', 'video/quicktime': '.mov', 'video/x-matroska': '.mkv',
    'audio/mpeg': '.mp3', 'audio/ogg': '.ogg', 'audio/wav': '.wav',
    'audio/aac': '.aac', 'audio/flac': '.flac', 'audio/mp4': '.m4a',
  }
  return map[mime] || ''
}

function downloadMediaAsset(asset: any) {
  const ext = extByMIME(asset.mimeType)
  const a = document.createElement('a')
  a.href = asset.publicUrl
  a.download = asset.mediaId + ext
  a.click()
}

function formatBytes(bytes: number) {
  if (!bytes || bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

function formatDate(dateStr: string) {
  if (!dateStr) return '--'
  try {
    const d = new Date(dateStr)
    return d.toLocaleString('zh-CN', { hour12: false })
  } catch (e) {
    return dateStr
  }
}

// Watch props trigger from App component
watch(() => props.triggerUpload, (val) => {
  if (val) {
    openUploadDialog()
    emit('uploadHandled')
  }
})

onMounted(() => {
  fetchAssets()
})
</script>

<template>
  <div class="panel-view active" id="panel_explorer">
    <!-- Filters Card -->
    <div class="m3-card media-filter-bar">
      <div class="filter-inputs">
        <input class="filter-input search-field" v-model="searchQueryInput" placeholder="搜索资源 ID 或 MIME 格式..." />
        <select class="filter-input" v-model="selectedProject">
          <option value="">全部项目 (Projects)</option>
          <option v-for="p in projectsList" :key="p" :value="p">{{ p }}</option>
        </select>
        <select class="filter-input" v-model="selectedUsage">
          <option value="">全部用途 (Usages)</option>
          <option v-for="u in usagesList" :key="u" :value="u">{{ u }}</option>
        </select>
        <label style="display: flex; align-items: center; gap: 8px; font-size: 13px; cursor: pointer; user-select: none; color: hsl(var(--md-sys-color-on-surface-variant));">
          <input type="checkbox" v-model="showDeleted" style="accent-color: hsl(var(--md-sys-color-primary));" />
          <span>显示已删除资源</span>
        </label>
      </div>

      <div style="display: flex; gap: 12px; align-items: center;">
        <div class="view-toggle-btns">
          <button :class="['view-toggle-btn', { active: layoutMode === 'grid' }]" @click="setExplorerLayout('grid')">
            <span class="material-symbols-rounded" style="font-size: 18px;">grid_view</span>
            <span>网格</span>
          </button>
          <button :class="['view-toggle-btn', { active: layoutMode === 'list' }]" @click="setExplorerLayout('list')">
            <span class="material-symbols-rounded" style="font-size: 18px;">format_list_bulleted</span>
            <span>列表</span>
          </button>
        </div>

        <button class="m3-btn m3-btn-secondary m3-btn-sm" @click="fetchAssets">
          <span class="material-symbols-rounded" style="font-size: 16px;">refresh</span>
          <span>刷新</span>
        </button>
      </div>
    </div>

    <!-- Assets Render Area: Grid -->
    <div v-if="layoutMode === 'grid' && filteredAssets.length > 0" class="media-grid">
      <div v-for="asset in filteredAssets" :key="asset.mediaId" class="media-card">
        <div class="media-thumb" @click="openDetailSheet(asset.mediaId)" style="cursor: pointer;">
          <img v-if="asset.mimeType.startsWith('image/') && asset.mimeType !== 'image/heic' && asset.status === 'active'" :src="asset.publicUrl" alt="preview" loading="lazy" />
          <div v-else-if="asset.mimeType === 'image/heic' && asset.status === 'active'" style="display:flex; flex-direction:column; align-items:center; justify-content:center; height:100%; gap:4px; padding:12px;">
            <span class="material-symbols-rounded" style="font-size: 36px; color: #fbbf24;">image</span>
            <span style="font-size: 10px; color: hsl(var(--md-sys-color-on-surface-variant));">HEIC 原图</span>
          </div>
          <video v-else-if="asset.mimeType.startsWith('video/') && asset.status === 'active'" :src="asset.publicUrl" preload="metadata" muted style="width:100%; height:100%; object-fit:cover;"></video>
          <div v-else-if="asset.mimeType.startsWith('audio/') && asset.status === 'active'" style="display:flex; flex-direction:column; align-items:center; justify-content:center; height:100%; gap:8px; padding:12px;">
            <span class="material-symbols-rounded" style="font-size: 36px; color: hsl(var(--md-sys-color-primary));">music_note</span>
            <audio :src="asset.publicUrl" controls style="width:100%; max-width:200px;"></audio>
          </div>
          <span v-else class="material-symbols-rounded file-icon">description</span>

          <span v-if="asset.mimeType.startsWith('image/')" class="media-type-icon media-type-image">
            <span class="material-symbols-rounded" style="font-size: 14px;">image</span>
          </span>
          <span v-else-if="asset.mimeType.startsWith('video/')" class="media-type-icon media-type-video">
            <span class="material-symbols-rounded" style="font-size: 14px;">play_arrow</span>
          </span>
          <span v-else-if="asset.mimeType.startsWith('audio/')" class="media-type-icon" style="background: rgba(168,85,247,0.25); color: #d8b4fe;">
            <span class="material-symbols-rounded" style="font-size: 14px;">headphones</span>
          </span>

          <span v-if="asset.isChunked" class="badge badge-primary" style="position: absolute; top: 8px; left: 8px;">分片上传</span>
        </div>
        <div class="media-card-info">
          <div class="media-id" :title="asset.mediaId">{{ asset.mediaId }}</div>
          <div class="media-meta-row">
            <span>{{ formatBytes(asset.sizeBytes) }}</span>
            <span>{{ formatDate(asset.createdAt).split(' ')[0] }}</span>
          </div>
          <div class="media-card-tags">
            <span class="badge">{{ asset.project }}</span>
            <span class="badge">{{ asset.usage }}</span>
            <span :class="['badge', asset.status === 'active' ? 'badge-success' : 'badge-error']">
              {{ asset.status === 'active' ? '活动' : '已删除' }}
            </span>
          </div>
        </div>
        <div class="media-actions">
          <button class="m3-btn m3-btn-secondary m3-btn-sm" style="flex: 1; padding: 6px 0;" @click="copyText(asset.publicUrl)">
            <span class="material-symbols-rounded" style="font-size: 14px;">link</span>
          </button>
          <button class="m3-btn m3-btn-secondary m3-btn-sm" style="flex: 1; padding: 6px 0;" @click="openDetailSheet(asset.mediaId)">
            <span class="material-symbols-rounded" style="font-size: 14px;">visibility</span>
          </button>
          <button v-if="asset.status === 'active'" class="m3-btn m3-btn-danger m3-btn-sm" style="flex: 1; padding: 6px 0;" @click="deleteAsset(asset.mediaId)">
            <span class="material-symbols-rounded" style="font-size: 14px;">delete</span>
          </button>
        </div>
      </div>
    </div>

    <!-- Assets Render Area: List -->
    <div v-else-if="layoutMode === 'list' && filteredAssets.length > 0" class="m3-table-wrapper">
      <table class="m3-table">
        <thead>
          <tr>
            <th style="width: 60px;">预览</th>
            <th>ID</th>
            <th>类型 (MIME)</th>
            <th>大小</th>
            <th>项目/用途</th>
            <th>状态</th>
            <th style="text-align: right;">操作</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="asset in filteredAssets" :key="asset.mediaId">
            <td>
              <div style="width: 40px; height: 40px; border-radius: 8px; background: rgba(0,0,0,0.2); display: flex; align-items: center; justify-content: center; overflow: hidden;">
                <img v-if="asset.mimeType.startsWith('image/') && asset.mimeType !== 'image/heic' && asset.status === 'active'" :src="asset.publicUrl" style="width: 100%; height: 100%; object-fit: cover;" />
                <span v-else-if="asset.mimeType === 'image/heic' && asset.status === 'active'" class="material-symbols-rounded" style="font-size: 20px; color: #fbbf24;" title="HEIC 格式图片">image</span>
                <video v-else-if="asset.mimeType.startsWith('video/') && asset.status === 'active'" :src="asset.publicUrl" preload="metadata" muted style="width: 100%; height: 100%; object-fit: cover;"></video>
                <span v-else-if="asset.mimeType.startsWith('audio/')" class="material-symbols-rounded" style="font-size: 20px; color: #d8b4fe;">music_note</span>
                <span v-else class="material-symbols-rounded" style="font-size: 20px; color: hsl(var(--md-sys-color-primary));">description</span>
              </div>
            </td>
            <td>
              <span style="font-weight: 600; color: #fff; cursor: pointer;" @click="openDetailSheet(asset.mediaId)">{{ asset.mediaId }}</span>
            </td>
            <td style="color: hsl(var(--md-sys-color-on-surface-variant)); font-family: monospace;">{{ asset.mimeType }}</td>
            <td style="color: hsl(var(--md-sys-color-on-surface-variant));">{{ formatBytes(asset.sizeBytes) }}</td>
            <td>
              <span class="badge" style="margin-right: 4px;">{{ asset.project }}</span>
              <span class="badge">{{ asset.usage }}</span>
            </td>
            <td>
              <span :class="['badge', asset.status === 'active' ? 'badge-success' : 'badge-error']">
                {{ asset.status === 'active' ? '活动' : '已删除' }}
              </span>
            </td>
            <td style="text-align: right;">
              <div style="display: inline-flex; gap: 6px;">
                <button class="m3-btn m3-btn-secondary m3-btn-sm" @click="copyText(asset.publicUrl)">复制链接</button>
                <button class="m3-btn m3-btn-secondary m3-btn-sm" @click="openDetailSheet(asset.mediaId)">详情</button>
                <button v-if="asset.status === 'active'" class="m3-btn m3-btn-danger m3-btn-sm" @click="deleteAsset(asset.mediaId)">删除</button>
              </div>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    
    <!-- Empty State -->
    <div v-else style="text-align: center; padding: 80px 0; color: hsl(var(--md-sys-color-on-surface-variant));">
      <span class="material-symbols-rounded" style="font-size: 64px; color: rgba(255,255,255,0.08); margin-bottom: 16px;">folder_off</span>
      <p style="font-size: 15px;">未检索到符合条件的媒体资源文件</p>
    </div>

    <!-- Pagination bar -->
    <div v-if="totalCount > 0" class="m3-card" style="display: flex; flex-wrap: wrap; justify-content: space-between; align-items: center; gap: 16px; margin-top: 24px; padding: 12px 24px; border-radius: 16px; background: rgba(19, 27, 32, 0.4); border: 1px solid rgba(255, 255, 255, 0.04); margin-bottom: 16px;">
      <div style="font-size: 13px; color: hsl(var(--md-sys-color-on-surface-variant)); display: flex; align-items: center; gap: 12px;">
        <span>共 <strong>{{ totalCount }}</strong> 条数据</span>
        <span style="color: rgba(255,255,255,0.15)">|</span>
        <div style="display: flex; align-items: center; gap: 6px;">
          <span>每页</span>
          <select v-model="pageSize" style="background: rgba(255,255,255,0.05); border: 1px solid rgba(255,255,255,0.08); border-radius: 8px; color: #fff; padding: 4px 8px; font-size: 12px; outline: none; cursor: pointer;">
            <option :value="10">10</option>
            <option :value="20">20</option>
            <option :value="50">50</option>
          </select>
          <span>条</span>
        </div>
      </div>
      
      <div style="display: flex; align-items: center; gap: 8px;">
        <button class="m3-btn m3-btn-secondary m3-btn-sm" style="padding: 6px 8px; min-width: auto; border-radius: 8px; display: inline-flex; align-items: center;" :disabled="currentPage === 1" @click="firstPage" title="第一页">
          <span class="material-symbols-rounded" style="font-size: 18px;">first_page</span>
        </button>
        <button class="m3-btn m3-btn-secondary m3-btn-sm" style="padding: 6px 8px; min-width: auto; border-radius: 8px; display: inline-flex; align-items: center;" :disabled="currentPage === 1" @click="prevPage" title="上一页">
          <span class="material-symbols-rounded" style="font-size: 18px;">chevron_left</span>
        </button>
        
        <span style="font-size: 13px; color: #fff; padding: 0 8px;">
          第 <strong>{{ currentPage }}</strong> / {{ totalPages || 1 }} 页
        </span>
        
        <button class="m3-btn m3-btn-secondary m3-btn-sm" style="padding: 6px 8px; min-width: auto; border-radius: 8px; display: inline-flex; align-items: center;" :disabled="currentPage >= totalPages" @click="nextPage" title="下一页">
          <span class="material-symbols-rounded" style="font-size: 18px;">chevron_right</span>
        </button>
        <button class="m3-btn m3-btn-secondary m3-btn-sm" style="padding: 6px 8px; min-width: auto; border-radius: 8px; display: inline-flex; align-items: center;" :disabled="currentPage >= totalPages" @click="lastPage" title="最后一页">
          <span class="material-symbols-rounded" style="font-size: 18px;">last_page</span>
        </button>
      </div>
    </div>

    <!-- Floating Action Button -->
    <button class="fab" id="uploadFab" @click="openUploadDialog">
      <span class="material-symbols-rounded" style="font-size: 32px;">add</span>
    </button>

    <!-- Dialog Modal: File Upload -->
    <div :class="['m3-dialog-overlay', { active: uploadDialogOpen }]" id="uploadDialogOverlay">
      <div class="m3-dialog">
        <div class="m3-dialog-header">
          <h3>📤 上传新媒体文件</h3>
          <button class="m3-dialog-close" @click="closeUploadDialog">&times;</button>
        </div>
        <div class="m3-dialog-body">
          <div class="upload-drop-zone"
               :class="{ 'drop-active': dragOver, 'has-file': selectedFileName }"
               @click="fileInputRef?.click()"
               @dragover.prevent="dragOver = true"
               @dragleave.prevent="dragOver = false"
               @drop.prevent="onDrop">
             <input ref="fileInputRef" type="file" accept="image/*,video/*,audio/*,.mp3,.ogg,.wav,.aac,.flac,.m4a,.pdf,.doc,.docx,.xls,.xlsx,.ppt,.pptx,.csv,.txt,.zip,.rar,.7z" @change="onFileSelected" style="display: none;" />
             <template v-if="!selectedFileName">
               <span class="material-symbols-rounded" style="font-size: 36px; color: hsl(var(--md-sys-color-primary)); margin-bottom: 8px;">cloud_upload</span>
               <p style="font-size: 14px; color: #fff; margin: 0;">点击或拖拽文件到此处</p>
               <p style="font-size: 12px; color: hsl(var(--md-sys-color-on-surface-variant)); margin: 4px 0 0;">{{ sizeLimitHint }}</p>
             </template>
             <template v-else>
               <span class="material-symbols-rounded" style="font-size: 28px; color: hsl(var(--md-sys-color-primary));">
                 {{ selectedFileKind === 'video' ? 'videocam' : selectedFileKind === 'audio' ? 'headphones' : selectedFileKind === 'file' ? 'description' : 'image' }}
               </span>
               <div style="flex: 1; min-width: 0; margin-left: 12px;">
                 <p style="font-size: 14px; color: #fff; margin: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;">{{ selectedFileName }}</p>
                 <p style="font-size: 12px; color: hsl(var(--md-sys-color-on-surface-variant)); margin: 2px 0 0;">
                   {{ selectedFileKind === 'image' ? '图片文件' : selectedFileKind === 'video' ? '视频文件' : selectedFileKind === 'audio' ? '音频文件' : '文档/归档文件' }}
                 </p>
               </div>
              <button class="m3-btn m3-btn-secondary m3-btn-sm" style="padding: 4px 8px; flex-shrink: 0;" @click.stop="clearSelectedFile">
                <span class="material-symbols-rounded" style="font-size: 16px;">close</span>
              </button>
            </template>
          </div>

          <template v-if="selectedFileKind">
            <div class="form-field">
              <label>所属项目 (Project)</label>
              <div class="input-wrapper">
                <input v-model="uploadProject" type="text" placeholder="如 myproject" />
              </div>
            </div>

            <div v-if="selectedFileKind === 'image'" class="form-field">
              <label>使用场景 (Usage)</label>
              <div class="input-wrapper">
                <select v-model="uploadUsage">
                  <option value="cover">cover (封面大图)</option>
                  <option value="scene">scene (场景/正片)</option>
                  <option value="avatar">avatar (头像/缩略图)</option>
                </select>
              </div>
            </div>

            <div v-if="selectedFileKind === 'audio'" class="form-field">
              <label>使用场景 (Usage)</label>
              <div class="input-wrapper">
                <select v-model="uploadUsage">
                  <option value="audio">audio (音频文件)</option>
                  <option value="scene">scene (场景音效)</option>
                </select>
              </div>
            </div>

            <div v-if="selectedFileKind === 'file'" class="form-field">
              <label>使用场景 (Usage)</label>
              <div class="input-wrapper">
                <select v-model="uploadUsage">
                  <option value="file">file (普通文档与归档文件)</option>
                </select>
              </div>
            </div>

            <div v-if="selectedFileKind === 'image'" class="form-field">
              <label style="display: flex; align-items: center; gap: 8px; cursor: pointer; user-select: none; color: hsl(var(--md-sys-color-on-surface-variant));">
                <input type="checkbox" v-model="uploadAutoWebp" style="accent-color: hsl(var(--md-sys-color-primary));" />
                <span>自动优化并转换为 WebP 格式（缩减体积，加速加载）</span>
              </label>
            </div>

            <div v-if="selectedFileKind === 'video'" class="form-field" style="font-size: 13px; color: hsl(var(--md-sys-color-on-surface-variant)); display: flex; align-items: center; gap: 6px;">
              <span>⚡ 大文件/长视频支持最高 2GB，网关将自动并发分片存储并优化流式播放</span>
            </div>
          </template>
        </div>
        <div class="m3-dialog-footer">
          <button class="m3-btn m3-btn-secondary" @click="closeUploadDialog">取消</button>
          <button class="m3-btn m3-btn-primary" @click="submitUploadFile">确认上传</button>
        </div>
      </div>
    </div>

    <!-- Detail Drawer Panel (Sheet) -->
    <div :class="['m3-sheet', { active: sheetActive }]" id="detailSheet">
      <div class="m3-sheet-header">
        <h3 style="font-size: 18px; font-weight: 600; color: #fff;">📁 媒体资源元数据</h3>
        <button class="m3-dialog-close" @click="closeDetailSheet">&times;</button>
      </div>
      <div class="m3-sheet-body" id="detailSheetBody" v-if="activeAsset">
        <div v-if="activeAsset.mimeType.startsWith('image/') && activeAsset.mimeType !== 'image/heic' && activeAsset.status === 'active'" style="width: 100%; height: 180px; border-radius: 16px; overflow: hidden; background: #000; border: 1px solid rgba(255,255,255,0.08);">
          <img :src="activeAsset.publicUrl" style="width:100%; height:100%; object-fit:contain;" />
        </div>
        <div v-else-if="activeAsset.mimeType === 'image/heic' && activeAsset.status === 'active'" style="width: 100%; height: 180px; border-radius: 16px; background: rgba(251,191,36,0.08); border: 1px solid rgba(251,191,36,0.2); display:flex; flex-direction:column; align-items:center; justify-content:center; gap:8px;">
          <span class="material-symbols-rounded" style="font-size: 48px; color: #fbbf24;">image</span>
          <span style="font-size: 13px; color: #fff;">HEIC 格式原图</span>
        </div>
        <div v-else-if="activeAsset.mimeType.startsWith('video/') && activeAsset.status === 'active'" style="width: 100%; border-radius: 16px; background: #000; border: 1px solid rgba(255,255,255,0.08); overflow: hidden;">
          <video :src="activeAsset.publicUrl" controls style="width:100%; max-height:320px; object-fit:contain;"></video>
        </div>
        <div v-else-if="activeAsset.mimeType.startsWith('audio/') && activeAsset.status === 'active'" style="width: 100%; border-radius: 16px; background: rgba(168,85,247,0.08); border: 1px solid rgba(168,85,247,0.2); padding: 24px; display:flex; flex-direction:column; align-items:center; gap:12px;">
          <span class="material-symbols-rounded" style="font-size: 48px; color: #d8b4fe;">music_note</span>
          <audio :src="activeAsset.publicUrl" controls style="width:100%;"></audio>
        </div>
        <div v-else-if="activeAsset.status === 'active'" style="width: 100%; height: 120px; border-radius: 16px; background: rgba(255,255,255,0.03); border: 1px solid rgba(255,255,255,0.08); display:flex; flex-direction:column; align-items:center; justify-content:center; gap:8px;">
          <span class="material-symbols-rounded" style="font-size: 48px; color: hsl(var(--md-sys-color-primary));">description</span>
          <span style="font-size: 13px; color: #fff;">文档/文件资源</span>
        </div>
        
        <div style="display: flex; flex-direction: column; gap: 14px; font-size: 13px; margin-top: 12px;">
          <div>
            <div style="color: hsl(var(--md-sys-color-primary)); font-weight:600; margin-bottom: 4px;">媒体 ID (Media ID)</div>
            <div style="font-family: monospace; color:#fff; word-break:break-all; background: rgba(0,0,0,0.2); padding: 8px; border-radius: 8px;">{{ activeAsset.mediaId }}</div>
          </div>
          <div>
            <div style="color: hsl(var(--md-sys-color-primary)); font-weight:600; margin-bottom: 4px;">公共访问地址 (Public Link)</div>
            <div style="font-family: monospace; color:#fff; word-break:break-all; background: rgba(0,0,0,0.2); padding: 8px; border-radius: 8px; font-size:11px;">{{ activeAsset.publicUrl }}</div>
          </div>
          <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 10px;">
            <div>
              <div style="color: hsl(var(--md-sys-color-primary)); font-weight:600; margin-bottom: 2px;">大小</div>
              <div style="color:#fff;">{{ formatBytes(activeAsset.sizeBytes) }}</div>
            </div>
            <div>
              <div style="color: hsl(var(--md-sys-color-primary)); font-weight:600; margin-bottom: 2px;">类型</div>
              <div style="color:#fff; font-family: monospace;">{{ activeAsset.mimeType }}</div>
            </div>
          </div>
          <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 10px;">
            <div>
              <div style="color: hsl(var(--md-sys-color-primary)); font-weight:600; margin-bottom: 2px;">项目</div>
              <div style="color:#fff;">{{ activeAsset.project }}</div>
            </div>
            <div>
              <div style="color: hsl(var(--md-sys-color-primary)); font-weight:600; margin-bottom: 2px;">用途</div>
              <div style="color:#fff;">{{ activeAsset.usage }}</div>
            </div>
          </div>
          <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 10px;">
            <div>
              <div style="color: hsl(var(--md-sys-color-primary)); font-weight:600; margin-bottom: 2px;">分片上传</div>
              <div style="color:#fff;">{{ activeAsset.isChunked ? '是' : '否' }}</div>
            </div>
            <div>
              <div style="color: hsl(var(--md-sys-color-primary)); font-weight:600; margin-bottom: 2px;">所属 Provider</div>
              <div style="color:#fff; text-transform: uppercase;">{{ activeAsset.provider }}</div>
            </div>
          </div>
          <div>
            <div style="color: hsl(var(--md-sys-color-primary)); font-weight:600; margin-bottom: 2px;">SHA-256</div>
            <div style="color:#fff; font-family: monospace; font-size:11px; word-break:break-all;">{{ activeAsset.sha256 || '暂无' }}</div>
          </div>
          <div>
            <div style="color: hsl(var(--md-sys-color-primary)); font-weight:600; margin-bottom: 2px;">入库时间</div>
            <div style="color:#fff;">{{ formatDate(activeAsset.createdAt) }}</div>
          </div>
        </div>
      </div>
      <div style="margin-top: 24px; display: flex; gap: 12px;" v-if="activeAsset">
        <button class="m3-btn m3-btn-primary m3-btn-sm" style="flex: 1;" @click="openMediaAsset(activeAsset.publicUrl)">在新标签页打开</button>
        <button class="m3-btn m3-btn-secondary m3-btn-sm" style="flex: 1;" @click="downloadMediaAsset(activeAsset)">下载</button>
        <button class="m3-btn m3-btn-secondary m3-btn-sm" style="flex: 1;" @click="copyText(activeAsset.publicUrl)">复制链接</button>
      </div>
    </div>
  </div>
</template>
