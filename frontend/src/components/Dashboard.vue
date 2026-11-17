<script setup lang="ts">
import { ref, onMounted, computed, inject } from 'vue'
import { apiRequest, getApiToken } from '../api'
import type { HealthInfo, MediaStats } from '../api'

const emit = defineEmits(['switch-tab'])

const showToast = inject<(msg: string, type?: 'success' | 'error') => void>('showToast', () => {})

const loading = ref(true)
const health = ref<HealthInfo | null>(null)
const statsData = ref<MediaStats | null>(null)
const apiToken = ref(getApiToken())

// Dynamic data counters computed from real database aggregate statistics
const mediaStats = computed(() => {
  if (loading.value || !statsData.value) {
    return {
      totalCount: '--',
      totalSize: '--',
      images: { count: '0', size: '0 B', percent: 0 },
      videos: { count: '0', size: '0 B', percent: 0 },
      others: { count: '0', size: '0 B', percent: 0 }
    }
  }

  const s = statsData.value
  return {
    totalCount: String(s.total_count),
    totalSize: formatBytes(s.total_bytes),
    images: {
      count: String(s.images.count),
      size: formatBytes(s.images.bytes),
      percent: s.images.percent
    },
    videos: {
      count: String(s.videos.count),
      size: formatBytes(s.videos.bytes),
      percent: s.videos.percent
    },
    others: {
      count: String(s.others.count),
      size: formatBytes(s.others.bytes),
      percent: s.others.percent
    }
  }
})

// Prevent flash of red "断开/未运行" during initial health check load
const healthStatusText = computed(() => {
  if (loading.value && !health.value) {
    return '连接中...'
  }
  return health.value?.status === 'ok' ? '在线运行' : '断开/未运行'
})

const healthStatusColor = computed(() => {
  if (loading.value && !health.value) {
    return '#fff'
  }
  return health.value?.status === 'ok' ? 'hsl(var(--md-sys-color-success))' : 'var(--md-sys-color-error)'
})

async function fetchStats() {
  loading.value = true
  try {
    const [healthData, stats] = await Promise.all([
      apiRequest<HealthInfo>('/api/v1/health').catch((err) => {
        console.error('Failed to fetch health info:', err)
        return null
      }),
      apiRequest<MediaStats>('/api/v1/media/stats')
    ])
    if (healthData) health.value = healthData
    statsData.value = stats
  } catch (err: any) {
    showToast('拉取资源失败，请检查 Bearer Token 配置', 'error')
    console.error('Failed to fetch media assets stats:', err)
  } finally {
    loading.value = false
  }
}

