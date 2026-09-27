<!--
web/src/views/Lobby.vue
页面：对弈大厅
职责：
- 展示用户战绩概览与最近对局历史
- 提供创建房间（时间模式、时长、选边配置）与输入房间号加入
- 维护大厅 WebSocket 长连接与断线自动重连
-->

<script setup>
import { onMounted, onBeforeUnmount, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useUserStore } from '../stores/user'
import { openWS, send } from '../lib/ws'

const router = useRouter()
const userStore = useUserStore()

const playMode = ref('pvp') // pvp: 棋友相约, pve: 人机演练
const aiDifficulty = ref('medium')
const visibility = ref('public')
const gameType = ref('xiangqi')
const timeMode = ref('budget')
const budgetMin = ref(20)
const moveSec = ref(60)
const side = ref('random')
const joinCode = ref('')
const recent = ref([])

const publicRooms = ref([])
const loadingPublicRooms = ref(false)
const matching = ref(false)

let ws = null

async function fetchPublicRooms() {
  loadingPublicRooms.value = true
  try {
    const res = await fetch('/api/rooms/public')
    const data = await res.json()
    publicRooms.value = data.rooms || []
  } catch {} finally {
    loadingPublicRooms.value = false
  }
}

async function quickMatch() {
  if (!userStore.isLogin) {
    ElMessage.warning('请先登录后再进行匹配')
    router.push('/login')
    return
  }
  matching.value = true
  try {
    const res = await fetch('/api/match', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ gameType: gameType.value })
    })
    const data = await res.json()
    if (data.matched && data.code) {
      ElMessage.success('已为您寻得旗鼓相当的弈友，正在入局！')
      router.push(`/room/${data.code}`)
      return
    }
    // 未匹配到，自动为棋友设公开擂台
    ElMessage.info('暂无空闲擂台，已自动为您广设擂台，静候弈友！')
    sendMsg({
      type: 'create_room',
      gameType: gameType.value,
      timeMode: timeMode.value,
      timeSeconds: timeMode.value === 'budget' ? budgetMin.value * 60 : moveSec.value,
      side: side.value,
      isAi: false,
      visibility: 'public'
    })
  } catch (err) {
    ElMessage.error('匹配出错，请重试')
  } finally {
    matching.value = false
  }
}

function connectWS() {
  if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) {
    return
  }
  ws = openWS(userStore.token)
  ws.onmessage = (ev) => {
    const msg = JSON.parse(ev.data)
    if (msg.type === 'room_state' && msg.code) {
      const code = msg.code
      ws.close()
      router.push(`/room/${code}`)
    } else if (msg.type === 'error') {
      ElMessage.error(msg.err)
    }
  }
}

function sendMsg(msg) {
  if (!ws || ws.readyState !== WebSocket.OPEN) {
    connectWS()
    ws.addEventListener('open', () => send(ws, msg), { once: true })
    return
  }
  send(ws, msg)
}

onMounted(async () => {
  if (userStore.isLogin) {
    await userStore.fetchMe()
    recent.value = await userStore.recentGames()
  }
  connectWS()
  fetchPublicRooms()
})

onBeforeUnmount(() => {
  if (ws) ws.close()
})

function createRoom() {
  if (!userStore.isLogin) {
    ElMessage.warning('请先登录后再开局')
    router.push('/login')
    return
  }
  const timeSeconds = timeMode.value === 'budget'
    ? budgetMin.value * 60
    : moveSec.value
  sendMsg({
    type: 'create_room',
    gameType: gameType.value,
    timeMode: timeMode.value,
    timeSeconds,
    side: side.value,
    isAi: playMode.value === 'pve',
    aiDifficulty: aiDifficulty.value,
    visibility: visibility.value
  })
}

function joinRoom() {
  const code = joinCode.value.trim().toUpperCase()
  if (!code) {
    ElMessage.warning('请输入房间号')
    return
  }
  if (code.length !== 6) {
    ElMessage.warning('房间号需为 6 位字母或数字')
    return
  }
  router.push(`/room/${code}`)
}

const REASON_CN = {
  checkmate: '将死', stalemate: '困毙', perpetual_check: '长将判负',
  perpetual_chase: '长捉判负', repeat_draw: '不变作和', timeout: '超时判负',
  resign: '认输', draw_accepted: '同意和棋', disconnect: '离线判负',
  five_in_a_row: '五连胜', board_full: '满盘作和'
}

