<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, watch, inject } from 'vue'
import { apiRequest, openMediaAsset, copyToClipboard } from '../api'
import type { MediaAsset } from '../api'
import FireflySelect from './FireflySelect.vue'

const showToast = inject<(msg: string, type?: 'success' | 'error') => void>('showToast', () => {})

const loading = ref(true)
const assets = ref<MediaAsset[]>([])

// Query Filters & Search Debounce
const searchQueryInput = ref('')
const searchKeyword = ref('')
const selectedProject = ref('')
const selectedUsage = ref('')
const mediaTypeFilter = ref('') // 'image' | 'video' | 'audio' | ''
const hideDummy = ref(true)

const projectsList = ref<string[]>([])
const usagesList = ref<string[]>([])

// Dropdown Options
const projectOptions = computed(() => [
  { label: '全部项目 (Projects)', value: '' },
  ...projectsList.value.map(p => ({ label: p, value: p }))
])

const usageOptions = computed(() => [
  { label: '全部用途 (Usages)', value: '' },
  ...usagesList.value.map(u => ({ label: u, value: u }))
])

const mediaTypeOptions = [
  { label: '全部类别', value: '' },
  { label: '图片类 (Image)', value: 'image', icon: 'image' },
  { label: '视频类 (Video)', value: 'video', icon: 'play_arrow' },
  { label: '音频类 (Audio)', value: 'audio', icon: 'headphones' },
  { label: '文档/文件类 (File)', value: 'file', icon: 'description' }
]

// Pagination
const PAGE_SIZE = 24
const offset = ref(0)
const totalCount = ref(0)
const hasMore = computed(() => assets.value.length < totalCount.value)

// Details dialog states
const activeAsset = ref<MediaAsset | null>(null)
const detailsOpen = ref(false)

// Track broken image URLs to display graceful fallback placeholders
const failedImages = ref<Record<string, boolean>>({})
function onImageError(mediaId: string) {
  failedImages.value[mediaId] = true
}

function retryImage(mediaId: string) {
  failedImages.value[mediaId] = false
  loadedImages.value[mediaId] = false
}

