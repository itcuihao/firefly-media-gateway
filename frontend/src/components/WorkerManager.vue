<script setup lang="ts">
import { ref, onMounted, onUnmounted, inject } from 'vue'
import { apiRequest } from '../api'

interface WorkerNode {
  id: string
  name: string
  url: string
  token: string
  isActive: boolean
  verifyStatus?: 'idle' | 'success' | 'error'
  verifyMessage?: string
}

const showToast = inject<(msg: string, type?: 'success' | 'error') => void>('showToast', () => {})

const workers = ref<WorkerNode[]>([])
const activeTab = ref<'list' | 'add'>('list')

// Form bindings
const formName = ref('')
const formUrl = ref('')
const formToken = ref('')
const formShowToken = ref(false)

const verifyLoading = ref<Record<string, boolean>>({})

function loadWorkers() {
  const stored = localStorage.getItem('media_gateway_worker_list')
  if (stored) {
    try {
      workers.value = JSON.parse(stored)
    } catch (_) {
      workers.value = []
    }
  } else {
    // Migrate single worker config if it exists
    const oldUrl = localStorage.getItem('media_gateway_worker_url') || ''
    const oldToken = localStorage.getItem('media_gateway_worker_token') || ''
    if (oldUrl) {
      const defaultNode: WorkerNode = {
        id: 'default',
        name: '默认代理节点',
        url: oldUrl,
        token: oldToken,
        isActive: localStorage.getItem('active_worker_url') === oldUrl
      }
      workers.value = [defaultNode]
      saveWorkers()
    }
  }
}

function saveWorkers() {
  localStorage.setItem('media_gateway_worker_list', JSON.stringify(workers.value))
}

function selectTab(tab: 'list' | 'add') {
  activeTab.value = tab
  if (tab === 'add') {
    formName.value = ''
    formUrl.value = ''
    formToken.value = ''
    formShowToken.value = false
  }
}

function addNewWorker() {
  const name = formName.value.trim()
  let url = formUrl.value.trim()
  const token = formToken.value.trim()

  if (url && !url.startsWith('http://') && !url.startsWith('https://')) {
    url = 'https://' + url
  }

  if (!name || !url) {
    showToast('节点名称与 Worker API 地址不能为空！', 'error')
    return
  }

  const newNode: WorkerNode = {
    id: 'worker_' + Date.now(),
    name,
    url,
    token,
    isActive: false,
    verifyStatus: 'idle'
  }

  workers.value.push(newNode)
  saveWorkers()
  showToast('CF Worker 代理节点添加成功！')
  selectTab('list')
}

function deleteWorker(id: string) {
  const node = workers.value.find(w => w.id === id)
  if (!node) return
  if (!confirm(`确定要删除代理节点 "${node.name}" 吗？`)) return

  if (node.isActive) {
    deactivateWorker(node)
  }

  workers.value = workers.value.filter(w => w.id !== id)
  saveWorkers()
  showToast('代理节点已成功删除！')
}

function toggleTokenVisibility() {
  formShowToken.value = !formShowToken.value
}

async function testWorkerConnection(node: WorkerNode) {
  let url = node.url.trim()
  if (url && !url.startsWith('http://') && !url.startsWith('https://')) {
    url = 'https://' + url
    node.url = url
  }

  verifyLoading.value[node.id] = true
  node.verifyStatus = 'idle'
  node.verifyMessage = '测试中...'

  try {
    const data = await apiRequest('/api/v1/provider/worker/verify', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ url: node.url, token: node.token })
    })

    if (data.ok) {
      node.verifyStatus = 'success'
      node.verifyMessage = `连接成功 (HTTP ${data.info.status_code})`
      showToast(`节点 "${node.name}" 连接测试成功！`)
    } else {
      node.verifyStatus = 'error'
      node.verifyMessage = data.error || '连接失败'
      showToast(`节点 "${node.name}" 连通测试失败`, 'error')
    }
  } catch (err: any) {
    node.verifyStatus = 'error'
    node.verifyMessage = err.message || '请求异常'
    showToast(err.message || '测试异常', 'error')
  } finally {
    verifyLoading.value[node.id] = false
    saveWorkers()
  }
}

function activateWorker(node: WorkerNode) {
  workers.value.forEach(w => {
    w.isActive = (w.id === node.id)
  })
  
  localStorage.setItem('active_worker_url', node.url)
  localStorage.setItem('active_worker_token', node.token)
  // Sync to original single keys for backend config preview fallback compatibility
  localStorage.setItem('media_gateway_worker_url', node.url)
  localStorage.setItem('media_gateway_worker_token', node.token)
  
  saveWorkers()
  showToast(`代理节点 "${node.name}" 已成功激活！`)
  window.dispatchEvent(new Event('worker-status-changed'))
}

function deactivateWorker(node: WorkerNode) {
  node.isActive = false
  localStorage.removeItem('active_worker_url')
  localStorage.removeItem('active_worker_token')
  saveWorkers()
  showToast(`代理节点 "${node.name}" 已取消激活，切换回直连模式`)
  window.dispatchEvent(new Event('worker-status-changed'))
}

onMounted(() => {
  loadWorkers()
  window.addEventListener('worker-status-changed', loadWorkers)
})

onUnmounted(() => {
  window.removeEventListener('worker-status-changed', loadWorkers)
})
</script>