function myResult(g) {
  let mySide
  if (g.gameType === 'gomoku') {
    mySide = g.redName === userStore.username ? 'black' : 'white'
  } else {
    mySide = g.redName === userStore.username ? 'red' : 'black'
  }
  if (g.result === 'draw') return { text: '和', cls: 'draw' }
  return g.result === mySide
    ? { text: '胜', cls: 'win' }
    : { text: '负', cls: 'lose' }
}
</script>

<template>
  <div class="lobby">
    <div class="lobby-grid">
      <!-- 左：战绩 + 操作 -->
      <div class="col-left">
        <div class="wood-panel stats-card">
          <template v-if="userStore.isLogin">
            <h3 class="font-brush">棋友 · {{ userStore.username }}</h3>
            <div class="stats-row">
              <div class="stat win">
                <b>{{ userStore.stats?.wins ?? 0 }}</b><span>胜</span>
              </div>
              <div class="stat draw">
                <b>{{ userStore.stats?.draws ?? 0 }}</b><span>和</span>
              </div>
              <div class="stat lose">
                <b>{{ userStore.stats?.losses ?? 0 }}</b><span>负</span>
              </div>
            </div>
            <p class="total">
              共弈 {{ userStore.stats?.total ?? 0 }} 局 ·
              胜率
              {{
                userStore.stats?.total
                  ? Math.round((userStore.stats.wins / userStore.stats.total) * 100) + '%'
                  : '—'
              }}
            </p>
          </template>
          <template v-else>
            <h3 class="font-brush">游客模式</h3>
            <p class="guest-desc">登录后可创建房间、与人对弈并记录战绩；游客可凭房间号进入观战。</p>
            <el-button type="primary" @click="router.push('/login')">登录 / 注册</el-button>
          </template>
        </div>

        <div class="wood-panel create-card">
          <div class="card-top-header">
            <h3 class="font-brush">{{ playMode === 'pve' ? '演练 · 人机博弈' : '摆擂 · 好友约战' }}</h3>
            <div class="play-mode-switch">
              <el-radio-group v-model="playMode" size="small">
                <el-radio-button value="pvp">⚔️ 好友约战</el-radio-button>
                <el-radio-button value="pve">🤖 人机演练</el-radio-button>
              </el-radio-group>
            </div>
          </div>

          <!-- 人机对手品阶选择 -->
          <div class="form-row" v-if="playMode === 'pve'">
            <label>对手品阶</label>
            <el-radio-group v-model="aiDifficulty">
              <el-radio-button value="easy">🌱 初学</el-radio-button>
              <el-radio-button value="medium">🌿 进阶</el-radio-button>
              <el-radio-button value="hard">🌲 大师</el-radio-button>
            </el-radio-group>
          </div>

          <div class="form-row">
            <label>玩法类型</label>
            <el-radio-group v-model="gameType" @change="side = 'random'">
              <el-radio-button value="xiangqi">中国象棋</el-radio-button>
              <el-radio-button value="gomoku">五子棋</el-radio-button>
            </el-radio-group>
          </div>

          <div class="form-row">
            <label>计时规则</label>
            <el-radio-group v-model="timeMode">
              <el-radio-button value="budget">包干制</el-radio-button>
              <el-radio-button value="per_move">步时制</el-radio-button>
            </el-radio-group>
          </div>

          <div class="form-row" v-if="timeMode === 'budget'">
            <label>局时</label>
            <el-radio-group v-model="budgetMin">
              <el-radio-button :value="10">10 分钟</el-radio-button>
              <el-radio-button :value="20">20 分钟</el-radio-button>
              <el-radio-button :value="30">30 分钟</el-radio-button>
            </el-radio-group>
          </div>

          <div class="form-row" v-else>
            <label>每步限时</label>
            <el-radio-group v-model="moveSec">
              <el-radio-button :value="30">30 秒</el-radio-button>
              <el-radio-button :value="60">1 分钟</el-radio-button>
              <el-radio-button :value="120">2 分钟</el-radio-button>
            </el-radio-group>
          </div>

          <div class="form-row">
            <label>执子先后</label>
            <el-radio-group v-model="side" v-if="gameType === 'gomoku'">
              <el-radio-button value="black">执黑·先手</el-radio-button>
              <el-radio-button value="white">执白·后手</el-radio-button>
              <el-radio-button value="random">随机</el-radio-button>
            </el-radio-group>
            <el-radio-group v-model="side" v-else>
              <el-radio-button value="red">我执红</el-radio-button>
              <el-radio-button value="black">我执黑</el-radio-button>
              <el-radio-button value="random">随机</el-radio-button>
            </el-radio-group>
          </div>

          <!-- 入局准入可见度（仅好友约战显示） -->
          <div class="form-row" v-if="playMode === 'pvp'">
            <label>入局准入</label>
            <el-radio-group v-model="visibility">
              <el-radio-button value="public">🌐 公开招擂</el-radio-button>
              <el-radio-button value="private">🔒 仅凭房号</el-radio-button>
            </el-radio-group>
          </div>

          <div class="btn-double-row">
            <el-button type="primary" size="large" class="create-btn" @click="createRoom">
              {{ playMode === 'pve' ? '入 局 演 练' : '开 局 设 擂' }}
            </el-button>
            <el-button
              v-if="playMode === 'pvp'"
              type="warning"
              size="large"
              class="match-btn font-brush"
              :loading="matching"
              @click="quickMatch"
            >
              ⚡ 极速随缘匹配
            </el-button>
          </div>
        </div>

        <!-- 经典残局与每日一题快捷横幅 -->
        <div class="wood-panel daily-puzzle-card" @click="router.push('/puzzle')">
          <div class="dpc-left">
            <span class="dpc-badge font-brush">每日名局</span>
            <div class="dpc-titles">
              <h4 class="font-brush dpc-title">经典残局闯关 · 每日一题</h4>
              <p class="dpc-desc">红先连照绝杀，探历代古谱参透玄机打卡</p>
            </div>
          </div>
          <el-button type="primary" size="default" class="dpc-btn">去破局 ></el-button>
        </div>

        <div class="wood-panel join-card">
          <h3 class="font-brush">赴约 · 加入房间</h3>
          <div class="join-row">
            <el-input
              v-model="joinCode"
              size="large"
              placeholder="输入 6 位房间号"
              maxlength="6"
              @keyup.enter="joinRoom"
            />
            <el-button type="primary" size="large" @click="joinRoom">进入</el-button>
          </div>
        </div>
      </div>

      <!-- 右侧栏：公开待弈擂榜 + 最近战报 -->
      <div class="col-right">
        <!-- 江湖擂台 · 待弈榜 -->
        <div class="dark-panel public-card">
          <div class="card-head-between">
            <h3 class="font-brush public-title">江 湖 擂 台 · 待 弈 榜</h3>
            <el-button size="small" :loading="loadingPublicRooms" @click="fetchPublicRooms">刷新擂台</el-button>
          </div>
          <p class="pub-desc">路过江湖，见猎心喜？点击应擂即刻与在线棋友切磋过招！</p>

          <div v-if="!publicRooms.length" class="empty-public">
            江湖风平浪静，暂无公开招擂… 不妨自己设擂或点击极速匹配！
          </div>
          <div v-for="r in publicRooms" :key="r.code" class="public-room-item">
            <div class="pri-left">
              <div class="pri-title">
                <span class="pri-owner font-brush">{{ r.ownerName }}</span>
                <el-tag size="small" :type="r.gameType === 'gomoku' ? 'success' : 'danger'">
                  {{ r.gameType === 'gomoku' ? '五子棋' : '中国象棋' }}
                </el-tag>
              </div>
              <div class="pri-meta">
                <span>房号: <b class="font-brush">{{ r.code }}</b></span> ·
                <span>{{ r.timeMode === 'budget' ? `包干 ${Math.round(r.timeSeconds / 60)}分` : `步时 ${r.timeSeconds}秒` }}</span> ·
                <span>擂主{{ r.side === 'red' ? '执红' : (r.side === 'black' ? '执黑' : (r.side === 'white' ? '执白' : '随缘')) }}</span>
              </div>
            </div>
            <el-button type="danger" size="default" class="challenge-btn font-brush" @click="router.push('/room/' + r.code)">
              应 擂
            </el-button>
          </div>
        </div>

        <!-- 战报 · 最近对局 -->
        <div class="dark-panel recent-card">
          <h3 class="font-brush recent-title">战 报 · 最 近 对 局</h3>
          <div v-if="!recent.length" class="empty-recent">
            尚无对局记录，去开一局吧。
          </div>
          <div v-for="g in recent" :key="g.id" class="recent-item">
            <span class="result-badge" :class="myResult(g).cls">{{ myResult(g).text }}</span>
            <div class="recent-main">
              <div class="recent-players">
                <span class="game-tag">{{ g.gameType === 'gomoku' ? '五子棋' : '象棋' }}</span>
                <template v-if="g.gameType === 'gomoku'">
                  <span :class="{ winner: g.result === 'black' }">
                    {{ g.redName }} <small>黑</small>
                  </span>
                  <span class="vs">vs</span>
                  <span :class="{ winner: g.result === 'white' }">
                    <small>白</small> {{ g.blackName }}
                  </span>
                </template>
                <template v-else>
                  <span :class="{ winner: g.result === 'red' }">
                    {{ g.redName }} <small>红</small>
                  </span>
                  <span class="vs">vs</span>
                  <span :class="{ winner: g.result === 'black' }">
                    <small>黑</small> {{ g.blackName }}
                  </span>
                </template>
              </div>
              <div class="recent-meta">
                {{ REASON_CN[g.reason] || g.reason }} · 房间 {{ g.code }} · {{ g.endedAt }}
              </div>
            </div>
            <el-button size="small" type="primary" plain class="replay-btn" @click="router.push('/replay/' + g.id)">
              复盘
            </el-button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.lobby-grid {
  display: grid;
  grid-template-columns: minmax(0, 5fr) minmax(0, 4fr);
  gap: 20px;
}