// Track successfully loaded images for silky smooth progressive fade-in
const loadedImages = ref<Record<string, boolean>>({})
function onImageLoad(mediaId: string) {
  loadedImages.value[mediaId] = true
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

async function fetchAssets(reset = false) {
  if (reset) {
    offset.value = 0
    assets.value = []
  }
  loading.value = true
  try {
    const params = new URLSearchParams()
    params.set('limit', String(PAGE_SIZE))
    params.set('offset', String(offset.value))
    params.set('hide_dummy', hideDummy.value ? 'true' : 'false')
    if (selectedProject.value) params.set('project', selectedProject.value)
    if (selectedUsage.value) params.set('usage', selectedUsage.value)
    if (searchKeyword.value) params.set('search', searchKeyword.value)
    if (mediaTypeFilter.value) params.set('type', mediaTypeFilter.value)

    interface MediaResponse {
      total: number
      limit: number
      offset: number
      items: MediaAsset[]
    }

    const data = await apiRequest<MediaResponse>(`/api/v1/media?${params.toString()}`)
    if (data && data.items) {
      if (reset) {
        assets.value = data.items
      } else {
        assets.value = [...assets.value, ...data.items]
      }
      totalCount.value = data.total || 0
    }
  } catch (err: any) {
    showToast(err.message || '拉取资源失败', 'error')
  } finally {
    loading.value = false
  }
}

function loadMore() {
  if (loading.value || !hasMore.value) return
  offset.value += PAGE_SIZE
  fetchAssets(false)
}

// Watch filters
watch([selectedProject, selectedUsage, mediaTypeFilter, searchKeyword, hideDummy], () => {
  fetchAssets(true)
})

// Since filtering is done server-side, filteredAssets is just the assets array
const filteredAssets = computed(() => assets.value)

function openDetails(asset: MediaAsset) {
  activeAsset.value = asset
  detailsOpen.value = true
}

function closeDetails() {
  detailsOpen.value = false
  activeAsset.value = null
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

// Copy Code Builders
const rawUrl = computed(() => activeAsset.value?.publicUrl || '')

const htmlCode = computed(() => {
  if (!activeAsset.value) return ''
  if (activeAsset.value.mimeType.startsWith('image/')) {
    return `<img src="${activeAsset.value.publicUrl}" alt="${activeAsset.value.mediaId}" />`
  }
  return `<video src="${activeAsset.value.publicUrl}" controls playsinline width="640" height="360"></video>`
})

const markdownCode = computed(() => {
  if (!activeAsset.value) return ''
  return `![${activeAsset.value.mediaId}](${activeAsset.value.publicUrl})`
})

const bbCode = computed(() => {
  if (!activeAsset.value) return ''
  return `[img]${activeAsset.value.publicUrl}[/img]`
})

async function copyText(txt: string, typeName: string) {
  const ok = await copyToClipboard(txt)
  if (ok) {
    showToast(`复制 ${typeName} 成功！`)
  } else {
    showToast('复制失败，请手动选择复制', 'error')
  }
}

function extByMIME(mime: string) {
  const map: Record<string, string> = {
    'image/jpeg': '.jpg', 'image/png': '.png', 'image/webp': '.webp', 'image/gif': '.gif',
    'image/svg+xml': '.svg', 'image/avif': '.avif', 'image/heic': '.heic', 'image/x-icon': '.ico', 'image/vnd.microsoft.icon': '.ico',
    'video/mp4': '.mp4', 'video/webm': '.webm', 'video/quicktime': '.mov', 'video/x-matroska': '.mkv',
    'audio/mpeg': '.mp3', 'audio/ogg': '.ogg', 'audio/wav': '.wav',
    'audio/aac': '.aac', 'audio/flac': '.flac', 'audio/mp4': '.m4a',
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

function downloadMediaAsset(asset: MediaAsset) {
  const ext = extByMIME(asset.mimeType)
  const a = document.createElement('a')
  a.href = asset.publicUrl
  a.download = asset.mediaId + ext
  a.click()
}

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && detailsOpen.value) {
    closeDetails()
  }
}

onMounted(() => {
  window.addEventListener('keydown', handleKeydown)
  fetchMetadata()
  fetchAssets(true)
})

onUnmounted(() => {
  window.removeEventListener('keydown', handleKeydown)
})
</script>

<template>
  <div class="gallery-view">
    <!-- Filter bar -->
    <div class="m3-card media-filter-bar">
      <div class="filter-inputs">
        <div class="search-input-wrapper">
          <span class="material-symbols-rounded search-icon">search</span>
          <input class="filter-input search-field" v-model="searchQueryInput" placeholder="搜索资源 ID 或项目标签..." />
          <button v-if="searchQueryInput" class="clear-search-btn" @click="searchQueryInput = ''" title="清空搜索">
            <span class="material-symbols-rounded">close</span>
          </button>
        </div>
        <FireflySelect v-model="selectedProject" :options="projectOptions" placeholder="全部项目" clearable min-width="160px" />
        <FireflySelect v-model="selectedUsage" :options="usageOptions" placeholder="全部用途" clearable min-width="160px" />
        <FireflySelect v-model="mediaTypeFilter" :options="mediaTypeOptions" placeholder="全部类别" clearable min-width="150px" />
        <button 
          :class="['filter-toggle-pill', { active: hideDummy }]" 
          @click="hideDummy = !hideDummy" 
          :title="hideDummy ? '已过滤 <100B 占位透明图，点击显示全量' : '当前展示全量，点击过滤占位透明图'"
        >
          <span class="material-symbols-rounded toggle-icon" style="font-size: 16px;">{{ hideDummy ? 'filter_alt' : 'filter_alt_off' }}</span>
          <span>{{ hideDummy ? '已过滤占位图' : '全量展示' }}</span>
        </button>
      </div>

      <button class="m3-btn m3-btn-secondary m3-btn-sm refresh-btn" @click="fetchAssets(true)" :disabled="loading">
        <span :class="['material-symbols-rounded', { 'rotate-sync': loading }]" style="font-size: 16px;">refresh</span>
        <span>刷新列表</span>
      </button>
    </div>

    <!-- Masonry Grid -->
    <div v-if="loading" class="gallery-loading">
      <span class="material-symbols-rounded rotate-sync">sync</span>
      <p>正在获取公开图库，请稍候...</p>
    </div>

    <div v-else-if="filteredAssets.length > 0" class="masonry-wrapper">
      <div v-for="asset in filteredAssets" :key="asset.mediaId" class="masonry-card" @click="openDetails(asset)">
        <div class="masonry-thumb">
          <!-- Skeleton Shimmer Placeholder while loading -->
          <div 
            v-if="asset.mimeType.startsWith('image/') && asset.mimeType !== 'image/heic' && asset.sizeBytes > 100 && !loadedImages[asset.mediaId] && !failedImages[asset.mediaId]" 
            class="thumb-skeleton"
          >
            <div class="shimmer-sweep"></div>
            <span class="material-symbols-rounded skeleton-icon">image</span>
          </div>

          <!-- Progressive Smooth Loading Image (> 100 bytes) -->
          <img 
            v-if="asset.mimeType.startsWith('image/') && asset.mimeType !== 'image/heic' && asset.sizeBytes > 100 && !failedImages[asset.mediaId]" 
            :src="asset.publicUrl" 
            :class="['masonry-img', { 'is-loaded': loadedImages[asset.mediaId] }]"
            alt="preview" 
            loading="lazy"
            @load="onImageLoad(asset.mediaId)"
            @error="onImageError(asset.mediaId)" 
          />
          <!-- 1x1 / Empty Dummy Placeholder Image (e.g. 67B transparent PNGs) -->
          <div v-else-if="asset.mimeType.startsWith('image/') && asset.sizeBytes <= 100" class="thumb-fallback thumb-dummy">
            <span class="material-symbols-rounded fallback-icon" style="color: rgba(0, 229, 255, 0.45);">crop_square</span>
            <span class="fallback-text">占位图 ({{ formatBytes(asset.sizeBytes) }})</span>
            <span class="fallback-sub">1x1 占位透明图</span>
          </div>
          <!-- Broken / Network Error Fallback -->
          <div v-else-if="failedImages[asset.mediaId]" class="thumb-fallback thumb-error">
            <span class="material-symbols-rounded fallback-icon">broken_image</span>
            <span class="fallback-text">预览图加载失败</span>
            <span class="fallback-sub">({{ formatBytes(asset.sizeBytes) }})</span>
            <button class="retry-load-btn" @click.stop="retryImage(asset.mediaId)" title="重新加载">
              <span class="material-symbols-rounded" style="font-size: 13px;">refresh</span>
              <span>重试</span>
            </button>
          </div>
          <!-- HEIC -->
          <div v-else-if="asset.mimeType === 'image/heic'" class="thumb-fallback thumb-heic">
            <span class="material-symbols-rounded fallback-icon" style="color: #fbbf24;">image</span>
            <span class="fallback-text">HEIC 格式原图</span>
          </div>
          <!-- Video -->
          <video v-else-if="asset.mimeType.startsWith('video/')" :src="asset.publicUrl" preload="metadata" muted playsinline></video>
          <!-- Audio -->
          <div v-else-if="asset.mimeType.startsWith('audio/')" class="thumb-fallback thumb-audio">
            <span class="material-symbols-rounded fallback-icon" style="color: #d8b4fe;">music_note</span>
            <audio :src="asset.publicUrl" controls style="width:100%; max-width:240px;"></audio>
          </div>
          <!-- Other File Types -->
          <div v-else class="thumb-fallback">
            <span class="material-symbols-rounded fallback-icon" style="color: hsl(var(--md-sys-color-primary));">description</span>
            <span class="fallback-text">{{ asset.mimeType || '未知格式' }}</span>
          </div>

          <span v-if="asset.mimeType.startsWith('image/')" class="media-badge media-type-image">
            <span class="material-symbols-rounded" style="font-size: 13px;">image</span>
          </span>
          <span v-else-if="asset.mimeType.startsWith('video/')" class="media-badge media-type-video">
            <span class="material-symbols-rounded" style="font-size: 13px;">play_arrow</span>
          </span>
          <span v-else-if="asset.mimeType.startsWith('audio/')" class="media-badge" style="background: rgba(168,85,247,0.3); color: #d8b4fe;">
            <span class="material-symbols-rounded" style="font-size: 13px;">headphones</span>
          </span>
        </div>
        <div class="masonry-info">
          <div class="masonry-title" :title="asset.mediaId">
            <span class="title-hash">{{ asset.mediaId.slice(0, 10) }}...{{ asset.mediaId.slice(-6) }}</span>
            <button class="title-copy-btn" @click.stop="copyText(asset.mediaId, '资源 ID')" title="复制资源 ID">
              <span class="material-symbols-rounded">content_copy</span>
            </button>
          </div>
          <div class="masonry-meta">
            <span class="meta-pill size-pill">{{ formatBytes(asset.sizeBytes) }}</span>
            <span v-if="asset.usage" class="meta-pill usage-pill">{{ asset.usage }}</span>
            <span class="meta-pill project-pill">{{ asset.project }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Load More Button -->
    <div v-if="hasMore" style="display: flex; justify-content: center; margin-top: 32px; margin-bottom: 24px;">
      <button class="m3-btn m3-btn-primary" @click="loadMore" :disabled="loading" style="padding: 10px 24px; border-radius: 100px; display: flex; align-items: center; gap: 8px; font-weight: 600; box-shadow: var(--md-sys-elevation-2);">
        <span v-if="loading" class="material-symbols-rounded rotate-sync" style="font-size: 20px;">sync</span>
        <span v-else class="material-symbols-rounded" style="font-size: 20px;">expand_more</span>
        <span>加载更多 (已展示 {{ assets.length }} / 共 {{ totalCount }} 条)</span>
      </button>
    </div>

    <div v-else class="gallery-empty">
      <span class="material-symbols-rounded" style="font-size: 64px; color: rgba(255,255,255,0.08); margin-bottom: 16px;">folder_off</span>
      <p>图库目前为空或未检索到符合条件的公开文件</p>
    </div>

    <!-- Media Share Panel Dialog -->
    <div :class="['m3-dialog-overlay', { active: detailsOpen }]" @click.self="closeDetails">
      <div class="m3-dialog share-dialog" v-if="activeAsset">
        <div class="m3-dialog-header">
          <h3>🔗 分享面板 / 元数据</h3>
          <button class="m3-dialog-close" @click="closeDetails">&times;</button>
        </div>
        <div class="m3-dialog-body share-dialog-body">
          <div class="share-preview">
            <img v-if="activeAsset.mimeType.startsWith('image/')" :src="activeAsset.publicUrl" alt="Preview" />
            <video v-else-if="activeAsset.mimeType.startsWith('video/')" :src="activeAsset.publicUrl" controls playsinline></video>
          </div>

          <!-- Sharing Links Blocks -->
          <div class="share-links-list">
            <div class="share-field">
              <label>直链地址 (Raw Link)</label>
              <div class="copy-box">
                <input type="text" readonly :value="rawUrl" />
                <button class="m3-btn m3-btn-primary m3-btn-sm" @click="copyText(rawUrl, '直链')">复制</button>
              </div>
            </div>

            <div class="share-field">
              <label>Markdown 代码</label>
              <div class="copy-box">
                <input type="text" readonly :value="markdownCode" />
                <button class="m3-btn m3-btn-primary m3-btn-sm" @click="copyText(markdownCode, 'Markdown')">复制</button>
              </div>
            </div>

            <div class="share-field">
              <label>HTML 代码</label>
              <div class="copy-box">
                <input type="text" readonly :value="htmlCode" />
                <button class="m3-btn m3-btn-primary m3-btn-sm" @click="copyText(htmlCode, 'HTML')">复制</button>
              </div>
            </div>

            <div class="share-field">
              <label>BBCode 论坛代码</label>
              <div class="copy-box">
                <input type="text" readonly :value="bbCode" />
                <button class="m3-btn m3-btn-primary m3-btn-sm" @click="copyText(bbCode, 'BBCode')">复制</button>
              </div>
            </div>
          </div>

          <!-- Metadata table info -->
          <div class="share-metadata-table">
            <div class="meta-row">
              <span class="meta-label">文件大小</span>
              <span class="meta-value">{{ formatBytes(activeAsset.sizeBytes) }}</span>
            </div>
            <div class="meta-row">
              <span class="meta-label">MIME 类型</span>
              <span class="meta-value">{{ activeAsset.mimeType }}</span>
            </div>
            <div class="meta-row">
              <span class="meta-label">所属项目</span>
              <span class="meta-value">{{ activeAsset.project }}</span>
            </div>
            <div class="meta-row">
              <span class="meta-label">存储源</span>
              <span class="meta-value" style="text-transform: uppercase;">{{ activeAsset.provider }}</span>
            </div>
            <div class="meta-row">
              <span class="meta-label">上传时间</span>
              <span class="meta-value">{{ formatDate(activeAsset.createdAt) }}</span>
            </div>
          </div>
        </div>
        <div class="m3-dialog-footer">
          <button class="m3-btn m3-btn-secondary" @click="openMediaAsset(activeAsset.publicUrl)">在新窗口打开</button>
          <button class="m3-btn m3-btn-secondary" @click="downloadMediaAsset(activeAsset)">立即下载</button>
          <button class="m3-btn m3-btn-primary" @click="closeDetails">关闭</button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.gallery-view {
  display: flex;
  flex-direction: column;
  gap: 24px;
  width: 100%;
}

.gallery-loading, .gallery-empty {
  text-align: center;
  padding: 80px 0;
  color: hsl(var(--md-sys-color-on-surface-variant));
}

.gallery-loading .rotate-sync {
  font-size: 48px;
  color: hsl(var(--md-sys-color-primary));
  animation: rotate 1.5s linear infinite;
  margin-bottom: 16px;
}

@keyframes rotate {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}

/* Masonry/Waterfall Grid using CSS Column */
.masonry-wrapper {
  column-count: 4;
  column-gap: 20px;
  width: 100%;
}

@media (max-width: 1200px) {
  .masonry-wrapper {
    column-count: 3;
  }
}

@media (max-width: 768px) {
  .masonry-wrapper {
    column-count: 2;
    column-gap: 12px;
  }
}

@media (max-width: 480px) {
  .masonry-wrapper {
    column-count: 2;
    column-gap: 10px;
  }
  .masonry-card {
    margin-bottom: 10px;
    border-radius: 12px;
  }
  .masonry-info {
    padding: 10px;
  }
  .masonry-title {
    font-size: 12px;
  }
}

@media (max-width: 330px) {
  .masonry-wrapper {
    column-count: 1;
  }
}

/* Search & Filter Bar Enhancements */
.search-input-wrapper {
  position: relative;
  display: flex;
  align-items: center;
  flex: 1;
  min-width: 240px;
}

.search-input-wrapper .search-icon {
  position: absolute;
  left: 12px;
  font-size: 18px;
  color: rgba(255, 255, 255, 0.4);
  pointer-events: none;
}

.search-input-wrapper .filter-input.search-field {
  padding-left: 38px;
  padding-right: 32px;
  width: 100%;
}

.search-input-wrapper .clear-search-btn {
  position: absolute;
  right: 10px;
  background: none;
  border: none;
  color: rgba(255, 255, 255, 0.4);
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 4px;
  border-radius: 50%;
  transition: all 0.15s ease;
}

.search-input-wrapper .clear-search-btn:hover {
  color: #fff;
  background: rgba(255, 255, 255, 0.1);
}

.search-input-wrapper .clear-search-btn .material-symbols-rounded {
  font-size: 14px;
}

.refresh-btn {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  border-radius: 10px;
  font-weight: 500;
  transition: all 0.2s ease;
}

.filter-toggle-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.1);
  color: rgba(255, 255, 255, 0.65);
  padding: 6px 14px;
  border-radius: 10px;
  font-size: 13px;
  font-weight: 500;
  cursor: pointer;
  transition: all 0.25s ease;
  user-select: none;
  white-space: nowrap;
}

.filter-toggle-pill:hover {
  background: rgba(0, 229, 255, 0.08);
  border-color: rgba(0, 229, 255, 0.3);
  color: #fff;
}

.filter-toggle-pill.active {
  background: rgba(0, 229, 255, 0.12);
  border-color: rgba(0, 229, 255, 0.4);
  color: hsl(var(--md-sys-color-primary));
  box-shadow: 0 0 12px -2px rgba(0, 229, 255, 0.2);
}

.filter-toggle-pill .toggle-icon {
  font-size: 16px;
}

.retry-load-btn {
  margin-top: 6px;
  display: inline-flex;
  align-items: center;
  gap: 4px;
  background: rgba(0, 229, 255, 0.1);
  border: 1px solid rgba(0, 229, 255, 0.25);
  color: hsl(var(--md-sys-color-primary));
  padding: 3px 10px;
  border-radius: 6px;
  font-size: 11px;
  cursor: pointer;
  transition: all 0.2s ease;
}

.retry-load-btn:hover {
  background: rgba(0, 229, 255, 0.2);
  border-color: rgba(0, 229, 255, 0.5);
  color: #fff;
}

.masonry-card {
  break-inside: avoid;
  background: linear-gradient(160deg, rgba(16, 25, 33, 0.75) 0%, rgba(8, 13, 17, 0.88) 100%);
  backdrop-filter: blur(20px);
  -webkit-backdrop-filter: blur(20px);
  border: 1px solid rgba(255, 255, 255, 0.07);
  border-radius: 16px;
  margin-bottom: 22px;
  overflow: hidden;
  cursor: pointer;
  position: relative;
  transition: transform 0.4s cubic-bezier(0.34, 1.4, 0.64, 1),
              box-shadow 0.4s ease,
              border-color 0.3s ease,
              background 0.3s ease;
  box-shadow: 0 8px 24px -6px rgba(0, 0, 0, 0.55), 
              inset 0 1px 1px 0 rgba(255, 255, 255, 0.08);
}

.masonry-card::before {
  content: '';
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  height: 1px;
  background: linear-gradient(90deg, transparent, rgba(0, 229, 255, 0.4), transparent);
  opacity: 0;
  transition: opacity 0.3s ease;
  z-index: 2;
}

.masonry-card:hover {
  transform: translateY(-8px) scale(1.015);
  border-color: rgba(0, 229, 255, 0.45);
  background: linear-gradient(160deg, rgba(22, 34, 44, 0.85) 0%, rgba(10, 16, 22, 0.94) 100%);
  box-shadow: 0 20px 45px -10px rgba(0, 0, 0, 0.75),
              0 0 32px -4px rgba(0, 229, 255, 0.25),
              inset 0 1px 1px 0 rgba(255, 255, 255, 0.2);
}

.masonry-card:hover::before {
  opacity: 1;
}

.masonry-thumb {
  position: relative;
  width: 100%;
  background: #06090d;
  min-height: 140px;
  max-height: 380px;
  overflow: hidden;
  display: flex;
  justify-content: center;
  align-items: center;
}

/* Skeleton Loading Shimmer */
.thumb-skeleton {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: linear-gradient(135deg, rgba(14, 22, 28, 0.9) 0%, rgba(8, 12, 16, 0.95) 100%);
  overflow: hidden;
  z-index: 1;
}

.shimmer-sweep {
  position: absolute;
  inset: 0;
  background: linear-gradient(
    90deg, 
    transparent 0%, 
    rgba(0, 229, 255, 0.08) 50%, 
    transparent 100%
  );
  transform: translateX(-100%);
  animation: fireflyShimmer 1.8s infinite cubic-bezier(0.4, 0, 0.2, 1);
}

@keyframes fireflyShimmer {
  0% { transform: translateX(-100%); }
  100% { transform: translateX(100%); }
}

.skeleton-icon {
  font-size: 32px;
  color: rgba(0, 229, 255, 0.22);
  animation: fireflyPulse 2.2s infinite ease-in-out;
}

@keyframes fireflyPulse {
  0%, 100% { opacity: 0.3; transform: scale(0.95); }
  50% { opacity: 0.75; transform: scale(1.05); }
}

.masonry-img {
  width: 100%;
  height: auto;
  min-height: 140px;
  object-fit: contain;
  display: block;
  opacity: 0;
  transform: scale(1.03);
  filter: blur(8px);
  transition: opacity 0.5s ease-out, transform 0.6s cubic-bezier(0.16, 1, 0.3, 1), filter 0.5s ease-out;
}

.masonry-img.is-loaded {
  opacity: 1;
  transform: scale(1);
  filter: blur(0);
}

.masonry-thumb video {
  width: 100%;
  height: auto;
  min-height: 140px;
  object-fit: contain;
  display: block;
}

/* Fallback / Broken Placeholder */
.thumb-fallback {
  width: 100%;
  min-height: 140px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 6px;
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.02) 0%, rgba(0, 0, 0, 0.25) 100%);
  padding: 24px 12px;
  user-select: none;
}

