<!--
web/src/views/Room.vue
页面：对局室（对弈与观战）
职责：
- 维持 WebSocket 房间连接与状态实时同步
- 协调棋盘渲染、玩家状态卡片展示与红黑方视角翻转
- 提供走棋、悔棋、提和、认输、再来一局等对局控制
- 集成棋谱回放列表、局内聊天与观战席位面板
-->

<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import XqBoard from '../components/XqBoard.vue'
import GomokuBoard from '../components/GomokuBoard.vue'
import PlayerCard from '../components/PlayerCard.vue'
import MoveList from '../components/MoveList.vue'
import ChatPanel from '../components/ChatPanel.vue'
import { useUserStore } from '../stores/user'
import { openWS, send } from '../lib/ws'
import {
  playMoveSound,
  playCaptureSound,
  playCheckSound,
  playWinSound,
  playLoseSound,
  isMuted,
  setMuted
} from '../lib/audio'

const route = useRoute()
const router = useRouter()
const userStore = useUserStore()

const code = route.params.code.toUpperCase()
const isSpectate = computed(() => route.query.spectate === '1' || route.query.spectate === 'true')
const state = ref(null)
const liveChats = ref([])
const activeTab = ref('moves')
const tick = ref(0)
const soundMuted = ref(isMuted())

function toggleSound() {
  soundMuted.value = !soundMuted.value
  setMuted(soundMuted.value)
  if (!soundMuted.value) {
    playMoveSound()
  }
}

let ws = null
let clockOffset = 0
let timer = null
let handledPending = ''
let intentionalLeave = false
let overShown = false

let prevMovesLen = 0
let prevInCheck = false
let prevStatus = ''

const role = computed(() => state.value?.you?.role || 'spectator')
const isPlayer = computed(() => role.value === 'red' || role.value === 'black' || role.value === 'white')
const myColor = computed(() => (isPlayer.value ? role.value : ''))
const canInteract = computed(
  () =>
    state.value?.status === 'playing' &&
    isPlayer.value &&
    state.value.side === role.value
)
const checkedColor = computed(() =>
  state.value?.inCheck ? state.value.side : ''
)
const lastMove = computed(() => {
  const ms = state.value?.moves
  if (!ms || !ms.length) return null
  const m = ms[ms.length - 1]
  return { from: m.from, to: m.to || m.pos }
})

function seatDisplay(sd) {
  const s = state.value?.[sd]
  if (!s) return null
  let t = s.timeLeft
  if (state.value.status === 'playing' && state.value.side === sd) {
    const now = Date.now() + clockOffset
    t -= Math.max(0, now - state.value.serverTime)
  }
  return { ...s, timeLeft: Math.max(0, t) }
}

const redSeat = computed(() => {
  tick.value
  return seatDisplay('red')
})
const blackSeat = computed(() => {
  tick.value
  return seatDisplay('black')
})
const whiteSeat = computed(() => {
  tick.value
  return seatDisplay('white')
})

// 玩家视角自适应上下对调，自己的阵营始终在下方
const topSide = computed(() => {
  if (state.value?.gameType === 'gomoku') {
    return role.value === 'white' ? 'black' : 'white'
  }
  return role.value === 'black' ? 'red' : 'black'
})

const topSeat = computed(() => {
  if (state.value?.gameType === 'gomoku') {
    return topSide.value === 'black' ? blackSeat.value : whiteSeat.value
  }
  return role.value === 'black' ? redSeat.value : blackSeat.value
})

const bottomSide = computed(() => {
  if (state.value?.gameType === 'gomoku') {
    return role.value === 'white' ? 'white' : 'black'
  }
  return role.value === 'black' ? 'black' : 'red'
})

const bottomSeat = computed(() => {
  if (state.value?.gameType === 'gomoku') {
    return bottomSide.value === 'white' ? whiteSeat.value : blackSeat.value
  }
  return role.value === 'black' ? blackSeat.value : redSeat.value
})

const chats = computed(() =>
  liveChats.value.length ? liveChats.value : state.value?.chats || []
)