.card-top-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
  flex-wrap: wrap;
  gap: 10px;
}

.card-top-header h3 {
  margin: 0 !important;
}

.daily-puzzle-card {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 16px 20px;
  cursor: pointer;
  background: linear-gradient(145deg, rgba(247, 230, 195, 0.98), rgba(228, 202, 155, 0.98));
  border: 1.5px solid #a83323;
  margin-bottom: 20px;
  transition: all 0.25s cubic-bezier(0.2, 0.8, 0.2, 1);
}

.daily-puzzle-card:hover {
  transform: translateY(-2px);
  box-shadow: 0 12px 30px rgba(168, 51, 35, 0.35);
  border-color: #d84530;
}

.dpc-left {
  display: flex;
  align-items: center;
  gap: 12px;
}

.dpc-badge {
  background: #a83323;
  color: #f5e4c0;
  font-size: 14px;
  padding: 6px 8px;
  border-radius: 6px;
  letter-spacing: 2px;
  line-height: 1.2;
}

.dpc-title {
  margin: 0;
  font-size: 19px;
  color: #7a2415;
  letter-spacing: 2px;
}

.dpc-desc {
  margin: 2px 0 0;
  font-size: 12px;
  color: #8c6a38;
}

.wood-panel {
  padding: 22px 24px;
  margin-bottom: 20px;
  transition: transform 0.25s cubic-bezier(0.2, 0.8, 0.2, 1), box-shadow 0.25s cubic-bezier(0.2, 0.8, 0.2, 1);
}

