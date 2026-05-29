<script setup lang="ts">
import { ref, onMounted, computed, watch, inject, type Ref } from 'vue'
import { 
  NCard, 
  NGrid, 
  NGi, 
  NButton, 
  NProgress, 
  NTag, 
  NSpin,
  useMessage 
} from 'naive-ui'
import { 
  CloudUpload, 
  Database, 
  ShieldAlert, 
  ShieldCheck, 
  ChartPie, 
  Info,
  RefreshCw
} from 'lucide-vue-next'
import { apiRequest } from '../api'
import type { HealthInfo, MediaAsset } from '../api'

const emit = defineEmits(['open-upload'])

const message = useMessage()
const loading = ref(true)
const health = ref<HealthInfo | null>(null)
const mediaList = ref<MediaAsset[]>([])
const refreshTrigger = inject<Ref<number>>('refreshStats')

// Dynamic data counters computed from loaded media assets
const mediaStats = computed(() => {
  let imagesCount = 0
  let imagesSize = 0
  let videosCount = 0
  let videosSize = 0
  let othersCount = 0
  let othersSize = 0

  mediaList.value.forEach(asset => {
    const mime = asset.mimeType.toLowerCase()
    if (mime.startsWith('image/')) {
      imagesCount++
      imagesSize += asset.fileSize
    } else if (mime.startsWith('video/')) {
      videosCount++
      videosSize += asset.fileSize
    } else {
      othersCount++
      othersSize += asset.fileSize
    }
  })

  const totalCount = mediaList.value.length

  return {
    images: {
      count: imagesCount,
      size: formatBytes(imagesSize),
      percent: totalCount > 0 ? (imagesCount / totalCount) * 100 : 0
    },
    videos: {
      count: videosCount,
      size: formatBytes(videosSize),
      percent: totalCount > 0 ? (videosCount / totalCount) * 100 : 0
    },
    others: {
      count: othersCount,
      size: formatBytes(othersSize),
      percent: totalCount > 0 ? (othersCount / totalCount) * 100 : 0
    }
  }
})