onMounted(() => {
  ws = openWS(userStore.token)
  ws.onopen = () => send(ws, { type: 'join_room', code, spectate: isSpectate.value })
  ws.onmessage = (ev) => {
    const msg = JSON.parse(ev.data)
    switch (msg.type) {
      case 'room_state': {
        const curMovesLen = msg.moves ? msg.moves.length : 0
        const isNewMove = curMovesLen > prevMovesLen && prevMovesLen > 0

        if (isNewMove) {
          const lastM = msg.moves[msg.moves.length - 1]
          if (msg.gameType !== 'gomoku' && lastM && lastM.captured && lastM.captured !== 0) {
            playCaptureSound()
          } else {
            playMoveSound()
          }
        }

        if (msg.inCheck && !prevInCheck) {
          playCheckSound()
        }

        if (msg.status === 'finished' && prevStatus === 'playing') {
          if (msg.winner === role.value) {
            playWinSound()
          } else if (isPlayer.value && msg.winner !== 'draw') {
            playLoseSound()
          }
        }

        prevMovesLen = curMovesLen
        prevInCheck = !!msg.inCheck
        prevStatus = msg.status

        msg.chat = undefined
        msg.request = undefined
        state.value = msg
        clockOffset = msg.serverTime - Date.now()
        liveChats.value = msg.chats ? [...msg.chats] : []
        break
      }
      case 'chat':
        liveChats.value.push(msg.chat)
        break
      case 'request':
        handleRequest(msg.request)
        break
      case 'notify':
        ElMessage(msg.notify)
        break
      case 'error':
        ElMessage.error(msg.err)
        if (!state.value) {
          setTimeout(() => {
            if (!state.value) router.push('/')
          }, 1500)
        }
        break
    }
  }
  ws.onclose = () => {
    if (!intentionalLeave) {
      ElMessage.warning('连接已断开，正在返回大厅')
    }
    router.push('/')
  }
  timer = setInterval(() => {
    tick.value++
  }, 250)
})

onBeforeUnmount(() => {
  if (timer) clearInterval(timer)
  if (ws) ws.close()
})

function leaveToLobby() {
  intentionalLeave = true
  router.push('/')
}

async function handleRequest(req) {
  const key = `${req.kind}-${req.from}`
  if (handledPending === key) return
  handledPending = key
  const actionText = req.kind === 'undo' ? '悔棋' : '和棋'
  try {
    await ElMessageBox.confirm(
      `${req.from} 请求${actionText}，是否同意？`,
      `${actionText}请求`,
      {
        confirmButtonText: '同意',
        cancelButtonText: '拒绝',
        distinguishCancelAndClose: true
      }
    )
    send(ws, { type: req.kind === 'undo' ? 'undo_accept' : 'draw_accept' })
  } catch (action) {
    if (action === 'cancel' || action === 'close') {
      send(ws, { type: req.kind === 'undo' ? 'undo_reject' : 'draw_reject' })
    }
  } finally {
    handledPending = ''
  }
}

function doMove(m) {
  send(ws, { type: 'move', from: m.from, to: m.to })
}

function requestUndo() {
  send(ws, { type: 'undo_request' })
}
function requestDraw() {
  send(ws, { type: 'draw_request' })
}
function resign() {
  ElMessageBox.confirm('确定要认输吗？', '认输确认', {
    confirmButtonText: '认输',
    cancelButtonText: '再想想'
  })
    .then(() => send(ws, { type: 'resign' }))
    .catch(() => {})
}
function rematch() {
  send(ws, { type: 'rematch' })
  if (!state.value?.isAi) {
    ElMessage.success('已同意再来一局，等待对方')
  }
}
function sendChat(text) {
  send(ws, { type: 'chat', text })
}

async function copyInvite() {
  try {
    await navigator.clipboard.writeText(location.href)
    ElMessage.success('邀请链接已复制，发给好友即可')
  } catch {
    ElMessage.info(`房间号 ${code}，把链接发给好友即可`)
  }
}

const REASON_CN = {
  checkmate: '将死',
  stalemate: '困毙',
  perpetual_check: '长将判负',
  perpetual_chase: '长捉判负',
  repeat_draw: '双方不变作和',
  timeout: '超时判负',
  resign: '认输',
  draw_accepted: '同意和棋',
  disconnect: '离线超时判负',
  five_in_a_row: '五子连珠胜',
  board_full: '满盘作和'
}

const overInfo = computed(() => {
  const s = state.value
  if (!s || s.status !== 'finished') return null
  if (!overShown) {
    overShown = true
  }
  let title
  if (s.winner === 'draw') title = '和 棋'
  else if (s.winner === role.value) title = '你 赢 了'
  else if (isPlayer.value) title = '你 输 了'
  else {
    if (s.gameType === 'gomoku') {
      title = s.winner === 'black' ? '黑方胜' : '白方胜'
    } else {
      title = s.winner === 'red' ? '红方胜' : '黑方胜'
    }
  }
  return { title, reason: REASON_CN[s.reason] || s.reason }
})

const timeRuleText = computed(() => {
  const s = state.value
  if (!s) return ''
  return s.timeMode === 'budget'
    ? `包干制 · 每方 ${Math.round(s.timeSeconds / 60)} 分钟`
    : `步时制 · 每步 ${s.timeSeconds} 秒`
})

const iVotedRematch = computed(() => {
  const s = state.value
  if (s?.isAi) return false
  return s?.rematchVote?.includes(userStore.username)
})
</script>

