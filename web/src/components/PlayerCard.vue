<!--
web/src/components/PlayerCard.vue
组件：棋手信息与时钟卡片
职责：
- 展示红/黑方座席名称、在线/离线标记与本方专属徽章
- 实时计算与渲染剩余思考时间时钟，支持读秒紧急闪烁
- 突出标记当前行动方激活高亮光晕与“你”标识
-->

<script setup>
import { computed } from 'vue'
import { formatClock } from '../lib/board.js'

const props = defineProps({
  seat: { type: Object, default: null },
  side: { type: String, required: true },
  active: { type: Boolean, default: false },
  you: { type: Boolean, default: false },
  gameType: { type: String, default: 'xiangqi' }
})

const clock = computed(() => formatClock(props.seat?.timeLeft ?? 0))
const lowTime = computed(() => (props.seat?.timeLeft ?? 0) < 30000 && props.active)

const badgeText = computed(() => {
  if (props.gameType === 'gomoku') {
    return props.side === 'black' ? '●' : '○'
  }
  return props.side === 'red' ? '帅' : '将'
})

const subText = computed(() => {
  if (props.gameType === 'gomoku') {
    return props.side === 'black' ? '黑方 · 先手' : '白方 · 后手'
  }
  return props.side === 'red' ? '红方 · 先手' : '黑方 · 后手'
})
</script>

<template>
  <div class="player-card" :class="[side, { active, offline: seat && !seat.online }]">
    <div class="piece-badge">
      {{ badgeText }}
    </div>
    <div class="info">
      <div class="name-line">
        <span class="name">{{ seat ? seat.name : '虚位以待' }}</span>
        <span v-if="you" class="you-tag">你</span>
        <span v-if="seat && seat.isAi" class="ai-tag">机巧</span>
        <span v-if="seat && !seat.online && !seat.isAi" class="offline-tag">离线</span>
      </div>
      <div class="sub">{{ subText }}</div>
    </div>
    <div class="clock" :class="{ low: lowTime }">
      {{ clock }}
    </div>
  </div>
</template>

<style scoped>
.player-card {
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 12px;
  border-radius: 8px;
  background: rgba(243, 228, 196, 0.92);
  border: 2px solid transparent;
  color: #432a12;
  transition: all 0.25s cubic-bezier(0.2, 0.8, 0.2, 1);
}

.player-card.active {
  border-color: #c9a24b;
  animation: activeBreathing 2.4s ease-in-out infinite;
}

@keyframes activeBreathing {
  0%, 100% {
    box-shadow: 0 0 0 2px rgba(201, 162, 75, 0.3), 0 2px 8px rgba(0, 0, 0, 0.08);
  }
  50% {
    box-shadow: 0 0 0 4px rgba(201, 162, 75, 0.65), 0 0 16px rgba(201, 162, 75, 0.35);
  }
}

.piece-badge {
  display: grid;
  place-items: center;
  width: 38px;
  height: 38px;
  border-radius: 50%;
  background: radial-gradient(circle at 35% 30%, #fbf3df, #ddc99d);
  border: 1.5px solid #8a6a3c;
  font-size: 20px;
  font-weight: 900;
  flex-shrink: 0;
}

.red .piece-badge {
  color: #b23020;
}

.black .piece-badge {
  color: #232323;
}

.white .piece-badge {
  color: #555555;
  background: radial-gradient(circle at 35% 30%, #ffffff, #dedede);
  border-color: #8c8273;
}

.info {
  flex: 1;
  min-width: 0;
}

.name-line {
  display: flex;
  align-items: center;
  gap: 6px;
}

.name {
  font-weight: 700;
  font-size: 16px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.you-tag {
  background: #a83323;
  color: #f5e4c0;
  font-size: 11px;
  padding: 0 6px;
  border-radius: 4px;
}

.ai-tag {
  background: #2a4c6a;
  color: #e6f0fa;
  font-size: 11px;
  font-weight: 700;
  padding: 0 6px;
  border-radius: 4px;
  border: 1px solid #5a88b0;
}

.offline-tag {
  background: rgba(90, 70, 50, 0.5);
  color: #efe0c0;
  font-size: 11px;
  padding: 0 6px;
  border-radius: 4px;
}

.sub {
  font-size: 12px;
  color: #8a6638;
}

.clock {
  font-variant-numeric: tabular-nums;
  font-size: 18px;
  font-weight: 700;
  padding: 4px 10px;
  border-radius: 6px;
  background: rgba(67, 42, 18, 0.12);
}

.clock.low {
  color: #b23020;
  animation: urgentHeartbeat 1s ease-in-out infinite;
  background: rgba(178, 48, 32, 0.15);
}

@keyframes urgentHeartbeat {
  0%, 100% {
    transform: scale(1);
    opacity: 1;
  }
  50% {
    transform: scale(1.05);
    opacity: 0.72;
    box-shadow: 0 0 10px rgba(178, 48, 32, 0.35);
  }
}

.offline {
  opacity: 0.6;
}

@keyframes blink {
  to {
    visibility: hidden;
  }
}
</style>