.wood-panel:hover {
  transform: translateY(-2px);
  box-shadow: 0 14px 36px rgba(0, 0, 0, 0.52), inset 0 0 0 3px rgba(120, 78, 34, 0.35);
}

.wood-panel h3 {
  margin: 0 0 16px;
  font-size: 26px;
  color: #7a2415;
  letter-spacing: 4px;
}

.stats-row {
  display: flex;
  justify-content: space-around;
  margin: 14px 0 8px;
}

.stat {
  text-align: center;
}

.stat b {
  display: block;
  font-size: 34px;
  line-height: 1.2;
}

.stat span {
  font-size: 14px;
  color: #7a5a34;
}

.stat.win b {
  color: #a83323;
}

.stat.lose b {
  color: #5a4632;
}

.stat.draw b {
  color: #9a7a3a;
}

.total {
  text-align: center;
  color: #6b4a24;
  margin: 6px 0 0;
}

.guest-desc {
  line-height: 1.8;
  color: #6b4a24;
  margin: 0 0 14px;
}

.form-row {
  display: flex;
  align-items: center;
  gap: 14px;
  margin-bottom: 14px;
  flex-wrap: wrap;
}

.form-row label {
  min-width: 72px;
  color: #5a3c1e;
  font-weight: 600;
}

.btn-double-row {
  display: flex;
  gap: 10px;
  width: 100%;
  margin-top: 6px;
}

