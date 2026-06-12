<script setup lang="ts">
import { ref, onMounted, computed, inject } from 'vue'
import { apiRequest, openMediaAsset } from '../api'
import type { MediaAsset } from '../api'

const showToast = inject<(msg: string, type?: 'success' | 'error') => void>('showToast', () => {})

const loading = ref(true)
const assets = ref<MediaAsset[]>([])

// Query Filters
const searchKeyword = ref('')
const selectedProject = ref('')
const selectedUsage = ref('')
const mediaTypeFilter = ref('') // 'image' | 'video' | ''

// Details dialog states
const activeAsset = ref<MediaAsset | null>(null)
const detailsOpen = ref(false)

async function fetchAssets() {
  loading.value = true
  try {
    const data = await apiRequest<MediaAsset[]>('/api/v1/media?limit=100')
    assets.value = data || []
  } catch (err: any) {
    showToast(err.message || '拉取资源失败', 'error')
  } finally {
    loading.value = false
  }
}

// Populate filters dynamically based on loaded assets
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

// Filter assets
const filteredAssets = computed(() => {
  const keyword = searchKeyword.value.trim().toLowerCase()
  return assets.value.filter(asset => {
    // Hide deleted files in public gallery view
    if (asset.status !== 'active') return false

    // Keyword check
    const matchesKeyword = !keyword || 
      asset.mediaId.toLowerCase().includes(keyword) || 
      (asset.mimeType && asset.mimeType.toLowerCase().includes(keyword)) ||
      (asset.project && asset.project.toLowerCase().includes(keyword))

    // Project check
    const matchesProj = !selectedProject.value || asset.project === selectedProject.value
    // Usage check
    const matchesUsage = !selectedUsage.value || asset.usage === selectedUsage.value
    // Media type check
    const matchesType = !mediaTypeFilter.value || 
      (mediaTypeFilter.value === 'image' && asset.mimeType.startsWith('image/')) ||
      (mediaTypeFilter.value === 'video' && asset.mimeType.startsWith('video/'))

    return matchesKeyword && matchesProj && matchesUsage && matchesType
  })
})

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
  return `<video src="${activeAsset.value.publicUrl}" controls width="640" height="360"></video>`
})

const markdownCode = computed(() => {
  if (!activeAsset.value) return ''
  return `![${activeAsset.value.mediaId}](${activeAsset.value.publicUrl})`
})

const bbCode = computed(() => {
  if (!activeAsset.value) return ''
  return `[img]${activeAsset.value.publicUrl}[/img]`
})

function copyText(txt: string, typeName: string) {
  navigator.clipboard.writeText(txt).then(() => {
    showToast(`复制 ${typeName} 成功！`)
  }).catch(() => {
    showToast('复制失败，请手动选择复制', 'error')
  })
}

function extByMIME(mime: string) {
  const map: Record<string, string> = {
    'image/jpeg': '.jpg', 'image/png': '.png', 'image/webp': '.webp', 'image/gif': '.gif',
    'video/mp4': '.mp4', 'video/webm': '.webm', 'video/quicktime': '.mov',
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

onMounted(() => {
  fetchAssets()
})
</script>

<template>
  <div class="gallery-view">
    <!-- Filter bar -->
    <div class="m3-card media-filter-bar">
      <div class="filter-inputs">
        <input class="filter-input search-field" v-model="searchKeyword" placeholder="搜索资源 ID 或项目标签..." />
        <select class="filter-input" v-model="selectedProject">
          <option value="">全部项目 (Projects)</option>
          <option v-for="p in projectsList" :key="p" :value="p">{{ p }}</option>
        </select>
        <select class="filter-input" v-model="selectedUsage">
          <option value="">全部用途 (Usages)</option>
          <option v-for="u in usagesList" :key="u" :value="u">{{ u }}</option>
        </select>
        <select class="filter-input" v-model="mediaTypeFilter">
          <option value="">全部类别</option>
          <option value="image">图片类</option>
          <option value="video">视频类</option>
        </select>
      </div>

      <button class="m3-btn m3-btn-secondary m3-btn-sm" @click="fetchAssets" :disabled="loading">
        <span class="material-symbols-rounded" style="font-size: 16px;">refresh</span>
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
          <img v-if="asset.mimeType.startsWith('image/')" :src="asset.publicUrl" alt="preview" loading="lazy" />
          <video v-else-if="asset.mimeType.startsWith('video/')" :src="asset.publicUrl" preload="metadata" muted></video>
          <span v-else class="material-symbols-rounded">description</span>

          <span v-if="asset.mimeType.startsWith('image/')" class="media-badge media-type-image">
            <span class="material-symbols-rounded" style="font-size: 12px;">image</span>
          </span>
          <span v-else-if="asset.mimeType.startsWith('video/')" class="media-badge media-type-video">
            <span class="material-symbols-rounded" style="font-size: 12px;">play_arrow</span>
          </span>
        </div>
        <div class="masonry-info">
          <div class="masonry-title">{{ asset.mediaId }}</div>
          <div class="masonry-meta">
            <span>{{ formatBytes(asset.sizeBytes) }}</span>
            <span>{{ asset.project }}</span>
          </div>
        </div>
      </div>
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
            <video v-else-if="activeAsset.mimeType.startsWith('video/')" :src="activeAsset.publicUrl" controls></video>
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
  }
}

@media (max-width: 480px) {
  .masonry-wrapper {
    column-count: 1;
  }
}

.masonry-card {
  break-inside: avoid;
  background: rgba(255, 255, 255, 0.03);
  border: 1px solid rgba(255, 255, 255, 0.06);
  border-radius: 16px;
  margin-bottom: 20px;
  overflow: hidden;
  cursor: pointer;
  transition: all 0.3s cubic-bezier(0.25, 0.8, 0.25, 1);
}

.masonry-card:hover {
  transform: translateY(-4px);
  border-color: rgba(0, 229, 255, 0.25);
  background: rgba(255, 255, 255, 0.06);
  box-shadow: var(--elevation-2);
}

.masonry-thumb {
  position: relative;
  width: 100%;
  background: #0d1216;
  max-height: 360px;
  overflow: hidden;
  display: flex;
  justify-content: center;
  align-items: center;
}

.masonry-thumb img, .masonry-thumb video {
  width: 100%;
  height: auto;
  object-fit: contain;
  display: block;
}

.masonry-thumb span.material-symbols-rounded {
  font-size: 48px;
  color: hsl(var(--md-sys-color-primary));
  padding: 40px 0;
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
  padding: 14px;
  text-align: left;
}

.masonry-title {
  font-size: 13px;
  font-weight: 600;
  color: #fff;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.masonry-meta {
  display: flex;
  justify-content: space-between;
  font-size: 11px;
  color: hsl(var(--md-sys-color-on-surface-variant));
  margin-top: 4px;
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