<template>
  <div class="panel-view active" id="panel_worker_manager">
    <!-- Sub tabs bar -->
    <div class="tab-nav">
      <button :class="['tab-btn', { active: activeTab === 'list' }]" @click="selectTab('list')">节点列表</button>
      <button :class="['tab-btn', { active: activeTab === 'add' }]" @click="selectTab('add')">添加代理节点</button>
    </div>

    <!-- Workers list panel -->
    <div v-if="activeTab === 'list'" style="display: flex; flex-direction: column; gap: 24px;">
      <div class="m3-card">
        <h2 class="section-title">
          <span class="material-symbols-rounded" style="color: #F38020;">cloud_done</span>
          Cloudflare Worker 代理节点管理
        </h2>
        <p style="font-size: 13px; color: hsl(var(--md-sys-color-on-surface-variant)); margin-bottom: 20px;">
          在此管理多个 Cloudflare Worker 代理节点，您可以添加多套环境，并一键激活切换当前生效的节点，甚至测试各个节点的连通性。
        </p>

        <!-- No data alert -->
        <div v-if="workers.length === 0" style="text-align: center; padding: 48px 0; color: hsl(var(--md-sys-color-on-surface-variant));">
          <span class="material-symbols-rounded" style="font-size: 48px; margin-bottom: 12px; opacity: 0.3;">cloud_off</span>
          <p>暂无配置的 Cloudflare Worker 节点</p>
          <button class="m3-btn m3-btn-primary m3-btn-sm" style="margin-top: 16px;" @click="selectTab('add')">立即添加节点</button>
        </div>

        <!-- Render list -->
        <div v-else class="m3-table-wrapper">
          <table class="m3-table">
            <thead>
              <tr>
                <th>状态/名称</th>
                <th>Worker API 调试地址</th>
                <th>连通测试结果</th>
                <th style="text-align: right; width: 320px;">操作</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="node in workers" :key="node.id">
                <td>
                  <div style="display: flex; align-items: center; gap: 10px;">
                    <!-- Pulse dot for active node -->
                    <span v-if="node.isActive" style="width: 8px; height: 8px; border-radius: 50%; background: hsl(var(--md-sys-color-success)); box-shadow: 0 0 8px hsl(var(--md-sys-color-success)); display: inline-block;"></span>
                    <span v-else style="width: 8px; height: 8px; border-radius: 50%; background: rgba(255,255,255,0.2); display: inline-block;"></span>
                    
                    <span style="font-weight: 600; color: #fff;">{{ node.name }}</span>
                    <span v-if="node.isActive" class="badge badge-success">当前激活</span>
                  </div>
                </td>
                <td style="font-family: monospace; font-size: 12px;">{{ node.url }}</td>
                <td>
                  <div style="display: flex; align-items: center; gap: 6px;">
                    <span v-if="node.verifyStatus === 'success'" class="badge badge-success">{{ node.verifyMessage }}</span>
                    <span v-else-if="node.verifyStatus === 'error'" class="badge badge-error">{{ node.verifyMessage }}</span>
                    <span v-else class="badge" style="color: hsl(var(--md-sys-color-on-surface-variant));">{{ node.verifyMessage || '未测试' }}</span>
                  </div>
                </td>
                <td style="text-align: right;">
                  <div style="display: flex; gap: 8px; justify-content: flex-end;">
                    <button v-if="!node.isActive" class="m3-btn m3-btn-primary m3-btn-sm" style="padding: 6px 12px;" @click="activateWorker(node)">激活</button>
                    <button v-else class="m3-btn m3-btn-secondary m3-btn-sm" style="padding: 6px 12px; background: rgba(255, 180, 171, 0.1); color: #ffb4ab;" @click="deactivateWorker(node)">取消激活</button>
                    
                    <button class="m3-btn m3-btn-secondary m3-btn-sm" style="padding: 6px 12px;" @click="testWorkerConnection(node)" :disabled="verifyLoading[node.id]">
                      <span v-if="verifyLoading[node.id]">测试中...</span>
                      <span v-else>测试</span>
                    </button>
                    <button class="m3-btn m3-btn-danger m3-btn-sm" style="padding: 6px 12px;" @click="deleteWorker(node.id)">删除</button>
                  </div>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>

    <!-- Add worker form panel -->
    <div v-if="activeTab === 'add'" class="m3-card" style="max-width: 600px;">
      <h2 class="section-title">
        <span class="material-symbols-rounded" style="color: hsl(var(--md-sys-color-primary));">add_box</span>
        添加新的 Cloudflare Worker 代理节点
      </h2>
      <p style="font-size: 13px; color: hsl(var(--md-sys-color-on-surface-variant)); margin-bottom: 24px;">
        配置一个能够上传/下载文件的 Cloudflare Worker 节点代理。
      </p>

      <div class="form-field">
        <label>节点名称</label>
        <div class="input-wrapper">
          <input v-model="formName" type="text" placeholder="例如：生产主节点 / 测试备用节点" />
        </div>
      </div>

      <div class="form-field">
        <label>Worker API 地址 (Base URL)</label>
        <div class="input-wrapper">
          <input v-model="formUrl" type="text" placeholder="https://your-worker.workers.dev" />
        </div>
      </div>

      <div class="form-field">
        <label>Worker Auth Token (Bearer)</label>
        <div class="input-wrapper">
          <input v-model="formToken" :type="formShowToken ? 'text' : 'password'" placeholder="输入该 Worker 接口的 Bearer 秘钥 Token" />
          <button class="input-icon-btn" @click="toggleTokenVisibility">
            <span class="material-symbols-rounded">{{ formShowToken ? 'visibility_off' : 'visibility' }}</span>
          </button>
        </div>
      </div>

      <div style="display: flex; gap: 12px; margin-top: 28px;">
        <button class="m3-btn m3-btn-primary" @click="addNewWorker">保存代理节点</button>
        <button class="m3-btn m3-btn-secondary" @click="selectTab('list')">取消</button>
      </div>
    </div>
  </div>
</template>
