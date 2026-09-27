<!--
web/src/components/GomokuBoard.vue
组件：五子棋 Canvas 棋盘
职责：
- 绘制 15×15 古风木质棋盘底图、精细经纬格线与天元/四隅星位
- 渲染立体高光黑曜石黑子与温润羊脂玉白子
- 处理落子交互、悬浮落子半透明预瞄、最后落子醒目标记
-->

<script setup>
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'

const props = defineProps({
  board: { type: Array, default: () => [] }, // 15x15 数组，1为黑，-1为白，0为空
  interactive: { type: Boolean, default: false },
  myColor: { type: String, default: '' }, // 'black' 或 'white'
  lastMove: { type: Object, default: null }, // { to: { row, col } }
  flipped: { type: Boolean, default: false }
})

const emit = defineEmits(['move'])

const wrap = ref(null)
const canvas = ref(null)
let ctx = null
let cssW = 360
let dpr = 1
let ro = null

const hoverPos = ref(null)
let rippleAnim = null
let rippleStart = 0
const ripplePos = ref(null)

function triggerRipple(target) {
  const tr = target.r !== undefined ? target.r : target.row
  const tc = target.c !== undefined ? target.c : target.col
  if (tr === undefined || tc === undefined) return
  ripplePos.value = { r: tr, c: tc }
  rippleStart = performance.now()
  if (rippleAnim) cancelAnimationFrame(rippleAnim)
  function step(now) {
    const elapsed = now - rippleStart
    if (elapsed < 320) {
      draw()
      rippleAnim = requestAnimationFrame(step)
    } else {
      ripplePos.value = null
      draw()
    }
  }
  rippleAnim = requestAnimationFrame(step)
}

function toScreen(r, c) {
  if (!props.flipped) return { r, c }
  return { r: 14 - r, c: 14 - c }
}

function toModel(r, c) {
  if (!props.flipped) return { r, c }
  return { r: 14 - r, c: 14 - c }
}

function metrics() {
  const cell = cssW / 15.6
  return {
    cell,
    ox: 0.8 * cell,
    oy: 0.8 * cell,
    radius: cell * 0.43
  }
}

function resize() {
  if (!wrap.value || !canvas.value) return
  const availW = wrap.value.clientWidth || 360
  const stage = wrap.value.parentElement
  const availH = (stage ? stage.clientHeight : wrap.value.clientHeight) || (window.innerHeight - 240)

  // 五子棋为 1:1 正方形网格，同时受宽度与高度双向限制
  cssW = Math.max(240, Math.min(availW, availH))

  dpr = window.devicePixelRatio || 1
  const cv = canvas.value
  const { cell } = metrics()
  const cssH = 15.6 * cell
  cv.style.width = `${Math.round(cssW)}px`
  cv.style.height = `${Math.round(cssH)}px`
  cv.width = Math.round(cssW * dpr)
  cv.height = Math.round(cssH * dpr)
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
  draw()
}