<template>
  <div v-if="state" class="room-page">
    <div class="board-area">
      <PlayerCard
        :seat="topSeat"
        :side="topSide"
        :game-type="state.gameType"
        :active="state.status === 'playing' && state.side === topSide"
        :you="role === topSide"
      />

      <div class="board-stage">
        <GomokuBoard
          v-if="state.gameType === 'gomoku'"
          :board="state.gomokuBoard"
          :interactive="canInteract"
          :my-color="myColor"
          :last-move="lastMove"
          :flipped="role === 'white'"
          @move="doMove"
        />
        <XqBoard
          v-else
          :fen="state.fen"
          :interactive="canInteract"
          :my-color="myColor"
          :checked-color="checkedColor"
          :last-move="lastMove"
          :flipped="role === 'black'"
          @move="doMove"
        />

        <!-- 等待开局遮罩 -->
        <transition name="overlay-pop">
          <div v-if="state.status === 'waiting'" class="overlay waiting-overlay">
            <div class="overlay-card wood-panel">
              <h3 class="font-brush">等待棋友赴约</h3>
              <p>房间号</p>
              <div class="big-code font-brush">{{ code }}</div>
              <el-button type="primary" @click="copyInvite">复制邀请链接</el-button>
              <p class="tip">{{ timeRuleText }}</p>
            </div>
          </div>
        </transition>

        <!-- 对局结束遮罩 -->
        <transition name="overlay-pop">
          <div v-if="overInfo" class="overlay over-overlay">
            <div class="overlay-card wood-panel">
              <h3 class="font-brush" :class="{ win: overInfo.title.includes('赢') }">
                {{ overInfo.title }}
              </h3>
              <p class="reason">{{ overInfo.reason }}</p>
              <div class="over-actions">
                <el-button
                  v-if="isPlayer && !iVotedRematch"
                  type="primary"
                  @click="rematch"
                >
                  再来一局
                </el-button>
                <span v-else-if="isPlayer" class="waiting-rematch">
                  等待对方同意…
                </span>
                <el-button @click="leaveToLobby">返回大厅</el-button>
              </div>
            </div>
          </div>
        </transition>
      </div>

      <PlayerCard
        :seat="bottomSeat"
        :side="bottomSide"
        :game-type="state.gameType"
        :active="state.status === 'playing' && state.side === bottomSide"
        :you="role === bottomSide"
      />

      <!-- 对局操作 -->
      <div v-if="isPlayer && state.status === 'playing'" class="actions">
        <el-button @click="requestUndo">申请悔棋</el-button>
        <el-button @click="requestDraw">提议和棋</el-button>
        <el-button type="danger" plain @click="resign">认输</el-button>
        <el-button size="default" :type="soundMuted ? 'info' : 'warning'" plain @click="toggleSound">
          {{ soundMuted ? '🔇 静音' : '🔊 音效开' }}
        </el-button>
      </div>
      <div v-else class="actions spectator-bar">
        <span v-if="!isPlayer" class="spectator-tip">
          <span v-if="userStore.isAdmin && isSpectate" class="admin-spectating-tag">督抚巡察中</span>
          你正在观战 · 房间 {{ code }}
        </span>
        <el-button size="small" :type="soundMuted ? 'info' : 'warning'" plain @click="toggleSound">
          {{ soundMuted ? '🔇 静音' : '🔊 音效开' }}
        </el-button>
      </div>
    </div>

    <!-- 侧栏 -->
    <div class="side-area dark-panel">
      <el-tabs v-model="activeTab" class="side-tabs">
        <el-tab-pane label="棋谱" name="moves">
          <div class="pane-body">
            <MoveList :moves="state.moves" :game-type="state.gameType" />
          </div>
        </el-tab-pane>
        <el-tab-pane label="聊天" name="chat">
          <div class="pane-body chat-pane">
            <ChatPanel
              :chats="chats"
              :can-send="isPlayer || (userStore.isLogin && role === 'spectator')"
              @send="sendChat"
            />
          </div>
        </el-tab-pane>
        <el-tab-pane :label="`观战 (${state.spectators.length})`" name="specs">
          <div class="pane-body specs-pane">
            <p class="rule-line">{{ timeRuleText }}</p>
            <div v-if="!state.spectators.length" class="no-specs">暂无观战者</div>
            <div v-for="sp in state.spectators" :key="sp.connId" class="spec-line">
              <span class="spec-dot">观</span>
              {{ sp.name }}
              <span v-if="sp.you" class="spec-you">（你）</span>
            </div>
          </div>
        </el-tab-pane>
      </el-tabs>
    </div>
  </div>
  <div v-else class="room-loading">
    <div class="wood-panel loading-card">
      <h3 class="font-brush">正在进入对局室…</h3>
      <p>房间号：{{ code }}</p>
      <el-button @click="leaveToLobby">返回大厅</el-button>
    </div>
  </div>
