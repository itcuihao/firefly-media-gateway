<script setup lang="ts">
import { ref, inject } from 'vue'
import { setApiToken, apiRequest } from '../api'

const emit = defineEmits(['login-success'])
const showToast = inject<(msg: string, type?: 'success' | 'error') => void>('showToast', () => {})

const password = ref('')
const showPassword = ref(false)
const loading = ref(false)

function togglePasswordVisibility() {
  showPassword.value = !showPassword.value
}

async function handleLogin() {
  const pwd = password.value.trim()
  if (!pwd) {
    showToast('请输入管理员密码！', 'error')
    return
  }

  loading.value = true
  // Temporarily set token to test auth
  setApiToken(pwd)

  try {
    // Attempt a light API request to verify the token
    await apiRequest('/api/v1/media?limit=1')
    showToast('登录成功！')
    emit('login-success')
  } catch (err: any) {
    // Clear token on failure
    setApiToken('')
    showToast(err.message || '密码错误，鉴权失败！', 'error')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <div class="login-container">
    <div class="m3-card login-card">
      <div class="login-header">
        <span class="material-symbols-rounded lock-icon">lock</span>
        <h2>管理员登录</h2>
        <p>请输入网关 Bearer Token 进行后台认证</p>
      </div>

      <form @submit.prevent="handleLogin" class="login-form">
        <div class="form-field">
          <label>安全密码</label>
          <div class="input-wrapper">
            <input 
              v-model="password" 
              :type="showPassword ? 'text' : 'password'" 
              placeholder="请输入管理员密码 / Token" 
              required
              :disabled="loading"
            />
            <button 
              type="button" 
              class="input-icon-btn" 
              @click="togglePasswordVisibility"
              :disabled="loading"
            >
              <span class="material-symbols-rounded">
                {{ showPassword ? 'visibility_off' : 'visibility' }}
              </span>
            </button>
          </div>
        </div>

        <button 
          type="submit" 
          class="m3-btn m3-btn-primary login-btn" 
          :disabled="loading"
        >
          <span v-if="loading" class="material-symbols-rounded rotate-sync">sync</span>
          <span>{{ loading ? '正在验证...' : '确认登录' }}</span>
        </button>
      </form>
    </div>
  </div>
</template>

<style scoped>
.login-container {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: calc(100vh - 140px);
  width: 100%;
}

.login-card {
  width: 100%;
  max-width: 400px;
  background: rgba(19, 27, 32, 0.75);
  border-color: rgba(255, 255, 255, 0.08);
  box-shadow: var(--elevation-3);
  text-align: center;
  padding: 40px 32px;
}

.login-header {
  margin-bottom: 32px;
}

.lock-icon {
  font-size: 48px;
  color: hsl(var(--md-sys-color-primary));
  margin-bottom: 12px;
}

.login-header h2 {
  font-size: 22px;
  font-weight: 700;
  color: #fff;
  margin-bottom: 8px;
}

.login-header p {
  font-size: 13px;
  color: hsl(var(--md-sys-color-on-surface-variant));
}

.login-form {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.login-btn {
  width: 100%;
  margin-top: 12px;
  height: 48px;
  border-radius: 14px;
}

.rotate-sync {
  animation: rotate 1.5s linear infinite;
}

@keyframes rotate {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}
</style>
