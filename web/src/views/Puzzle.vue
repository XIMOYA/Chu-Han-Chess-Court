<!--
web/src/views/Puzzle.vue
页面：经典残局闯关 · 每日一题
职责：
- 展现每日一题晨钟破局打卡与历代古谱竹简关卡名册
- 驱动红方交互式解谜、黑方智能自动应手与连照绝杀判定
- 颁发古风朱砂印章、连胜天数记录与关键着法点拨
-->

<script setup>
import { computed, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import XqBoard from '../components/XqBoard.vue'
import { useUserStore } from '../stores/user'
import { parseFEN } from '../lib/board.js'
import { boardToFEN } from '../lib/replay.js'
import { playMoveSound, playCaptureSound, playWinSound, isMuted, setMuted } from '../lib/audio.js'

const router = useRouter()
const userStore = useUserStore()

const activeTab = ref('daily')
const dailyPuzzle = ref(null)
const puzzles = ref([])
const streak = ref(0)
const loading = ref(true)

// 当前正在推演的题目与盘面状态
const currentPuzzle = ref(null)
const currentFen = ref('')
const currentBoard = ref(null)
const currentLastMove = ref(null)
const stepIndex = ref(0)
const isSolved = ref(false)
const showHint = ref(false)
const soundMuted = ref(isMuted())

function toggleSound() {
  soundMuted.value = !soundMuted.value
  setMuted(soundMuted.value)
  if (!soundMuted.value) playMoveSound()
}

// 初始化加载每日一题与关卡
async function fetchDailyAndList() {
  loading.value = true
  try {
    const headers = userStore.token ? { Authorization: `Bearer ${userStore.token}` } : {}
    const [resDaily, resList] = await Promise.all([
      fetch('/api/puzzles/daily', { headers }),
      fetch('/api/puzzles', { headers })
    ])
    const dataDaily = await resDaily.json()
    const dataList = await resList.json()

    if (dataDaily.puzzle) {
      dailyPuzzle.value = dataDaily.puzzle
      streak.value = dataDaily.streak || 0
      loadPuzzle(dataDaily.puzzle)
    }
    if (dataList.puzzles) {
      puzzles.value = dataList.puzzles
    }
  } catch (err) {
    ElMessage.error('加载残局题库失败')
  } finally {
    loading.value = false
  }
}

// 载入题目至棋盘
function loadPuzzle(pz) {
  currentPuzzle.value = pz
  stepIndex.value = 0
  isSolved.value = false
  showHint.value = false
  currentLastMove.value = null

  currentFen.value = pz.fen
  const parsed = parseFEN(pz.fen)
  currentBoard.value = parsed.board
}

// 重置当前残局
function resetCurrentPuzzle() {
  if (currentPuzzle.value) {
    loadPuzzle(currentPuzzle.value)
    ElMessage.info('已重新摆好开局阵势')
  }
}

// 切换关卡
function selectPuzzle(pz) {
  loadPuzzle(pz)
  activeTab.value = 'board'
}

// 玩家走子
async function onPlayerMove(m) {
  if (isSolved.value) {
    ElMessage.success('本局已成功参透破局，可进入下一关')
    return
  }
  const fr = m.from.r !== undefined ? m.from.r : m.from.row
  const fc = m.from.c !== undefined ? m.from.c : m.from.col
  const tr = m.to.r !== undefined ? m.to.r : m.to.row
  const tc = m.to.c !== undefined ? m.to.c : m.to.col

  try {
    const headers = { 'Content-Type': 'application/json' }
    if (userStore.token) headers['Authorization'] = `Bearer ${userStore.token}`

    const res = await fetch(`/api/puzzles/${currentPuzzle.value.id}/move`, {
      method: 'POST',
      headers,
      body: JSON.stringify({
        from: { r: fr, c: fc },
        to: { r: tr, c: tc },
        stepIndex: stepIndex.value
      })
    })
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '走法校验失败')

    if (!data.valid) {
      ElMessage.warning(data.message || '此招未中要害，请再思量～')
      return
    }

    // 红方走法正确！更新本地盘面
    const b = currentBoard.value
    const targetPiece = b[tr][tc]
    if (targetPiece) {
      playCaptureSound()
    } else {
      playMoveSound()
    }
    b[tr][tc] = b[fr][fc]
    b[fr][fc] = null
    currentLastMove.value = { from: { r: fr, c: fc }, to: { r: tr, c: tc } }
    currentFen.value = boardToFEN(b, 'b')

    // 是否已经达成绝杀
    if (data.finished) {
      isSolved.value = true
      playWinSound()
      ElMessage.success(data.message || '神来之笔！绝杀破局成功！')
      if (currentPuzzle.value.id === dailyPuzzle.value?.id) {
        dailyPuzzle.value.completed = true
        if (data.streak) streak.value = data.streak
      }
      // 更新列表中的完成状态
      const found = puzzles.value.find((p) => p.id === currentPuzzle.value.id)
      if (found) found.completed = true
      return
    }

    // 黑方自动应手回防
    if (data.reply) {
      stepIndex.value++
      const rp = data.reply
      const rfr = rp.from.r !== undefined ? rp.from.r : rp.from.row
      const rfc = rp.from.c !== undefined ? rp.from.c : rp.from.col
      const rtr = rp.to.r !== undefined ? rp.to.r : rp.to.row
      const rtc = rp.to.c !== undefined ? rp.to.c : rp.to.col

      setTimeout(() => {
        const replyTarget = b[rtr][rtc]
        if (replyTarget) {
          playCaptureSound()
        } else {
          playMoveSound()
        }
        b[rtr][rtc] = b[rfr][rfc]
        b[rfr][rfc] = null
        currentLastMove.value = { from: { r: rfr, c: rfc }, to: { r: rtr, c: rtc } }
        currentFen.value = boardToFEN(b, 'w')
        ElMessage.info(`黑方被迫应对：${rp.notation || '变着防守'}`)
      }, 420)
    }
  } catch (err) {
    ElMessage.error(err.message)
  }
}

