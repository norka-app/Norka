// Сводка капсулы: один взгляд, без создания и правки туннелей.
// Глаза: зелёные когда всё хорошо, жёлтые на подключении, красные на последней
// ошибке (даже если другие туннели живы), закрытые когда ничего не работает.

const CONNECTING = new Set(['busy', 'reconnecting'])

export function islandAddress(tunnel) {
  const host = String(tunnel?.localHost || '').trim()
  const local = !host || host === '127.0.0.1' || host === '0.0.0.0' || host === '::1' || host === 'localhost'
  return `${local ? 'localhost' : host}:${Number(tunnel?.localPort) || 0}`
}

export function tunnelCountForm(count, locale) {
  const n = Math.abs(Number(count) || 0)
  if (locale !== 'ru') return n === 1 ? 'one' : 'many'
  const mod10 = n % 10
  const mod100 = n % 100
  if (mod10 === 1 && mod100 !== 11) return 'one'
  if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) return 'few'
  return 'many'
}

function byRecent(list, since) {
  return [...list].sort((a, b) => (since?.get?.(b.id) || 0) - (since?.get?.(a.id) || 0))
}

export function islandSnapshot(tunnels, since = {}) {
  const list = Array.isArray(tunnels) ? tunnels.filter(Boolean) : []
  const running = list.filter((tunnel) => tunnel.status === 'running')
  const connecting = byRecent(list.filter((tunnel) => CONNECTING.has(tunnel.status)), since.connecting)
  const errored = byRecent(list.filter((tunnel) => tunnel.status === 'error'), since.error)
  return {
    list,
    running,
    connecting,
    errored,
    total: list.length,
    active: running.length,
  }
}

// conflict — ожидающее «Подключить вместо». expanded — список открыт пользователем.
// Иначе взгляд: подключение, обрыв, один туннель, несколько, сон.
export function islandPhase({ tunnels, expanded = false, conflict = null, since = null } = {}) {
  if (conflict) return 'conflict'
  const snap = islandSnapshot(tunnels, since || {})
  if (expanded) return 'expanded'
  if (snap.connecting.length) return 'connecting'
  if (snap.errored.length) return 'error'
  if (snap.running.length === 1) return 'one'
  if (snap.running.length > 1) return 'multi'
  return 'idle'
}

export function islandEyes(phase, aggregate) {
  if (phase === 'conflict' || phase === 'connecting') return 'connecting'
  if (phase === 'error') return 'error'
  if (phase === 'idle') return 'stopped'
  if (phase === 'one' || phase === 'multi') return 'connected'
  if (aggregate === 'connecting' || aggregate === 'error' || aggregate === 'connected' || aggregate === 'stopped') {
    return aggregate
  }
  return 'stopped'
}

export function islandFocusTunnel(phase, snap) {
  if (phase === 'connecting') return snap.connecting[0] || null
  if (phase === 'error') return snap.errored[0] || null
  if (phase === 'one') return snap.running[0] || null
  return null
}

export function islandDotCount(runningCount, limit = 8) {
  const count = Math.max(0, Number(runningCount) || 0)
  return Math.min(count, limit)
}

export function islandSwitchOn(status) {
  return status === 'running' || status === 'busy' || status === 'reconnecting'
}
