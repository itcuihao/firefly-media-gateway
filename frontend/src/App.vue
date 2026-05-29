<script setup lang="ts">
import { ref, provide } from 'vue'
import {
  NConfigProvider,
  NMessageProvider,
  NDialogProvider,
  darkTheme,
  zhCN,
  dateZhCN,
  NButton,
  NInput,
  NForm,
  NFormItem,
  NPopover
} from 'naive-ui'
import { 
  LayoutDashboard, 
  FolderOpen, 
  KeyRound, 
  FlaskConical,
  Settings, 
  Menu,
  Eye,
  EyeOff
} from 'lucide-vue-next'
import { LOGO_BASE64 } from './logo'
import { getApiBaseUrl, setApiBaseUrl, getApiToken, setApiToken } from './api'
import Dashboard from './components/Dashboard.vue'
import MediaExplorer from './components/MediaExplorer.vue'
import BotVerifier from './components/BotVerifier.vue'
import ApiSandbox from './components/ApiSandbox.vue'

// Active Panel Tab state
const activeTab = ref('dashboard')
const menuOpen = ref(false)

// Config Settings dropdown states
const tempBaseUrl = ref(getApiBaseUrl())
const tempToken = ref(getApiToken())
const showToken = ref(false)
const popoverShow = ref(false)

// Ref to communicate file upload triggering from dashboard to explorer
const triggerUpload = ref(false)

// Setup global message trigger notifier
provide('refreshStats', ref(0))

function toggleMenu() {
  menuOpen.value = !menuOpen.value
}

function handleSwitchTab(tab: string) {
  activeTab.value = tab
  menuOpen.value = false
}

function saveGlobalConfig() {
  setApiBaseUrl(tempBaseUrl.value)
  setApiToken(tempToken.value)
  popoverShow.value = false
  window.location.reload() // Reload to apply global api url / credentials across all fetch instances
}
</script>