.create-btn {
  flex: 1;
  letter-spacing: 4px;
  font-size: 16px;
}

.match-btn {
  flex: 1;
  letter-spacing: 2px;
  font-size: 15px;
  background: linear-gradient(135deg, #c9933b, #a06e1e);
  border-color: #d8aa50;
  color: #fff8e8;
  box-shadow: 0 4px 14px rgba(201, 147, 59, 0.4);
}

.join-row {
  display: flex;
  gap: 10px;
}

.col-right {
  display: flex;
  flex-direction: column;
  gap: 20px;
  align-self: start;
}

.public-card {
  padding: 20px 22px;
}

.card-head-between {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.public-title {
  margin: 0;
  font-size: 22px;
  color: #f0d89a;
  letter-spacing: 3px;
}

.pub-desc {
  font-size: 12px;
  color: #b9a171;
  margin: 4px 0 12px;
}

.empty-public {
  color: #b9a171;
  font-size: 13px;
  text-align: center;
  padding: 24px 10px;
  line-height: 1.6;
}

.public-room-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 12px;
  border-bottom: 1px dashed rgba(201, 162, 75, 0.3);
  border-radius: 6px;
  margin-bottom: 6px;
  background: rgba(45, 26, 12, 0.6);
  transition: all 0.22s;
}

.public-room-item:hover {
  background: rgba(201, 162, 75, 0.15);
  transform: translateX(3px);
}

.pri-left {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.pri-title {
  display: flex;
  align-items: center;
  gap: 8px;
}

.pri-owner {
  font-size: 16px;
  color: #f0d89a;
}

.pri-meta {
  font-size: 12px;
  color: #b9a171;
}

.pri-meta b {
  color: #f5eedf;
  font-size: 14px;
}

.challenge-btn {
  letter-spacing: 2px;
  font-size: 14px;
  padding: 6px 14px;
}

.recent-card {
  padding: 22px;
}

.recent-title {
  margin: 0 0 16px;
  font-size: 24px;
  color: #f0d89a;
  letter-spacing: 4px;
  text-align: center;
}

.empty-recent {
  color: #b9a171;
  text-align: center;
  padding: 40px 0;
}

.recent-item {
  display: flex;
  gap: 12px;
  padding: 12px 10px;
  border-bottom: 1px dashed rgba(201, 162, 75, 0.3);
  border-radius: 6px;
  transition: all 0.22s ease-out;
}

.recent-item:hover {
  background: rgba(201, 162, 75, 0.1);
  transform: translateX(4px);
}

.result-badge {
  display: grid;
  place-items: center;
  width: 34px;
  height: 34px;
  border-radius: 50%;
  font-weight: 700;
  flex-shrink: 0;
}

.result-badge.win {
  background: #a83323;
  color: #f5e4c0;
}

.result-badge.lose {
  background: rgba(120, 100, 70, 0.5);
  color: #d8c39a;
}

.result-badge.draw {
  background: rgba(180, 150, 80, 0.4);
  color: #f0d89a;
}

.recent-players {
  font-weight: 700;
  font-size: 16px;
  display: flex;
  align-items: center;
  gap: 8px;
}

.game-tag {
  font-size: 11px;
  padding: 1px 6px;
  border-radius: 4px;
  background: rgba(122, 36, 21, 0.15);
  color: #a83323;
  border: 1px solid rgba(168, 51, 35, 0.35);
  font-weight: 600;
}

.recent-players .winner {
  color: #e8a594;
  font-weight: 700;
}

.recent-players small {
  opacity: 0.7;
}

.vs {
  opacity: 0.5;
  font-size: 12px;
}

.recent-meta {
  font-size: 12px;
  color: #b9a171;
  margin-top: 4px;
}

@media (max-width: 900px) {
  .lobby-grid {
    grid-template-columns: 1fr;
  }
}
</style>
