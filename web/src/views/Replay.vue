<!--
web/src/views/Replay.vue
页面：对局交互式复盘与回放系统
职责：
- 加载历史对局详情、双方棋手与棋谱记录
- 支持逐步推演、自动播放/暂停（1.0x / 2.0x 调速）、进度条无级拖拽与键盘快捷键
- 联动 Canvas 棋盘（中国象棋/五子棋）动态渲染第 k 步盘面与真实落子音效
-->

<script setup>
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import XqBoard from '../components/XqBoard.vue'
import GomokuBoard from '../components/GomokuBoard.vue'
import PlayerCard from '../components/PlayerCard.vue'
import { computeXiangqiStep, computeGomokuStep, INITIAL_XIANGQI_FEN } from '../lib/replay.js'
import { playMoveSound, playCaptureSound, isMuted, setMuted } from '../lib/audio.js'

const route = useRoute()
const router = useRouter()
const gameId = route.params.id

const game = ref(null)
const moves = ref([])
const currentStep = ref(0)
const isPlaying = ref(false)
const playSpeed = ref(1200) // 1.2s 一步
const loading = ref(true)
const soundMuted = ref(isMuted())

let playTimer = null

const totalSteps = computed(() => moves.value.length)

// 象棋当前局面
const xiangqiState = computed(() => {
  if (!game.value || game.value.gameType === 'gomoku') {
    return { fen: INITIAL_XIANGQI_FEN, lastMove: null }
  }
  return computeXiangqiStep(moves.value, currentStep.value)
})

// 五子棋当前局面
const gomokuState = computed(() => {
  if (!game.value || game.value.gameType !== 'gomoku') {
    return { board: [], lastMove: null }
  }
  return computeGomokuStep(moves.value, currentStep.value)
})

const REASON_CN = {
  checkmate: '将死', stalemate: '困毙', perpetual_check: '长将判负',
  perpetual_chase: '长捉判负', repeat_draw: '双方不变作和', timeout: '超时判负',
  resign: '认输', draw_accepted: '同意和棋', disconnect: '离线判负',
  five_in_a_row: '五子连珠胜', board_full: '满盘作和'
}

// 整理棋谱回合
const rounds = computed(() => {
  const out = []
  for (let i = 0; i < moves.value.length; i += 2) {
    out.push({
      no: i / 2 + 1,
      first: moves.value[i],
      firstStep: i + 1,
      second: moves.value[i + 1] || null,
      secondStep: i + 2
    })
  }
  return out
})

async function fetchGameDetail() {
  loading.value = true
  try {
    const res = await fetch(`/api/games/${gameId}`)
    const data = await res.json()
    if (!res.ok) throw new Error(data.error || '获取对局记录失败')
    game.value = data.game
    try {
      moves.value = data.game.moves ? JSON.parse(data.game.moves) : []
    } catch {
      moves.value = []
    }
    currentStep.value = moves.value.length // 默认停在终局
  } catch (err) {
    ElMessage.error(err.message)
  } finally {
    loading.value = false
  }
}

function toggleSound() {
  soundMuted.value = !soundMuted.value
  setMuted(soundMuted.value)
  if (!soundMuted.value) {
    playMoveSound()
  }
}

function goToStep(step, playAudio = true) {
  const clamped = Math.max(0, Math.min(step, totalSteps.value))
  if (clamped !== currentStep.value) {
    if (playAudio && clamped > currentStep.value) {
      const m = moves.value[clamped - 1]
      if (game.value?.gameType !== 'gomoku' && m && m.captured && m.captured !== 0) {
        playCaptureSound()
      } else {
        playMoveSound()
      }
    }
    currentStep.value = clamped
  }
}

function firstStep() {
  pause()
  goToStep(0, false)
}

function prevStep() {
  pause()
  goToStep(currentStep.value - 1, false)
}

function nextStep() {
  goToStep(currentStep.value + 1, true)
}

function lastStep() {
  pause()
  goToStep(totalSteps.value, false)
}

function togglePlay() {
  if (isPlaying.value) {
    pause()
  } else {
    play()
  }
}

function play() {
  if (currentStep.value >= totalSteps.value) {
    currentStep.value = 0
  }
  isPlaying.value = true
  playTimer = setInterval(() => {
    if (currentStep.value < totalSteps.value) {
      nextStep()
    } else {
      pause()
    }
  }, playSpeed.value)
}

function pause() {
  isPlaying.value = false
  if (playTimer) {
    clearInterval(playTimer)
    playTimer = null
  }
}

