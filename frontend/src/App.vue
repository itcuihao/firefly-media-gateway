<script setup lang="ts">
import { ref, provide, onMounted, onUnmounted } from 'vue'
import {
  NConfigProvider,
  NMessageProvider,
  NDialogProvider,
  darkTheme,
  zhCN,
  dateZhCN
} from 'naive-ui'
import { LOGO_BASE64 } from './logo'
import { getApiBaseUrl, setApiBaseUrl, getApiToken, setApiToken } from './api'
import Dashboard from './components/Dashboard.vue'
import MediaExplorer from './components/MediaExplorer.vue'
import BotVerifier from './components/BotVerifier.vue'
import ApiSandbox from './components/ApiSandbox.vue'

// Active Panel Tab state
const activeTab = ref('dashboard')
const drawerOpen = ref(false)

// Config Settings dropdown states
const baseUrl = ref(getApiBaseUrl() || window.location.origin)
const authToken = ref(getApiToken())
const showPassword = ref(false)
const configOpen = ref(false)

// Toast system
interface Toast {
  id: number
  message: string
  type: 'success' | 'error'
}
const toasts = ref<Toast[]>([])
let toastIdCounter = 0

function showToast(message: string, type: 'success' | 'error' = 'success') {
  const id = toastIdCounter++
  toasts.value.push({ id, message, type })
  setTimeout(() => {
    // Fade out transition trigger could be done via CSS class
    toasts.value = toasts.value.filter(t => t.id !== id)
  }, 3500)
}

provide('showToast', showToast)

function toggleDrawer() {
  drawerOpen.value = !drawerOpen.value
}

function switchTab(tabId: string) {
  activeTab.value = tabId
  drawerOpen.value = false
}

function toggleConfigDropdown() {
  configOpen.value = !configOpen.value
}

function togglePasswordVisibility() {
  showPassword.value = !showPassword.value
}

function saveGlobalConfig() {
  const url = baseUrl.value.trim()
  const token = authToken.value.trim()
  
  // Save to both namespaces for full compatibility
  localStorage.setItem('media_gateway_url', url)
  localStorage.setItem('media_gateway_token', token)
  setApiBaseUrl(url)
  setApiToken(token)
  
  showToast('全局配置已成功保存！')
  configOpen.value = false
  
  // Trigger a full reload or component refresh
  setTimeout(() => {
    window.location.reload()
  }, 500)
}

// Global click handler to close dropdown
function handleWindowClick(e: MouseEvent) {
  const target = e.target as HTMLElement
  if (configOpen.value && !target.closest('.config-dropdown') && !target.closest('#configBtn')) {
    configOpen.value = false
  }
}

onMounted(() => {
  window.addEventListener('click', handleWindowClick)
})

onUnmounted(() => {
  window.removeEventListener('click', handleWindowClick)
})
</script>

