<!--
web/src/views/Login.vue
页面：登录与注册
职责：
- 提供账号注册与登录表单切换及字段前端基础校验
- 调度 Pinia 用户状态存储完成鉴权并存证本地令牌
- 登录/注册成功后平滑重定向至大厅页面
-->

<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useUserStore } from '../stores/user'

const router = useRouter()
const userStore = useUserStore()

const mode = ref('login')
const username = ref('')
const password = ref('')
const email = ref('')
const code = ref('')
const loading = ref(false)

// 系统注册策略配置
const allowRegistration = ref(true)
const requireEmail = ref(false)
const minPasswordLength = ref(6)

// 验证码倒计时
const countdown = ref(0)
const codeSending = ref(false)
let countdownTimer = null

async function fetchConfig() {
  try {
    const res = await fetch('/api/config')
    const data = await res.json()
    allowRegistration.value = data.allowRegistration !== false
    requireEmail.value = !!data.requireEmail
    minPasswordLength.value = data.minPasswordLength || 6
  } catch {}
}

const emailRe = /^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$/

async function sendCode() {
  const em = email.value.trim()
  if (!em) {
    ElMessage.warning('请先输入电子邮箱')
    return
  }
  if (!emailRe.test(em)) {
    ElMessage.warning('电子邮箱格式不正确')
    return
  }
  codeSending.value = true
  try {
    const res = await fetch('/api/send-code', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email: em })
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '验证码发送失败')
    ElMessage.success('验证码已发送至邮箱，请查收')

    countdown.value = 60
    if (countdownTimer) clearInterval(countdownTimer)
    countdownTimer = setInterval(() => {
      countdown.value--
      if (countdown.value <= 0) {
        clearInterval(countdownTimer)
        countdownTimer = null
      }
    }, 1000)
  } catch (err) {
    ElMessage.error(err.message)
  } finally {
    codeSending.value = false
  }
}

async function submit() {
  const u = username.value.trim()
  const p = password.value
  if (!u || !p) {
    ElMessage.warning('请输入用户名和密码')
    return
  }

  if (mode.value === 'register') {
    if (!allowRegistration.value) {
      ElMessage.warning('当前棋苑暂未开放新棋友公开注册')
      return
    }
    if (p.length < minPasswordLength.value) {
      ElMessage.warning(`注册密码长度需至少 ${minPasswordLength.value} 位`)
      return
    }
    if (requireEmail.value) {
      if (!email.value.trim() || !code.value.trim()) {
        ElMessage.warning('请填写电子邮箱并输入 6 位验证码')
        return
      }
    }
  }

  loading.value = true
  try {
    await userStore.authenticate(mode.value, u, p, email.value.trim(), code.value.trim())
    ElMessage.success(mode.value === 'login' ? '欢迎回来，棋友！' : '注册成功，开始对弈吧！')
    router.push('/')
  } catch (e) {
    ElMessage.error(e.message)
  } finally {
    loading.value = false
  }
}

onMounted(() => {
  fetchConfig()
})
</script>

<template>
  <div class="login-wrap">
    <div class="login-card wood-panel">
      <div class="card-head">
        <h2 class="font-brush">{{ mode === 'login' ? '棋苑登录' : '新友注册' }}</h2>
        <p>楚河两岸，以棋会友</p>
      </div>

      <div v-if="mode === 'register' && !allowRegistration" class="reg-closed-alert">
        <p class="font-brush closed-title">暂 未 开 放 注 册</p>
        <p class="closed-desc">棋苑通规现已临时关闭公开注册，请联系督抚或以游客身份入苑观战。</p>
        <el-button type="primary" plain @click="mode = 'login'">返回登录</el-button>
      </div>

      <el-form v-else @submit.prevent="submit">
        <el-form-item>
          <el-input
            v-model="username"
            size="large"
            placeholder="用户名（3-20 位中文/字母/数字）"
            clearable
          />
        </el-form-item>

        <!-- 注册且开启邮箱验证 -->
        <template v-if="mode === 'register' && requireEmail">
          <el-form-item>
            <div class="email-row">
              <el-input
                v-model="email"
                size="large"
                placeholder="电子邮箱（接收验证码）"
                clearable
              />
              <el-button
                type="warning"
                size="large"
                class="code-btn"
                :disabled="countdown > 0"
                :loading="codeSending"
                @click="sendCode"
              >
                {{ countdown > 0 ? `${countdown}s 后重发` : '获取验证码' }}
              </el-button>
            </div>
          </el-form-item>

          <el-form-item>
            <el-input
              v-model="code"
              size="large"
              placeholder="请输入 6 位邮件验证码"
              maxlength="6"
              clearable
            />
          </el-form-item>
        </template>

        <el-form-item>
          <el-input
            v-model="password"
            size="large"
            type="password"
            :placeholder="mode === 'register' ? `密码（至少 ${minPasswordLength} 位）` : '密码（6-32 位）'"
            show-password
            @keyup.enter="submit"
          />
        </el-form-item>

        <el-button
          type="primary"
          size="large"
          class="submit-btn"
          :loading="loading"
          @click="submit"
        >
          {{ mode === 'login' ? '登 录' : '注 册' }}
        </el-button>
      </el-form>

      <div class="switch-mode">
        <span v-if="mode === 'login'">
          还没有账号？<el-link type="primary" @click="mode = 'register'">立即注册</el-link>
        </span>
        <span v-else>
          已有账号？<el-link type="primary" @click="mode = 'login'">返回登录</el-link>
        </span>
      </div>

      <div class="guest-tip">
        <el-link @click="router.push('/')">先以游客身份逛逛（可观战）</el-link>
      </div>
    </div>
  </div>
</template>

<style scoped>
.login-wrap {
  display: grid;
  place-items: center;
  min-height: calc(100vh - 160px);
  padding: 20px;
}

.login-card {
  width: 100%;
  max-width: 440px;
  padding: 34px 38px;
}

.card-head {
  text-align: center;
  margin-bottom: 24px;
}

.card-head h2 {
  font-size: 30px;
  color: #7a2415;
  letter-spacing: 4px;
  margin: 0 0 6px;
}

.card-head p {
  color: #8c6a38;
  font-size: 13px;
  letter-spacing: 2px;
  margin: 0;
}

.email-row {
  display: flex;
  gap: 10px;
  width: 100%;
}

.code-btn {
  flex-shrink: 0;
  min-width: 120px;
}

.submit-btn {
  width: 100%;
  letter-spacing: 8px;
  font-size: 17px;
  margin-top: 8px;
}

.switch-mode,
.guest-tip {
  text-align: center;
  margin-top: 16px;
  font-size: 14px;
  color: #6b4a24;
}

.guest-tip {
  margin-top: 10px;
  font-size: 13px;
}

.reg-closed-alert {
  text-align: center;
  padding: 20px 0;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 12px;
}

.closed-title {
  font-size: 24px;
  color: #7a2415;
  margin: 0;
}

.closed-desc {
  font-size: 13px;
  color: #8c6a38;
  line-height: 1.6;
  margin: 0;
}
</style>