function formatBytes(bytes: number): string {
  if (!bytes || bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

onMounted(() => {
  fetchStats()
})
</script>

<template>
  <div class="panel-view active" id="panel_dashboard">
    <div class="m3-grid-3">
      <div class="m3-card stat-card">
        <div class="stat-icon primary">
          <span class="material-symbols-rounded">cloud_upload</span>
        </div>
        <div class="stat-info">
          <span class="stat-val" id="stat_total_count">{{ mediaStats.totalCount }}</span>
          <span class="stat-label">库中文件总数</span>
        </div>
      </div>
      <div class="m3-card stat-card">
        <div class="stat-icon secondary">
          <span class="material-symbols-rounded">database</span>
        </div>
        <div class="stat-info">
          <span class="stat-val" id="stat_total_size">{{ mediaStats.totalSize }}</span>
          <span class="stat-label">总计占用容量</span>
        </div>
      </div>
      <div class="m3-card stat-card">
        <div class="stat-icon success">
          <span class="material-symbols-rounded">health_and_safety</span>
        </div>
        <div class="stat-info">
          <span class="stat-val" id="stat_health_status" :style="{ color: healthStatusColor }">
            {{ healthStatusText }}
          </span>
          <span class="stat-label">网关服务状态</span>
        </div>
      </div>
    </div>

    <div class="m3-grid-2">
      <!-- Storage Info -->
      <div class="m3-card">
        <h2 class="section-title">
          <span class="material-symbols-rounded" style="color: hsl(var(--md-sys-color-primary));">donut_large</span>
          媒体资源构成
        </h2>
        <p style="font-size: 13px; color: hsl(var(--md-sys-color-on-surface-variant)); margin-bottom: 24px;">网关内保存的各类媒体文件占比及详细数据。</p>
        
        <div style="display: flex; flex-direction: column; gap: 18px;">
          <div>
            <div style="display: flex; justify-content: space-between; font-size: 13px; margin-bottom: 6px;">
              <span>图片类 (Images)</span>
              <span id="chart_img_txt" style="font-weight: 600;">
                <template v-if="loading">--</template>
                <template v-else>{{ mediaStats.images.count }} 个 ({{ mediaStats.images.percent.toFixed(0) }}%) - {{ mediaStats.images.size }}</template>
              </span>
            </div>
            <div style="height: 8px; background: rgba(255,255,255,0.05); border-radius: 10px; overflow: hidden;">
              <div id="chart_img_bar" :style="{ width: mediaStats.images.percent + '%' }" style="height: 100%; background: hsl(var(--md-sys-color-primary)); transition: width 1s;"></div>
            </div>
          </div>

          <div>
            <div style="display: flex; justify-content: space-between; font-size: 13px; margin-bottom: 6px;">
              <span>视频类 (Videos)</span>
              <span id="chart_vid_txt" style="font-weight: 600;">
                <template v-if="loading">--</template>
                <template v-else>{{ mediaStats.videos.count }} 个 ({{ mediaStats.videos.percent.toFixed(0) }}%) - {{ mediaStats.videos.size }}</template>
              </span>
            </div>
            <div style="height: 8px; background: rgba(255,255,255,0.05); border-radius: 10px; overflow: hidden;">
              <div id="chart_vid_bar" :style="{ width: mediaStats.videos.percent + '%' }" style="height: 100%; background: hsl(var(--md-sys-color-secondary)); transition: width 1s;"></div>
            </div>
          </div>

          <div>
            <div style="display: flex; justify-content: space-between; font-size: 13px; margin-bottom: 6px;">
              <span>其他分片/归档 (Others)</span>
              <span id="chart_other_txt" style="font-weight: 600;">
                <template v-if="loading">--</template>
                <template v-else>{{ mediaStats.others.count }} 个 ({{ mediaStats.others.percent.toFixed(0) }}%) - {{ mediaStats.others.size }}</template>
              </span>
            </div>
            <div style="height: 8px; background: rgba(255,255,255,0.05); border-radius: 10px; overflow: hidden;">
              <div id="chart_other_bar" :style="{ width: mediaStats.others.percent + '%' }" style="height: 100%; background: #9ca3af; transition: width 1s;"></div>
            </div>
          </div>
        </div>
      </div>

      <!-- Server Environment -->
      <div class="m3-card">
        <h2 class="section-title">
          <span class="material-symbols-rounded" style="color: hsl(var(--md-sys-color-success));">info</span>
          系统环境配置
        </h2>
        <p style="font-size: 13px; color: hsl(var(--md-sys-color-on-surface-variant)); margin-bottom: 20px;">运行中的媒体网关后端核心环境参数。</p>
        
        <table style="width: 100%; font-size: 13px; border-collapse: collapse;">
          <tbody>
            <tr style="border-bottom: 1px solid rgba(255,255,255,0.05);">
              <td style="padding: 10px 0; color: hsl(var(--md-sys-color-on-surface-variant));">接口鉴权状态</td>
              <td id="env_auth_state" style="padding: 10px 0; text-align: right; font-weight: 600;" :style="{ color: apiToken ? 'hsl(var(--md-sys-color-primary))' : 'var(--md-sys-color-error)' }">
                {{ apiToken ? '已配置 (Bearer)' : '未配置' }}
              </td>
            </tr>
            <tr style="border-bottom: 1px solid rgba(255,255,255,0.05);">
              <td style="padding: 10px 0; color: hsl(var(--md-sys-color-on-surface-variant));">存储运行模式</td>
              <td style="padding: 10px 0; text-align: right; font-family: monospace; color: #fff; text-transform: uppercase;">
                {{ health?.storage_driver || 'DIRECT' }}
              </td>
            </tr>
            <tr style="border-bottom: 1px solid rgba(255,255,255,0.05);">
              <td style="padding: 10px 0; color: hsl(var(--md-sys-color-on-surface-variant));">数据库驱动</td>
              <td style="padding: 10px 0; text-align: right; font-family: monospace; color: #fff; text-transform: uppercase;">
                {{ health?.database_driver || 'SQLITE' }}
              </td>
            </tr>
            <tr style="border-bottom: 1px solid rgba(255,255,255,0.05);">
              <td style="padding: 10px 0; color: hsl(var(--md-sys-color-on-surface-variant));">CF Worker 状态</td>
              <td style="padding: 10px 0; text-align: right; font-weight: 600;" :style="{ color: health?.worker_url ? 'hsl(var(--md-sys-color-success))' : 'hsl(var(--md-sys-color-on-surface-variant))' }">
                {{ health?.worker_url ? '已配置' : '未连接/未配置' }}
              </td>
            </tr>
            <tr v-if="health?.worker_url" style="border-bottom: 1px solid rgba(255,255,255,0.05);">
              <td style="padding: 10px 0; color: hsl(var(--md-sys-color-on-surface-variant));">CF Worker 地址</td>
              <td style="padding: 10px 0; text-align: right; font-family: monospace; color: #fff; font-size: 11px; word-break: break-all; max-width: 180px; margin-left: auto;">
                {{ health?.worker_url }}
              </td>
            </tr>
            <tr style="border-bottom: 1px solid rgba(255,255,255,0.05);">
              <td style="padding: 10px 0; color: hsl(var(--md-sys-color-on-surface-variant));">响应时区</td>
              <td style="padding: 10px 0; text-align: right; font-family: monospace; color: #fff;">UTC / Local</td>
            </tr>
            <tr>
              <td style="padding: 10px 0; color: hsl(var(--md-sys-color-on-surface-variant));">服务器版本</td>
              <td style="padding: 10px 0; text-align: right; font-family: monospace; font-weight: 600; color: hsl(var(--md-sys-color-primary));">v1.2.0-release</td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>