function toggleSpeed() {
  playSpeed.value = playSpeed.value === 1200 ? 600 : 1200
  if (isPlaying.value) {
    pause()
    play()
  }
}

function handleKeydown(e) {
  if (e.target && (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA')) return
  if (e.key === 'ArrowLeft') {
    e.preventDefault()
    prevStep()
  } else if (e.key === 'ArrowRight') {
    e.preventDefault()
    nextStep()
  } else if (e.key === ' ') {
    e.preventDefault()
    togglePlay()
  } else if (e.key === 'Home') {
    e.preventDefault()
    firstStep()
  } else if (e.key === 'End') {
    e.preventDefault()
    lastStep()
  }
}

onMounted(() => {
  fetchGameDetail()
  window.addEventListener('keydown', handleKeydown)
})

onBeforeUnmount(() => {
  pause()
  window.removeEventListener('keydown', handleKeydown)
})
</script>

<template>
  <div class="replay-page" v-if="game">
    <!-- 左侧：棋盘推演与播放控制 -->
    <div class="board-area">
      <!-- 上方棋手卡片 -->
      <PlayerCard
        :seat="{ name: game.gameType === 'gomoku' ? game.blackName : game.blackName, online: true, timeLeft: 0 }"
        :side="game.gameType === 'gomoku' ? 'white' : 'black'"
        :game-type="game.gameType"
        :active="false"
      />

      <!-- 棋盘舞台 -->
      <div class="board-stage">
        <GomokuBoard
          v-if="game.gameType === 'gomoku'"
          :board="gomokuState.board"
          :interactive="false"
          :last-move="gomokuState.lastMove"
        />
        <XqBoard
          v-else
          :fen="xiangqiState.fen"
          :interactive="false"
          :last-move="xiangqiState.lastMove"
        />
      </div>

      <!-- 下方棋手卡片 -->
      <PlayerCard
        :seat="{ name: game.gameType === 'gomoku' ? game.redName : game.redName, online: true, timeLeft: 0 }"
        :side="game.gameType === 'gomoku' ? 'black' : 'red'"
        :game-type="game.gameType"
        :active="false"
      />

      <!-- 播放器控制控制台 -->
      <div class="wood-panel controls-card">
        <div class="slider-row">
          <span class="step-num font-brush">{{ currentStep }} / {{ totalSteps }}</span>
          <el-slider
            v-model="currentStep"
            :max="totalSteps"
            :show-tooltip="true"
            @change="(val) => goToStep(val, false)"
          />
        </div>

        <div class="buttons-row">
          <div class="ctrl-group">
            <el-button size="default" :disabled="currentStep <= 0" @click="firstStep">|< 开局</el-button>
            <el-button size="default" :disabled="currentStep <= 0" @click="prevStep">< 上步</el-button>
            <el-button size="default" type="primary" @click="togglePlay">
              {{ isPlaying ? '❚❚ 暂停' : '▶ 播放' }}
            </el-button>
            <el-button size="default" :disabled="currentStep >= totalSteps" @click="nextStep">下步 ></el-button>
            <el-button size="default" :disabled="currentStep >= totalSteps" @click="lastStep">终局 >|</el-button>
          </div>

          <div class="opt-group">
            <el-button size="default" plain @click="toggleSpeed">
              {{ playSpeed === 1200 ? '1.0x 步速' : '2.0x 快速' }}
            </el-button>
            <el-button size="default" :type="soundMuted ? 'info' : 'warning'" plain @click="toggleSound">
              {{ soundMuted ? '🔇 静音' : '🔊 音效开' }}
            </el-button>
            <el-button size="default" plain @click="router.push('/')">大厅</el-button>
          </div>
        </div>
      </div>
    </div>

    <!-- 右侧：对局档案信息与棋谱列表 -->
    <div class="side-area dark-panel">
      <!-- 档案简报 -->
      <div class="game-meta-card">
        <h3 class="font-brush">对 局 史 卷 · 复 盘</h3>
        <div class="meta-line">
          <span>房间号：<b class="font-brush">{{ game.code }}</b></span>
          <el-tag size="small" :type="game.gameType === 'gomoku' ? 'success' : 'danger'">
            {{ game.gameType === 'gomoku' ? '五子棋' : '中国象棋' }}
          </el-tag>
        </div>
        <div class="vs-line">
          <template v-if="game.gameType === 'gomoku'">
            <span :class="{ winner: game.result === 'black' }">{{ game.redName }} (黑)</span>
            <span class="vs">vs</span>
            <span :class="{ winner: game.result === 'white' }">{{ game.blackName }} (白)</span>
          </template>
          <template v-else>
            <span :class="{ winner: game.result === 'red' }">{{ game.redName }} (红)</span>
            <span class="vs">vs</span>
            <span :class="{ winner: game.result === 'black' }">{{ game.blackName }} (黑)</span>
          </template>
        </div>
        <div class="result-line">
          <span>终局裁定：<b>{{ REASON_CN[game.reason] || game.reason }}</b></span>
        </div>
        <div class="time-line">
          <span>结束时刻：{{ game.endedAt }}</span>
        </div>
      </div>

      <!-- 棋谱明细表格 -->
      <div class="moves-container">
        <table class="moves-table">
          <thead>
            <tr>
              <th class="c-no">回</th>
              <th class="c-col">{{ game.gameType === 'gomoku' ? '黑方落子' : '红方走法' }}</th>
              <th class="c-col">{{ game.gameType === 'gomoku' ? '白方落子' : '黑方走法' }}</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="r in rounds" :key="r.no">
              <td class="c-no">{{ r.no }}</td>
              <td
                class="c-cell"
                :class="{ active: currentStep === r.firstStep }"
                @click="goToStep(r.firstStep)"
              >
                {{ r.first?.notation || '' }}
              </td>
              <td
                class="c-cell"
                :class="{ active: currentStep === r.secondStep }"
                @click="r.second && goToStep(r.secondStep)"
              >
                {{ r.second?.notation || '' }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>

  <div v-else-if="loading" class="loading-state">
    <div class="wood-panel loading-card">
      <h3 class="font-brush">正在展开对局卷轴…</h3>
    </div>
  </div>
</template>

<style scoped>
.replay-page {
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

.controls-card {
  padding: 10px 16px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.slider-row {
  display: flex;
  align-items: center;
  gap: 14px;
}

.slider-row :deep(.el-slider) {
  flex: 1;
}

.step-num {
  font-size: 18px;
  font-weight: 700;
  color: #7a2415;
  min-width: 72px;
}

.buttons-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
}

.ctrl-group,
.opt-group {
  display: flex;
  gap: 6px;
}

.side-area {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
  overflow: hidden;
}

.game-meta-card {
  padding: 14px 16px;
  border-bottom: 1px solid rgba(201, 162, 75, 0.35);
  background: rgba(30, 16, 7, 0.65);
}

.game-meta-card h3 {
  margin: 0 0 8px;
  font-size: 20px;
  color: #f0d89a;
  letter-spacing: 2px;
}

.meta-line {
  display: flex;
  justify-content: space-between;
  align-items: center;
  color: #d8c39a;
  font-size: 13px;
  margin-bottom: 6px;
}

.vs-line {
  font-size: 15px;
  font-weight: 700;
  color: #efe0c0;
  margin: 6px 0;
  display: flex;
  align-items: center;
  gap: 8px;
}

.vs {
  font-size: 12px;
  color: #c9a24b;
}

.winner {
  color: #e05030;
}

.result-line,
.time-line {
  font-size: 12px;
  color: #b9a171;
  margin-top: 3px;
}

.moves-container {
  flex: 1;
  overflow-y: auto;
  min-height: 0;
}

.moves-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 14px;
}

.moves-table th {
  position: sticky;
  top: 0;
  background: #3e240f;
  color: #f0d89a;
  padding: 8px 6px;
  font-weight: 600;
  z-index: 2;
  border-bottom: 1px solid rgba(201, 162, 75, 0.3);
}

.moves-table td {
  padding: 6px;
  text-align: center;
  border-bottom: 1px dashed rgba(120, 80, 40, 0.25);
}

.c-no {
  width: 38px;
  color: #b9a171;
  font-size: 12px;
}

.c-cell {
  cursor: pointer;
  color: #e8d8be;
  transition: all 0.2s;
  border-radius: 4px;
}

.c-cell:hover {
  background: rgba(201, 162, 75, 0.2);
  color: #ffffff;
}

.c-cell.active {
  background: #a83323;
  color: #ffffff;
  font-weight: 700;
  box-shadow: 0 0 8px rgba(168, 51, 35, 0.6);
}

.loading-state {
  display: grid;
  place-items: center;
  min-height: calc(100vh - 120px);
}

.loading-card {
  padding: 30px 40px;
}
</style>
