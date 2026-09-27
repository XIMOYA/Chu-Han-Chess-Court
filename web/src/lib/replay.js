/*
web/src/lib/replay.js
模块：对局历史步骤瞬时推演计算器
职责：
- 从对局开局初始状态单向快照推演第 k 步的盘面
- 提供中国象棋 10x9 动态 FEN 还原与五子棋 15x15 盘面矩阵重建
*/

import { parseFEN } from './board.js'

export const INITIAL_XIANGQI_FEN = 'rnbakabnr/9/1c5c1/p1p1p1p1p/9/9/P1P1P1P1P/1C5C1/9/RNBAKABNR w'

const TYPE_CHARS = {
  king: 'k',
  advisor: 'a',
  elephant: 'b',
  horse: 'n',
  rook: 'r',
  cannon: 'c',
  pawn: 'p'
}

// boardToFEN 将 10x9 棋盘对象转回标准 FEN 局面串
export function boardToFEN(board, side = 'w') {
  const rows = []
  for (let r = 0; r < 10; r++) {
    let emptyCount = 0
    let rowStr = ''
    for (let c = 0; c < 9; c++) {
      const p = board[r][c]
      if (!p) {
        emptyCount++
      } else {
        if (emptyCount > 0) {
          rowStr += emptyCount
          emptyCount = 0
        }
        const ch = TYPE_CHARS[p.type] || 'p'
        rowStr += p.color === 'red' ? ch.toUpperCase() : ch.toLowerCase()
      }
    }
    if (emptyCount > 0) {
      rowStr += emptyCount
    }
    rows.push(rowStr)
  }
  return rows.join('/') + ' ' + side
}

// computeXiangqiStep 计算象棋在第 k 步时的 FEN 局面与最后一步着法
export function computeXiangqiStep(moves, step) {
  const { board } = parseFEN(INITIAL_XIANGQI_FEN)
  const clampedStep = Math.max(0, Math.min(step, moves.length))

  for (let i = 0; i < clampedStep; i++) {
    const m = moves[i]
    if (!m || !m.from || !m.to) continue
    const fr = m.from.r !== undefined ? m.from.r : m.from.row
    const fc = m.from.c !== undefined ? m.from.c : m.from.col
    const tr = m.to.r !== undefined ? m.to.r : m.to.row
    const tc = m.to.c !== undefined ? m.to.c : m.to.col

    const piece = board[fr][fc]
    if (piece) {
      board[tr][tc] = piece
      board[fr][fc] = null
    }
  }

  const side = clampedStep % 2 === 0 ? 'w' : 'b'
  const fen = boardToFEN(board, side)
  const lastMove = clampedStep > 0 ? moves[clampedStep - 1] : null

  return { fen, lastMove }
}

// computeGomokuStep 计算五子棋在第 k 步时的 15x15 盘面与最后落子
export function computeGomokuStep(moves, step) {
  const board = Array.from({ length: 15 }, () => Array(15).fill(0))
  const clampedStep = Math.max(0, Math.min(step, moves.length))

  for (let i = 0; i < clampedStep; i++) {
    const m = moves[i]
    if (!m) continue
    const to = m.to || m.pos
    if (!to) continue
    const r = to.r !== undefined ? to.r : to.row
    const c = to.c !== undefined ? to.c : to.col
    if (r >= 0 && r < 15 && c >= 0 && c < 15) {
      const color = m.color ? m.color : (i % 2 === 0 ? 1 : -1)
      board[r][c] = color
    }
  }

  const lastMove = clampedStep > 0 ? moves[clampedStep - 1] : null
  return { board, lastMove }
}
