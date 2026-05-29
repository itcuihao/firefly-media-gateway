<script setup lang="ts">
import { ref } from 'vue'
import {
  NCard,
  NButton,
  NInput,
  NForm,
  NFormItem,
  NTabs,
  NTabPane,
  useMessage
} from 'naive-ui'
import {
  Send,
  MessageSquare,
  ShieldAlert,
  ShieldCheck,
  HelpCircle,
  Copy,
  FolderDot,
  Server,
  Eye,
  EyeOff
} from 'lucide-vue-next'
import { apiRequest } from '../api'

const message = useMessage()

// TG state
const tgToken = ref('')
const tgShowToken = ref(false)
const tgVerifyLoading = ref(false)
const tgChatsLoading = ref(false)
const tgStatus = ref<'idle' | 'success' | 'error'>('idle')
const tgErrorMsg = ref('')
const tgBotProfile = ref({
  id: '',
  name: '',
  username: ''
})
const tgChats = ref<{ id: number; name: string }[]>([])

// Discord state
const discordToken = ref('')
const discordShowToken = ref(false)
const discordVerifyLoading = ref(false)
const discordGuildsLoading = ref(false)
const discordStatus = ref<'idle' | 'success' | 'error'>('idle')
const discordErrorMsg = ref('')
const discordBotProfile = ref({
  id: '',
  name: '',
  tag: ''
})
const discordGuilds = ref<{ id: string; name: string }[]>([])

// TG API actions
async function verifyTelegramBot() {
  tgVerifyLoading.value = true
  tgStatus.value = 'idle'
  try {
    const data = await apiRequest('/api/v1/provider/telegram/verify', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ token: tgToken.value })
    })

    if (data.success && data.bot) {
      tgBotProfile.value = {
        id: String(data.bot.id),
        name: data.bot.first_name || '',
        username: data.bot.username || ''
      }
      tgStatus.value = 'success'
      message.success('Telegram Bot 连通校验成功！')
    } else {
      throw new Error(data.error || '未知错误')
    }
  } catch (err: any) {
    tgStatus.value = 'error'
    tgErrorMsg.value = err.message
    message.error(`校验失败: ${err.message}`)
  } finally {
    tgVerifyLoading.value = false
  }
}

async function fetchTelegramChats() {
  tgChatsLoading.value = true
  try {
    const data = await apiRequest('/api/v1/provider/telegram/chat-ids', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ token: tgToken.value })
    })

    if (data.success && Array.isArray(data.chats)) {
      tgChats.value = data.chats
      message.success(`成功拉取 ${data.chats.length} 个最新互动的群组/频道`)
    } else {
      throw new Error(data.error || '无可用更新')
    }
  } catch (err: any) {
    message.error(`拉取群组失败: ${err.message}`)
  } finally {
    tgChatsLoading.value = false
  }
}

// Discord API actions
async function verifyDiscordBot() {
  if (!discordToken.value) {
    message.error('请输入 Discord Bot Token')
    return
  }

  discordVerifyLoading.value = true
  discordStatus.value = 'idle'
  try {
    const data = await apiRequest('/api/v1/provider/discord/verify', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ token: discordToken.value })
    })

    if (data.success && data.bot) {
      discordBotProfile.value = {
        id: String(data.bot.id),
        name: data.bot.username || '',
        tag: `${data.bot.username}#${data.bot.discriminator || '0000'}`
      }
      discordStatus.value = 'success'
      message.success('Discord Bot 连通校验成功！')
    } else {
      throw new Error(data.error || '未知错误')
    }
  } catch (err: any) {
    discordStatus.value = 'error'
    discordErrorMsg.value = err.message
    message.error(`校验失败: ${err.message}`)
  } finally {
    discordVerifyLoading.value = false
  }
}

async function fetchDiscordGuilds() {
  if (!discordToken.value) {
    message.error('请在输入框内填入 Discord Bot Token')
    return
  }

  discordGuildsLoading.value = true
  try {
    const data = await apiRequest('/api/v1/provider/discord/guilds', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ token: discordToken.value })
    })

    if (data.success && Array.isArray(data.guilds)) {
      discordGuilds.value = data.guilds
      message.success(`成功拉取 ${data.guilds.length} 个所在的 Discord 服务器`)
    } else {
      throw new Error(data.error || '无可用列表')
    }
  } catch (err: any) {
    message.error(`拉取服务器失败: ${err.message}`)
  } finally {
    discordGuildsLoading.value = false
  }
}

