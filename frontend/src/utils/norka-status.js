// Сводный статус SSH-туннелей для иконки Norka в боковой панели.
//
// Статусы туннеля из Go-бэкенда (internal/biz/tunnel.go → GetState()):
//   'running'      — туннель поднят;
//   'busy'         — запускается (автостарт или локальная отметка на время переключения);
//   'reconnecting' — соединение потеряно, бэкенд переподключается с backoff;
//   'error'        — не удалось запустить или переподключение сдалось;
//   'stopped'      — остановлен.
//
// Глаза всегда одного цвета и показывают ПОСЛЕДНЕЕ событие:
//   • идёт подключение (busy/reconnecting) — жёлтые, моргают;
//   • подключение закончилось ошибкой — красные, даже если другие туннели работают;
//   • успешное подключение — зелёные, даже если у других туннелей ошибка;
//   • остановка туннеля снимает его ошибку/подключение — зелёные, если что-то ещё работает,
//     красные, если работающих нет, но остались ошибки, иначе закрыты.
// Когда свежего события нет (запуск приложения, несколько переходов за один опрос) —
// приоритет: connecting > error > connected > stopped.

// Ошибка, которая висит дольше этого времени, для иконки считается остановкой
// (глаза закрыты, не красит иконку). Статус туннеля в списке не меняется.
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

// Сводный статус: connected | connecting | error | stopped.
// errorSince / runningSince / stoppedSince — Map<id, ms> из trackTunnelStatusSince: когда туннель
// перешёл в 'error' / 'running' / 'stopped'. По ним выбирается последнее событие, если одновременно
// есть работающие туннели и туннели с ошибкой.
export function aggregateNorkaStatus(tunnels, { errorSince = null, runningSince = null, stoppedSince = null, now = Date.now() } = {}) {
  let running = 0
  let busy = 0
  let failed = 0
  // последнее событие: время и виды событий в этот момент (одновременные — по приоритету)
  let lastAt = -Infinity
  let lastKinds = new Set()
  const note = (at, kind) => {
    if (at === undefined) return
    if (at > lastAt) {
      lastAt = at
      lastKinds = new Set([kind])
    } else if (at === lastAt) {
      lastKinds.add(kind)
    }
  }
  for (const tunnel of Array.isArray(tunnels) ? tunnels : []) {
    const status = tunnel?.status
    if (status === 'running') {
      running += 1
      note(runningSince?.get(tunnel.id), 'connected')
    } else if (status === 'busy' || status === 'reconnecting') {
      busy += 1
    } else if (status === 'error') {
      const since = errorSince?.get(tunnel.id)
      if (since === undefined || now - since <= NORKA_STALE_ERROR_MS) {
        failed += 1
        note(since, 'error')
      }
    } else if (status === 'stopped') {
      note(stoppedSince?.get(tunnel.id), 'stopped')
    }
  }
  if (busy > 0) return 'connecting'
  if (failed > 0 && running > 0) {
    // ошибка — последнее событие (или событий нет вовсе) → красные; успешное подключение или
    // остановка туннеля после ошибки → зелёные
    return lastAt === -Infinity || lastKinds.has('error') ? 'error' : 'connected'
  }
  if (failed > 0) return 'error'
  if (running > 0) return 'connected'
  return 'stopped'
}
