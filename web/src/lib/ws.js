// openWS 建立到服务端的 WebSocket，登录用户携带 token
export function openWS(token) {
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  const qs = token ? `?token=${encodeURIComponent(token)}` : ''
  const url = `${proto}://${location.host}/ws${qs}`
  return new WebSocket(url)
}

export function send(ws, msg) {
  if (ws && ws.readyState === WebSocket.OPEN) {
    ws.send(JSON.stringify(msg))
  }
}
