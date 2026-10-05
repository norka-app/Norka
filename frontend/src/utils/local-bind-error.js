// Локальный bind, когда порт занят не туннелем Norka.
// «Подключить вместо» остаётся на findPortConflicts: если порт держит другой
// туннель Norka, подсказку про Docker не показываем.
// suggested — пример (порт+10000 или число из движка), не обещание, что порт свободен.

import { findPortConflicts } from './port-conflicts.js'

const MARKER = /^norka:local-port-external port=(\d+) suggested=(\d+) remote=(\d+)\r?\n([\s\S]*)$/

export function suggestAlternateLocalPort(port) {
  const value = Number(port) || 0
  if (value >= 1 && value <= 55535) return value + 10000
  if (value !== 13000) return 13000
  return 13001
}

export function parseExternalBindError(message) {
  const match = String(message || '').trim().match(MARKER)
  if (!match) return null
  const port = Number(match[1])
  const suggested = Number(match[2])
  const remote = Number(match[3])
  if (port < 1 || port > 65535 || suggested < 1 || suggested > 65535 || remote < 0 || remote > 65535) {
    return null
  }
  return {
    port,
    suggested,
    remote,
    raw: match[4].trim(),
  }
}

export function isForeignLocalBindError(message) {
  const text = String(message || '').toLowerCase()
  if (!text || text.includes('remote listen')) return false
  const inUse = text.includes('address already in use')
    || text.includes('only one usage of each socket address')
    || text.includes('только одно использование адреса сокета')
    || text.includes('eaddrinuse')
    || text.includes('wsaeaddrinuse')
  if (!inUse) return false
  return text.includes('listen') || text.includes('bind')
}

// Текст для журнала: исходная ошибка ОС, без служебной строки.
export function rawTunnelError(message) {
  const parsed = parseExternalBindError(message)
  if (parsed) return parsed.raw
  return String(message || '')
}

export function foreignLocalBindHint(tunnel, tunnels) {
  if (!tunnel || tunnel.status !== 'error') return null
  const mode = String(tunnel.mode || 'local').trim() || 'local'
  if (mode !== 'local') return null
  if (findPortConflicts(tunnel, tunnels).length > 0) return null

  const parsed = parseExternalBindError(tunnel.lastError)
  if (parsed) return parsed
  if (!isForeignLocalBindError(tunnel.lastError)) return null

  const port = Number(tunnel.localPort) || 0
  if (port < 1 || port > 65535) return null
  const remotePort = Number(tunnel.remotePort)
  const remote = Number.isFinite(remotePort) && remotePort >= 0 && remotePort <= 65535 ? remotePort : 0
  return {
    port,
    suggested: suggestAlternateLocalPort(port),
    remote,
    raw: String(tunnel.lastError || ''),
  }
}
