<script setup lang="ts">
import { ref, inject } from 'vue'
import { apiRequest } from '../api'
import WorkerManager from './WorkerManager.vue'

const showToast = inject<(msg: string, type?: 'success' | 'error') => void>('showToast', () => {})

const verifierTab = ref('tg')

// TG state
const tgBotTokenInput = ref(localStorage.getItem('media_gateway_tg_bot_token') || '')
const tgChatIdInput = ref(localStorage.getItem('media_gateway_tg_chat_id') || '')
const tgShowToken = ref(false)
const tgVerifyLoading = ref(false)
const tgChatsLoading = ref(false)
const tgStatus = ref<'idle' | 'success' | 'error'>('idle')
const tgStatusTitle = ref('未连接测试')
const tgStatusDesc = ref('请点击左侧按钮进行通讯测试')
const tgErrorMsg = ref('')
const tgBotProfile = ref({
  id: '',
  name: '',
  username: ''
})
const tgChats = ref<{ id: number; title: string; type: string }[]>([])

// Discord state
const discordBotTokenInput = ref(localStorage.getItem('media_gateway_discord_bot_token') || '')
const discordGuildIdInput = ref(localStorage.getItem('media_gateway_discord_guild_id') || '')
const discordShowToken = ref(false)
const discordVerifyLoading = ref(false)
const discordGuildsLoading = ref(false)
const discordStatus = ref<'idle' | 'success' | 'error'>('idle')
const discordStatusTitle = ref('未连接测试')
const discordStatusDesc = ref('请输 Token 并测试')
const discordErrorMsg = ref('')
const discordBotProfile = ref({
  id: '',
  name: '',
  tag: ''
})
const discordGuilds = ref<{ id: string; name: string; permissions?: string }[]>([])

function switchVerifierTab(tab: string) {
  verifierTab.value = tab
}

function togglePasswordVisibility(type: 'tg' | 'discord') {
  if (type === 'tg') {
    tgShowToken.value = !tgShowToken.value
  } else {
    discordShowToken.value = !discordShowToken.value
  }
}

// TG Verify
async function verifyTelegramBot() {
  tgVerifyLoading.value = true
  tgStatus.value = 'idle'
  tgStatusTitle.value = '正在验证...'
  tgStatusDesc.value = '请稍候...'
  tgErrorMsg.value = ''
  
  try {
    const data = await apiRequest('/api/v1/provider/telegram/verify', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ token: tgBotTokenInput.value.trim() })
    })
    
    if (data.ok && data.bot_info) {
      tgStatus.value = 'success'
      tgStatusTitle.value = '验证成功！'
      tgStatusDesc.value = 'Telegram 机器人连接正常'
      tgBotProfile.value = {
        id: data.bot_info.id || '--',
        name: data.bot_info.first_name || '--',
        username: '@' + (data.bot_info.username || '--')
      }
      showToast('Telegram 机器人验证成功！')
      // Automatically load active group list upon successful verification
      fetchTelegramChatIDsPost()
    } else {
      tgStatus.value = 'error'
      tgStatusTitle.value = '验证失败'
      tgStatusDesc.value = data.error || '无效的 Token 或网络连接失败'
      showToast(data.error || '验证失败', 'error')
    }
  } catch (err: any) {
    tgStatus.value = 'error'
    tgStatusTitle.value = '异常错误'
    tgStatusDesc.value = err.message
    showToast(err.message || '请求失败', 'error')
  } finally {
    tgVerifyLoading.value = false
  }
}

// TG Get Chats
async function fetchTelegramChatIDsPost() {
  tgChatsLoading.value = true
  tgChats.value = []
  
  try {
    const data = await apiRequest('/api/v1/provider/telegram/chat-ids', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ token: tgBotTokenInput.value.trim() })
    })
    
    if (Array.isArray(data)) {
      tgChats.value = data
      showToast('获取 Telegram 群组列表成功！')
    } else {
      showToast(data.error || '获取失败', 'error')
    }
  } catch (err: any) {
    showToast(err.message || '请求异常', 'error')
  } finally {
    tgChatsLoading.value = false
  }
}

