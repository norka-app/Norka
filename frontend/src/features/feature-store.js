import { computed, reactive } from 'vue'

// Каталог совпадает с internal/features. Бэкенд присылает то же самое;
// эти значения держат первый кадр, пока GetFeatures ещё не ответил.
const DEFAULTS = {
  profiles: false,
  quick_search: true,
  notifications: false,
  auto_update: true,
  traffic_monitor: true,
  mascot: true,
  ssh_command: true,
  wake_reconnect: true,
  automation: false,
  tunnel_stats: true,
  onboarding: true,
  diagnostics: true,
  tunnel_diagnostics: true,
  autostart_hidden: true,
  background_mode: false,
}

const COPY = {
  profiles: ['features.profiles', 'features.profilesDesc'],
  quick_search: ['features.quickSearch', 'features.quickSearchDesc'],
  notifications: ['features.notifications', 'features.notificationsDesc'],
  auto_update: ['features.autoUpdate', 'features.autoUpdateDesc'],
  traffic_monitor: ['features.trafficMonitor', 'features.trafficMonitorDesc'],
  mascot: ['features.mascot', 'features.mascotDesc'],
  ssh_command: ['features.sshCommand', 'features.sshCommandDesc'],
  wake_reconnect: ['features.wakeReconnect', 'features.wakeReconnectDesc'],
  automation: ['features.automation', 'features.automationDesc'],
  tunnel_stats: ['features.tunnelStats', 'features.tunnelStatsDesc'],
  onboarding: ['features.onboarding', 'features.onboardingDesc'],
  diagnostics: ['features.diagnostics', 'features.diagnosticsDesc'],
  tunnel_diagnostics: ['features.tunnelDiagnostics', 'features.tunnelDiagnosticsDesc'],
  autostart_hidden: ['features.autostartHidden', 'features.autostartHiddenDesc'],
  background_mode: ['features.backgroundMode', 'features.backgroundModeDesc'],
}

function fallbackItems() {
  return Object.keys(DEFAULTS).map((id) => ({
    id,
    enabled: DEFAULTS[id],
    default: DEFAULTS[id],
    titleKey: COPY[id][0],
    descriptionKey: COPY[id][1],
  }))
}

export const featureState = reactive({
  items: fallbackItems(),
})

export function featureEnabled(id) {
  const item = featureState.items.find((entry) => entry.id === id)
  if (item) return !!item.enabled
  return DEFAULTS[id] ?? false
}

export function useFeature(id) {
  return computed(() => featureEnabled(id))
}

export function applyFeatureViews(views) {
  if (!Array.isArray(views) || views.length === 0) return
  const byId = new Map(featureState.items.map((item) => [item.id, { ...item }]))
  for (const view of views) {
    const id = view?.id
    if (!id) continue
    const copy = COPY[id]
    byId.set(id, {
      id,
      enabled: !!view.enabled,
      default: typeof view.default === 'boolean' ? view.default : (DEFAULTS[id] ?? false),
      titleKey: view.titleKey || copy?.[0] || id,
      descriptionKey: view.descriptionKey || copy?.[1] || '',
    })
  }
  const order = views.map((view) => view.id).filter(Boolean)
  const rest = [...byId.keys()].filter((id) => !order.includes(id))
  featureState.items = [...order, ...rest].map((id) => byId.get(id)).filter(Boolean)
}

export function setFeatureEnabled(id, enabled) {
  const item = featureState.items.find((entry) => entry.id === id)
  if (item) item.enabled = !!enabled
}