<template>
  <n-config-provider :theme="darkTheme" :locale="zhCN" :date-locale="dateZhCN">
    <n-message-provider>
      <n-dialog-provider>
        <div class="min-h-screen bg-[#0d1216] text-[#e0e3e7] flex font-sans antialiased selection:bg-cyan-500/20 select-none">
          
          <!-- Sidebar Navigation Drawer -->
          <nav 
            class="w-[280px] bg-[#11181c] border-r border-white/5 flex flex-col p-6 fixed top-0 bottom-0 left-0 z-50 transition-transform duration-300 md:translate-x-0"
            :class="menuOpen ? 'translate-x-0' : '-translate-x-full'"
          >
            <!-- Logo Section -->
            <div class="flex items-center gap-3.5 mb-8 pl-3">
              <div class="logo-wrapper">
                <img :src="`data:image/png;base64,${LOGO_BASE64}`" alt="Firefly Logo" class="logo-img" />
                <span class="logo-badge">嘿嘿</span>
              </div>
              <h1 class="text-lg font-bold tracking-tight bg-gradient-to-r from-cyan-400 to-white bg-clip-text text-transparent">
                Firefly Gateway
              </h1>
            </div>

            <!-- Menu Items -->
            <ul class="flex flex-col gap-1.5 flex-1">
              <li>
                <button 
                  @click="handleSwitchTab('dashboard')"
                  class="w-full flex items-center gap-4 px-5 py-3.5 rounded-full text-sm font-medium transition-all duration-200 cursor-pointer"
                  :class="activeTab === 'dashboard' 
                    ? 'bg-cyan-950/40 text-cyan-400 border border-cyan-500/10' 
                    : 'text-gray-400 hover:bg-white/5 hover:text-white'"
                >
                  <LayoutDashboard class="w-5 h-5" />
                  <span>仪表盘总览</span>
                </button>
              </li>
              <li>
                <button 
                  @click="handleSwitchTab('explorer')"
                  class="w-full flex items-center gap-4 px-5 py-3.5 rounded-full text-sm font-medium transition-all duration-200 cursor-pointer"
                  :class="activeTab === 'explorer' 
                    ? 'bg-cyan-950/40 text-cyan-400 border border-cyan-500/10' 
                    : 'text-gray-400 hover:bg-white/5 hover:text-white'"
                >
                  <FolderOpen class="w-5 h-5" />
                  <span>媒体库管理器</span>
                </button>
              </li>
              <li>
                <button 
                  @click="handleSwitchTab('verifier')"
                  class="w-full flex items-center gap-4 px-5 py-3.5 rounded-full text-sm font-medium transition-all duration-200 cursor-pointer"
                  :class="activeTab === 'verifier' 
                    ? 'bg-cyan-950/40 text-cyan-400 border border-cyan-500/10' 
                    : 'text-gray-400 hover:bg-white/5 hover:text-white'"
                >
                  <KeyRound class="w-5 h-5" />
                  <span>机器人连通验证</span>
                </button>
              </li>
              <li>
                <button 
                  @click="handleSwitchTab('sandbox')"
                  class="w-full flex items-center gap-4 px-5 py-3.5 rounded-full text-sm font-medium transition-all duration-200 cursor-pointer"
                  :class="activeTab === 'sandbox' 
                    ? 'bg-cyan-950/40 text-cyan-400 border border-cyan-500/10' 
                    : 'text-gray-400 hover:bg-white/5 hover:text-white'"
                >
                  <FlaskConical class="w-5 h-5" />
                  <span>API 联调沙盒</span>
                </button>
              </li>
            </ul>

            <!-- Footer Meta -->
            <div class="text-[11px] text-gray-600 pl-3">
              Firefly Media Gateway v1.2.0
            </div>
          </nav>

          <!-- Main Layout Content Wrapper -->
          <div class="flex-1 md:ml-[280px] flex flex-col min-w-0">
            
            <!-- Top App Bar Header -->
            <header class="h-[72px] bg-[#0d1216]/40 backdrop-blur-md border-b border-white/5 flex items-center justify-between px-6 md:px-8 sticky top-0 z-40">
              <div class="flex items-center gap-3">
                <button @click="toggleMenu" class="md:hidden p-1 hover:bg-white/5 rounded-lg text-gray-400 hover:text-white">
                  <Menu class="w-6 h-6" />
                </button>
                <div class="text-base font-bold md:text-lg text-white">
                  <span v-if="activeTab === 'dashboard'">仪表盘总览</span>
                  <span v-else-if="activeTab === 'explorer'">媒体库管理器</span>
                  <span v-else-if="activeTab === 'verifier'">机器人连通验证</span>
                  <span v-else-if="activeTab === 'sandbox'">API 联调沙盒</span>
                </div>
              </div>

              <!-- Top Global settings actions -->
              <div>
                <n-popover 
                  trigger="click" 
                  placement="bottom-end" 
                  v-model:show="popoverShow"
                  :raw="true"
                >
                  <template #trigger>
                    <button class="flex items-center gap-2 px-4 py-2 bg-white/5 hover:bg-white/10 rounded-full border border-white/5 text-xs text-cyan-400 font-medium cursor-pointer transition">
                      <Settings class="w-4 h-4" />
                      <span>网关配置</span>
                    </button>
                  </template>
                  
                  <!-- Dropdown Form -->
                  <div class="w-[300px] bg-[#1a2126] border border-white/10 p-5 rounded-2xl shadow-xl flex flex-col gap-4 text-sm mt-2 select-text">
                    <h3 class="text-sm font-semibold pb-2 border-b border-white/5 text-white">
                      全局连接配置
                    </h3>
                    
                    <n-form :show-feedback="false" class="flex flex-col gap-4">
                      <n-form-item label="API 基础地址">
                        <n-input 
                          v-model:value="tempBaseUrl" 
                          placeholder="http://localhost:8080" 
                        />
                      </n-form-item>
                      
                      <n-form-item label="网关 Bearer Token">
                        <n-input 
                          v-model:value="tempToken" 
                          :type="showToken ? 'text' : 'password'"
                          placeholder="输入 API Token"
                        >
                          <template #suffix>
                            <button 
                              @click="showToken = !showToken" 
                              class="text-gray-400 hover:text-white cursor-pointer"
                              type="button"
                            >
                              <Eye v-if="!showToken" class="w-4 h-4" />
                              <EyeOff v-else class="w-4 h-4" />
                            </button>
                          </template>
                        </n-input>
                      </n-form-item>
                    </n-form>

                    <div class="flex justify-end gap-2.5 mt-2">
                      <n-button size="small" type="default" @click="popoverShow = false">
                        取消
                      </n-button>
                      <n-button size="small" type="primary" @click="saveGlobalConfig">
                        保存配置
                      </n-button>
                    </div>
                  </div>
                </n-popover>
              </div>
            </header>

            <!-- Page Body Switcher -->
            <main class="flex-1 p-6 md:p-8 overflow-y-auto select-text">
              <Dashboard 
                v-if="activeTab === 'dashboard'" 
                @open-upload="activeTab = 'explorer'; triggerUpload = true" 
              />
              <MediaExplorer 
                v-else-if="activeTab === 'explorer'" 
                :trigger-upload="triggerUpload"
                @upload-handled="triggerUpload = false"
              />
              <BotVerifier v-else-if="activeTab === 'verifier'" />
              <ApiSandbox v-else-if="activeTab === 'sandbox'" />
            </main>
          </div>

          <!-- Overlay Drawer Menu for mobile screens -->
          <div 
            v-if="menuOpen" 
            @click="menuOpen = false" 
            class="fixed inset-0 bg-black/60 backdrop-blur-sm z-40 md:hidden"
          ></div>
        </div>
      </n-dialog-provider>
    </n-message-provider>
  </n-config-provider>
</template>

<style>
/* Logo styling replicating the original custom design with '嘿嘿' badge */
.logo-wrapper {
  position: relative;
  display: inline-block;
  width: 42px;
  height: 42px;
}

.logo-img {
  width: 100%;
  height: 100%;
  object-fit: contain;
  filter: drop-shadow(0 0 8px rgba(0, 229, 255, 0.5));
  transition: transform 0.3s cubic-bezier(0.4, 0, 0.2, 1);
}

.logo-wrapper:hover .logo-img {
  transform: scale(1.1) rotate(6deg);
}

.logo-badge {
  position: absolute;
  top: -4px;
  right: -8px;
  background: linear-gradient(135deg, #fbbf24, #f59e0b);
  color: #030712;
  font-size: 9px;
  font-weight: 800;
  padding: 1px 4.5px;
  border-radius: 9999px;
  box-shadow: 0 0 6px rgba(245, 158, 11, 0.6);
  white-space: nowrap;
  pointer-events: none;
  animation: logoPulse 2.5s infinite;
  border: 1.5px solid #11181c;
  letter-spacing: 0.05em;
  line-height: 1;
}

@keyframes logoPulse {
  0%, 100% {
    transform: scale(1);
    box-shadow: 0 0 6px rgba(245, 158, 11, 0.6);
  }
  50% {
    transform: scale(1.08);
    box-shadow: 0 0 10px rgba(245, 158, 11, 0.9);
  }
}
</style>