async function copyToClipboard(val: string | number) {
  try {
    await navigator.clipboard.writeText(String(val))
    message.success(`已成功复制 ID: ${val}`)
  } catch (_) {
    message.error('复制失败')
  }
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <div>
      <h2 class="text-xl font-bold text-white mb-1">机器人连通验证</h2>
      <p class="text-xs text-gray-400">实时校验 Telegram 或 Discord Bot Token，并拉取其交互的群组或服务器 ID 作为网关存储规则路径。</p>
    </div>

    <n-tabs type="segment" class="w-full">
      
      <!-- Telegram Tab Pane -->
      <n-tab-pane name="tg" tab="Telegram 机器人验证">
        <div class="flex flex-col gap-6 mt-4 select-text">
          <n-card class="border border-white/5 shadow-md">
            
            <div class="flex flex-col md:flex-row gap-6">
              
              <!-- Form controls -->
              <div class="flex-1 flex flex-col gap-4">
                <h3 class="text-base font-bold text-white flex items-center gap-2 mb-1">
                  <Send class="w-5 h-5 text-[#229ED9]" />
                  Telegram Bot 在线校验与 Group ID 获取
                </h3>
                <p class="text-xs text-gray-400">
                  输入您想要调试的 Telegram 机器人 Token。我们将调用 <code>getMe</code> 接口校验其真实性，并可通过 <code>getUpdates</code> 拉取最新加入的群组或频道。
                </p>

                <n-form :show-feedback="false" class="mt-2">
                  <n-form-item label="Telegram Bot Token">
                    <n-input
                      v-model:value="tgToken"
                      :type="tgShowToken ? 'text' : 'password'"
                      placeholder="123456789:ABCDefGhIjKlMnOpQrStUvWxYz"
                    >
                      <template #suffix>
                        <button 
                          @click="tgShowToken = !tgShowToken" 
                          class="text-gray-400 hover:text-white cursor-pointer"
                          type="button"
                        >
                          <Eye v-if="!tgShowToken" class="w-4 h-4" />
                          <EyeOff v-else class="w-4 h-4" />
                        </button>
                      </template>
                    </n-input>
                  </n-form-item>
                  <span class="text-[11px] text-gray-600 block mt-1">
                    为空则默认加载后端的 <code>TELEGRAM_BOT_TOKEN</code> 环境变量
                  </span>
                </n-form>

                <div class="flex gap-3 mt-4">
                  <n-button 
                    type="primary" 
                    @click="verifyTelegramBot" 
                    :loading="tgVerifyLoading"
                    class="cursor-pointer"
                  >
                    <span>测试机器人连通性</span>
                  </n-button>
                  
                  <n-button 
                    type="default" 
                    @click="fetchTelegramChats" 
                    :loading="tgChatsLoading"
                    class="cursor-pointer"
                  >
                    <span>获取最近互动群组 ID</span>
                  </n-button>
                </div>
              </div>

              <!-- Output Status Card -->
              <div class="w-full md:w-[320px] bg-black/20 border border-white/5 border-dashed rounded-2xl p-5 flex flex-col justify-center select-none">
                <div class="flex items-center gap-3.5 mb-4">
                  <div class="w-10 h-10 rounded-xl flex items-center justify-center bg-white/5 border border-white/5">
                    <ShieldCheck v-if="tgStatus === 'success'" class="w-5 h-5 text-green-400" />
                    <ShieldAlert v-else-if="tgStatus === 'error'" class="w-5 h-5 text-red-400" />
                    <HelpCircle v-else class="w-5 h-5 text-gray-500" />
                  </div>
                  <div>
                    <h4 class="text-sm font-bold text-white">
                      <span v-if="tgStatus === 'success'">校验成功</span>
                      <span v-else-if="tgStatus === 'error'">校验失败</span>
                      <span v-else>待进行测试</span>
                    </h4>
                    <p class="text-xs text-gray-400">
                      <span v-if="tgStatus === 'success'">Bot 在线认证正常</span>
                      <span v-else-if="tgStatus === 'error'">接口连接异常</span>
                      <span v-else>请输入 Token 发起校验</span>
                    </p>
                  </div>
                </div>

                <div v-if="tgStatus === 'success'" class="flex flex-col gap-3 pt-3 border-t border-white/5 text-xs select-text">
                  <div class="flex justify-between">
                    <span class="text-gray-500">Bot ID</span>
                    <span class="text-white font-mono font-bold">{{ tgBotProfile.id }}</span>
                  </div>
                  <div class="flex justify-between">
                    <span class="text-gray-500">账号昵称</span>
                    <span class="text-white font-semibold">{{ tgBotProfile.name }}</span>
                  </div>
                  <div class="flex justify-between">
                    <span class="text-gray-500">用户名</span>
                    <span class="text-cyan-400 font-mono">@{{ tgBotProfile.username }}</span>
                  </div>
                </div>
                <div v-else-if="tgStatus === 'error'" class="text-xs text-red-400 bg-red-950/20 border border-red-500/10 p-3 rounded-xl break-all select-text">
                  错误日志: {{ tgErrorMsg }}
                </div>
              </div>

            </div>
          </n-card>

          <!-- Resolved Chats Grid list -->
          <n-card v-if="tgChats.length > 0" class="border border-white/5">
            <h3 class="text-sm font-bold text-white flex items-center gap-2 mb-1.5">
              <MessageSquare class="w-4 h-4 text-cyan-400" />
              检测到的最新互动群组 / 频道 (Chat IDs)
            </h3>
            <p class="text-xs text-gray-400 mb-4">
              注意：Bot 只能拉取到最近 24 小时内有新消息的群组。请在群组中艾特 Bot 发送测试消息，然后再次点击刷新。点击复制 Chat ID。
            </p>

            <n-grid cols="1 s:2 m:3" :x-gap="12" :y-gap="12">
              <n-gi v-for="chat in tgChats" :key="chat.id">
                <div 
                  @click="copyToClipboard(chat.id)"
                  class="bg-white/5 border border-white/5 hover:border-cyan-500/30 p-4 rounded-xl flex items-center justify-between cursor-pointer group transition"
                >
                  <div class="flex flex-col gap-1 min-w-0">
                    <span class="text-xs font-bold text-white truncate">{{ chat.name }}</span>
                    <span class="text-[10px] font-mono text-gray-500 truncate">{{ chat.id }}</span>
                  </div>
                  <Copy class="w-4 h-4 text-gray-600 group-hover:text-white transition" />
                </div>
              </n-gi>
            </n-grid>
          </n-card>
        </div>
      </n-tab-pane>

      <!-- Discord Tab Pane -->
      <n-tab-pane name="discord" tab="Discord 机器人验证">
        <div class="flex flex-col gap-6 mt-4 select-text">
          <n-card class="border border-white/5 shadow-md">
            
            <div class="flex flex-col md:flex-row gap-6">
              
              <!-- Form controls -->
              <div class="flex-1 flex flex-col gap-4">
                <h3 class="text-base font-bold text-white flex items-center gap-2 mb-1">
                  <FolderDot class="w-5 h-5 text-[#5865F2]" />
                  Discord Bot 鉴权有效性及服务器 (Guild) ID 获取
                </h3>
                <p class="text-xs text-gray-400">
                  输入您创建的 Discord 机器人 token。我们将使用 <code>Authorization: Bot &lt;token&gt;</code> 调用 Discord API 校验状态，并能列出该 Bot 目前加入的所有服务器（Guilds）。
                </p>

                <n-form :show-feedback="false" class="mt-2">
                  <n-form-item label="Discord Bot Token">
                    <n-input
                      v-model:value="discordToken"
                      :type="discordShowToken ? 'text' : 'password'"
                      placeholder="MTIzNDU2Nzg5MD... (输入完整的 Discord Bot 秘钥)"
                    >
                      <template #suffix>
                        <button 
                          @click="discordShowToken = !discordShowToken" 
                          class="text-gray-400 hover:text-white cursor-pointer"
                          type="button"
                        >
                          <Eye v-if="!discordShowToken" class="w-4 h-4" />
                          <EyeOff v-else class="w-4 h-4" />
                        </button>
                      </template>
                    </n-input>
                  </n-form-item>
                </n-form>

                <div class="flex gap-3 mt-4">
                  <n-button 
                    type="primary" 
                    @click="verifyDiscordBot" 
                    :loading="discordVerifyLoading"
                    class="cursor-pointer"
                  >
                    <span>测试 Bot 鉴权</span>
                  </n-button>
                  
                  <n-button 
                    type="default" 
                    @click="fetchDiscordGuilds" 
                    :loading="discordGuildsLoading"
                    class="cursor-pointer"
                  >
                    <span>拉取加入的服务器</span>
                  </n-button>
                </div>
              </div>

              <!-- Output Status Card -->
              <div class="w-full md:w-[320px] bg-black/20 border border-white/5 border-dashed rounded-2xl p-5 flex flex-col justify-center select-none">
                <div class="flex items-center gap-3.5 mb-4">
                  <div class="w-10 h-10 rounded-xl flex items-center justify-center bg-white/5 border border-white/5">
                    <ShieldCheck v-if="discordStatus === 'success'" class="w-5 h-5 text-green-400" />
                    <ShieldAlert v-else-if="discordStatus === 'error'" class="w-5 h-5 text-red-400" />
                    <HelpCircle v-else class="w-5 h-5 text-gray-500" />
                  </div>
                  <div>
                    <h4 class="text-sm font-bold text-white">
                      <span v-if="discordStatus === 'success'">校验成功</span>
                      <span v-else-if="discordStatus === 'error'">校验失败</span>
                      <span v-else>待进行测试</span>
                    </h4>
                    <p class="text-xs text-gray-400">
                      <span v-if="discordStatus === 'success'">Bot 在线认证正常</span>
                      <span v-else-if="discordStatus === 'error'">接口连接异常</span>
                      <span v-else>请输入 Token 发起校验</span>
                    </p>
                  </div>
                </div>

                <div v-if="discordStatus === 'success'" class="flex flex-col gap-3 pt-3 border-t border-white/5 text-xs select-text">
                  <div class="flex justify-between">
                    <span class="text-gray-500">Bot ID</span>
                    <span class="text-white font-mono font-bold">{{ discordBotProfile.id }}</span>
                  </div>
                  <div class="flex justify-between">
                    <span class="text-gray-500">Bot 昵称</span>
                    <span class="text-white font-semibold">{{ discordBotProfile.name }}</span>
                  </div>
                  <div class="flex justify-between">
                    <span class="text-gray-500">用户名/标识</span>
                    <span class="text-cyan-400 font-mono">{{ discordBotProfile.tag }}</span>
                  </div>
                </div>
                <div v-else-if="discordStatus === 'error'" class="text-xs text-red-400 bg-red-950/20 border border-red-500/10 p-3 rounded-xl break-all select-text">
                  错误日志: {{ discordErrorMsg }}
                </div>
              </div>

            </div>
          </n-card>

          <!-- Resolved Guilds list -->
          <n-card v-if="discordGuilds.length > 0" class="border border-white/5">
            <h3 class="text-sm font-bold text-white flex items-center gap-2 mb-1.5">
              <Server class="w-4 h-4 text-cyan-400" />
              Bot 加入的 Discord 服务器列表 (Guilds)
            </h3>
            <p class="text-xs text-gray-400 mb-4">
              Bot 具有管理员或普通读取权限的服务器。点击复制 ID 用于设置 Discord 默认存储目标。
            </p>

            <n-grid cols="1 s:2 m:3" :x-gap="12" :y-gap="12">
              <n-gi v-for="guild in discordGuilds" :key="guild.id">
                <div 
                  @click="copyToClipboard(guild.id)"
                  class="bg-white/5 border border-white/5 hover:border-cyan-500/30 p-4 rounded-xl flex items-center justify-between cursor-pointer group transition"
                >
                  <div class="flex flex-col gap-1 min-w-0">
                    <span class="text-xs font-bold text-white truncate">{{ guild.name }}</span>
                    <span class="text-[10px] font-mono text-gray-500 truncate">{{ guild.id }}</span>
                  </div>
                  <Copy class="w-4 h-4 text-gray-600 group-hover:text-white transition" />
                </div>
              </n-gi>
            </n-grid>
          </n-card>
        </div>
      </n-tab-pane>

    </n-tabs>
  </div>
</template>