.thumb-fallback .fallback-icon {
  font-size: 36px;
  color: rgba(255, 255, 255, 0.25);
}

.thumb-fallback .fallback-text {
  font-size: 11px;
  font-weight: 500;
  color: rgba(255, 255, 255, 0.5);
  letter-spacing: 0.3px;
}

.thumb-fallback .fallback-sub {
  font-size: 10px;
  color: rgba(255, 255, 255, 0.3);
}

.thumb-fallback.thumb-heic {
  background: linear-gradient(180deg, rgba(251, 191, 36, 0.05) 0%, rgba(0, 0, 0, 0.25) 100%);
}

.thumb-fallback.thumb-audio {
  background: linear-gradient(180deg, rgba(168, 85, 247, 0.05) 0%, rgba(0, 0, 0, 0.25) 100%);
}

.media-badge {
  position: absolute;
  bottom: 8px;
  right: 8px;
  width: 24px;
  height: 24px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
  backdrop-filter: blur(6px);
}
.media-type-image {
  background: rgba(255, 255, 255, 0.15);
  color: rgba(255, 255, 255, 0.8);
}
.media-type-video {
  background: rgba(255, 255, 255, 0.2);
  color: #fff;
}

.masonry-info {
  padding: 12px 14px;
  text-align: left;
}