function draw() {
  const cv = canvas.value
  if (!cv || !ctx) return
  const W = cssW
  const H = parseFloat(cv.style.height) || W
  const { cell, ox, oy, radius } = metrics()

  // 1. 棋盘古木底色
  const g = ctx.createLinearGradient(0, 0, W, H)
  g.addColorStop(0, '#e9c788')
  g.addColorStop(0.5, '#dfb36e')
  g.addColorStop(1, '#d2a25c')
  ctx.fillStyle = g
  ctx.fillRect(0, 0, W, H)

  // 2. 细腻木纹
  ctx.save()
  for (let i = 0; i < 50; i++) {
    const y = (H * i) / 50 + Math.sin(i * 1.8) * 3
    ctx.strokeStyle = `rgba(120,72,28,${0.04 + (i % 3) * 0.02})`
    ctx.lineWidth = 1
    ctx.beginPath()
    ctx.moveTo(0, y)
    ctx.bezierCurveTo(W * 0.3, y + Math.sin(i) * 4, W * 0.7, y - Math.cos(i) * 4, W, y + Math.sin(i * 0.6) * 3)
    ctx.stroke()
  }
  ctx.restore()

  // 3. 边框与外阴影
  ctx.strokeStyle = '#6b441d'
  ctx.lineWidth = Math.max(2, cell * 0.08)
  ctx.strokeRect(ox - cell * 0.4, oy - cell * 0.4, 14 * cell + cell * 0.8, 14 * cell + cell * 0.8)

  // 4. 15x15 网格线
  ctx.strokeStyle = '#5a3818'
  ctx.lineWidth = 1
  for (let i = 0; i < 15; i++) {
    // 横线
    ctx.beginPath()
    ctx.moveTo(ox, oy + i * cell)
    ctx.lineTo(ox + 14 * cell, oy + i * cell)
    ctx.stroke()

    // 竖线
    ctx.beginPath()
    ctx.moveTo(ox + i * cell, oy)
    ctx.lineTo(ox + i * cell, oy + 14 * cell)
    ctx.stroke()
  }

  // 5. 天元与星位（3,3 / 3,11 / 7,7 / 11,3 / 11,11）
  const stars = [
    { r: 3, c: 3 }, { r: 3, c: 11 },
    { r: 7, c: 7 },
    { r: 11, c: 3 }, { r: 11, c: 11 }
  ]
  ctx.fillStyle = '#5a3818'
  for (const s of stars) {
    const sp = toScreen(s.r, s.c)
    ctx.beginPath()
    ctx.arc(ox + sp.c * cell, oy + sp.r * cell, Math.max(3, cell * 0.1), 0, Math.PI * 2)
    ctx.fill()
  }

  // 6. 渲染所有已落下的棋子
  const b = props.board
  if (b && b.length === 15) {
    for (let r = 0; r < 15; r++) {
      for (let c = 0; c < 15; c++) {
        const val = b[r][c]
        if (val !== 0) {
          const sp = toScreen(r, c)
          let currentRadius = radius
          if (ripplePos.value && ripplePos.value.r === r && ripplePos.value.c === c) {
            const elapsed = performance.now() - rippleStart
            const progress = Math.min(1, elapsed / 320)
            currentRadius = radius * (1.18 - 0.18 * progress)
          }
          drawStone(ox + sp.c * cell, oy + sp.r * cell, currentRadius, val === 1 ? 'black' : 'white')
        }
      }
    }
  }

  // 6.1 落子扩散冲击波
  if (ripplePos.value) {
    const elapsed = performance.now() - rippleStart
    if (elapsed < 320) {
      const p = Math.min(1, elapsed / 320)
      const sp = toScreen(ripplePos.value.r, ripplePos.value.c)
      const cx = ox + sp.c * cell
      const cy = oy + sp.r * cell
      ctx.save()
      ctx.beginPath()
      ctx.arc(cx, cy, radius + p * cell * 0.8, 0, Math.PI * 2)
      ctx.strokeStyle = `rgba(201, 162, 75, ${0.85 * (1 - p)})`
      ctx.lineWidth = 2.5 * (1 - p * 0.5)
      ctx.stroke()
      ctx.restore()
    }
  }

  // 7. 最后一手标记
  if (props.lastMove && props.lastMove.to) {
    const target = props.lastMove.to
    const tr = target.r !== undefined ? target.r : target.row
    const tc = target.c !== undefined ? target.c : target.col
    if (tr !== undefined && tc !== undefined && tr >= 0 && tr < 15 && tc >= 0 && tc < 15) {
      const sp = toScreen(tr, tc)
      const x = ox + sp.c * cell
      const y = oy + sp.r * cell
      ctx.fillStyle = '#e84128'
      ctx.beginPath()
      ctx.arc(x, y, radius * 0.22, 0, Math.PI * 2)
      ctx.fill()
      ctx.strokeStyle = '#ffffff'
      ctx.lineWidth = 1.5
      ctx.stroke()
    }
  }

  // 8. 悬停半透明预瞄棋子
  if (props.interactive && hoverPos.value && props.myColor) {
    const { r, c } = hoverPos.value
    if (b && b[r] && b[r][c] === 0) {
      const sp = toScreen(r, c)
      ctx.save()
      ctx.globalAlpha = 0.48
      drawStone(ox + sp.c * cell, oy + sp.r * cell, radius, props.myColor)
      ctx.restore()
    }
  }
}

