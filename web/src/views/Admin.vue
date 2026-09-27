<!--
web/src/views/Admin.vue
页面：棋苑督抚 · 管理员后台
职责：
- 展现全盘统计大盘（总用户、今日新增、对局总览、当前在线与活跃房间）
- 实时监控活跃房间，支持直接进房观战督局与一键强制解散
- 棋友用户列表检索、状态管控（封禁/解封）与密码重置
- 历史对局归档记录分页查询与棋谱详情查验
-->

<script setup>
import { onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useUserStore } from '../stores/user'

const router = useRouter()
const userStore = useUserStore()

const activeTab = ref('dashboard')

// 1. 仪表盘数据
const stats = ref(null)
const liveRooms = ref(0)
const liveConns = ref(0)
const loadingStats = ref(false)

async function fetchStats() {
  loadingStats.value = true
  try {
    const res = await fetch('/api/admin/stats', {
      headers: { Authorization: `Bearer ${userStore.token}` }
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '获取大盘失败')
    stats.value = data.stats
    liveRooms.value = data.liveRooms
    liveConns.value = data.liveConns
  } catch (err) {
    ElMessage.error(err.message)
  } finally {
    loadingStats.value = false
  }
}

// 2. 实时房间
const rooms = ref([])
const loadingRooms = ref(false)

async function fetchRooms() {
  loadingRooms.value = true
  try {
    const res = await fetch('/api/admin/rooms', {
      headers: { Authorization: `Bearer ${userStore.token}` }
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '获取房间失败')
    rooms.value = data.rooms || []
  } catch (err) {
    ElMessage.error(err.message)
  } finally {
    loadingRooms.value = false
  }
}

async function closeRoom(code) {
  try {
    await ElMessageBox.confirm(`确定要强制解散房间 ${code} 吗？对局将立即中断并清理。`, '解散确认', {
      confirmButtonText: '强制解散',
      cancelButtonText: '取消',
      type: 'warning'
    })
    const res = await fetch('/api/admin/rooms/close', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${userStore.token}`
      },
      body: JSON.stringify({ code })
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '操作失败')
    ElMessage.success('房间已强制解散')
    fetchRooms()
    fetchStats()
  } catch (e) {
    // canceled
  }
}

function spectateRoom(code) {
  router.push(`/room/${code}?spectate=1`)
}

// 3. 棋友用户管理
const users = ref([])
const usersTotal = ref(0)
const usersPage = ref(1)
const usersQuery = ref('')
const loadingUsers = ref(false)

async function fetchUsers() {
  loadingUsers.value = true
  try {
    const params = new URLSearchParams({
      page: usersPage.value,
      limit: 15,
      query: usersQuery.value
    })
    const res = await fetch(`/api/admin/users?${params}`, {
      headers: { Authorization: `Bearer ${userStore.token}` }
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '获取用户列表失败')
    users.value = data.users || []
    usersTotal.value = data.total || 0
  } catch (err) {
    ElMessage.error(err.message)
  } finally {
    loadingUsers.value = false
  }
}

async function toggleUserStatus(u) {
  const nextStatus = u.status === 'banned' ? 'active' : 'banned'
  const actionText = nextStatus === 'banned' ? '封禁' : '解封'
  try {
    await ElMessageBox.confirm(`确定要${actionText}棋友「${u.username}」吗？`, `${actionText}确认`, {
      confirmButtonText: actionText,
      cancelButtonText: '取消',
      type: nextStatus === 'banned' ? 'danger' : 'info'
    })
    const res = await fetch('/api/admin/users/status', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${userStore.token}`
      },
      body: JSON.stringify({ id: u.id, status: nextStatus })
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '操作失败')
    ElMessage.success(`已成功${actionText}棋友 ${u.username}`)
    fetchUsers()
  } catch (e) {}
}

async function resetPassword(u) {
  try {
    const { value: newPassword } = await ElMessageBox.prompt(
      `请输入棋友「${u.username}」的新密码（至少6位）：`,
      '重置密码',
      {
        confirmButtonText: '确定重置',
        cancelButtonText: '取消',
        inputPattern: /^.{6,32}$/,
        inputErrorMessage: '密码长度需为 6-32 位'
      }
    )
    if (!newPassword) return
    const res = await fetch('/api/admin/users/reset-pwd', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${userStore.token}`
      },
      body: JSON.stringify({ id: u.id, password: newPassword })
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '重置失败')
    ElMessage.success(`棋友 ${u.username} 的密码重置成功`)
  } catch (e) {}
}

// 4. 对局档案
const games = ref([])
const gamesTotal = ref(0)
const gamesPage = ref(1)
const gamesType = ref('')
const gamesQuery = ref('')
const loadingGames = ref(false)

const viewMovesDialog = ref(false)
const viewingGame = ref(null)
const viewingMoves = ref([])

const REASON_CN = {
  checkmate: '将死', stalemate: '困毙', perpetual_check: '长将判负',
  perpetual_chase: '长捉判负', repeat_draw: '双方不变作和', timeout: '超时判负',
  resign: '认输', draw_accepted: '同意和棋', disconnect: '离线判负',
  five_in_a_row: '五子连珠胜', board_full: '满盘作和'
}

async function fetchGames() {
  loadingGames.value = true
  try {
    const params = new URLSearchParams({
      page: gamesPage.value,
      limit: 15,
      gameType: gamesType.value,
      query: gamesQuery.value
    })
    const res = await fetch(`/api/admin/games?${params}`, {
      headers: { Authorization: `Bearer ${userStore.token}` }
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '获取档案失败')
    games.value = data.games || []
    gamesTotal.value = data.total || 0
  } catch (err) {
    ElMessage.error(err.message)
  } finally {
    loadingGames.value = false
  }
}

function openMovesViewer(g) {
  viewingGame.value = g
  try {
    viewingMoves.value = g.moves ? JSON.parse(g.moves) : []
  } catch {
    viewingMoves.value = []
  }
  viewMovesDialog.value = true
}

// 5. 棋苑通规 · 系统设置
const settings = ref({
  allow_registration: 'true',
  require_email: 'false',
  min_password_length: '6',
  smtp_host: '',
  smtp_port: '465',
  smtp_user: '',
  smtp_pass: '',
  smtp_ssl: 'true',
  llm_enabled: 'false',
  llm_base_url: 'https://api.deepseek.com/v1',
  llm_api_key: '',
  llm_model: 'deepseek-chat'
})
const minPasswordNum = ref(6)
const loadingSettings = ref(false)
const savingSettings = ref(false)
const testEmail = ref('')
const testEmailSending = ref(false)

// 大模型配置与统计
const availableModels = ref([])
const fetchingModels = ref(false)
const testLlmRunning = ref(false)
const llmTestResult = ref(null)
const llmStats = ref({
  totalRequests: 0,
  promptTokens: 0,
  completionTokens: 0,
  totalTokens: 0
})
const loadingLlmStats = ref(false)

async function fetchLlmStats() {
  loadingLlmStats.value = true
  try {
    const res = await fetch('/api/admin/llm/stats', {
      headers: { Authorization: `Bearer ${userStore.token}` }
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '获取用量失败')
    if (data.stats) {
      llmStats.value = data.stats
    }
  } catch (err) {
    ElMessage.error(err.message)
  } finally {
    loadingLlmStats.value = false
  }
}

async function resetLlmStats() {
  try {
    await ElMessageBox.confirm('确定要清空大模型的累计调用次数与 Token 用量统计吗？', '重置确认', {
      confirmButtonText: '确定重置',
      cancelButtonText: '取消',
      type: 'warning'
    })
    const res = await fetch('/api/admin/llm/reset-stats', {
      method: 'POST',
      headers: { Authorization: `Bearer ${userStore.token}` }
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '重置失败')
    ElMessage.success('用量统计已成功清零')
    fetchLlmStats()
  } catch (e) {}
}

async function fetchModels() {
  const baseUrl = settings.value.llm_base_url.trim()
  const apiKey = settings.value.llm_api_key.trim()
  if (!baseUrl) {
    ElMessage.warning('请先填写大模型的 API Base URL')
    return
  }
  fetchingModels.value = true
  try {
    const res = await fetch('/api/admin/llm/models', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${userStore.token}`
      },
      body: JSON.stringify({ baseUrl, apiKey })
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '拉取模型列表失败')
    availableModels.value = data.models || []
    ElMessage.success(`成功拉取到 ${availableModels.value.length} 个可用模型`)
  } catch (err) {
    ElMessage.error(err.message)
  } finally {
    fetchingModels.value = false
  }
}