.masonry-title {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  font-size: 12px;
  font-weight: 600;
  color: #fff;
  font-family: 'JetBrains Mono', monospace;
}

.title-hash {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  letter-spacing: -0.2px;
}

.title-copy-btn {
  background: none;
  border: none;
  color: rgba(255, 255, 255, 0.3);
  padding: 2px 4px;
  border-radius: 4px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.15s ease;
  flex-shrink: 0;
}

.title-copy-btn:hover {
  color: hsl(var(--md-sys-color-primary));
  background: rgba(0, 229, 255, 0.1);
}

.title-copy-btn .material-symbols-rounded {
  font-size: 14px;
}

.masonry-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: center;
  margin-top: 8px;
}

.meta-pill {
  display: inline-flex;
  align-items: center;
  padding: 2px 8px;
  border-radius: 6px;
  font-size: 10px;
  font-weight: 500;
  background: rgba(255, 255, 255, 0.05);
  color: rgba(255, 255, 255, 0.65);
  border: 1px solid rgba(255, 255, 255, 0.06);
}

.size-pill {
  font-family: 'JetBrains Mono', monospace;
  color: rgba(255, 255, 255, 0.75);
}

.project-pill {
  background: rgba(0, 229, 255, 0.08);
  color: hsl(var(--md-sys-color-primary));
  border-color: rgba(0, 229, 255, 0.2);
}

