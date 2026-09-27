import { parseFEN } from './board.js'

const ROWS = 10
const COLS = 9
const ORTHO = [[-1, 0], [1, 0], [0, -1], [0, 1]]

function cloneBoard(b) {
  return b.map((row) => row.map((p) => (p ? { ...p } : null)))
}

function valid(r, c) {
  return r >= 0 && r < ROWS && c >= 0 && c < COLS
}

function inPalace(r, c, color) {
  if (c < 3 || c > 5) return false
  return color === 'red' ? r >= 7 && r <= 9 : r >= 0 && r <= 2
}

function crossed(r, color) {
  return color === 'red' ? r <= 4 : r >= 5
}

function findKing(b, color) {
  for (let r = 0; r < ROWS; r++) {
    for (let c = 0; c < COLS; c++) {
      const p = b[r][c]
      if (p && p.type === 'king' && p.color === color) return { r, c }
    }
  }
  return null
}

function addMove(b, ms, fr, fc, tr, tc) {
  if (!valid(tr, tc)) return
  const t = b[tr][tc]
  if (t && t.color === b[fr][fc].color) return
  ms.push({ fr, fc, tr, tc, capt: t ? t.type : null })
}

function pieceMoves(b, r, c, p, ms) {
  switch (p.type) {
    case 'king':
      for (const [dr, dc] of ORTHO) {
        if (inPalace(r + dr, c + dc, p.color)) addMove(b, ms, r, c, r + dr, c + dc)
      }
      {
        const kp = findKing(b, p.color === 'red' ? 'black' : 'red')
        if (kp && kp.c === c) {
          let blocked = false
          const lo = Math.min(r, kp.r)
          const hi = Math.max(r, kp.r)
          for (let rr = lo + 1; rr < hi; rr++) if (b[rr][c]) blocked = true
          if (!blocked) addMove(b, ms, r, c, kp.r, kp.c)
        }
      }
      break
    case 'advisor':
      for (const [dr, dc] of [[-1, -1], [-1, 1], [1, -1], [1, 1]]) {
        if (inPalace(r + dr, c + dc, p.color)) addMove(b, ms, r, c, r + dr, c + dc)
      }
      break
    case 'elephant':
      for (const [dr, dc] of [[-2, -2], [-2, 2], [2, -2], [2, 2]]) {
        const tr = r + dr
        const tc = c + dc
        if (!valid(tr, tc) || crossed(tr, p.color)) continue
        if (b[r + dr / 2][c + dc / 2]) continue
        addMove(b, ms, r, c, tr, tc)
      }
      break
    case 'horse': {
      const steps = [[-2, -1], [-2, 1], [2, -1], [2, 1], [-1, -2], [-1, 2], [1, -2], [1, 2]]
      const legs = [[-1, 0], [-1, 0], [1, 0], [1, 0], [0, -1], [0, 1], [0, -1], [0, 1]]
      for (let i = 0; i < 8; i++) {
        const tr = r + steps[i][0]
        const tc = c + steps[i][1]
        if (!valid(tr, tc)) continue
        if (b[r + legs[i][0]][c + legs[i][1]]) continue
        addMove(b, ms, r, c, tr, tc)
      }
      break
    }
    case 'rook':
      for (const [dr, dc] of ORTHO) {
        let tr = r + dr
        let tc = c + dc
        while (valid(tr, tc)) {
          const t = b[tr][tc]
          if (!t) {
            addMove(b, ms, r, c, tr, tc)
          } else {
            if (t.color !== p.color) addMove(b, ms, r, c, tr, tc)
            break
          }
          tr += dr
          tc += dc
        }
      }
      break
    case 'cannon':
      for (const [dr, dc] of ORTHO) {
        let tr = r + dr
        let tc = c + dc
        while (valid(tr, tc) && !b[tr][tc]) {
          addMove(b, ms, r, c, tr, tc)
          tr += dr
          tc += dc
        }
        if (valid(tr, tc)) {
          tr += dr
          tc += dc
          while (valid(tr, tc)) {
            const t = b[tr][tc]
            if (t) {
              if (t.color !== p.color) addMove(b, ms, r, c, tr, tc)
              break
            }
            tr += dr
            tc += dc
          }
        }
      }
      break
    case 'pawn': {
      const fwd = p.color === 'red' ? -1 : 1
      const cands = [[r + fwd, c]]
      if (crossed(r, p.color)) cands.push([r, c - 1], [r, c + 1])
      for (const [tr, tc] of cands) addMove(b, ms, r, c, tr, tc)
      break
    }
  }
}

function isAttacked(b, rr, cc, byColor) {
  for (let r = 0; r < ROWS; r++) {
    for (let c = 0; c < COLS; c++) {
      const p = b[r][c]
      if (!p || p.color !== byColor) continue
      const ms = []
      pieceMoves(b, r, c, p, ms)
      for (const m of ms) if (m.tr === rr && m.tc === cc) return true
    }
  }
  return false
}

function inCheck(b, color) {
  const k = findKing(b, color)
  if (!k) return false
  return isAttacked(b, k.r, k.c, color === 'red' ? 'black' : 'red')
}

function applyMove(b, m) {
  const nb = cloneBoard(b)
  nb[m.tr][m.tc] = { ...nb[m.fr][m.fc] }
  nb[m.fr][m.fc] = null
  return nb
}

// legalTargets 返回指定位置棋子的合法目标点
export function legalTargets(fen, pos) {
  const { board, side } = parseFEN(fen)
  const p = board[pos.r][pos.c]
  if (!p || p.color !== side) return []
  const ms = []
  pieceMoves(board, pos.r, pos.c, p, ms)
  return ms
    .filter((m) => !inCheck(applyMove(board, m), side))
    .map((m) => ({ r: m.tr, c: m.tc }))
}