// Discord Verify
async function verifyDiscordBot() {
  const token = discordBotTokenInput.value.trim()
  if (!token) {
    showToast('请输入 Discord Bot Token', 'error')
    return
  }

  discordVerifyLoading.value = true
  discordStatus.value = 'idle'
  discordStatusTitle.value = '正在验证...'
  discordStatusDesc.value = '请稍候...'
  discordErrorMsg.value = ''
  
  try {
    const data = await apiRequest('/api/v1/provider/discord/verify', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ token: token })
    })
    
    if (data.ok && data.bot_info) {
      discordStatus.value = 'success'
      discordStatusTitle.value = '验证成功！'
      discordStatusDesc.value = 'Discord 机器人授权成功'
      discordBotProfile.value = {
        id: data.bot_info.id || '--',
        name: data.bot_info.username || '--',
        tag: data.bot_info.username + '#' + (data.bot_info.discriminator || '0000')
      }
      showToast('Discord 机器人鉴权验证成功！')
    } else {
      discordStatus.value = 'error'
      discordStatusTitle.value = '验证失败'
      discordStatusDesc.value = data.error || '无效的 Token 或网络连接失败'
      showToast(data.error || '验证失败', 'error')
    }
  } catch (err: any) {
    discordStatus.value = 'error'
    discordStatusTitle.value = '异常错误'
    discordStatusDesc.value = err.message
    showToast(err.message || '请求失败', 'error')
  } finally {
    discordVerifyLoading.value = false
  }
}

// Discord Get Guilds
async function fetchDiscordGuilds() {
  const token = discordBotTokenInput.value.trim()
  if (!token) {
    showToast('请输入 Discord Bot Token', 'error')
    return
  }

  discordGuildsLoading.value = true
  discordGuilds.value = []
  
  try {
    const data = await apiRequest('/api/v1/provider/discord/guilds', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ token: token })
    })
    
    if (Array.isArray(data)) {
      discordGuilds.value = data
      showToast('拉取 Discord 服务器成功！')
    } else {
      showToast(data.error || '拉取失败', 'error')
    }
  } catch (err: any) {
    showToast(err.message || '请求异常', 'error')
  } finally {
    discordGuildsLoading.value = false
  }
}

function saveTelegramConfig() {
  const token = tgBotTokenInput.value.trim()
  const chatId = tgChatIdInput.value.trim()
  
  localStorage.setItem('media_gateway_tg_bot_token', token)
  localStorage.setItem('media_gateway_tg_chat_id', chatId)
  showToast('Telegram 机器人配置保存成功！')
}

function saveDiscordConfig() {
  const token = discordBotTokenInput.value.trim()
  const guildId = discordGuildIdInput.value.trim()
  
  localStorage.setItem('media_gateway_discord_bot_token', token)
  localStorage.setItem('media_gateway_discord_guild_id', guildId)
  showToast('Discord 机器人配置保存成功！')
}

function copyText(txt: string) {
  navigator.clipboard.writeText(txt).then(() => {
    showToast('已成功复制到剪贴板！')
  }).catch(() => {
    showToast('复制失败，请手动选择复制', 'error')
  })
}
</script>