async function testLlm() {
  const baseUrl = settings.value.llm_base_url.trim()
  const apiKey = settings.value.llm_api_key.trim()
  const model = settings.value.llm_model.trim()
  if (!baseUrl) {
    ElMessage.warning('请先填写大模型的 API Base URL')
    return
  }
  testLlmRunning.value = true
  llmTestResult.value = null
  try {
    const res = await fetch('/api/admin/llm/test', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${userStore.token}`
      },
      body: JSON.stringify({ baseUrl, apiKey, model })
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '模型测试失败')
    llmTestResult.value = data
    ElMessage.success(`测试成功！响应耗时 ${data.latencyMs} ms`)
    fetchLlmStats()
  } catch (err) {
    ElMessage.error(err.message)
  } finally {
    testLlmRunning.value = false
  }
}

async function fetchSettings() {
  loadingSettings.value = true
  try {
    const res = await fetch('/api/admin/settings', {
      headers: { Authorization: `Bearer ${userStore.token}` }
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '获取设置失败')
    if (data.settings) {
      settings.value = { ...settings.value, ...data.settings }
      minPasswordNum.value = parseInt(settings.value.min_password_length) || 6
    }
  } catch (err) {
    ElMessage.error(err.message)
  } finally {
    loadingSettings.value = false
  }
}

async function saveSettings() {
  savingSettings.value = true
  settings.value.min_password_length = String(minPasswordNum.value)
  try {
    const res = await fetch('/api/admin/settings', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${userStore.token}`
      },
      body: JSON.stringify({ settings: settings.value })
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '保存失败')
    ElMessage.success('棋苑通规已成功保存更新！')
  } catch (err) {
    ElMessage.error(err.message)
  } finally {
    savingSettings.value = false
  }
}