</template>

<style scoped>
.room-loading {
  display: grid;
  place-items: center;
  min-height: calc(100vh - 120px);
  padding: 20px;
}
.loading-card {
  padding: 30px 40px;
  text-align: center;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 14px;
}
.room-page {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 340px;
  gap: 16px;
  align-items: stretch;
  height: 100%;
  min-height: 0;
  flex: 1;
}

.board-area {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
  height: 100%;
  min-height: 0;
  justify-content: space-between;
}

.board-stage {
  position: relative;
  flex: 1;
  min-height: 0;
  display: flex;
  align-items: center;
  justify-content: center;
}

.overlay {
  position: absolute;
  inset: 0;
  display: grid;
  place-items: center;
  background: rgba(30, 18, 8, 0.55);
  border-radius: 8px;
  z-index: 5;
}

.overlay-card {
  padding: 26px 36px;
  text-align: center;
  min-width: 260px;
  box-shadow: 0 16px 40px rgba(0, 0, 0, 0.6), inset 0 0 0 3px rgba(120, 78, 34, 0.35);
  animation: cardScaleIn 0.32s cubic-bezier(0.34, 1.56, 0.64, 1);
}

@keyframes cardScaleIn {
  from {
    transform: scale(0.88);
    opacity: 0;
  }
  to {
    transform: scale(1);
    opacity: 1;
  }
}

.overlay-card h3.win {
  color: #a83323;
  text-shadow: 0 0 14px rgba(201, 162, 75, 0.6);
  animation: winTitlePulse 2s ease-in-out infinite;
}

@keyframes winTitlePulse {
  0%, 100% {
    transform: scale(1);
  }
  50% {
    transform: scale(1.05);
  }
}

.overlay-card h3 {
  margin: 0 0 8px;
  font-size: 30px;
  color: #7a2415;
  letter-spacing: 6px;
}

.overlay-card h3.win {
  color: #a83323;
}

.overlay-card p {
  color: #6b4a24;
  margin: 8px 0;
}

.big-code {
  font-size: 46px;
  color: #7a2415;
  letter-spacing: 10px;
  margin: 6px 0 14px;
}

.tip {
  font-size: 13px;
}

.reason {
  font-size: 15px;
}

.over-actions {
  display: flex;
  gap: 10px;
  justify-content: center;
  margin-top: 10px;
  flex-wrap: wrap;
}

.waiting-rematch {
  color: #8a6638;
  font-size: 14px;
}

.actions {
  display: flex;
  gap: 10px;
  justify-content: center;
  flex-wrap: wrap;
  padding: 4px 0;
}

.spectator-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 4px 6px;
}

.spectator-tip {
  text-align: center;
  color: #b9a171;
  font-size: 14px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
}

.admin-spectating-tag {
  background: #a83323;
  color: #f5e4c0;
  font-size: 11px;
  font-weight: 700;
  padding: 1px 7px;
  border-radius: 4px;
  border: 1px solid #d8b36a;
}

.side-area {
  overflow: hidden;
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
}

.side-tabs {
  display: flex;
  flex-direction: column;
  flex: 1;
  height: 100%;
}

:deep(.el-tabs__header) {
  margin: 0;
  flex-shrink: 0;
}

:deep(.el-tabs__content) {
  flex: 1;
  height: 100%;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

:deep(.el-tab-pane) {
  height: 100%;
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.pane-body {
  height: 100%;
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.chat-pane {
  background: rgba(243, 228, 196, 0.95);
  height: 100%;
  flex: 1;
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.specs-pane {
  padding: 14px 16px;
  color: #d8c39a;
  flex: 1;
  overflow-y: auto;
}

.rule-line {
  color: #f0d89a;
  border-bottom: 1px solid rgba(201, 162, 75, 0.3);
  padding-bottom: 10px;
  margin-top: 0;
}

.no-specs {
  color: #b9a171;
  text-align: center;
  padding-top: 30px;
}

.spec-line {
  padding: 8px 0;
  border-bottom: 1px dashed rgba(201, 162, 75, 0.2);
}

.spec-dot {
  display: inline-grid;
  place-items: center;
  width: 22px;
  height: 22px;
  border-radius: 50%;
  background: rgba(201, 162, 75, 0.3);
  font-size: 12px;
  margin-right: 6px;
}

.spec-you {
  color: #f0d89a;
  font-size: 12px;
}

@media (max-width: 900px) {
  .room-page {
    grid-template-columns: 1fr;
  }
  :deep(.el-tabs__content) {
    height: 42vh;
  }
}
</style>
