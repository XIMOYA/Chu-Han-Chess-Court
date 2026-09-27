<!--
web/src/components/MoveList.vue
组件：局内历史棋谱记录列表
职责：
- 将连续走步归整为“回合数 + 红方着法 + 黑方着法”表格
- 监听落子推进并自动平滑滚动贴紧最新着法
-->

<script setup>
import { computed, nextTick, ref, watch } from 'vue'

const props = defineProps({
  moves: { type: Array, default: () => [] },
  gameType: { type: String, default: 'xiangqi' }
})

const listEl = ref(null)

const firstTitle = computed(() => (props.gameType === 'gomoku' ? '黑方' : '红方'))
const secondTitle = computed(() => (props.gameType === 'gomoku' ? '白方' : '黑方'))

const rounds = computed(() => {
  const out = []
  for (let i = 0; i < props.moves.length; i += 2) {
    out.push({
      no: i / 2 + 1,
      red: props.moves[i],
      black: props.moves[i + 1] || null
    })
  }
  return out
})

watch(
  () => props.moves.length,
  async () => {
    await nextTick()
    if (listEl.value) listEl.value.scrollTop = listEl.value.scrollHeight
  }
)
</script>

<template>
  <div ref="listEl" class="move-list">
    <table>
      <thead>
        <tr>
          <th class="c-no">回</th>
          <th class="c-red">{{ firstTitle }}</th>
          <th class="c-black">{{ secondTitle }}</th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="r in rounds" :key="r.no">
          <td class="c-no">{{ r.no }}</td>
          <td class="c-red">{{ r.red ? r.red.notation : '' }}</td>
          <td class="c-black">{{ r.black ? r.black.notation : '' }}</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<style scoped>
.move-list {
  height: 100%;
  overflow-y: auto;
}

table {
  width: 100%;
  border-collapse: collapse;
  font-size: 15px;
}

th {
  position: sticky;
  top: 0;
  background: #4a2e15;
  color: #f0d89a;
  padding: 8px 6px;
  font-weight: 600;
}

td {
  padding: 6px;
  text-align: center;
  border-bottom: 1px dashed rgba(120, 80, 40, 0.3);
}

tbody tr {
  transition: background 0.3s;
}

tbody tr:last-child {
  animation: moveHighlight 1.4s ease-out;
}

@keyframes moveHighlight {
  0% {
    background: rgba(201, 162, 75, 0.45);
  }
  100% {
    background: transparent;
  }
}

.c-no {
  width: 34px;
  opacity: 0.7;
}

.c-red {
  color: #a83323;
  font-weight: 600;
}

.c-black {
  color: #2c2c2c;
  font-weight: 600;
}
</style>