<template>
  <div class="panel-view active" id="panel_verifier">
    <div class="tab-nav">
      <button :class="['tab-btn', { active: verifierTab === 'tg' }]" @click="switchVerifierTab('tg')">Telegram 机器人验证</button>
      <button :class="['tab-btn', { active: verifierTab === 'discord' }]" @click="switchVerifierTab('discord')">Discord 机器人验证</button>
      <button :class="['tab-btn', { active: verifierTab === 'worker' }]" @click="switchVerifierTab('worker')">CF Worker 代理验证</button>
    </div>

    <!-- TG Tab Content -->
    <div v-if="verifierTab === 'tg'" style="display: flex; flex-direction: column; gap: 24px;">
      <div class="m3-card">
        <h2 class="section-title">
          <span class="material-symbols-rounded" style="color: #229ED9;">send</span>
          Telegram Bot 在线校验与 Group ID 获取
        </h2>
        <p style="font-size: 13px; color: hsl(var(--md-sys-color-on-surface-variant)); margin-bottom: 24px;">
          输入您想要调试的 Telegram 机器人 Token。我们将调用 <code>getMe</code> 接口校验其真实性，并可通过 <code>getUpdates</code> 拉取最新加入的群组或频道。
        </p>

        <div class="m3-grid-2">
          <div>
            <div class="form-field">
              <label>Telegram Bot Token</label>
              <div class="input-wrapper">
                <input v-model="tgBotTokenInput" :type="tgShowToken ? 'text' : 'password'" placeholder="123456789:ABCDefGhIjKlMnOpQrStUvWxYz" />
                <button class="input-icon-btn" @click="togglePasswordVisibility('tg')">
                  <span class="material-symbols-rounded">{{ tgShowToken ? 'visibility_off' : 'visibility' }}</span>
                </button>
              </div>
              <span style="font-size: 11px; color: hsl(var(--md-sys-color-on-surface-variant)); margin-top: 4px;">为空则默认加载后端的 <code>TELEGRAM_BOT_TOKEN</code> 环境变量</span>
            </div>

            <div class="form-field" style="margin-top: 16px;">
              <label>默认 Chat ID / 群组 ID</label>
              <div class="input-wrapper" style="display: flex; gap: 10px;">
                <input v-model="tgChatIdInput" type="text" placeholder="例如：-1001234567890" style="flex: 1;" />
                <select 
                  v-if="tgChats.length > 0"
                  @change="(e) => { tgChatIdInput = (e.target as HTMLSelectElement).value }"
                  :value="tgChats.some(c => String(c.id) === tgChatIdInput) ? tgChatIdInput : ''"
                  style="width: 200px; background: rgba(255, 255, 255, 0.04); border: 1px solid rgba(255, 255, 255, 0.1); border-radius: 12px; color: #fff; padding: 12px; font-size: 14px; outline: none;"
                >
                  <option value="" style="background: #1e293b;">-- 选择群组 --</option>
                  <option v-for="chat in tgChats" :key="chat.id" :value="String(chat.id)" style="background: #1e293b;">
                    {{ chat.title }} ({{ chat.type }})
                  </option>
                </select>
              </div>
              <span style="font-size: 11px; color: hsl(var(--md-sys-color-on-surface-variant)); margin-top: 4px;">为空则默认加载后端 <code>TELEGRAM_CHAT_ID</code> 环境变量</span>
            </div>

            <div style="display: flex; gap: 12px; margin-top: 24px; flex-wrap: wrap;">
              <button class="m3-btn m3-btn-primary" @click="verifyTelegramBot" :disabled="tgVerifyLoading">
                <span class="material-symbols-rounded">verified_user</span>
                <span>{{ tgVerifyLoading ? '验证中...' : '测试并获取群列表' }}</span>
              </button>
              <button class="m3-btn m3-btn-secondary" @click="fetchTelegramChatIDsPost" :disabled="tgChatsLoading">
                <span class="material-symbols-rounded">groups</span>
                <span>{{ tgChatsLoading ? '获取中...' : '重新获取群列表' }}</span>
              </button>
              <button class="m3-btn m3-btn-secondary" @click="saveTelegramConfig">
                <span class="material-symbols-rounded">save</span>
                <span>保存配置</span>
              </button>
            </div>
          </div>

          <!-- Bot Info Output Card -->
          <div style="background: rgba(0,0,0,0.15); border-radius: 20px; padding: 20px; border: 1px dashed rgba(255,255,255,0.06); display: flex; flex-direction: column; justify-content: center;">
            <div style="display: flex; align-items: center; gap: 12px; margin-bottom: 16px;">
              <span :class="['material-symbols-rounded', tgVerifyLoading ? 'spin-icon' : '']" :style="{ fontSize: '28px', color: tgStatus === 'success' ? 'hsl(var(--md-sys-color-success))' : tgStatus === 'error' ? 'var(--md-sys-color-error)' : 'rgba(255,255,255,0.2)' }">
                {{ tgStatus === 'success' ? 'check_circle' : tgStatus === 'error' ? 'cancel' : 'help' }}
              </span>
              <div>
                <h4 style="font-size: 15px; font-weight: 600;" :style="{ color: tgStatus === 'success' ? 'hsl(var(--md-sys-color-success))' : tgStatus === 'error' ? 'var(--md-sys-color-error)' : '#fff' }">
                  {{ tgStatusTitle }}
                </h4>
                <p style="font-size: 12px; color: hsl(var(--md-sys-color-on-surface-variant));">{{ tgStatusDesc }}</p>
              </div>
            </div>

            <div class="bot-details-grid" id="tg_bot_details" v-if="tgStatus === 'success'">
              <div class="bot-detail-item">
                <div class="bot-detail-label">Bot ID</div>
                <div class="bot-detail-value">{{ tgBotProfile.id }}</div>
              </div>
              <div class="bot-detail-item">
                <div class="bot-detail-label">账号昵称</div>
                <div class="bot-detail-value">{{ tgBotProfile.name }}</div>
              </div>
              <div class="bot-detail-item">
                <div class="bot-detail-label">用户名</div>
                <div class="bot-detail-value">{{ tgBotProfile.username }}</div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- TG Chats list output -->
      <div class="m3-card" id="tg_chats_card" v-if="tgChats.length > 0 || tgChatsLoading">
        <h3 style="font-size: 15px; font-weight: 600; margin-bottom: 12px; display: flex; align-items: center; gap: 8px;">
          <span class="material-symbols-rounded" style="color: hsl(var(--md-sys-color-primary));">groups</span>
          检测到的最新互动群组 / 频道 (Chat IDs)
        </h3>
        <p style="font-size: 13px; color: hsl(var(--md-sys-color-on-surface-variant)); margin-bottom: 16px;">
          注意：Bot 只能拉取到最近 24 小时内有新消息的群组。请在群组中艾特 Bot 发送测试消息，然后再次点击刷新。
        </p>
        
        <div class="chat-grid" id="tg_chats_container">
          <div v-for="chat in tgChats" :key="chat.id" class="chat-card">
            <div class="chat-card-info">
              <div class="chat-card-title">{{ chat.title }}</div>
              <div class="chat-card-id">ID: <code>{{ chat.id }}</code></div>
              <div style="font-size:11px; color:hsl(var(--md-sys-color-primary)); margin-top:2px; text-transform: capitalize;">类型: {{ chat.type }}</div>
            </div>
            <div style="display: flex; gap: 8px;">
              <button class="m3-btn m3-btn-secondary m3-btn-sm" style="padding: 6px 12px;" @click="tgChatIdInput = String(chat.id); showToast('已成功填入此群组 ID，请记得保存！')">选用</button>
              <button class="m3-btn m3-btn-secondary m3-btn-sm" style="padding: 6px 12px;" @click="copyText(String(chat.id))">复制 ID</button>
            </div>
          </div>
          <p v-if="tgChats.length === 0 && !tgChatsLoading" style="grid-column: 1/-1; text-align:center; color:hsl(var(--md-sys-color-on-surface-variant)); padding: 20px 0;">
            未检测到最新交互。请先向 Bot 所在的群组发送消息，然后重试。
          </p>
          <p v-if="tgChatsLoading" style="grid-column: 1/-1; text-align:center; color:hsl(var(--md-sys-color-on-surface-variant));">正在拉取，请确保有群组最近发过消息...</p>
        </div>
      </div>
    </div>

    <!-- Discord Tab Content -->
    <div v-if="verifierTab === 'discord'" style="display: flex; flex-direction: column; gap: 24px;">
      <div class="m3-card">
        <h2 class="section-title">
          <span class="material-symbols-rounded" style="color: #5865F2;">forum</span>
          Discord Bot 鉴权有效性及服务器 (Guild) ID 获取
        </h2>
        <p style="font-size: 13px; color: hsl(var(--md-sys-color-on-surface-variant)); margin-bottom: 24px;">
          输入您创建的 Discord 机器人 token。我们将使用 <code>Authorization: Bot &lt;token&gt;</code> 调用 Discord API 校验状态，并能列出该 Bot 目前加入的所有服务器（Guilds）。
        </p>

        <div class="m3-grid-2">
          <div>
            <div class="form-field">
              <label>Discord Bot Token</label>
              <div class="input-wrapper">
                <input v-model="discordBotTokenInput" :type="discordShowToken ? 'text' : 'password'" placeholder="MTIzNDU2Nzg5MD... (输入完整的 Discord Bot 秘钥)" />
                <button class="input-icon-btn" @click="togglePasswordVisibility('discord')">
                  <span class="material-symbols-rounded">{{ discordShowToken ? 'visibility_off' : 'visibility' }}</span>
                </button>
              </div>
            </div>

            <div class="form-field" style="margin-top: 16px;">
              <label>默认 Guild ID / 服务器 ID</label>
              <div class="input-wrapper">
                <input v-model="discordGuildIdInput" type="text" placeholder="例如：123456789012345678" />
              </div>
              <span style="font-size: 11px; color: hsl(var(--md-sys-color-on-surface-variant)); margin-top: 4px;">为空则默认加载后端 <code>DISCORD_GUILD_ID</code> 环境变量</span>
            </div>

            <div style="display: flex; gap: 12px; margin-top: 24px; flex-wrap: wrap;">
              <button class="m3-btn m3-btn-primary" @click="verifyDiscordBot" :disabled="discordVerifyLoading">
                <span class="material-symbols-rounded">verified_user</span>
                <span>{{ discordVerifyLoading ? '验证中...' : '测试 Bot 鉴权' }}</span>
              </button>
              <button class="m3-btn m3-btn-secondary" @click="fetchDiscordGuilds" :disabled="discordGuildsLoading">
                <span class="material-symbols-rounded">dns</span>
                <span>{{ discordGuildsLoading ? '拉取中...' : '拉取加入的服务器' }}</span>
              </button>
              <button class="m3-btn m3-btn-secondary" @click="saveDiscordConfig">
                <span class="material-symbols-rounded">save</span>
                <span>保存配置</span>
              </button>
            </div>
          </div>

          <!-- Bot Info Output Card -->
          <div style="background: rgba(0,0,0,0.15); border-radius: 20px; padding: 20px; border: 1px dashed rgba(255,255,255,0.06); display: flex; flex-direction: column; justify-content: center;">
            <div style="display: flex; align-items: center; gap: 12px; margin-bottom: 16px;">
              <span :class="['material-symbols-rounded', discordVerifyLoading ? 'spin-icon' : '']" :style="{ fontSize: '28px', color: discordStatus === 'success' ? 'hsl(var(--md-sys-color-success))' : discordStatus === 'error' ? 'var(--md-sys-color-error)' : 'rgba(255,255,255,0.2)' }">
                {{ discordStatus === 'success' ? 'check_circle' : discordStatus === 'error' ? 'cancel' : 'help' }}
              </span>
              <div>
                <h4 style="font-size: 15px; font-weight: 600;" :style="{ color: discordStatus === 'success' ? 'hsl(var(--md-sys-color-success))' : discordStatus === 'error' ? 'var(--md-sys-color-error)' : '#fff' }">
                  {{ discordStatusTitle }}
                </h4>
                <p style="font-size: 12px; color: hsl(var(--md-sys-color-on-surface-variant));">{{ discordStatusDesc }}</p>
              </div>
            </div>

            <div class="bot-details-grid" id="discord_bot_details" v-if="discordStatus === 'success'">
              <div class="bot-detail-item">
                <div class="bot-detail-label">应用 (ID)</div>
                <div class="bot-detail-value">{{ discordBotProfile.id }}</div>
              </div>
              <div class="bot-detail-item">
                <div class="bot-detail-label">Bot 昵称</div>
                <div class="bot-detail-value">{{ discordBotProfile.name }}</div>
              </div>
              <div class="bot-detail-item">
                <div class="bot-detail-label">用户名/标识</div>
                <div class="bot-detail-value">{{ discordBotProfile.tag }}</div>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Discord Servers list output -->
      <div class="m3-card" id="discord_guilds_card" v-if="discordGuilds.length > 0 || discordGuildsLoading">
        <h3 style="font-size: 15px; font-weight: 600; margin-bottom: 12px; display: flex; align-items: center; gap: 8px;">
          <span class="material-symbols-rounded" style="color: hsl(var(--md-sys-color-primary));">dns</span>
          Bot 加入的 Discord 服务器列表 (Guilds)
        </h3>
        <p style="font-size: 13px; color: hsl(var(--md-sys-color-on-surface-variant)); margin-bottom: 16px;">
          Bot 具有管理员或普通读取权限的服务器。点击复制 ID 用于设置 Discord 默认存储目标。
        </p>
        
        <div class="chat-grid" id="discord_guilds_container">
          <div v-for="guild in discordGuilds" :key="guild.id" class="chat-card">
            <div class="chat-card-info">
              <div class="chat-card-title">{{ guild.name }}</div>
              <div class="chat-card-id">Guild ID: <code>{{ guild.id }}</code></div>
              <div style="font-size:11px; color:hsl(var(--md-sys-color-secondary)); margin-top:2px;">权限权重: {{ guild.permissions || '默认' }}</div>
            </div>
            <div style="display: flex; gap: 8px;">
              <button class="m3-btn m3-btn-secondary m3-btn-sm" style="padding: 6px 12px;" @click="discordGuildIdInput = String(guild.id); showToast('已成功填入此服务器 ID，请记得保存！')">选用</button>
              <button class="m3-btn m3-btn-secondary m3-btn-sm" style="padding: 6px 12px;" @click="copyText(guild.id)">复制 ID</button>
            </div>
          </div>
          <p v-if="discordGuilds.length === 0 && !discordGuildsLoading" style="grid-column: 1/-1; text-align:center; color:hsl(var(--md-sys-color-on-surface-variant)); padding: 20px 0;">
            该机器人目前未加入任何服务器，请先邀请机器人到您的 Discord 服务器中。
          </p>
          <p v-if="discordGuildsLoading" style="grid-column: 1/-1; text-align:center; color:hsl(var(--md-sys-color-on-surface-variant));">正在拉取加入的服务器...</p>
        </div>
      </div>
    </div>

    <!-- Cloudflare Worker Tab Content -->
    <div v-if="verifierTab === 'worker'" style="display: flex; flex-direction: column; gap: 24px;">
      <WorkerManager />
    </div>
  </div>
</template>

<style scoped>
.spin-icon {
  animation: spin 2s linear infinite;
}
</style>
