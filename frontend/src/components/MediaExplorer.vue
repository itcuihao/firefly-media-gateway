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

// Query Filters
const searchKeyword = ref('')
const selectedProject = ref('')
const selectedUsage = ref('')
const showDeleted = ref(false)

// Details Drawer state
const sheetActive = ref(false)
const activeAsset = ref<MediaAsset | null>(null)

// Upload Modal state
const uploadDialogOpen = ref(false)
const uploadProject = ref('interactive-video')
const uploadUsage = ref('cover')
const uploadIsMember = ref(false)
const uploadAutoWebp = ref(true)
const fileInputRef = ref<HTMLInputElement | null>(null)

// Computed dynamic options for filters based on loaded media list
const projectsList = computed(() => {
  const projects = new Set<string>()
  assets.value.forEach(asset => {
    if (asset.project) projects.add(asset.project)
  })
  return Array.from(projects)
})

const usagesList = computed(() => {
  const usages = new Set<string>()
  assets.value.forEach(asset => {
    if (asset.usage) usages.add(asset.usage)
  })
  return Array.from(usages)
})

// Filter Logic
const filteredAssets = computed(() => {
  const keyword = searchKeyword.value.trim().toLowerCase()
  return assets.value.filter(asset => {
    // Keyword check
    const matchesKeyword = !keyword || 
      asset.mediaId.toLowerCase().includes(keyword) || 
      (asset.mimeType && asset.mimeType.toLowerCase().includes(keyword)) ||
      (asset.project && asset.project.toLowerCase().includes(keyword))

    // Project check
    const matchesProj = !selectedProject.value || asset.project === selectedProject.value
    // Usage check
    const matchesUsage = !selectedUsage.value || asset.usage === selectedUsage.value
    // Status check
    const matchesStatus = showDeleted.value || asset.status === 'active'

    return matchesKeyword && matchesProj && matchesUsage && matchesStatus
  })
})

async function fetchAssets() {
  loading.value = true
  try {
    const data = await apiRequest<MediaAsset[]>('/api/v1/media?limit=100')
    assets.value = data
  } catch (err: any) {
    showToast(err.message || '拉取资源失败', 'error')
  } finally {
    loading.value = false
  }
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
  form.append('member', uploadIsMember.value ? 'true' : 'false')

  showToast('正在上传，请耐心等待...', 'success')
  closeUploadDialog()

  try {
    await apiRequest('/api/v1/media/upload', {
      method: 'POST',
      body: form
    })
    showToast('媒体文件上传成功！')
    fetchAssets()
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
        <input class="filter-input search-field" v-model="searchKeyword" placeholder="搜索资源 ID 或 MIME 格式..." />
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
          <img v-if="asset.mimeType.startsWith('image/') && asset.status === 'active'" :src="asset.publicUrl" alt="preview" loading="lazy" />
          <span v-else-if="asset.mimeType.startsWith('video/') && asset.status === 'active'" class="material-symbols-rounded file-icon" style="color: hsl(var(--md-sys-color-secondary));">video_library</span>
          <span v-else class="material-symbols-rounded file-icon">description</span>
          
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
                <img v-if="asset.mimeType.startsWith('image/') && asset.status === 'active'" :src="asset.publicUrl" style="width: 100%; height: 100%; object-fit: cover;" />
                <span v-else class="material-symbols-rounded" style="font-size: 20px; color: hsl(var(--md-sys-color-primary));">
                  {{ asset.mimeType.startsWith('video/') ? 'video_library' : 'description' }}
                </span>
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
          <p style="font-size: 13px; color: hsl(var(--md-sys-color-on-surface-variant)); margin-bottom: 20px;">
            文件最大支持限制：图片类最大 10MB (jpg/png/webp)，视频类最大 120MB (mp4/webm/mov)。
          </p>

          <div class="form-field">
            <label>所属项目 (Project)</label>
            <div class="input-wrapper">
              <input v-model="uploadProject" type="text" placeholder="如 myproject" />
            </div>
          </div>

          <div class="form-field">
            <label>使用场景 (Usage)</label>
            <div class="input-wrapper">
              <select v-model="uploadUsage">
                <option value="cover">cover (封面大图)</option>
                <option value="scene">scene (场景/正片)</option>
                <option value="avatar">avatar (头像/缩略图)</option>
              </select>
            </div>
          </div>

          <div class="form-field">
            <label style="display: flex; align-items: center; gap: 8px; cursor: pointer; user-select: none; color: hsl(var(--md-sys-color-on-surface-variant));">
              <input type="checkbox" v-model="uploadIsMember" style="accent-color: hsl(var(--md-sys-color-primary));" />
              <span>是否启用大文件分片上传 (需要会员身份)</span>
            </label>
          </div>

          <div class="form-field" style="margin-top: 8px;">
            <label style="display: flex; align-items: center; gap: 8px; cursor: pointer; user-select: none; color: hsl(var(--md-sys-color-on-surface-variant));">
              <input type="checkbox" v-model="uploadAutoWebp" style="accent-color: hsl(var(--md-sys-color-primary));" />
              <span>自动优化图片并转换为 WebP 格式（缩减文件体积，加速网页加载）</span>
            </label>
          </div>

          <div class="form-field" style="margin-top: 16px;">
            <label>选择媒体文件</label>
            <div class="input-wrapper">
              <input ref="fileInputRef" type="file" accept="image/*,video/*" />
            </div>
          </div>
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
        <div v-if="activeAsset.mimeType.startsWith('image/') && activeAsset.status === 'active'" style="width: 100%; height: 180px; border-radius: 16px; overflow: hidden; background: #000; border: 1px solid rgba(255,255,255,0.08);">
          <img :src="activeAsset.publicUrl" style="width:100%; height:100%; object-fit:contain;" />
        </div>
        <video v-else-if="activeAsset.mimeType.startsWith('video/') && activeAsset.status === 'active'" :src="activeAsset.publicUrl" controls style="width:100%; height:180px; border-radius: 16px; background: #000; border: 1px solid rgba(255,255,255,0.08); object-fit:contain;"></video>
        
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
        <button class="m3-btn m3-btn-secondary m3-btn-sm" style="flex: 1;" @click="copyText(activeAsset.publicUrl)">复制链接</button>
      </div>
    </div>
  </div>
</template>
