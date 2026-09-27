<!--
web/src/components/XqBoard.vue
组件：中国象棋 Canvas 棋盘
职责：
- 绘制古风木质棋盘底图、线条、九宫、楚河汉界与立体象牙质感棋子
- 支持黑方视角 180° 翻转（屏幕像素坐标与棋局模型坐标自动转换）
- 处理玩家触控/点击交互，高亮选中子、合法落子点、将军警示圈与最后一步痕迹
-->

<script setup>
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { parseFEN, PIECE_TEXT } from '../lib/board.js'
import { legalTargets } from '../lib/xiangqi.js'

const props = defineProps({
  fen: { type: String, required: true },
  interactive: { type: Boolean, default: false },
  myColor: { type: String, default: '' },
  checkedColor: { type: String, default: '' },
  lastMove: { type: Object, default: null },
  flipped: { type: Boolean, default: false }
})

const emit = defineEmits(['move'])

const wrap = ref(null)
const canvas = ref(null)
let ctx = null
let cssW = 360
let dpr = 1
let ro = null

const selected = ref(null)
let targets = []
let rippleAnim = null
let rippleStart = 0
const ripplePos = ref(null)
let checkAnim = null

function triggerRipple(target) {
  if (!target) return
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

function toScreen(pos) {
  if (!props.flipped) return { r: pos.r, c: pos.c }
  return { r: 9 - pos.r, c: 8 - pos.c }
}

function toModel(r, c) {
  if (!props.flipped) return { r, c }
  return { r: 9 - r, c: 8 - c }
}

function metrics() {
  const cell = cssW / 9.2
  return {
    cell,
    ox: 0.6 * cell,
    oy: 0.6 * cell,
    radius: cell * 0.44
  }
}

function resize() {
  if (!wrap.value || !canvas.value) return
  const availW = wrap.value.clientWidth || 360
  const stage = wrap.value.parentElement
  const availH = (stage ? stage.clientHeight : wrap.value.clientHeight) || (window.innerHeight - 240)

  // 纵向约束：按可用高度等比例折算最大允许宽度（象棋高宽比为 10.2 : 9.2）
  const maxWByHeight = availH * (9.2 / 10.2)
  cssW = Math.max(240, Math.min(availW, maxWByHeight))

  dpr = window.devicePixelRatio || 1
  const cv = canvas.value
  const { cell } = metrics()
  const cssH = 10.2 * cell
  cv.style.width = `${Math.round(cssW)}px`
  cv.style.height = `${Math.round(cssH)}px`
  cv.width = Math.round(cssW * dpr)
  cv.height = Math.round(cssH * dpr)
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
  draw()
}

function draw() {
  const cv = canvas.value
  const W = cssW
  const H = parseFloat(cv.style.height)
  const { cell, ox, oy, radius } = metrics()

  // 棋盘木底
  const g = ctx.createLinearGradient(0, 0, W, H)
  g.addColorStop(0, '#e9c788')
  g.addColorStop(0.5, '#dfb36e')
  g.addColorStop(1, '#d2a25c')
  ctx.fillStyle = g
  ctx.fillRect(0, 0, W, H)

  // 木纹（确定性细条纹）
  ctx.save()
  for (let i = 0; i < 46; i++) {
    const y = (H * i) / 46 + Math.sin(i * 1.7) * 3
    ctx.strokeStyle = `rgba(120,72,28,${0.05 + (i % 3) * 0.02})`
    ctx.lineWidth = 1
    ctx.beginPath()
    ctx.moveTo(0, y)
    ctx.bezierCurveTo(W * 0.3, y + Math.sin(i) * 4, W * 0.7, y - Math.cos(i) * 4, W, y + Math.sin(i * 0.6) * 3)
    ctx.stroke()
  }
  ctx.restore()

  // 外框
  ctx.strokeStyle = '#6b441d'
  ctx.lineWidth = Math.max(2, cell * 0.06)
  roundRect(ox - cell * 0.28, oy - cell * 0.28, 8 * cell + cell * 0.56, 9 * cell + cell * 0.56, 6)
  ctx.stroke()

  drawGrid(cell, ox, oy)
  drawMarks(cell, ox, oy)
  drawRiver(cell, ox, oy)

  const { board } = parseFEN(props.fen)

  // 最后一步标记
  if (props.lastMove) {
    for (const rawP of [props.lastMove.from, props.lastMove.to]) {
      const p = toScreen(rawP)
      const x = ox + p.c * cell
      const y = oy + p.r * cell
      ctx.strokeStyle = 'rgba(168,51,35,0.55)'
      ctx.lineWidth = 2
      roundRect(x - cell * 0.42, y - cell * 0.42, cell * 0.84, cell * 0.84, 4)
      ctx.stroke()
    }
  }

  // 棋子
  for (let r = 0; r < 10; r++) {
    for (let c = 0; c < 9; c++) {
      const p = board[r][c]
      if (p) {
        const sp = toScreen({ r, c })
        drawPiece(ox + sp.c * cell, oy + sp.r * cell, radius, p)
      }
    }
  }

  // 选中与可走点
  if (selected.value) {
    const sp = toScreen(selected.value)
    const x = ox + sp.c * cell
    const y = oy + sp.r * cell
    ctx.strokeStyle = '#e0b13e'
    ctx.lineWidth = 3
    ctx.beginPath()
    ctx.arc(x, y, radius + 2, 0, Math.PI * 2)
    ctx.stroke()
    for (const t of targets) {
      const st = toScreen(t)
      const tx = ox + st.c * cell
      const ty = oy + st.r * cell
      const occupied = board[t.r][t.c]
      if (occupied) {
        ctx.strokeStyle = 'rgba(168,51,35,0.85)'
        ctx.lineWidth = 3
        ctx.beginPath()
        ctx.arc(tx, ty, radius + 2, 0, Math.PI * 2)
        ctx.stroke()
      } else {
        ctx.fillStyle = 'rgba(168,51,35,0.55)'
        ctx.beginPath()
        ctx.arc(tx, ty, cell * 0.12, 0, Math.PI * 2)
        ctx.fill()
      }
    }
  }

  // 6.1 落子扩散冲击波
  if (ripplePos.value) {
    const elapsed = performance.now() - rippleStart
    if (elapsed < 320) {
      const p = Math.min(1, elapsed / 320)
      const sp = toScreen(ripplePos.value)
      const cx = ox + sp.c * cell
      const cy = oy + sp.r * cell
      ctx.save()
      ctx.beginPath()
      ctx.arc(cx, cy, radius + p * cell * 0.7, 0, Math.PI * 2)
      ctx.strokeStyle = `rgba(180, 50, 30, ${0.85 * (1 - p)})`
      ctx.lineWidth = 2.5 * (1 - p * 0.5)
      ctx.stroke()
      ctx.restore()
    }
  }

  // 将军：将位红色醒目呼吸环
  if (props.checkedColor) {
    for (let r = 0; r < 10; r++) {
      for (let c = 0; c < 9; c++) {
        const p = board[r][c]
        if (p && p.type === 'king' && p.color === props.checkedColor) {
          const sp = toScreen({ r, c })
          const x = ox + sp.c * cell
          const y = oy + sp.r * cell
          const pulse = (Math.sin(performance.now() / 160) + 1) / 2
          ctx.save()
          ctx.strokeStyle = `rgba(210, 40, 25, ${0.65 + 0.35 * pulse})`
          ctx.lineWidth = 3.5 + 2 * pulse
          ctx.beginPath()
          ctx.arc(x, y, radius + 3 + 3.5 * pulse, 0, Math.PI * 2)
          ctx.stroke()
          ctx.restore()
        }
      }
    }
  }
}

function drawGrid(cell, ox, oy) {
  ctx.strokeStyle = '#5a3a18'
  ctx.lineWidth = 1.4
  // 横线 10 条
  for (let r = 0; r <= 9; r++) {
    ctx.beginPath()
    ctx.moveTo(ox, oy + r * cell)
    ctx.lineTo(ox + 8 * cell, oy + r * cell)
    ctx.stroke()
  }
  // 竖线：两侧贯通，中间河界断开
  for (let c = 0; c <= 8; c++) {
    if (c === 0 || c === 8) {
      ctx.beginPath()
      ctx.moveTo(ox + c * cell, oy)
      ctx.lineTo(ox + c * cell, oy + 9 * cell)
      ctx.stroke()
    } else {
      ctx.beginPath()
      ctx.moveTo(ox + c * cell, oy)
      ctx.lineTo(ox + c * cell, oy + 4 * cell)
      ctx.moveTo(ox + c * cell, oy + 5 * cell)
      ctx.lineTo(ox + c * cell, oy + 9 * cell)
      ctx.stroke()
    }
  }
  // 九宫斜线
  ctx.beginPath()
  ctx.moveTo(ox + 3 * cell, oy)
  ctx.lineTo(ox + 5 * cell, oy + 2 * cell)
  ctx.moveTo(ox + 5 * cell, oy)
  ctx.lineTo(ox + 3 * cell, oy + 2 * cell)
  ctx.moveTo(ox + 3 * cell, oy + 7 * cell)
  ctx.lineTo(ox + 5 * cell, oy + 9 * cell)
  ctx.moveTo(ox + 5 * cell, oy + 7 * cell)
  ctx.lineTo(ox + 3 * cell, oy + 9 * cell)
  ctx.stroke()
}

// 炮位、兵位十字角标
function drawMarks(cell, ox, oy) {
  const mark = (r, c) => {
    const x = ox + c * cell
    const y = oy + r * cell
    const d = cell * 0.14
    const gap = cell * 0.06
    ctx.strokeStyle = '#5a3a18'
    ctx.lineWidth = 1.2
    const corners = [
      [-1, -1], [-1, 1], [1, -1], [1, 1]
    ]
    for (const [sr, sc] of corners) {
      if ((c === 0 && sc === -1) || (c === 8 && sc === 1)) continue
      ctx.beginPath()
      ctx.moveTo(x + sc * gap, y + sr * gap + sr * d)
      ctx.lineTo(x + sc * gap, y + sr * gap)
      ctx.lineTo(x + sc * gap + sc * d, y + sr * gap)
      ctx.stroke()
    }
  }
  ;[[2, 1], [2, 7], [7, 1], [7, 7]].forEach(([r, c]) => mark(r, c))
  for (let c = 0; c <= 8; c += 2) {
    mark(3, c)
    mark(6, c)
  }
}

function drawRiver(cell, ox, oy) {
  const midY = oy + 4.5 * cell
  ctx.fillStyle = '#6b441d'
  ctx.font = `bold ${cell * 0.62}px 'Ma Shan Zheng','Noto Serif SC',serif`
  ctx.textAlign = 'center'
  ctx.textBaseline = 'middle'
  const chars = props.flipped
    ? [
        { ch: '漢 界', x: ox + 2 * cell },
        { ch: '楚 河', x: ox + 6 * cell }
      ]
    : [
        { ch: '楚 河', x: ox + 2 * cell },
        { ch: '漢 界', x: ox + 6 * cell }
      ]
  for (const c of chars) ctx.fillText(c.ch, c.x, midY)
}

function drawPiece(x, y, radius, p) {
  // 阴影
  ctx.fillStyle = 'rgba(40,20,5,0.35)'
  ctx.beginPath()
  ctx.ellipse(x + 2, y + 3, radius, radius * 0.92, 0, 0, Math.PI * 2)
  ctx.fill()

  // 棋身（象牙色径向渐变）
  const grad = ctx.createRadialGradient(
    x - radius * 0.35, y - radius * 0.4, radius * 0.1,
    x, y, radius
  )
  grad.addColorStop(0, '#fbf3df')
  grad.addColorStop(0.7, '#f0e2c2')
  grad.addColorStop(1, '#d9c69a')
  ctx.fillStyle = grad
  ctx.beginPath()
  ctx.arc(x, y, radius, 0, Math.PI * 2)
  ctx.fill()

  ctx.strokeStyle = '#8a6a3c'
  ctx.lineWidth = 1.2
  ctx.stroke()

  // 内圈
  ctx.strokeStyle = p.color === 'red' ? 'rgba(168,51,35,0.7)' : 'rgba(40,40,40,0.7)'
  ctx.lineWidth = 1
  ctx.beginPath()
  ctx.arc(x, y, radius * 0.82, 0, Math.PI * 2)
  ctx.stroke()

  // 文字
  ctx.fillStyle = p.color === 'red' ? '#b23020' : '#232323'
  ctx.font = `900 ${radius * 1.18}px 'Noto Serif SC','Songti SC',serif`
  ctx.textAlign = 'center'
  ctx.textBaseline = 'middle'
  ctx.fillText(PIECE_TEXT[p.color][p.type], x, y + radius * 0.06)
}

function roundRect(x, y, w, h, r) {
  ctx.beginPath()
  ctx.moveTo(x + r, y)
  ctx.arcTo(x + w, y, x + w, y + h, r)
  ctx.arcTo(x + w, y + h, x, y + h, r)
  ctx.arcTo(x, y + h, x, y, r)
  ctx.arcTo(x, y, x + w, y, r)
  ctx.closePath()
}

function eventPoint(e) {
  const rect = canvas.value.getBoundingClientRect()
  const t = e.touches ? e.touches[0] : e
  return { x: t.clientX - rect.left, y: t.clientY - rect.top }
}

function hitPos(x, y) {
  const { cell, ox, oy } = metrics()
  const sc = Math.round((x - ox) / cell)
  const sr = Math.round((y - oy) / cell)
  if (sr < 0 || sr > 9 || sc < 0 || sc > 8) return null
  const px = ox + sc * cell
  const py = oy + sr * cell
  if (Math.hypot(x - px, y - py) > cell * 0.46) return null
  return toModel(sr, sc)
}

function select(pos) {
  selected.value = pos
  targets = legalTargets(props.fen, pos)
  draw()
}

function onTap(e) {
  if (!props.interactive) return
  if (e.touches) e.preventDefault()
  const { x, y } = eventPoint(e)
  const pos = hitPos(x, y)
  if (!pos) {
    selected.value = null
    targets = []
    draw()
    return
  }
  const { board, side } = parseFEN(props.fen)
  const p = board[pos.r][pos.c]

  if (selected.value) {
    if (targets.some((t) => t.r === pos.r && t.c === pos.c)) {
      emit('move', { from: selected.value, to: pos })
      selected.value = null
      targets = []
      return
    }
    if (p && p.color === props.myColor) {
      select(pos)
      return
    }
    selected.value = null
    targets = []
    draw()
  } else if (p && p.color === props.myColor && p.color === side) {
    select(pos)
  }
}

watch(() => props.lastMove, (newVal) => {
  if (newVal && newVal.to) {
    triggerRipple(newVal.to)
  }
}, { deep: true })

watch(() => props.checkedColor, (val) => {
  if (val) {
    function loop() {
      draw()
      checkAnim = requestAnimationFrame(loop)
    }
    if (checkAnim) cancelAnimationFrame(checkAnim)
    checkAnim = requestAnimationFrame(loop)
  } else {
    if (checkAnim) {
      cancelAnimationFrame(checkAnim)
      checkAnim = null
    }
    draw()
  }
})

watch(
  () => [props.fen, props.flipped],
  () => {
    selected.value = null
    targets = []
    draw()
  }
)

onMounted(() => {
  ctx = canvas.value.getContext('2d')
  ro = new ResizeObserver(resize)
  ro.observe(wrap.value)
  resize()
})

onBeforeUnmount(() => {
  if (rippleAnim) cancelAnimationFrame(rippleAnim)
  if (checkAnim) cancelAnimationFrame(checkAnim)
  if (ro) ro.disconnect()
})
</script>

<template>
  <div ref="wrap" class="board-wrap">
    <canvas
      ref="canvas"
      class="board-canvas"
      @click="onTap"
      @touchstart="onTap"
    ></canvas>
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
}

.board-canvas {
  display: block;
  border-radius: 8px;
  box-shadow:
    0 14px 40px rgba(0, 0, 0, 0.5),
    0 0 0 6px #5a3818,
    0 0 0 8px rgba(201, 162, 75, 0.5);
  touch-action: manipulation;
}
</style>