.usage-pill {
  background: rgba(168, 85, 247, 0.08);
  color: #d8b4fe;
  border-color: rgba(168, 85, 247, 0.2);
}

/* Share Dialog specifics */
.share-dialog {
  max-width: 600px;
  width: 90%;
}

.share-dialog-body {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.share-preview {
  width: 100%;
  max-height: 240px;
  background: #000;
  border-radius: 12px;
  overflow: hidden;
  display: flex;
  justify-content: center;
  align-items: center;
}

.share-preview img, .share-preview video {
  max-width: 100%;
  max-height: 240px;
  object-fit: contain;
}

.share-links-list {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.share-field {
  display: flex;
  flex-direction: column;
  gap: 4px;
  text-align: left;
}

.share-field label {
  font-size: 11px;
  font-weight: 600;
  color: hsl(var(--md-sys-color-primary));
  text-transform: uppercase;
}

.copy-box {
  display: flex;
  gap: 8px;
}

.copy-box input {
  flex: 1;
  background: rgba(255, 255, 255, 0.04);
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 8px;
  color: #fff;
  padding: 8px 12px;
  font-size: 12px;
  outline: none;
  font-family: 'JetBrains Mono', monospace;
}

.share-metadata-table {
  border-top: 1px solid rgba(255, 255, 255, 0.06);
  padding-top: 16px;
  display: flex;
  flex-direction: column;
  gap: 8px;
  font-size: 12px;
}

.meta-row {
  display: flex;
  justify-content: space-between;
}

.meta-label {
  color: hsl(var(--md-sys-color-on-surface-variant));
}

.meta-value {
  color: #fff;
  font-weight: 500;
}
</style>
