const TYPE_MAP = {
  k: 'king', a: 'advisor', b: 'elephant', n: 'horse',
  r: 'rook', c: 'cannon', p: 'pawn'
}

export const PIECE_TEXT = {
  red: { king: '帅', advisor: '仕', elephant: '相', horse: '马', rook: '车', cannon: '炮', pawn: '兵' },
  black: { king: '将', advisor: '士', elephant: '象', horse: '马', rook: '车', cannon: '砲', pawn: '卒' }
}

// parseFEN 解析前端所需的棋盘：10 行 9 列，元素为 {type,color} 或 null
export function parseFEN(fen) {
  const board = Array.from({ length: 10 }, () => Array(9).fill(null))
  const fields = fen.trim().split(/\s+/)
  const ranks = fields[0].split('/')
  ranks.forEach((rank, r) => {
    let c = 0
    for (const ch of rank) {
      if (ch >= '1' && ch <= '9') {
        c += Number(ch)
        continue
      }
      const lower = ch.toLowerCase()
      board[r][c] = { type: TYPE_MAP[lower], color: ch === lower ? 'black' : 'red' }
      c++
    }
  })
  return { board, side: fields[1] === 'b' ? 'black' : 'red' }
}

export function formatClock(ms) {
  if (ms < 0) ms = 0
  const total = Math.ceil(ms / 1000)
  const m = Math.floor(total / 60)
  const s = total % 60
  return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
}