// 绘制立体感棋子
function drawStone(x, y, r, color) {
  ctx.save()

  // 投射微阴影
  ctx.beginPath()
  ctx.arc(x + r * 0.1, y + r * 0.12, r, 0, Math.PI * 2)
  ctx.fillStyle = 'rgba(40, 20, 10, 0.35)'
  ctx.fill()

  // 棋子球体立体渐变
  const rad = ctx.createRadialGradient(
    x - r * 0.32, y - r * 0.35, r * 0.08,
    x, y, r
  )

  if (color === 'black') {
    rad.addColorStop(0, '#585858')
    rad.addColorStop(0.3, '#2a2a2a')
    rad.addColorStop(0.85, '#121212')
    rad.addColorStop(1, '#050505')
  } else {
    rad.addColorStop(0, '#ffffff')
    rad.addColorStop(0.35, '#faf7ee')
    rad.addColorStop(0.85, '#e4dccb')
    rad.addColorStop(1, '#cbc0ab')
  }

  ctx.beginPath()
  ctx.arc(x, y, r, 0, Math.PI * 2)
  ctx.fillStyle = rad
  ctx.fill()

  // 白子柔和边缘圈
  if (color === 'white') {
    ctx.strokeStyle = 'rgba(150, 130, 100, 0.4)'
    ctx.lineWidth = 1
    ctx.stroke()
  }

  // 高光反射点
  ctx.beginPath()
  ctx.arc(x - r * 0.34, y - r * 0.36, r * 0.18, 0, Math.PI * 2)
  ctx.fillStyle = color === 'black' ? 'rgba(255, 255, 255, 0.28)' : 'rgba(255, 255, 255, 0.75)'
  ctx.fill()

  ctx.restore()
}

function handlePointerMove(e) {
  if (!props.interactive) {
    hoverPos.value = null
    return
  }
  const rect = canvas.value.getBoundingClientRect()
  const scaleX = rect.width ? cssW / rect.width : 1
  const scaleY = rect.height ? (parseFloat(canvas.value.style.height) || cssW) / rect.height : 1
  const px = (e.clientX - rect.left) * scaleX
  const py = (e.clientY - rect.top) * scaleY
  const { cell, ox, oy } = metrics()

  const c = Math.round((px - ox) / cell)
  const r = Math.round((py - oy) / cell)

  if (r >= 0 && r < 15 && c >= 0 && c < 15) {
    const model = toModel(r, c)
    if (!hoverPos.value || hoverPos.value.r !== model.r || hoverPos.value.c !== model.c) {
      hoverPos.value = model
      draw()
    }
  } else {
    if (hoverPos.value) {
      hoverPos.value = null
      draw()
    }
  }
}

function handlePointerLeave() {
  if (hoverPos.value) {
    hoverPos.value = null
    draw()
  }
}

function handleClick(e) {
  if (!props.interactive) return
  const rect = canvas.value.getBoundingClientRect()
  const scaleX = rect.width ? cssW / rect.width : 1
  const scaleY = rect.height ? (parseFloat(canvas.value.style.height) || cssW) / rect.height : 1
  const px = (e.clientX - rect.left) * scaleX
  const py = (e.clientY - rect.top) * scaleY
  const { cell, ox, oy } = metrics()

  const c = Math.round((px - ox) / cell)
  const r = Math.round((py - oy) / cell)

  if (r >= 0 && r < 15 && c >= 0 && c < 15) {
    const model = toModel(r, c)
    if (props.board && props.board[model.r] && props.board[model.r][model.c] === 0) {
      emit('move', { to: { r: model.r, c: model.c, row: model.r, col: model.c } })
    }
  }
}

watch(() => props.lastMove, (newVal) => {
  if (newVal && newVal.to) {
    triggerRipple(newVal.to)
  }
}, { deep: true })

watch(() => [props.board, props.lastMove, props.interactive, props.flipped], () => {
  draw()
}, { deep: true })

onMounted(() => {
  ctx = canvas.value.getContext('2d')
  resize()
  ro = new ResizeObserver(() => resize())
  if (wrap.value) ro.observe(wrap.value)
})

onBeforeUnmount(() => {
  if (rippleAnim) cancelAnimationFrame(rippleAnim)
  if (ro) ro.disconnect()
})
</script>

<template>
  <div ref="wrap" class="board-wrap">
    <canvas
      ref="canvas"
      class="board-canvas"
      @mousemove="handlePointerMove"
      @mouseleave="handlePointerLeave"
      @click="handleClick"
    />
  </div>
</template>

<style scoped>
.board-wrap {
  width: 100%;
  height: 100%;
  max-height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  user-select: none;
  touch-action: manipulation;
  filter: drop-shadow(0 10px 24px rgba(0, 0, 0, 0.45));
}

.board-canvas {
  display: block;
  border-radius: 6px;
  cursor: pointer;
}
</style>