<template>
  <n-config-provider :theme="darkTheme" :locale="zhCN" :date-locale="dateZhCN">
    <n-message-provider>
      <n-dialog-provider>
        <!-- Toast Messages Container -->
        <div class="toast-container" id="toastContainer">
          <div v-for="toast in toasts" :key="toast.id" :class="['toast', `toast-${toast.type}`]">
            <span class="material-symbols-rounded" style="font-size: 18px;">
              {{ toast.type === 'error' ? 'error' : 'check_circle' }}
            </span>
            <span>{{ toast.message }}</span>
          </div>
        </div>

        <!-- Main M3 Layout -->
        <div class="app-layout">
          
          <!-- Navigation Drawer -->
          <nav :class="['nav-drawer', { active: drawerOpen }]" id="navDrawer">
            <div class="nav-brand">
              <div class="logo-wrapper">
                <img :src="`data:image/png;base64,${LOGO_BASE64}`" alt="Firefly Logo" class="logo-img" />
                <span class="logo-badge">嘿嘿</span>
              </div>
              <h1>Firefly Gateway</h1>
            </div>
            <ul class="nav-menu">
              <li>
                <button :class="['nav-item', { active: activeTab === 'dashboard' }]" @click="switchTab('dashboard')">
                  <span class="material-symbols-rounded">dashboard</span>
                  <span>仪表盘总览</span>
                </button>
              </li>
              <li>
                <button :class="['nav-item', { active: activeTab === 'explorer' }]" @click="switchTab('explorer')">
                  <span class="material-symbols-rounded">folder_open</span>
                  <span>媒体库管理器</span>
                </button>
              </li>
              <li>
                <button :class="['nav-item', { active: activeTab === 'verifier' }]" @click="switchTab('verifier')">
                  <span class="material-symbols-rounded">vpn_key</span>
                  <span>机器人连通验证</span>
                </button>
              </li>
              <li>
                <button :class="['nav-item', { active: activeTab === 'sandbox' }]" @click="switchTab('sandbox')">
                  <span class="material-symbols-rounded">science</span>
                  <span>API 联调沙盒</span>
                </button>
              </li>
            </ul>
          </nav>

          <!-- Content Workspace -->
          <div class="main-wrapper">
            
            <!-- Top App Bar Header -->
            <header class="top-app-bar">
              <div style="display: flex; align-items: center;">
                <button class="menu-toggle" @click="toggleDrawer">
                  <span class="material-symbols-rounded" style="font-size: 28px;">menu</span>
                </button>
                <div class="page-title" id="pageTitle">
                  <span v-if="activeTab === 'dashboard'">仪表盘总览</span>
                  <span v-else-if="activeTab === 'explorer'">媒体库管理器</span>
                  <span v-else-if="activeTab === 'verifier'">机器人连通验证</span>
                  <span v-else-if="activeTab === 'sandbox'">API 联调沙盒</span>
                </div>
              </div>

              <div class="global-actions">
                <button :class="['config-trigger', { active: configOpen }]" id="configBtn" @click="toggleConfigDropdown">
                  <span class="material-symbols-rounded" style="font-size: 18px;">settings</span>
                  <span>网关配置</span>
                </button>
                
                <!-- Quick Config dropdown panel -->
                <div :class="['config-dropdown', { active: configOpen }]" id="configDropdown">
                  <h3 style="font-size: 15px; font-weight: 600; margin-bottom: 8px; border-bottom: 1px solid rgba(255,255,255,0.08); padding-bottom: 6px; color: #fff;">全局连接配置</h3>
                  <div class="form-field">
                    <label>API 基础地址</label>
                    <div class="input-wrapper">
                      <input v-model="baseUrl" type="text" placeholder="http://localhost:8080" />
                    </div>
                  </div>
                  <div class="form-field">
                    <label>网关 Bearer Token</label>
                    <div class="input-wrapper">
                      <input v-model="authToken" :type="showPassword ? 'text' : 'password'" placeholder="输入 API Token" />
                      <button class="input-icon-btn" @click="togglePasswordVisibility">
                        <span class="material-symbols-rounded" id="authToken_eye">
                          {{ showPassword ? 'visibility_off' : 'visibility' }}
                        </span>
                      </button>
                    </div>
                  </div>
                  <button class="m3-btn m3-btn-primary m3-btn-sm" @click="saveGlobalConfig">保存配置</button>
                </div>
              </div>
            </header>

            <!-- App Body Panel Switcher -->
            <main class="content-body">
              <Dashboard v-if="activeTab === 'dashboard'" @switch-tab="switchTab" />
              <MediaExplorer v-else-if="activeTab === 'explorer'" />
              <BotVerifier v-else-if="activeTab === 'verifier'" />
              <ApiSandbox v-else-if="activeTab === 'sandbox'" />
            </main>
          </div>
        </div>
      </n-dialog-provider>
    </n-message-provider>
  </n-config-provider>
</template>
