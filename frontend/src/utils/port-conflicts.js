// Конфликт локального порта: адрес:порт может слушать только один туннель.
// Общая проверка для диалога смены порта (App.vue) и простого режима (SimpleMode.vue).

const ACTIVE_STATUSES = new Set(['running', 'busy', 'reconnecting'])

function normalizeBindHost(host) {
  const key = String(host || '').trim().toLowerCase() || '127.0.0.1'
  if (key === 'localhost' || key === '::1' || key === '[::1]') return '127.0.0.1'
  return key
}

// Другие туннели, которые сейчас работают или подключаются на том же локальном адресе и порту.
export function findPortConflicts(tunnel, tunnels) {
  const host = normalizeBindHost(tunnel?.localHost)
  const port = Number(tunnel?.localPort) || 0
  if (!port || !Array.isArray(tunnels)) return []
  return tunnels.filter((other) => {
    if (!other || other.id === tunnel.id) return false
    if (!ACTIVE_STATUSES.has(other.status)) return false
    return normalizeBindHost(other.localHost) === host && Number(other.localPort) === port
  })
}