onMounted(() => {
  fetchDailyAndList()
})
</script>

<template>
  <div class="puzzle-page" v-if="currentPuzzle">
    <!-- 左侧：残局棋盘与解谜舞台 -->
    <div class="board-area">
      <!-- 关卡铭文题匾 -->
      <div class="wood-panel puzzle-title-bar">
        <div class="pt-main">
          <span class="pt-source">{{ currentPuzzle.source }}</span>
          <h2 class="font-brush pt-title">{{ currentPuzzle.title }}</h2>
          <span class="diff-tag" :class="currentPuzzle.difficulty">
            {{ currentPuzzle.difficulty === 'easy' ? '初学' : (currentPuzzle.difficulty === 'medium' ? '精进' : '大师') }}
          </span>
        </div>
        <p class="pt-desc">{{ currentPuzzle.description }}</p>
      </div>

      <!-- 棋盘舞台 -->
      <div class="board-stage">
        <XqBoard
          :fen="currentFen"
          :interactive="!isSolved"
          my-color="red"
          :last-move="currentLastMove"
          @move="onPlayerMove"
        />

        <!-- 破局成功朱砂通关金印 -->
        <transition name="stamp-pop">
          <div v-if="isSolved" class="seal-overlay">
            <div class="ancient-seal">
              <span class="seal-txt font-brush">破局成功</span>
              <small>参透玄机</small>
            </div>
          </div>
        </transition>
      </div>

      <!-- 底部控制与锦囊操作 -->
      <div class="wood-panel puzzle-controls">
        <div class="action-buttons">
          <el-button size="default" @click="resetCurrentPuzzle">重摆棋阵</el-button>
          <el-button size="default" type="warning" plain @click="showHint = !showHint">
            {{ showHint ? '隐藏锦囊' : '妙着锦囊' }}
          </el-button>
          <el-button size="default" :type="soundMuted ? 'info' : 'warning'" plain @click="toggleSound">
            {{ soundMuted ? '🔇 静音' : '🔊 音效' }}
          </el-button>
          <el-button size="default" plain @click="router.push('/')">大厅</el-button>
        </div>

        <transition name="hint-slide">
          <div v-if="showHint" class="hint-box">
            <b>妙着点拨：</b>{{ currentPuzzle.hint }}
          </div>
        </transition>
      </div>
    </div>

    <!-- 右侧：每日打卡与历代关卡名录 -->
    <div class="side-area dark-panel">
      <el-tabs v-model="activeTab" class="side-tabs">
        <!-- Tab 1: 每日一题 -->
        <el-tab-pane label="晨钟暮鼓 · 每日一题" name="daily">
          <div class="pane-body daily-pane" v-if="dailyPuzzle">
            <div class="daily-hero-card">
              <div class="daily-head">
                <span class="daily-badge">今日名局</span>
                <span class="streak-tag">连续打卡 <b>{{ streak }}</b> 天</span>
              </div>
              <h3 class="font-brush daily-title">{{ dailyPuzzle.title }}</h3>
              <p class="daily-source">选自 {{ dailyPuzzle.source }}</p>
              <p class="daily-desc">{{ dailyPuzzle.description }}</p>

              <div class="daily-stamp-status">
                <span v-if="dailyPuzzle.completed" class="stamped-tag">已参透打卡 ✓</span>
                <span v-else class="unstamped-tag">今日待破局 ⏳</span>
              </div>

              <el-button
                type="primary"
                class="play-daily-btn"
                @click="loadPuzzle(dailyPuzzle)"
              >
                {{ currentPuzzle.id === dailyPuzzle.id ? '正在推演本局' : '载入今日名局' }}
              </el-button>
            </div>

            <div class="daily-intro">
              <h4 class="font-brush">破局通规</h4>
              <p>1. 棋友执红先行，步步紧逼连照绝杀；</p>
              <p>2. 黑方智能防守，一旦红方攻势走软即破局受挫；</p>
              <p>3. 每天零点系统准时轮换传世名局，参透即可盖戳打卡！</p>
            </div>
          </div>
        </el-tab-pane>

        <!-- Tab 2: 关卡名录 -->
        <el-tab-pane :label="`历代名谱 (${puzzles.length})`" name="stages">
          <div class="pane-body stages-pane">
            <div
              v-for="pz in puzzles"
              :key="pz.id"
              class="stage-card"
              :class="{ active: currentPuzzle.id === pz.id, done: pz.completed }"
              @click="selectPuzzle(pz)"
            >
              <div class="stage-left">
                <span class="stage-lvl">第 {{ pz.level }} 关</span>
                <div class="stage-title-line">
                  <span class="stage-title font-brush">{{ pz.title }}</span>
                  <span class="stage-source"><small>{{ pz.source }}</small></span>
                </div>
              </div>
              <div class="stage-right">
                <span v-if="pz.completed" class="done-mark">已破</span>
                <span v-else class="undone-mark">未破</span>
              </div>
            </div>
          </div>
        </el-tab-pane>
      </el-tabs>
    </div>
  </div>

  <div v-else class="loading-wrap">
    <div class="wood-panel loading-card">
      <h3 class="font-brush">正在开启残局密卷…</h3>
    </div>
  </div>