async function fetchStats() {
  loading.value = true
  try {
    const healthData = await apiRequest<HealthInfo>('/api/v1/health')
    health.value = healthData

    // Fetch media list to dynamically aggregate formatting distribution
    const listData = await apiRequest<MediaAsset[]>('/api/v1/media?show_deleted=true')
    mediaList.value = listData
  } catch (err: any) {
    message.error(`无法获取服务器状态: ${err.message}`)
  } finally {
    loading.value = false
  }
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 Bytes'
  const k = 1024
  const sizes = ['Bytes', 'KB', 'MB', 'GB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i]
}

watch(() => refreshTrigger?.value, () => {
  fetchStats()
})

onMounted(() => {
  fetchStats()
})
</script>

<template>
  <div class="flex flex-col gap-6">
    
    <!-- Top Action header -->
    <div class="flex justify-between items-center">
      <div>
        <h2 class="text-xl font-bold text-white mb-1">控制中心大盘</h2>
        <p class="text-xs text-gray-400">实时监控存储构成、节点连通及网关健康度指标。</p>
      </div>
      <div class="flex gap-2">
        <n-button size="medium" @click="fetchStats" :disabled="loading" secondary circle>
          <template #icon>
            <RefreshCw class="w-4 h-4" :class="loading ? 'animate-spin' : ''" />
          </template>
        </n-button>
        <n-button size="medium" type="primary" @click="emit('open-upload')" class="cursor-pointer">
          <template #icon>
            <CloudUpload class="w-4 h-4" />
          </template>
          <span>上传文件</span>
        </n-button>
      </div>
    </div>

    <n-spin :show="loading">
      <div class="flex flex-col gap-6">
        
        <!-- Status Stats Cards Grid -->
        <n-grid cols="1 s:3" responsive="screen" :x-gap="16" :y-gap="16">
          <n-gi>
            <n-card class="border border-white/5 hover:border-cyan-500/20 transition-all duration-300">
              <div class="flex items-center gap-4">
                <div class="w-12 h-12 bg-cyan-950/40 border border-cyan-500/20 text-cyan-400 rounded-2xl flex items-center justify-center">
                  <CloudUpload class="w-6 h-6" />
                </div>
                <div class="flex flex-col">
                  <span class="text-2xl font-bold text-white">{{ mediaList.filter(a => a.status === 'active').length }}</span>
                  <span class="text-xs text-gray-400 font-medium">库中文件总数</span>
                </div>
              </div>
            </n-card>
          </n-gi>

          <n-gi>
            <n-card class="border border-white/5 hover:border-cyan-500/20 transition-all duration-300">
              <div class="flex items-center gap-4">
                <div class="w-12 h-12 bg-purple-950/40 border border-purple-500/20 text-purple-400 rounded-2xl flex items-center justify-center">
                  <Database class="w-6 h-6" />
                </div>
                <div class="flex flex-col">
                  <span class="text-2xl font-bold text-white">
                    {{ formatBytes(mediaList.reduce((sum, item) => sum + item.fileSize, 0)) }}
                  </span>
                  <span class="text-xs text-gray-400 font-medium">总计占用容量</span>
                </div>
              </div>
            </n-card>
          </n-gi>

          <n-gi>
            <n-card class="border border-white/5 hover:border-cyan-500/20 transition-all duration-300">
              <div class="flex items-center gap-4">
                <div 
                  class="w-12 h-12 rounded-2xl flex items-center justify-center"
                  :class="health?.status === 'ok' 
                    ? 'bg-green-950/40 border border-green-500/20 text-green-400' 
                    : 'bg-red-950/40 border border-red-500/20 text-red-400'"
                >
                  <ShieldCheck v-if="health?.status === 'ok'" class="w-6 h-6" />
                  <ShieldAlert v-else class="w-6 h-6" />
                </div>
                <div class="flex flex-col">
                  <span class="text-2xl font-bold text-white uppercase">{{ health?.status || 'OFFLINE' }}</span>
                  <span class="text-xs text-gray-400 font-medium">网关服务状态</span>
                </div>
              </div>
            </n-card>
          </n-gi>
        </n-grid>

        <!-- Media distribution & Environment Grid -->
        <n-grid cols="1 m:2" responsive="screen" :x-gap="20" :y-gap="20">
          <n-gi>
            <n-card class="border border-white/5 h-full">
              <h3 class="text-base font-bold text-white flex items-center gap-2 mb-2">
                <ChartPie class="w-5 h-5 text-cyan-400" />
                媒体资源构成
              </h3>
              <p class="text-xs text-gray-400 mb-6">网关内保存的各类媒体文件占比及详细数据比例。</p>
              
              <div class="flex flex-col gap-5">
                <div>
                  <div class="flex justify-between text-xs font-semibold mb-1.5">
                    <span class="text-gray-300">图片类 (Images)</span>
                    <span class="text-white">{{ mediaStats.images.count }} 个 / {{ mediaStats.images.size }}</span>
                  </div>
                  <n-progress 
                    type="line" 
                    :percentage="parseFloat(mediaStats.images.percent.toFixed(1))" 
                    status="info" 
                    processing
                    :show-indicator="true" 
                  />
                </div>

                <div>
                  <div class="flex justify-between text-xs font-semibold mb-1.5">
                    <span class="text-gray-300">视频类 (Videos)</span>
                    <span class="text-white">{{ mediaStats.videos.count }} 个 / {{ mediaStats.videos.size }}</span>
                  </div>
                  <n-progress 
                    type="line" 
                    :percentage="parseFloat(mediaStats.videos.percent.toFixed(1))" 
                    status="warning" 
                    processing
                    :show-indicator="true" 
                  />
                </div>

                <div>
                  <div class="flex justify-between text-xs font-semibold mb-1.5">
                    <span class="text-gray-300">其他分片/归档 (Others)</span>
                    <span class="text-white">{{ mediaStats.others.count }} 个 / {{ mediaStats.others.size }}</span>
                  </div>
                  <n-progress 
                    type="line" 
                    :percentage="parseFloat(mediaStats.others.percent.toFixed(1))" 
                    status="default" 
                    processing
                    :show-indicator="true" 
                  />
                </div>
              </div>
            </n-card>
          </n-gi>

          <n-gi>
            <n-card class="border border-white/5 h-full">
              <h3 class="text-base font-bold text-white flex items-center gap-2 mb-2">
                <Info class="w-5 h-5 text-purple-400" />
                系统环境配置
              </h3>
              <p class="text-xs text-gray-400 mb-6">运行中的媒体网关后端核心环境参数列表。</p>
              
              <table class="w-full text-xs text-left border-collapse select-text">
                <tbody>
                  <tr class="border-b border-white/5">
                    <td class="py-3 text-gray-400 font-medium">接口鉴权模式</td>
                    <td class="py-3 text-right text-white font-semibold">
                      <n-tag size="small" :type="health?.is_private ? 'error' : 'success'">
                        {{ health?.is_private ? '私有受控鉴权' : '公共开放托管' }}
                      </n-tag>
                    </td>
                  </tr>
                  <tr class="border-b border-white/5">
                    <td class="py-3 text-gray-400 font-medium">数据库连接驱动</td>
                    <td class="py-3 text-right text-gray-200 font-mono font-bold uppercase">
                      {{ health?.storage_driver || health?.database_driver || 'SQLITE' }}
                    </td>
                  </tr>
                  <tr class="border-b border-white/5">
                    <td class="py-3 text-gray-400 font-medium">鉴权保护规则数</td>
                    <td class="py-3 text-right text-gray-200 font-bold">
                      {{ health?.rules_count || 0 }} 个匹配过滤规则
                    </td>
                  </tr>
                  <tr>
                    <td class="py-3 text-gray-400 font-medium">服务器运行版本</td>
                    <td class="py-3 text-right text-cyan-400 font-mono font-bold">
                      v1.2.0-release
                    </td>
                  </tr>
                </tbody>
              </table>
            </n-card>
          </n-gi>
        </n-grid>

      </div>
    </n-spin>
  </div>
</template>
