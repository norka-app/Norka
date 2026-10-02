// Сводный статус SSH-туннелей для иконки Norka в боковой панели.
//
// Статусы туннеля из Go-бэкенда (internal/biz/tunnel.go → GetState()):
//   'running'      — туннель поднят;
//   'busy'         — запускается (автостарт или локальная отметка на время переключения);
//   'reconnecting' — соединение потеряно, бэкенд переподключается с backoff;
//   'error'        — не удалось запустить или переподключение сдалось;
//   'stopped'      — остановлен.
//
// Приоритет: connecting > partial > error > connected > stopped.

// Ошибка, которая висит дольше этого времени, для иконки считается остановкой
// (глаза закрыты, не влияет на красный/partial). Статус туннеля в списке не меняется.
export const NORKA_STALE_ERROR_MS = 5 * 60 * 1000

// Запоминает, когда каждый туннель перешёл в статус status (since: Map<id, ms>).
export function trackTunnelStatusSince(tunnels, status, since, now = Date.now()) {
  const matched = new Set()
  for (const tunnel of Array.isArray(tunnels) ? tunnels : []) {
    if (tunnel?.status !== status) continue
    matched.add(tunnel.id)
    if (!since.has(tunnel.id)) since.set(tunnel.id, now)
  }
  for (const id of [...since.keys()]) {
    if (!matched.has(id)) since.delete(id)
  }
}

// Запоминает, когда каждый туннель перешёл в 'error' (errorSince: Map<id, ms>).
export function trackTunnelErrorSince(tunnels, errorSince, now = Date.now()) {
  trackTunnelStatusSince(tunnels, 'error', errorSince, now)
}

export function aggregateNorkaStatus(tunnels, { errorSince = null, now = Date.now() } = {}) {
  let running = 0
  let busy = 0
  let failed = 0
  for (const tunnel of Array.isArray(tunnels) ? tunnels : []) {
    const status = tunnel?.status
    if (status === 'running') running += 1
    else if (status === 'busy' || status === 'reconnecting') busy += 1
    else if (status === 'error') {
      const since = errorSince?.get(tunnel.id)
      if (since === undefined || now - since <= NORKA_STALE_ERROR_MS) failed += 1
    }
  }
  if (busy > 0) return 'connecting'
  if (failed > 0) return running > 0 ? 'partial' : 'error'
  if (running > 0) return 'connected'
  return 'stopped'
}