async function sendTestEmail() {
  const to = testEmail.value.trim()
  if (!to) {
    ElMessage.warning('请输入接收测试信件的邮箱')
    return
  }
  testEmailSending.value = true
  try {
    const res = await fetch('/api/admin/test-email', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${userStore.token}`
      },
      body: JSON.stringify({ to })
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '测试发信失败')
    ElMessage.success(data.message || '测试邮件已投递！')
  } catch (err) {
    ElMessage.error(err.message)
  } finally {
    testEmailSending.value = false
  }
}

// 6. 残局题库管理
const adminPuzzles = ref([])
const adminPuzzlesTotal = ref(0)
const adminPuzzlesPage = ref(1)
const loadingAdminPuzzles = ref(false)
const puzzleEditDialog = ref(false)
const puzzleForm = ref({
  id: 0,
  level: 1,
  title: '',
  description: '',
  source: '',
  fen: '',
  solution: '',
  hint: '',
  difficulty: 'easy'
})

async function fetchAdminPuzzles() {
  loadingAdminPuzzles.value = true
  try {
    const res = await fetch(`/api/admin/puzzles?page=${adminPuzzlesPage.value}&limit=15`, {
      headers: { Authorization: `Bearer ${userStore.token}` }
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '获取题库失败')
    adminPuzzles.value = data.puzzles || []
    adminPuzzlesTotal.value = data.total || 0
  } catch (err) {
    ElMessage.error(err.message)
  } finally {
    loadingAdminPuzzles.value = false
  }
}

function openPuzzleEditor(pz = null) {
  if (pz) {
    puzzleForm.value = { ...pz }
  } else {
    puzzleForm.value = {
      id: 0,
      level: adminPuzzlesTotal.value + 1,
      title: '',
      description: '',
      source: '《适情雅趣》',
      fen: '3ak4/4a4/9/4N4/9/9/9/9/4C4/4K4 w',
      solution: '[{"from":{"r":8,"c":4},"to":{"r":1,"c":4},"notation":"炮五进七"}]',
      hint: '',
      difficulty: 'easy'
    }
  }
  puzzleEditDialog.value = true
}

async function saveAdminPuzzle() {
  try {
    const res = await fetch('/api/admin/puzzles', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${userStore.token}`
      },
      body: JSON.stringify(puzzleForm.value)
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '保存失败')
    ElMessage.success('残局题库已成功保存！')
    puzzleEditDialog.value = false
    fetchAdminPuzzles()
  } catch (err) {
    ElMessage.error(err.message)
  }
}

async function deleteAdminPuzzle(id) {
  try {
    await ElMessageBox.confirm('确定要删除这道残局吗？', '删除确认', {
      confirmButtonText: '删除',
      cancelButtonText: '取消',
      type: 'warning'
    })
    const res = await fetch('/api/admin/puzzles/delete', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        Authorization: `Bearer ${userStore.token}`
      },
      body: JSON.stringify({ id })
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '删除失败')
    ElMessage.success('残局已删除')
    fetchAdminPuzzles()
  } catch (e) {}
}

onMounted(() => {
  if (!userStore.isAdmin) {
    ElMessage.warning('无权访问管理员后台')
    router.push('/')
    return
  }
  fetchStats()
  fetchRooms()
  fetchUsers()
  fetchGames()
  fetchSettings()
  fetchLlmStats()
  fetchAdminPuzzles()
})
</script>