</template>

<style scoped>
.puzzle-page {
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

.puzzle-title-bar {
  padding: 10px 18px;
}

.pt-main {
  display: flex;
  align-items: center;
  gap: 10px;
}

.pt-source {
  font-size: 12px;
  color: #8c6a38;
}

.pt-title {
  margin: 0;
  font-size: 22px;
  color: #7a2415;
  letter-spacing: 2px;
}

.diff-tag {
  font-size: 11px;
  font-weight: 700;
  padding: 1px 6px;
  border-radius: 4px;
}

.diff-tag.easy {
  background: #2a5a3a;
  color: #c0f5d0;
}

.diff-tag.medium {
  background: #a86c23;
  color: #fff0d0;
}

.diff-tag.hard {
  background: #a82323;
  color: #f5d0d0;
}

.pt-desc {
  margin: 4px 0 0;
  font-size: 13px;
  color: #6b4a24;
}

.board-stage {
  position: relative;
  flex: 1;
  min-height: 0;
  display: flex;
  align-items: center;
  justify-content: center;
}

.seal-overlay {
  position: absolute;
  top: 30%;
  pointer-events: none;
  z-index: 10;
}

.ancient-seal {
  width: 140px;
  height: 140px;
  border: 4px double #b22818;
  border-radius: 8px;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  background: rgba(220, 50, 30, 0.12);
  color: #b22818;
  box-shadow: 0 0 20px rgba(178, 40, 24, 0.4), inset 0 0 14px rgba(178, 40, 24, 0.3);
  transform: rotate(-14deg);
}

.seal-txt {
  font-size: 30px;
  letter-spacing: 4px;
  line-height: 1.1;
  font-weight: 900;
}

.ancient-seal small {
  font-size: 12px;
  letter-spacing: 2px;
  margin-top: 4px;
}

.stamp-pop-enter-active {
  animation: stampImpact 0.35s cubic-bezier(0.18, 0.89, 0.32, 1.28);
}

@keyframes stampImpact {
  0% {
    transform: scale(2.2) rotate(-28deg);
    opacity: 0;
  }
  100% {
    transform: scale(1) rotate(-14deg);
    opacity: 1;
  }
}

.puzzle-controls {
  padding: 10px 16px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.action-buttons {
  display: flex;
  gap: 8px;
  justify-content: center;
  flex-wrap: wrap;
}

.hint-box {
  background: rgba(201, 162, 75, 0.25);
  border: 1px dashed #8a6a3c;
  padding: 8px 12px;
  border-radius: 6px;
  font-size: 13px;
  color: #5a3818;
}

.hint-slide-enter-active,
.hint-slide-leave-active {
  transition: all 0.25s ease-out;
}

.hint-slide-enter-from,
.hint-slide-leave-to {
  opacity: 0;
  transform: translateY(-6px);
}

.side-area {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  overflow: hidden;
}

.side-tabs {
  display: flex;
  flex-direction: column;
  flex: 1;
  height: 100%;
  min-height: 0;
}

:deep(.el-tabs__header) {
  padding: 0 18px;
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
  min-height: 0;
  display: flex;
  flex-direction: column;
}

.pane-body {
  height: 100%;
  flex: 1;
  min-height: 0;
  overflow-y: auto;
  padding: 14px 16px;
}

.daily-hero-card {
  background: rgba(45, 26, 12, 0.85);
  border: 1px solid rgba(201, 162, 75, 0.35);
  border-radius: 8px;
  padding: 16px;
  margin-bottom: 16px;
}

.daily-head {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.daily-badge {
  background: #a83323;
  color: #f5e4c0;
  font-size: 11px;
  font-weight: 700;
  padding: 2px 6px;
  border-radius: 4px;
}

.streak-tag {
  color: #d8c39a;
  font-size: 12px;
}

.streak-tag b {
  color: #f0d89a;
  font-size: 16px;
}

.daily-title {
  margin: 4px 0 2px;
  font-size: 22px;
  color: #f0d89a;
  letter-spacing: 2px;
}

.daily-source {
  margin: 0 0 8px;
  font-size: 12px;
  color: #b9a171;
}

.daily-desc {
  font-size: 13px;
  color: #efe0c0;
  line-height: 1.6;
  margin-bottom: 12px;
}

.daily-stamp-status {
  margin-bottom: 12px;
}

.stamped-tag {
  color: #8ce09a;
  font-weight: 700;
  font-size: 13px;
}

.unstamped-tag {
  color: #e0b060;
  font-size: 13px;
}

.play-daily-btn {
  width: 100%;
  letter-spacing: 4px;
}

.daily-intro {
  color: #b9a171;
  font-size: 12px;
  line-height: 1.8;
  padding: 10px 0;
  border-top: 1px dashed rgba(201, 162, 75, 0.25);
}

.daily-intro h4 {
  margin: 0 0 6px;
  color: #f0d89a;
  font-size: 14px;
}

.stages-pane {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding-bottom: 24px;
}

.stage-card {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 10px 14px;
  background: rgba(40, 22, 10, 0.7);
  border: 1px solid rgba(201, 162, 75, 0.25);
  border-radius: 6px;
  cursor: pointer;
  transition: all 0.22s;
}

.stage-card:hover {
  transform: translateX(4px);
  background: rgba(201, 162, 75, 0.15);
  border-color: #c9a24b;
}

.stage-card.active {
  border-color: #a83323;
  box-shadow: 0 0 0 2px rgba(168, 51, 35, 0.4);
  background: rgba(168, 51, 35, 0.25);
}

.stage-lvl {
  font-size: 11px;
  color: #b9a171;
}

.stage-title {
  font-size: 16px;
  color: #f0d89a;
  margin-right: 6px;
}

.stage-source {
  color: #9a7a4a;
  font-size: 12px;
}

.done-mark {
  color: #8ce09a;
  font-weight: 700;
  font-size: 12px;
}

.undone-mark {
  color: #8a6638;
  font-size: 12px;
}

.loading-wrap {
  display: grid;
  place-items: center;
  min-height: calc(100vh - 120px);
}
</style>
