/*
web/src/lib/audio.js
模块：基于 Web Audio API 的原生物理建模对弈音效系统
职责：
- 实时合成清脆实木落子声、双子重击吃子声、肃然将军警示与终局和弦
- 0 网络请求、0 毫秒延迟，支持静音状态持久化记录
*/

let audioCtx = null

function getContext() {
  if (typeof window === 'undefined') return null
  if (!audioCtx) {
    const AudioCtx = window.AudioContext || window.webkitAudioContext
    if (AudioCtx) {
      audioCtx = new AudioCtx()
    }
  }
  if (audioCtx && audioCtx.state === 'suspended') {
    audioCtx.resume()
  }
  return audioCtx
}

export function isMuted() {
  if (typeof localStorage === 'undefined') return false
  return localStorage.getItem('xq_sound_muted') === 'true'
}

export function setMuted(muted) {
  if (typeof localStorage !== 'undefined') {
    localStorage.setItem('xq_sound_muted', muted ? 'true' : 'false')
  }
}

// 1. 实木落子声（清脆木石敲击，带木纹共振）
export function playMoveSound() {
  if (isMuted()) return
  const ctx = getContext()
  if (!ctx) return

  const now = ctx.currentTime

  // 1.1 高频瞬态冲击敲击（Transient Thwack）
  const noiseBuf = ctx.createBuffer(1, Math.floor(ctx.sampleRate * 0.025), ctx.sampleRate)
  const output = noiseBuf.getChannelData(0)
  for (let i = 0; i < noiseBuf.length; i++) {
    output[i] = (Math.random() * 2 - 1) * Math.exp(-i / (ctx.sampleRate * 0.004))
  }
  const noiseSource = ctx.createBufferSource()
  noiseSource.buffer = noiseBuf

  const filter = ctx.createBiquadFilter()
  filter.type = 'bandpass'
  filter.frequency.setValueAtTime(1600, now)
  filter.Q.setValueAtTime(3, now)

  const noiseGain = ctx.createGain()
  noiseGain.gain.setValueAtTime(0.7, now)
  noiseGain.gain.exponentialRampToValueAtTime(0.001, now + 0.025)

  noiseSource.connect(filter)
  filter.connect(noiseGain)
  noiseGain.connect(ctx.destination)
  noiseSource.start(now)

  // 1.2 木体沉稳共振腔（Body Resonance）
  const osc = ctx.createOscillator()
  osc.type = 'triangle'
  osc.frequency.setValueAtTime(420, now)
  osc.frequency.exponentialRampToValueAtTime(110, now + 0.09)

  const oscGain = ctx.createGain()
  oscGain.gain.setValueAtTime(0.85, now)
  oscGain.gain.exponentialRampToValueAtTime(0.001, now + 0.12)

  osc.connect(oscGain)
  oscGain.connect(ctx.destination)
  osc.start(now)
  osc.stop(now + 0.13)
}

// 2. 象棋吃子声（双木相撞，力度更为浑厚）
export function playCaptureSound() {
  if (isMuted()) return
  const ctx = getContext()
  if (!ctx) return

  const now = ctx.currentTime

  // 第一下主撞击
  playMoveSound()

  // 延后 28ms 触发第二颗被击退棋子的二次擦碰沉闷共鸣
  setTimeout(() => {
    if (isMuted()) return
    const c = getContext()
    if (!c) return
    const t = c.currentTime
    const osc = c.createOscillator()
    osc.type = 'sine'
    osc.frequency.setValueAtTime(260, t)
    osc.frequency.exponentialRampToValueAtTime(80, t + 0.14)

    const g = c.createGain()
    g.gain.setValueAtTime(0.6, t)
    g.gain.exponentialRampToValueAtTime(0.001, t + 0.15)

    osc.connect(g)
    g.connect(c.destination)
    osc.start(t)
    osc.stop(t + 0.16)
  }, 28)
}

// 3. 将军警示音（金石弦鸣，杀气肃然）
export function playCheckSound() {
  if (isMuted()) return
  const ctx = getContext()
  if (!ctx) return

  const now = ctx.currentTime
  const freqs = [587.33, 880] // D5, A5 和弦金鸣

  freqs.forEach((f) => {
    const osc = ctx.createOscillator()
    osc.type = 'sine'
    osc.frequency.setValueAtTime(f, now)

    const g = ctx.createGain()
    g.gain.setValueAtTime(0.35, now)
    g.gain.exponentialRampToValueAtTime(0.001, now + 0.5)

    osc.connect(g)
    g.connect(ctx.destination)
    osc.start(now)
    osc.stop(now + 0.52)
  })
}

// 4. 胜利和弦（明亮升华的宫廷雅乐和弦）
export function playWinSound() {
  if (isMuted()) return
  const ctx = getContext()
  if (!ctx) return

  const now = ctx.currentTime
  const notes = [523.25, 659.25, 783.99, 1046.5] // C5, E5, G5, C6 大三和弦琶音

  notes.forEach((freq, idx) => {
    const startTime = now + idx * 0.08
    const osc = ctx.createOscillator()
    osc.type = 'sine'
    osc.frequency.setValueAtTime(freq, startTime)

    const g = ctx.createGain()
    g.gain.setValueAtTime(0.32, startTime)
    g.gain.exponentialRampToValueAtTime(0.001, startTime + 0.45)

    osc.connect(g)
    g.connect(ctx.destination)
    osc.start(startTime)
    osc.stop(startTime + 0.48)
  })
}

// 5. 战败收官音（低回婉转的沉稳音阶）
export function playLoseSound() {
  if (isMuted()) return
  const ctx = getContext()
  if (!ctx) return

  const now = ctx.currentTime
  const notes = [392.0, 311.13, 261.63] // G4, Eb4, C4 小三和弦下行

  notes.forEach((freq, idx) => {
    const startTime = now + idx * 0.12
    const osc = ctx.createOscillator()
    osc.type = 'sine'
    osc.frequency.setValueAtTime(freq, startTime)

    const g = ctx.createGain()
    g.gain.setValueAtTime(0.28, startTime)
    g.gain.exponentialRampToValueAtTime(0.001, startTime + 0.5)

    osc.connect(g)
    g.connect(ctx.destination)
    osc.start(startTime)
    osc.stop(startTime + 0.52)
  })
}