<template>
  <div class="admin-page">
    <div class="admin-header wood-panel">
      <div class="title-line">
        <h2 class="font-brush">棋 苑 督 抚 · 治 理 枢 纽</h2>
        <span class="sub-seal">御赐督司</span>
      </div>
      <p class="desc">实时统摄棋苑全盘指标、监控在线对战擂台、督察棋友行为与翻阅对局史册档案。</p>
    </div>

    <div class="dark-panel admin-main">
      <el-tabs v-model="activeTab" class="admin-tabs">
        <!-- Tab 1: 仪表大盘 -->
        <el-tab-pane label="大盘总览" name="dashboard">
          <div class="pane-wrap">
            <div class="stat-cards" v-if="stats">
              <div class="wood-panel stat-card">
                <span class="sc-title">棋友总籍册</span>
                <b class="sc-val win">{{ stats.totalUsers }}</b>
                <span class="sc-sub">今日入苑：+{{ stats.todayUsers }}</span>
              </div>
              <div class="wood-panel stat-card">
                <span class="sc-title">累计对局</span>
                <b class="sc-val">{{ stats.totalGames }}</b>
                <span class="sc-sub">象棋 {{ stats.xiangqiGames }} · 五子棋 {{ stats.gomokuGames }}</span>
              </div>
              <div class="wood-panel stat-card">
                <span class="sc-title">当前对战擂台</span>
                <b class="sc-val gold">{{ liveRooms }}</b>
                <span class="sc-sub">正在交锋的房间</span>
              </div>
              <div class="wood-panel stat-card">
                <span class="sc-title">实时活跃连接</span>
                <b class="sc-val">{{ liveConns }}</b>
                <span class="sc-sub">WebSocket 长连接</span>
              </div>
            </div>

            <div class="dash-actions">
              <el-button type="primary" :loading="loadingStats" @click="fetchStats">
                刷新大盘数据
              </el-button>
            </div>
          </div>
        </el-tab-pane>

        <!-- Tab 2: 实时房间 -->
        <el-tab-pane label="实时房间监控" name="rooms">
          <div class="pane-wrap">
            <div class="toolbar">
              <el-button size="default" :loading="loadingRooms" @click="fetchRooms">刷新房间</el-button>
              <span class="tip-txt">共 {{ rooms.length }} 个活跃房间</span>
            </div>

            <el-table :data="rooms" v-loading="loadingRooms" style="width: 100%" class="custom-table">
              <el-table-column prop="code" label="房间号" width="100">
                <template #default="{ row }">
                  <span class="code-badge font-brush">{{ row.code }}</span>
                </template>
              </el-table-column>
              <el-table-column prop="gameType" label="玩法" width="100">
                <template #default="{ row }">
                  <el-tag :type="row.gameType === 'gomoku' ? 'success' : 'danger'" size="small">
                    {{ row.gameType === 'gomoku' ? '五子棋' : '中国象棋' }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="status" label="对局状态" width="100">
                <template #default="{ row }">
                  <span v-if="row.status === 'playing'" class="status-tag playing">交锋中</span>
                  <span v-else-if="row.status === 'waiting'" class="status-tag waiting">候弈中</span>
                  <span v-else class="status-tag finished">已终局</span>
                </template>
              </el-table-column>
              <el-table-column label="对弈棋手">
                <template #default="{ row }">
                  <span v-if="row.gameType === 'gomoku'">
                    <b>{{ row.blackName || '虚位' }}</b> (黑) vs <b>{{ row.whiteName || '虚位' }}</b> (白)
                  </span>
                  <span v-else>
                    <b>{{ row.redName || '虚位' }}</b> (红) vs <b>{{ row.blackName || '虚位' }}</b> (黑)
                  </span>
                </template>
              </el-table-column>
              <el-table-column prop="specsCount" label="观战" width="70" align="center" />
              <el-table-column prop="movesCount" label="步数" width="70" align="center" />
              <el-table-column label="操作" width="180" align="center">
                <template #default="{ row }">
                  <el-button size="small" type="primary" plain @click="spectateRoom(row.code)">督战观局</el-button>
                  <el-button size="small" type="danger" plain @click="closeRoom(row.code)">解散</el-button>
                </template>
              </el-table-column>
            </el-table>
          </div>
        </el-tab-pane>

        <!-- Tab 3: 棋友用户管理 -->
        <el-tab-pane label="棋友名册管理" name="users">
          <div class="pane-wrap">
            <div class="toolbar">
              <el-input
                v-model="usersQuery"
                placeholder="按用户名检索…"
                style="width: 220px;"
                clearable
                @keyup.enter="fetchUsers"
              />
              <el-button type="primary" @click="fetchUsers">搜索</el-button>
              <span class="tip-txt">共登记棋友 {{ usersTotal }} 人</span>
            </div>

            <el-table :data="users" v-loading="loadingUsers" style="width: 100%" class="custom-table">
              <el-table-column prop="id" label="编号" width="70" />
              <el-table-column prop="username" label="棋友雅号" width="160">
                <template #default="{ row }">
                  <b>{{ row.username }}</b>
                  <el-tag v-if="row.role === 'admin'" type="warning" size="small" style="margin-left: 6px;">督抚</el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="status" label="状态" width="100">
                <template #default="{ row }">
                  <el-tag :type="row.status === 'banned' ? 'danger' : 'success'" size="small">
                    {{ row.status === 'banned' ? '已封禁' : '正常' }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column label="战绩统计" width="200">
                <template #default="{ row }">
                  {{ row.wins }}胜 {{ row.draws }}和 {{ row.losses }}负
                  <small style="color: #9a7a4a;">
                    ({{ row.total ? Math.round((row.wins / row.total) * 100) : 0 }}%)
                  </small>
                </template>
              </el-table-column>
              <el-table-column prop="email" label="电子邮箱" min-width="160">
                <template #default="{ row }">
                  <span>{{ row.email || '未绑定' }}</span>
                </template>
              </el-table-column>
              <el-table-column prop="createdAt" label="入苑时刻" min-width="150" />
              <el-table-column label="治理操作" width="200" align="center">
                <template #default="{ row }">
                  <el-button
                    size="small"
                    :type="row.status === 'banned' ? 'success' : 'danger'"
                    plain
                    :disabled="row.role === 'admin'"
                    @click="toggleUserStatus(row)"
                  >
                    {{ row.status === 'banned' ? '解封' : '封禁' }}
                  </el-button>
                  <el-button size="small" plain @click="resetPassword(row)">改密</el-button>
                </template>
              </el-table-column>
            </el-table>

            <div class="pagination-bar">
              <el-pagination
                v-model:current-page="usersPage"
                :page-size="15"
                :total="usersTotal"
                layout="prev, pager, next"
                @current-change="fetchUsers"
              />
            </div>
          </div>
        </el-tab-pane>

        <!-- Tab 4: 对局档案 -->
        <el-tab-pane label="对局全卷档案" name="games">
          <div class="pane-wrap">
            <div class="toolbar">
              <el-select v-model="gamesType" placeholder="玩法筛选" style="width: 140px;" clearable @change="fetchGames">
                <el-option label="全部玩法" value="" />
                <el-option label="中国象棋" value="xiangqi" />
                <el-option label="五子棋" value="gomoku" />
              </el-select>
              <el-input
                v-model="gamesQuery"
                placeholder="房间号 / 选手名字…"
                style="width: 220px;"
                clearable
                @keyup.enter="fetchGames"
              />
              <el-button type="primary" @click="fetchGames">检索</el-button>
              <span class="tip-txt">已封存档案 {{ gamesTotal }} 局</span>
            </div>

            <el-table :data="games" v-loading="loadingGames" style="width: 100%" class="custom-table">
              <el-table-column prop="code" label="房间号" width="90" />
              <el-table-column prop="gameType" label="玩法" width="90">
                <template #default="{ row }">
                  <el-tag :type="row.gameType === 'gomoku' ? 'success' : 'danger'" size="small">
                    {{ row.gameType === 'gomoku' ? '五子棋' : '象棋' }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column label="交锋对决" min-width="180">
                <template #default="{ row }">
                  <template v-if="row.gameType === 'gomoku'">
                    <span :class="{ winner: row.result === 'black' }">{{ row.redName }} (黑)</span>
                    vs
                    <span :class="{ winner: row.result === 'white' }">{{ row.blackName }} (白)</span>
                  </template>
                  <template v-else>
                    <span :class="{ winner: row.result === 'red' }">{{ row.redName }} (红)</span>
                    vs
                    <span :class="{ winner: row.result === 'black' }">{{ row.blackName }} (黑)</span>
                  </template>
                </template>
              </el-table-column>
              <el-table-column label="终局判决" width="130">
                <template #default="{ row }">
                  {{ REASON_CN[row.reason] || row.reason }}
                </template>
              </el-table-column>
              <el-table-column prop="endedAt" label="弈毕时刻" width="160" />
              <el-table-column label="操作" width="160" align="center">
                <template #default="{ row }">
                  <el-button size="small" type="primary" plain @click="openMovesViewer(row)">谱面</el-button>
                  <el-button size="small" type="warning" plain @click="router.push('/replay/' + row.id)">复盘</el-button>
                </template>
              </el-table-column>
            </el-table>

            <div class="pagination-bar">
              <el-pagination
                v-model:current-page="gamesPage"
                :page-size="15"
                :total="gamesTotal"
                layout="prev, pager, next"
                @current-change="fetchGames"
              />
            </div>
          </div>
        </el-tab-pane>

        <!-- Tab 5: 系统设置 -->
        <el-tab-pane label="棋苑通规 · 系统设置" name="settings">
          <div class="pane-wrap settings-wrap" v-loading="loadingSettings">
            <!-- 模块 1: 注册准入策略 -->
            <div class="wood-panel settings-card">
              <h3 class="font-brush card-title">入苑准入 · 注册安全策略</h3>
              <div class="setting-item">
                <div class="setting-info">
                  <span class="st-name">开放新棋友公开注册</span>
                  <span class="st-desc">关闭后将暂停接收新用户注册，适合私享或赛事封闭期间使用</span>
                </div>
                <el-switch
                  v-model="settings.allow_registration"
                  active-value="true"
                  inactive-value="false"
                  active-text="开放"
                  inactive-text="关闭"
                />
              </div>

              <div class="setting-item">
                <div class="setting-info">
                  <span class="st-name">强制邮箱验证码校验</span>
                  <span class="st-desc">开启后注册必须填写真实邮箱并通过 6 位验证码核销方可入苑</span>
                </div>
                <el-switch
                  v-model="settings.require_email"
                  active-value="true"
                  inactive-value="false"
                  active-text="开启"
                  inactive-text="免验"
                />
              </div>

              <div class="setting-item">
                <div class="setting-info">
                  <span class="st-name">棋友注册最低密码位数</span>
                  <span class="st-desc">设置新注册账号密码的最少字符长度（范围 6 - 20 位）</span>
                </div>
                <el-input-number
                  v-model="minPasswordNum"
                  :min="6"
                  :max="20"
                  size="default"
                />
              </div>
            </div>

            <!-- 模块 2: SMTP 邮件服务配置 -->
            <div class="wood-panel settings-card">
              <h3 class="font-brush card-title">信函邮驿 · SMTP 邮件服务配置</h3>
              <p class="st-hint">用于向棋友投递入苑验证码、系统通告与安全通知。</p>

              <el-form label-position="top" class="smtp-form">
                <div class="form-grid">
                  <el-form-item label="SMTP 服务器主机（Host）">
                    <el-input v-model="settings.smtp_host" placeholder="例如 smtp.qq.com 或 smtp.163.com" />
                  </el-form-item>
                  <el-form-item label="SMTP 服务端口（Port）">
                    <el-input v-model="settings.smtp_port" placeholder="例如 465 (SSL) 或 587 (TLS)" />
                  </el-form-item>
                </div>

                <div class="form-grid">
                  <el-form-item label="发信人邮箱 / 账号（Username）">
                    <el-input v-model="settings.smtp_user" placeholder="例如 service@ximoya.top" />
                  </el-form-item>
                  <el-form-item label="授权码 / 密码（Password）">
                    <el-input v-model="settings.smtp_pass" type="password" show-password placeholder="邮箱授权码或发信密码" />
                  </el-form-item>
                </div>

                <div class="setting-item inline-switch">
                  <div class="setting-info">
                    <span class="st-name">SSL / TLS 安全加密连接</span>
                    <span class="st-desc">默认 465 端口推荐开启 SSL，587/25 端口建议关闭采用 STARTTLS 协商</span>
                  </div>
                  <el-switch
                    v-model="settings.smtp_ssl"
                    active-value="true"
                    inactive-value="false"
                  />
                </div>
              </el-form>
            </div>

            <!-- 模块 3: 邮件隧道联通测试 -->
            <div class="wood-panel settings-card">
              <h3 class="font-brush card-title">邮驿试剑 · 邮件服务联通测试</h3>
              <p class="st-hint">在正式对外开启验证码前，可向您自己的私人邮箱投递一封测试信函以验证连通性。</p>

              <div class="test-mail-row">
                <el-input
                  v-model="testEmail"
                  placeholder="输入接收测试信函的邮箱（如 your_email@qq.com）…"
                  style="max-width: 380px;"
                />
                <el-button
                  type="warning"
                  :loading="testEmailSending"
                  @click="sendTestEmail"
                >
                  发送测试邮件
                </el-button>
              </div>
            </div>

            <!-- 模块 4: 大语言模型（LLM）配置 -->
            <div class="wood-panel settings-card">
              <h3 class="font-brush card-title">神机妙算 · 大语言模型（LLM）配置</h3>
              <p class="st-hint">兼容所有遵循 OpenAI 标准协议的接口节点（如 DeepSeek、通义千问、Kimi、Ollama 等）。</p>

              <div class="setting-item">
                <div class="setting-info">
                  <span class="st-name">启用大模型互动与解说</span>
                  <span class="st-desc">开启后允许在对局中唤醒大模型进行性格扮演、局势锐评与心声互动</span>
                </div>
                <el-switch
                  v-model="settings.llm_enabled"
                  active-value="true"
                  inactive-value="false"
                  active-text="启用"
                  inactive-text="休眠"
                />
              </div>

              <el-form label-position="top" class="smtp-form">
                <div class="form-grid">
                  <el-form-item label="API 接口地址（Base URL）">
                    <el-input
                      v-model="settings.llm_base_url"
                      placeholder="例如 https://api.deepseek.com/v1 或 http://localhost:11434/v1"
                    />
                  </el-form-item>
                  <el-form-item label="API 密钥（API Key）">
                    <el-input
                      v-model="settings.llm_api_key"
                      type="password"
                      show-password
                      placeholder="sk-..."
                    />
                  </el-form-item>
                </div>

                <div class="form-grid">
                  <el-form-item label="当前选用模型（Model ID）">
                    <div class="model-picker-row">
                      <el-select
                        v-model="settings.llm_model"
                        filterable
                        allow-create
                        default-first-option
                        placeholder="输入或选择模型（如 deepseek-chat）"
                        style="flex: 1;"
                      >
                        <el-option
                          v-for="m in availableModels"
                          :key="m"
                          :label="m"
                          :value="m"
                        />
                      </el-select>
                      <el-button
                        type="warning"
                        plain
                        :loading="fetchingModels"
                        @click="fetchModels"
                      >
                        拉取模型列表
                      </el-button>
                    </div>
                  </el-form-item>

                  <el-form-item label="连通性诊断与对话实测">
                    <el-button
                      type="success"
                      plain
                      class="test-llm-btn"
                      :loading="testLlmRunning"
                      @click="testLlm"
                    >
                      发起模型对话测试
                    </el-button>
                  </el-form-item>
                </div>
              </el-form>

              <!-- 测试回答反馈面板 -->
              <div v-if="llmTestResult" class="llm-result-box">
                <div class="llm-res-header">
                  <span class="res-tag">测试成功</span>
                  <span class="res-latency">响应延时：<b>{{ llmTestResult.latencyMs }} ms</b></span>
                  <span class="res-tokens">本轮 Token：<b>{{ llmTestResult.totalTokens }}</b>（输入 {{ llmTestResult.promptTokens }} + 输出 {{ llmTestResult.completionTokens }}）</span>
                </div>
                <div class="llm-res-content">
                  <b>模型回复：</b>{{ llmTestResult.reply }}
                </div>
              </div>
            </div>

            <!-- 模块 5: 大模型用量统计看板 -->
            <div class="wood-panel settings-card">
              <div class="card-head-between">
                <h3 class="font-brush card-title" style="margin: 0;">研算度支 · Token 与请求用量统计</h3>
                <div class="stats-btns">
                  <el-button size="small" :loading="loadingLlmStats" @click="fetchLlmStats">刷新用量</el-button>
                  <el-button size="small" type="danger" plain @click="resetLlmStats">重置统计</el-button>
                </div>
              </div>
              <p class="st-hint" style="margin-top: 6px;">实时追踪全平台大模型会话调用的请求频次与累计 Token 开销。</p>

              <div class="llm-stat-grid" v-loading="loadingLlmStats">
                <div class="llm-stat-box">
                  <span class="ls-title">累计请求次数</span>
                  <b class="ls-val">{{ llmStats.totalRequests }}</b>
                  <span class="ls-sub">次 API 调用</span>
                </div>
                <div class="llm-stat-box">
                  <span class="ls-title">累计消耗总 Token</span>
                  <b class="ls-val gold">{{ (llmStats.totalTokens || 0).toLocaleString() }}</b>
                  <span class="ls-sub">Tokens 总计</span>
                </div>
                <div class="llm-stat-box">
                  <span class="ls-title">输入 Prompt Token</span>
                  <b class="ls-val">{{ (llmStats.promptTokens || 0).toLocaleString() }}</b>
                  <span class="ls-sub">上行提示词消耗</span>
                </div>
                <div class="llm-stat-box">
                  <span class="ls-title">输出 Completion Token</span>
                  <b class="ls-val win">{{ (llmStats.completionTokens || 0).toLocaleString() }}</b>
                  <span class="ls-sub">下行生成内容消耗</span>
                </div>
              </div>
            </div>

            <!-- 底部保存按钮 -->
            <div class="save-bar">
              <el-button
                type="primary"
                size="large"
                class="save-all-btn"
                :loading="savingSettings"
                @click="saveSettings"
              >
                保 存 棋 苑 通 规
              </el-button>
            </div>
          </div>
        </el-tab-pane>

        <!-- Tab 6: 残局题库 -->
        <el-tab-pane label="经史子集 · 残局题库" name="puzzles">
          <div class="pane-wrap">
            <div class="toolbar">
              <el-button type="primary" @click="openPuzzleEditor()">录入新名谱</el-button>
              <el-button size="default" :loading="loadingAdminPuzzles" @click="fetchAdminPuzzles">刷新题库</el-button>
              <span class="tip-txt">共珍藏名局 {{ adminPuzzlesTotal }} 局</span>
            </div>

            <el-table :data="adminPuzzles" v-loading="loadingAdminPuzzles" style="width: 100%" class="custom-table">
              <el-table-column prop="level" label="关卡" width="70" align="center" />
              <el-table-column prop="title" label="残局雅号" min-width="140">
                <template #default="{ row }">
                  <b>{{ row.title }}</b>
                </template>
              </el-table-column>
              <el-table-column prop="source" label="出处典籍" width="120" />
              <el-table-column prop="difficulty" label="品阶" width="80" align="center">
                <template #default="{ row }">
                  <el-tag size="small" :type="row.difficulty === 'easy' ? 'success' : (row.difficulty === 'medium' ? 'warning' : 'danger')">
                    {{ row.difficulty === 'easy' ? '初学' : (row.difficulty === 'medium' ? '精进' : '大师') }}
                  </el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="hint" label="妙着锦囊" min-width="160" show-overflow-tooltip />
              <el-table-column label="管辖操作" width="180" align="center">
                <template #default="{ row }">
                  <el-button size="small" type="primary" plain @click="openPuzzleEditor(row)">修缮</el-button>
                  <el-button size="small" type="warning" plain @click="router.push('/puzzle')">推演</el-button>
                  <el-button size="small" type="danger" plain @click="deleteAdminPuzzle(row.id)">除名</el-button>
                </template>
              </el-table-column>
            </el-table>

            <div class="pagination-bar">
              <el-pagination
                v-model:current-page="adminPuzzlesPage"
                :page-size="15"
                :total="adminPuzzlesTotal"
                layout="prev, pager, next"
                @current-change="fetchAdminPuzzles"
              />
            </div>
          </div>
        </el-tab-pane>
      </el-tabs>
    </div>

    <!-- 残局修缮对话框 -->
    <el-dialog v-model="puzzleEditDialog" :title="puzzleForm.id ? '修缮古谱残局' : '录入新名谱'" width="560px">
      <el-form label-position="top">
        <el-form-item label="关卡序数（第几关）">
          <el-input-number v-model="puzzleForm.level" :min="1" />
        </el-form-item>
        <el-form-item label="残局雅号（标题）">
          <el-input v-model="puzzleForm.title" placeholder="例如 马后炮绝杀" />
        </el-form-item>
        <el-form-item label="出处典籍">
          <el-input v-model="puzzleForm.source" placeholder="例如 《适情雅趣》或《橘中秘》" />
        </el-form-item>
        <el-form-item label="难度品阶">
          <el-radio-group v-model="puzzleForm.difficulty">
            <el-radio-button value="easy">初学</el-radio-button>
            <el-radio-button value="medium">精进</el-radio-button>
            <el-radio-button value="hard">大师</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="开局初始 FEN 局面串">
          <el-input v-model="puzzleForm.fen" placeholder="例如 3ak4/4a4/9/4N4/9/9/9/9/4C4/4K4 w" />
        </el-form-item>
        <el-form-item label="正解步骤序列（JSON 格式）">
          <el-input v-model="puzzleForm.solution" type="textarea" :rows="3" placeholder='[{"from":{"r":8,"c":4},"to":{"r":1,"c":4},"notation":"炮五进七"}]' />
        </el-form-item>
        <el-form-item label="破局锦囊点拨">
          <el-input v-model="puzzleForm.hint" placeholder="提示走法方向" />
        </el-form-item>
        <el-form-item label="局势说明">
          <el-input v-model="puzzleForm.description" type="textarea" :rows="2" placeholder="描写残局背景与意蕴" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="puzzleEditDialog = false">取消</el-button>
        <el-button type="primary" @click="saveAdminPuzzle">保 存 名 谱</el-button>
      </template>
    </el-dialog>

    <!-- 棋谱明细对话框 -->
    <el-dialog v-model="viewMovesDialog" title="对局实录谱面" width="500px">
      <div v-if="viewingGame" class="dialog-body">
        <p><b>房间：</b>{{ viewingGame.code }} · <b>玩法：</b>{{ viewingGame.gameType === 'gomoku' ? '五子棋' : '中国象棋' }}</p>
        <p><b>对局双方：</b>{{ viewingGame.redName }} vs {{ viewingGame.blackName }}</p>
        <p><b>终局：</b>{{ REASON_CN[viewingGame.reason] || viewingGame.reason }}</p>
        <div class="moves-scroll">
          <div v-for="(m, i) in viewingMoves" :key="i" class="move-row">
            <span class="m-step">第 {{ i + 1 }} 步</span>
            <span class="m-notation">{{ m.notation || `(${m.to?.r},${m.to?.c})` }}</span>
          </div>
          <div v-if="!viewingMoves.length" class="no-moves">无走棋记录</div>
        </div>
      </div>
    </el-dialog>
  </div>
</template>

<style scoped>
.admin-page {
  display: flex;
  flex-direction: column;
  gap: 16px;
  max-width: 1360px;
  margin: 0 auto;
  width: 100%;
}

.admin-header {
  padding: 16px 24px;
}

.title-line {
  display: flex;
  align-items: center;
  gap: 12px;
}

.title-line h2 {
  margin: 0;
  font-size: 26px;
  color: #7a2415;
  letter-spacing: 4px;
}

.sub-seal {
  background: #a83323;
  color: #f5e4c0;
  font-size: 11px;
  font-weight: 700;
  padding: 2px 8px;
  border-radius: 4px;
  border: 1px solid #d8b36a;
}

.admin-header .desc {
  margin: 6px 0 0;
  color: #6b4a24;
  font-size: 13px;
}

.admin-main {
  padding: 18px 24px;
}

.pane-wrap {
  padding: 12px 0;
}

.stat-cards {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 16px;
  margin-bottom: 20px;
}

.stat-card {
  padding: 18px 20px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 6px;
  text-align: center;
}

.sc-title {
  font-size: 13px;
  color: #7a5a34;
  font-weight: 600;
}

.sc-val {
  font-size: 38px;
  line-height: 1.1;
  color: #432a12;
}

.sc-val.win {
  color: #a83323;
}

.sc-val.gold {
  color: #c9a24b;
}

.sc-sub {
  font-size: 12px;
  color: #8c6a38;
}

.dash-actions {
  display: flex;
  justify-content: flex-end;
}

.toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 14px;
}

.tip-txt {
  color: #b9a171;
  font-size: 13px;
}

.code-badge {
  font-size: 18px;
  font-weight: 700;
  letter-spacing: 2px;
  color: #f0d89a;
}

.status-tag {
  font-size: 12px;
  padding: 2px 6px;
  border-radius: 4px;
}

.status-tag.playing {
  background: rgba(201, 162, 75, 0.35);
  color: #f0d89a;
}

.status-tag.waiting {
  background: rgba(100, 80, 50, 0.35);
  color: #d8c39a;
}

.status-tag.finished {
  background: rgba(60, 40, 20, 0.45);
  color: #9a7a4a;
}

.custom-table {
  background: transparent !important;
  --el-table-border-color: rgba(201, 162, 75, 0.2);
  --el-table-header-bg-color: rgba(45, 26, 12, 0.95);
  --el-table-header-text-color: #f0d89a;
  --el-table-tr-bg-color: rgba(35, 20, 9, 0.85);
  --el-table-text-color: #e0d0b0;
  --el-table-row-hover-bg-color: rgba(201, 162, 75, 0.15);
  border-radius: 6px;
  overflow: hidden;
}

.winner {
  color: #e05030;
  font-weight: 700;
}

.pagination-bar {
  display: flex;
  justify-content: flex-end;
  margin-top: 16px;
}

.dialog-body p {
  margin: 6px 0;
  color: #432a12;
}

.moves-scroll {
  margin-top: 12px;
  max-height: 280px;
  overflow-y: auto;
  border: 1px solid rgba(120, 80, 40, 0.3);
  border-radius: 6px;
  padding: 8px 12px;
  background: rgba(243, 228, 196, 0.6);
}

.move-row {
  display: flex;
  justify-content: space-between;
  padding: 4px 0;
  border-bottom: 1px dashed rgba(120, 80, 40, 0.2);
  font-size: 13px;
  color: #432a12;
}

.m-step {
  color: #7a5a34;
}

.m-notation {
  font-weight: 600;
  color: #a83323;
}

.no-moves {
  text-align: center;
  color: #9a7a4a;
  padding: 20px 0;
}

.settings-wrap {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.settings-card {
  padding: 20px 24px;
}

.card-title {
  margin: 0 0 12px;
  font-size: 22px;
  color: #7a2415;
  letter-spacing: 2px;
}

.st-hint {
  color: #8c6a38;
  font-size: 13px;
  margin: -6px 0 16px;
}

.setting-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 14px 0;
  border-bottom: 1px dashed rgba(120, 80, 40, 0.2);
}

.setting-info {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.st-name {
  font-weight: 700;
  font-size: 15px;
  color: #432a12;
}

.st-desc {
  font-size: 13px;
  color: #8c6a38;
}

.smtp-form {
  margin-top: 10px;
}

.form-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 16px;
}

.inline-switch {
  border-bottom: none;
  padding-bottom: 0;
}

.test-mail-row {
  display: flex;
  gap: 12px;
  align-items: center;
}

.save-bar {
  display: flex;
  justify-content: center;
  margin-top: 10px;
}

.save-all-btn {
  letter-spacing: 6px;
  font-size: 16px;
  padding: 12px 40px;
}

.model-picker-row {
  display: flex;
  gap: 10px;
  width: 100%;
}

.test-llm-btn {
  width: 100%;
  letter-spacing: 2px;
}

.llm-result-box {
  margin-top: 14px;
  padding: 12px 16px;
  background: rgba(30, 18, 8, 0.7);
  border: 1px solid #7094b8;
  border-radius: 8px;
  color: #efe0c0;
}

.llm-res-header {
  display: flex;
  align-items: center;
  gap: 14px;
  font-size: 13px;
  margin-bottom: 8px;
  flex-wrap: wrap;
}

.res-tag {
  background: #2a5a3a;
  color: #c0f5d0;
  font-size: 11px;
  font-weight: 700;
  padding: 1px 6px;
  border-radius: 4px;
}

.res-latency b {
  color: #f0d89a;
}

.res-tokens b {
  color: #c9a24b;
}

.llm-res-content {
  font-size: 14px;
  line-height: 1.6;
  color: #f5eedf;
  background: rgba(0, 0, 0, 0.25);
  padding: 8px 12px;
  border-radius: 6px;
}

.card-head-between {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 10px;
}

.stats-btns {
  display: flex;
  gap: 8px;
}

.llm-stat-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 14px;
  margin-top: 14px;
}

.llm-stat-box {
  background: rgba(40, 24, 10, 0.6);
  border: 1px solid rgba(201, 162, 75, 0.3);
  border-radius: 8px;
  padding: 14px 16px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 4px;
  text-align: center;
}

.ls-title {
  font-size: 12px;
  color: #b9a171;
  font-weight: 600;
}

.ls-val {
  font-size: 28px;
  line-height: 1.2;
  color: #f5eedf;
}

.ls-val.gold {
  color: #c9a24b;
}

.ls-val.win {
  color: #a83323;
}

.ls-sub {
  font-size: 11px;
  color: #8c6a38;
}
</style>
