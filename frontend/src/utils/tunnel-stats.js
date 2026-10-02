// Compact durations and byte totals for the tunnel statistics UI.

export function formatDuration(totalSeconds, t) {
  const total = Math.max(0, Math.floor(Number(totalSeconds) || 0))
  const days = Math.floor(total / 86400)
  const hours = Math.floor((total % 86400) / 3600)
  const minutes = Math.floor((total % 3600) / 60)
  const seconds = total % 60
  if (days > 0) {
    return hours > 0
      ? t('app.tunnels.stats.dayHour', { d: days, h: hours })
      : t('app.tunnels.stats.day', { d: days })
  }
  if (hours > 0) {
    return minutes > 0
      ? t('app.tunnels.stats.hourMin', { h: hours, m: minutes })
      : t('app.tunnels.stats.hour', { h: hours })
  }
  if (minutes > 0) {
    return seconds > 0 && minutes < 10
      ? t('app.tunnels.stats.minSec', { m: minutes, s: seconds })
      : t('app.tunnels.stats.min', { m: minutes })
  }
  return t('app.tunnels.stats.sec', { n: seconds })
}

export function formatBytes(value) {
  const amount = Math.max(0, Number(value) || 0)
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let size = amount
  let unit = 0
  while (size >= 1024 && unit < units.length - 1) {
    size /= 1024
    unit += 1
  }
  if (unit === 0) return `${Math.round(size)} ${units[unit]}`
  const digits = size >= 100 ? 0 : size >= 10 ? 1 : 2
  return `${trimTrailingZero(size.toFixed(digits))} ${units[unit]}`
}

export function formatWhen(unixSeconds, locale) {
  const seconds = Number(unixSeconds) || 0
  if (seconds <= 0) return ''
  const date = new Date(seconds * 1000)
  if (Number.isNaN(date.getTime())) return ''
  return new Intl.DateTimeFormat(locale === 'ru' ? 'ru-RU' : 'en-GB', {
    day: 'numeric',
    month: 'short',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date)
}

export function sessionUptimeSeconds(stat, nowMs) {
  if (!stat?.connected || !stat.sessionStartedUnix) return 0
  return Math.max(0, Math.floor((nowMs - stat.sessionStartedUnix) / 1000))
}

export function liveConnectedSeconds(baseSeconds, stat, nowMs) {
  const base = Math.max(0, Math.floor(Number(baseSeconds) || 0))
  if (!stat?.connected || !stat.asOfUnix) return base
  const extra = Math.max(0, Math.floor((nowMs - stat.asOfUnix) / 1000))
  return base + extra
}

export function reconnectCountLabel(count, t, locale) {
  const n = Math.max(0, Math.floor(Number(count) || 0))
  if (locale === 'ru') {
    const mod10 = n % 10
    const mod100 = n % 100
    if (mod10 === 1 && mod100 !== 11) return t('app.overview.reconnectsOne', { count: n })
    if (mod10 >= 2 && mod10 <= 4 && (mod100 < 12 || mod100 > 14)) {
      return t('app.overview.reconnectsFew', { count: n })
    }
    return t('app.overview.reconnectsMany', { count: n })
  }
  return n === 1
    ? t('app.overview.reconnectsOne', { count: n })
    : t('app.overview.reconnectsMany', { count: n })
}

function trimTrailingZero(text) {
  return text.replace(/\.0+$/, '').replace(/(\.\d*?)0+$/, '$1')
}
