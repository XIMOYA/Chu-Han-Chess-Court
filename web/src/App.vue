<script setup>
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useUserStore } from './stores/user'

const user = useUserStore()
const router = useRouter()

onMounted(() => {
  if (user.isLogin) user.fetchMe()
})

function logout() {
  user.logout()
  router.push('/login')
}
</script>

<template>
  <div class="app-shell">
    <header class="topbar">
      <div class="brand" @click="router.push('/')">
        <span class="seal">楚</span>
        <div class="brand-text">
          <h1 class="font-brush">楚汉棋苑</h1>
          <span class="subtitle">楚河汉界 · 在线对弈</span>
        </div>
      </div>
      <div class="user-area">
        <el-button size="small" plain @click="router.push('/puzzle')">残局闯关</el-button>
        <template v-if="user.isLogin">
          <span class="welcome">棋友 <b>{{ user.username }}</b></span>
          <el-button v-if="user.isAdmin" size="small" type="warning" plain @click="router.push('/admin')">棋苑督抚</el-button>
          <el-button size="small" plain @click="router.push('/')">大厅</el-button>
          <el-button size="small" type="primary" plain @click="logout">退出</el-button>
        </template>
        <el-button v-else size="small" type="primary" @click="router.push('/login')">
          登录 / 注册
        </el-button>
      </div>
    </header>

    <main class="content">
      <router-view v-slot="{ Component }">
        <transition name="page-fade" mode="out-in">
          <component :is="Component" />
        </transition>
      </router-view>
    </main>
  </div>
</template>

<style scoped>
.app-shell {
  height: 100vh;
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.topbar {
  flex-shrink: 0;
  height: 56px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 22px;
  background: linear-gradient(180deg, rgba(35, 20, 8, 0.95), rgba(28, 16, 7, 0.85));
  border-bottom: 2px solid rgba(201, 162, 75, 0.5);
  box-shadow: 0 4px 18px rgba(0, 0, 0, 0.5);
}

.brand {
  display: flex;
  align-items: center;
  gap: 12px;
  cursor: pointer;
}

.seal {
  display: grid;
  place-items: center;
  width: 44px;
  height: 44px;
  font-size: 26px;
  color: #f5e4c0;
  background: linear-gradient(145deg, #b23a26, #8c2517);
  border: 2px solid #d8b36a;
  border-radius: 8px;
  font-family: 'Ma Shan Zheng', cursive;
  box-shadow: 0 3px 8px rgba(0, 0, 0, 0.5);
}

.brand-text h1 {
  margin: 0;
  font-size: 26px;
  color: #f0d89a;
  letter-spacing: 4px;
  line-height: 1.1;
}

.subtitle {
  font-size: 12px;
  color: #b9a171;
  letter-spacing: 2px;
}

.user-area {
  display: flex;
  align-items: center;
  gap: 10px;
}

.welcome {
  color: #d8c39a;
  font-size: 14px;
}

.content {
  flex: 1;
  width: 100%;
  max-width: 1360px;
  margin: 0 auto;
  padding: 12px 20px;
  min-height: 0;
  display: flex;
  flex-direction: column;
  overflow-y: auto;
}

@media (max-width: 640px) {
  .topbar {
    padding: 8px 12px;
  }
  .brand-text h1 {
    font-size: 20px;
    letter-spacing: 2px;
  }
  .subtitle {
    display: none;
  }
  .seal {
    width: 36px;
    height: 36px;
    font-size: 21px;
  }
  .welcome {
    display: none;
  }
  .content {
    padding: 12px;
  }
}
</style>
