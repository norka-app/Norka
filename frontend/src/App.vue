<script setup>
import { computed, nextTick, onBeforeUnmount, onMounted, reactive, ref, watch, watchEffect } from 'vue'
import { useI18n } from 'vue-i18n'
import { darkTheme, dateRuRU, ruRU } from 'naive-ui'
import { naiveThemeOverrides } from './theme/naive-theme'
import {
  CreateGroup,
  CreateJumper,
  CreateTunnel,
  DeleteGroup,
  DeleteJumper,
  DeleteTunnel,
  DebugJumperFailure as DebugJumperFailureAPI,
  DebugSavedTunnelFailure as DebugSavedTunnelFailureAPI,
  DebugTunnelFailure as DebugTunnelFailureAPI,
  GetSSHConfigImportSources,
  GetState,
  GetTrafficMonitorEnabled,
  GetTrafficStats,
  LoadSSHConfigJumpersByPath,
  MoveTunnelToGroup,
  OpenReportEmail,
  ReorderGroups,
  SaveUILocale,
  TestJumperConnection as TestJumperConnectionAPI,
  TestTunnelConnection as TestTunnelConnectionAPI,
  ToggleTunnel,
  UpdateGroup,
  UpdateJumper,
  UpdateTunnel,
  CloseMainWindow,
  HasCustomTitleBar,
  ShiftDown
} from '../wailsjs/go/main/App'
import { EventsOn, WindowMinimise } from '../wailsjs/runtime/runtime'
import AppSidebar from './components/layout/AppSidebar.vue'
import AppTitleBar from './components/layout/AppTitleBar.vue'
import AppTopHeader from './components/layout/AppTopHeader.vue'
import OverviewPage from './components/pages/OverviewPage.vue'
import JumpersPage from './components/pages/JumpersPage.vue'
import TunnelsPage from './components/pages/TunnelsPage.vue'
import LogsPage from './components/pages/LogsPage.vue'
import ConfigPage from './components/pages/ConfigPage.vue'
import SimpleMode from './components/simple/SimpleMode.vue'
import AIDebugModal from './components/common/AIDebugModal.vue'
import JumperModal from './components/modals/JumperModal.vue'
import ImportJumperModal from './components/modals/ImportJumperModal.vue'
import TunnelModal from './components/modals/TunnelModal.vue'
import TunnelGroupModal from './components/modals/TunnelGroupModal.vue'
import ImportTunnelModal from './components/modals/ImportTunnelModal.vue'
import './styles/app-shell.css'
import { AI_DEBUG_ENABLED } from './config/features'
import { aggregateNorkaStatus, trackTunnelErrorSince, trackTunnelStatusSince } from './utils/norka-status'
import {
  SIMPLE_ON_TOP_STORAGE_KEY,
  SIMPLE_TUNNEL_STORAGE_KEY,
  WINDOW_MODE_STORAGE_KEY,
  applySimpleWindow,
  captureAdvancedBounds,
  ensureWindowOnScreen,
  restoreAdvancedWindow,
  setSimpleAlwaysOnTop
} from './utils/window-mode'
import { setWindowShown } from './utils/window-visibility'
import { findPortConflicts } from './utils/port-conflicts'

const { t, locale } = useI18n()

const pages = computed(() => [
  { key: 'overview', title: t('app.sidebar.overview'), subtitle: t('app.sidebar.overviewSubtitle'), icon: 'bi-speedometer2' },
  { key: 'jumpers', title: t('app.sidebar.jumpers'), subtitle: t('app.sidebar.jumpersSubtitle'), icon: 'bi-hdd-network' },
  { key: 'tunnels', title: t('app.sidebar.tunnels'), subtitle: t('app.sidebar.tunnelsSubtitle'), icon: 'bi-diagram-3' },
  { key: 'logs', title: t('app.sidebar.logs'), subtitle: t('app.sidebar.logsSubtitle'), icon: 'bi-journal-text' },
  { key: 'config', title: t('app.sidebar.config'), subtitle: t('app.sidebar.configSubtitle'), icon: 'bi-sliders2' }
])

const modeOptions = computed(() => [
  { value: 'local', label: t('app.options.mode.local') },
  { value: 'remote', label: t('app.options.mode.remote') },
  { value: 'dynamic', label: t('app.options.mode.dynamic') }
])

const authOptions = computed(() => [
  { value: 'password', label: t('app.options.auth.password') },
  { value: 'ssh_key', label: t('app.options.auth.sshKey') },
  { value: 'ssh_agent', label: t('app.options.auth.sshAgent') }
])

const savedTheme = typeof window !== 'undefined' ? window.localStorage.getItem('lt.theme') : null
const savedSidebarCollapsed = typeof window !== 'undefined' ? window.localStorage.getItem('lt.sidebar.collapsed') : null
const HIDE_EMPTY_UNGROUPED_STORAGE_KEY = 'lt.tunnel-groups.hide-empty-ungrouped'
const savedHideEmptyUngrouped =
  typeof window !== 'undefined' ? window.localStorage.getItem(HIDE_EMPTY_UNGROUPED_STORAGE_KEY) : null
function themeFromOs() {
  if (typeof window === 'undefined') return 'light'
  return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

const theme = ref(savedTheme === 'dark' || savedTheme === 'light' ? savedTheme : themeFromOs())
const naiveTheme = computed(() => (theme.value === 'dark' ? darkTheme : null))
const themeOverrides = computed(() => naiveThemeOverrides(theme.value))
const sidebarCollapsed = ref(savedSidebarCollapsed === '1')
const windowMode = ref(
  typeof window !== 'undefined' && window.localStorage.getItem(WINDOW_MODE_STORAGE_KEY) === 'simple' ? 'simple' : 'advanced'
)
const simpleOnTop = ref(typeof window !== 'undefined' && window.localStorage.getItem(SIMPLE_ON_TOP_STORAGE_KEY) === '1')
const simpleTunnelId = ref(
  typeof window !== 'undefined' ? Number(window.localStorage.getItem(SIMPLE_TUNNEL_STORAGE_KEY)) || null : null
)
// Ожидающее подтверждение смены порта в простом режиме: id туннеля, который просили запустить.
// Сбрасывается отменой, выбором другого туннеля, сменой режима или если конфликт исчез сам.
const simplePortSwitchId = ref(null)
const hideEmptyUngrouped = ref(savedHideEmptyUngrouped !== '0' && savedHideEmptyUngrouped !== 'false')
const activePage = ref('overview')
const selectedLogLevel = ref('all')
const configMessage = ref('')
const showOverviewActive = ref(true)
const showOverviewActivity = ref(true)
const showConfigToast = ref(false)
const CONFIG_TOAST_DURATION_MS = 3800
let configToastTimer = null

const appMeta = reactive({
  version: '0.0.1'
})
const AI_REPORT_SUPPORT_EMAIL = ''
const AI_REPORT_SUBJECT = '[Norka] Report'
const AI_REPORT_MAX_FIELD_LEN = 800

watchEffect(() => {
  if (typeof document !== 'undefined') {
    document.documentElement.setAttribute('data-theme', theme.value)
    document.documentElement.setAttribute('data-bs-theme', theme.value)
  }
})

let themePinned = savedTheme === 'dark' || savedTheme === 'light'

watch(theme, (newTheme) => {
  if (themePinned && typeof window !== 'undefined') {
    window.localStorage.setItem('lt.theme', newTheme)
  }
  if (typeof document === 'undefined') return
  const root = document.documentElement
  const style = document.createElement('style')
  style.appendChild(document.createTextNode('*,*::before,*::after{transition:none !important}'))
  root.appendChild(style)
  void root.offsetHeight
  requestAnimationFrame(() => {
    style.remove()
  })
})

watch(sidebarCollapsed, (collapsed) => {
  if (typeof window !== 'undefined') {
    window.localStorage.setItem('lt.sidebar.collapsed', collapsed ? '1' : '0')
  }
})

watch(windowMode, (mode) => {
  if (typeof window !== 'undefined') window.localStorage.setItem(WINDOW_MODE_STORAGE_KEY, mode)
})

watch(simpleOnTop, (onTop) => {
  if (typeof window !== 'undefined') window.localStorage.setItem(SIMPLE_ON_TOP_STORAGE_KEY, onTop ? '1' : '0')
  if (windowMode.value === 'simple') setSimpleAlwaysOnTop(onTop)
})

watch(hideEmptyUngrouped, (enabled) => {
  if (typeof window !== 'undefined') {
    window.localStorage.setItem(HIDE_EMPTY_UNGROUPED_STORAGE_KEY, enabled ? '1' : '0')
  }
})

function toggleSidebar() {
  sidebarCollapsed.value = !sidebarCollapsed.value
}

function selectSimpleTunnel(id) {
  if (simplePortSwitchId.value !== id) simplePortSwitchId.value = null
  simpleTunnelId.value = id
  if (typeof window !== 'undefined') window.localStorage.setItem(SIMPLE_TUNNEL_STORAGE_KEY, String(id))
}

let windowModeSwitching = false

async function enterSimpleWindow() {
  await captureAdvancedBounds()
  windowMode.value = 'simple'
  await nextTick()
  await applySimpleWindow({ onTop: simpleOnTop.value, titleBar: customTitleBar.value })
}

// Окно без системной рамки (Windows): заголовок рисует AppTitleBar. Первое значение — по
// User-Agent, чтобы не было скачка при старте, точное — от Go (HasCustomTitleBar, window_chrome.go).
const customTitleBar = ref(typeof navigator !== 'undefined' && /Windows/.test(navigator.userAgent))

async function detectCustomTitleBar() {
  if (typeof window === 'undefined' || !window.go?.main?.App) return
  try {
    customTitleBar.value = (await HasCustomTitleBar()) === true
  } catch (_) {
    /* оставляем значение по User-Agent */
  }
}

function minimiseWindow() {
  setWindowShown(false)
  WindowMinimise()
}

// «Закрыть» — как системный крестик: в трей (Windows) или выход (window_chrome.go)
function closeWindow() {
  void CloseMainWindow()
}

// Переключение «Расширенный / Простой»: размер и позиция расширенного окна запоминаются
// и восстанавливаются при возврате (см. utils/window-mode.js).
async function setWindowMode(mode) {
  const next = mode === 'simple' ? 'simple' : 'advanced'
  if (windowModeSwitching || windowMode.value === next) return
  windowModeSwitching = true
  try {
    if (next === 'simple') {
      await enterSimpleWindow()
    } else {
      await restoreAdvancedWindow()
      windowMode.value = 'advanced'
    }
  } finally {
    windowModeSwitching = false
  }
}

// события из меню трея (tray_menu.go): туннель переключён / открыть окно в нужном режиме
let offRuntimeEvents = []

function subscribeTrayEvents() {
  if (typeof window === 'undefined' || !window.runtime) return
  offRuntimeEvents = [
    EventsOn('tunnels:changed', () => syncStateSilently()),
    EventsOn('window:mode', (mode) => {
      if (mode === 'simple' && dialogOpen.value) return
      void setWindowMode(mode)
    })
  ]
}

function openTunnelsFromSimple() {
  activePage.value = 'tunnels'
  void setWindowMode('advanced')
}

function setHideEmptyUngrouped(enabled) {
  hideEmptyUngrouped.value = !!enabled
}

watch(configMessage, (message) => {
  if (!message) {
    hideConfigToast()
    return
  }
  showConfigToast.value = true
  if (configToastTimer !== null) {
    window.clearTimeout(configToastTimer)
  }
  configToastTimer = window.setTimeout(() => {
    showConfigToast.value = false
    configToastTimer = null
  }, CONFIG_TOAST_DURATION_MS)
})

const jumpers = ref([])
const tunnelGroups = ref([])
const tunnels = ref([])
const tunnelsLoaded = ref(false)
const jumperSearchQuery = ref('')
const tunnelSearchQuery = ref('')
const STATE_SYNC_INTERVAL_MS = 5000
const TRAFFIC_SYNC_INTERVAL_MS = 1000
const TRAFFIC_HISTORY_LEN = 40
const pendingToggleTunnelIds = new Set()
let stateSyncTimer = null
let trafficSyncTimer = null
let stateSyncInFlight = false
const trafficMonitorEnabled = ref(true)
const traffic = ref({ upBps: 0, downBps: 0 })
const trafficHistoryUp = ref([])
const trafficHistoryDown = ref([])

const logs = ref([
  { id: 1, level: 'info', time: nowLabel(), message: 'Config storage mode: TOML' }
])

const showJumperModal = ref(false)
const showTunnelModal = ref(false)
const showTunnelGroupModal = ref(false)
const showImportJumperModal = ref(false)
const showImportTunnelModal = ref(false)
const importJumperLoading = ref(false)
const importJumperError = ref('')
const importTunnelError = ref('')
const importJumperHasLoaded = ref(false)
const sshConfigSources = ref([])
const selectedImportJumperSourcePath = ref('')
const sshConfigCandidates = ref([])
const editingJumperId = ref(null)
const editingTunnelId = ref(null)
const showJumperBasic = ref(true)
const showJumperAdvanced = ref(false)

const jumperForm = reactive(defaultJumperForm())
const tunnelForm = reactive(defaultTunnelForm())
const inlineJumperForm = reactive(defaultInlineJumperForm())

const jumperValidationError = ref('')
const inlineJumperValidationError = ref('')
const tunnelValidationError = ref('')
const tunnelGroupModalError = ref('')
const pendingTunnelGroupEditId = ref(null)
const actionDialog = reactive({
  visible: false,
  mode: 'alert',
  message: '',
  confirmButtonClass: 'btn-primary',
  confirmLabel: '',
  onConfirm: null,
  secondaryLabel: '',
  secondaryButtonClass: 'btn-outline-primary',
  onSecondary: null,
  rememberLabel: '',
  remember: false
})
const PORT_SWITCH_SKIP_KEY = 'lt.tunnel.portSwitch.skipConfirm'
const jumperTest = reactive({
  status: 'idle',
  message: '',
  debuggable: false
})
const tunnelTest = reactive({
  status: 'idle',
  message: '',
  debuggable: false
})
const jumperAiDebug = reactive(defaultAIDebugState())
const tunnelAiDebug = reactive(defaultAIDebugState())
const tunnelErrorAiDebugStates = reactive({})
const selectedAIDebugTunnel = ref(null)

const JUMPER_LIMITS = {
  name: 20,
  user: 50,
  host: 255,
  keyPath: 260,
  agentSocketPath: 512,
  password: 128,
  notes: 300,
  keepAliveIntervalMin: 0,
  keepAliveIntervalMax: 120000,
  timeoutMin: 100,
  timeoutMax: 120000
}
const TUNNEL_LIMITS = {
  name: 20
}

const currentPage = computed(() => pages.value.find((page) => page.key === activePage.value))
const totalTunnels = computed(() => tunnels.value.length)
const runningTunnels = computed(() => tunnels.value.filter((tunnel) => tunnel.status === 'running' || tunnel.status === 'reconnecting'))
const stoppedTunnels = computed(() => tunnels.value.filter((tunnel) => tunnel.status === 'stopped' || tunnel.status === 'error'))
const autoStartTunnels = computed(() => tunnels.value.filter((tunnel) => tunnel.autoStart))
// когда туннель перешёл в 'error' — для «устаревших» ошибок в иконке Norka;
// когда в 'running' — для аптайма в простом режиме (считается с момента, как приложение увидело запуск);
// 'error' / 'running' / 'stopped' вместе — последнее событие для глаз Norka (utils/norka-status.js)
const tunnelErrorSince = new Map()
const tunnelRunningSince = new Map()
const tunnelStoppedSince = new Map()
watch(
  () => tunnels.value.map((tunnel) => `${tunnel.id}:${tunnel.status}`).join('|'),
  () => {
    // одно время на опрос: переходы, увиденные вместе, считаются одновременными
    const now = Date.now()
    trackTunnelErrorSince(tunnels.value, tunnelErrorSince, now)
    trackTunnelStatusSince(tunnels.value, 'running', tunnelRunningSince, now)
    trackTunnelStatusSince(tunnels.value, 'stopped', tunnelStoppedSince, now)
  },
  { flush: 'sync' }
)
const getTunnelRunningSince = (id) => tunnelRunningSince.get(id)
// туннель простого режима: сохранённый выбор, иначе первый активный, иначе первый в списке
const simpleTunnel = computed(() => (
  tunnels.value.find((tunnel) => tunnel.id === simpleTunnelId.value)
  || runningTunnels.value[0]
  || tunnels.value[0]
  || null
))
// null до первой загрузки состояния, чтобы иконка не подмигивала при старте приложения.
// Пересчитывается при каждом опросе бэкенда (tunnels.value заменяется каждые STATE_SYNC_INTERVAL_MS),
// поэтому устаревание ошибки подхватывается без отдельного таймера.
const norkaStatus = computed(() => (tunnelsLoaded.value
  ? aggregateNorkaStatus(tunnels.value, {
    errorSince: tunnelErrorSince,
    runningSince: tunnelRunningSince,
    stoppedSince: tunnelStoppedSince,
    now: Date.now()
  })
  : null))
const filteredLogs = computed(() => {
  if (selectedLogLevel.value === 'all') return logs.value
  return logs.value.filter((log) => log.level === selectedLogLevel.value)
})
const selectedAIDebugTunnelState = computed(() => {
  if (!selectedAIDebugTunnel.value?.id) return defaultAIDebugState()
  return ensureTunnelErrorAIDebugState(selectedAIDebugTunnel.value.id)
})
const selectedAIDebugTunnelTitle = computed(() => {
  const name = selectedAIDebugTunnel.value?.name || t('app.aiDebug.savedTunnelFallback')
  return t('app.aiDebug.modalTitle', { name })
})
const selectedAIDebugTunnelSubtitle = computed(() => {
  if (!selectedAIDebugTunnel.value) return ''
  return getTunnelJumperLabel(selectedAIDebugTunnel.value)
})

const filteredJumpers = computed(() => {
  const query = jumperSearchQuery.value.trim().toLowerCase()
  if (!query) return jumpers.value
  
  return jumpers.value.filter(jumper => {
    return (
      jumper.name.toLowerCase().includes(query) ||
      jumper.host.toLowerCase().includes(query) ||
      jumper.user.toLowerCase().includes(query) ||
      (jumper.notes && jumper.notes.toLowerCase().includes(query))
    )
  })
})

const filteredTunnels = computed(() => {
  const query = tunnelSearchQuery.value.trim().toLowerCase()
  if (!query) return tunnels.value
  
  return tunnels.value.filter(tunnel => {
    const jumperName = getTunnelJumperLabel(tunnel).toLowerCase()
    return (
      tunnel.name.toLowerCase().includes(query) ||
      tunnel.localHost.toLowerCase().includes(query) ||
      tunnel.remoteHost.toLowerCase().includes(query) ||
      jumperName.includes(query) ||
      (tunnel.description && tunnel.description.toLowerCase().includes(query))
    )
  })
})

const jumperNeedsPassword = computed(() => authNeedsPassword(jumperForm.authType))
const jumperShowsPassword = computed(() => authShowsPassword(jumperForm.authType))
const jumperNeedsKeyFile = computed(() => authNeedsKeyFile(jumperForm.authType))
const inlineJumperNeedsPassword = computed(() => authNeedsPassword(inlineJumperForm.authType))
const inlineJumperShowsPassword = computed(() => authShowsPassword(inlineJumperForm.authType))
const inlineJumperNeedsKeyFile = computed(() => authNeedsKeyFile(inlineJumperForm.authType))

function defaultJumperForm() {
  return {
    name: '',
    host: '',
    port: 22,
    user: '',
    authType: 'ssh_key',
    keyPath: '',
    agentSocketPath: '',
    password: '',
    bypassHostVerification: false,
    keepAliveIntervalMs: 5000,
    timeoutMs: 5000,
    notes: ''
  }
}

function defaultTunnelForm() {
  return {
    name: '',
    groupId: 0,
    mode: 'local',
    jumperIds: [],
    nextJumperId: '',
    appendNewJumper: false,
    localHost: '127.0.0.1',
    localPort: 10022,
    remoteHost: '',
    remotePort: 22,
    autoStart: false,
    description: ''
  }
}

function defaultInlineJumperForm() {
  return {
    name: '',
    host: '',
    port: 22,
    user: '',
    authType: 'ssh_key',
    keyPath: '',
    agentSocketPath: '',
    password: '',
    bypassHostVerification: false,
    keepAliveIntervalMs: 5000,
    timeoutMs: 5000,
    notes: ''
  }
}

function defaultAIDebugState() {
  return {
    status: 'idle',
    error: '',
    result: null
  }
}

function nowLabel() {
  return new Date().toLocaleString()
}

function toText(value) {
  return String(value ?? '').trim()
}

function clipText(value, limit = AI_REPORT_MAX_FIELD_LEN) {
  const text = toText(value)
  if (limit <= 0 || text.length <= limit) return text
  return `${text.slice(0, limit)}...`
}

async function writeTextToClipboard(text) {
  const value = String(text ?? '')
  if (!value) return false

  if (navigator?.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(value)
      return true
    } catch (_) {
      // fallback below
    }
  }

  if (typeof document === 'undefined') return false
  const textarea = document.createElement('textarea')
  textarea.value = value
  textarea.setAttribute('readonly', '')
  textarea.style.position = 'fixed'
  textarea.style.left = '-9999px'
  document.body.appendChild(textarea)
  textarea.select()
  textarea.setSelectionRange(0, textarea.value.length)
  let copied = false
  try {
    copied = document.execCommand('copy')
  } catch (_) {
    copied = false
  } finally {
    document.body.removeChild(textarea)
  }
  return copied
}

function nameUnits(text) {
  let units = 0
  for (const char of text || '') {
    units += /[\u3400-\u9fff\uf900-\ufaff]/.test(char) ? 2 : 1
  }
  return units
}

function isTunnelGroupNameTaken(name, excludeId = null) {
  const normalized = String(name || '').trim().toLowerCase()
  if (!normalized) return false
  return tunnelGroups.value.some((group) => {
    if (excludeId != null && Number(group.id) === Number(excludeId)) return false
    return String(group.name || '').trim().toLowerCase() === normalized
  })
}

function nextId(items) {
  return items.reduce((max, item) => Math.max(max, item.id), 0) + 1
}

function patchTunnelLocal(id, patch) {
  const index = tunnels.value.findIndex((item) => item.id === id)
  if (index === -1) return
  tunnels.value[index] = { ...tunnels.value[index], ...patch }
}

function formatLatencyLabel(latencyMs) {
  const ms = Number(latencyMs)
  if (!Number.isFinite(ms) || ms <= 0) return '--'
  if (ms < 1000) return `${Math.round(ms)}ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(ms < 10000 ? 2 : 1)}s`
  if (ms < 3600000) return `${(ms / 60000).toFixed(ms < 600000 ? 2 : 1)}m`
  return `${(ms / 3600000).toFixed(2)}h`
}

function authNeedsPassword(authType) {
  return authType === 'password'
}

function authNeedsKeyFile(authType) {
  return authType === 'ssh_key'
}

function authShowsPassword(authType) {
  return authType === 'password' || authType === 'ssh_key'
}

function getAuthLabel(authType) {
  const matched = authOptions.value.find((item) => item.value === authType)
  return matched ? matched.label : t('app.options.auth.unknown')
}

function getJumperName(jumperId) {
  const jumper = jumpers.value.find((item) => item.id === jumperId)
  return jumper ? jumper.name : t('app.options.jumper.unknown')
}

function normalizeJumperIdList(ids) {
  const seen = new Set()
  return (Array.isArray(ids) ? ids : [])
    .map((id) => Number(id))
    .filter((id) => Number.isInteger(id) && id > 0)
    .filter((id) => {
      if (seen.has(id)) return false
      seen.add(id)
      return true
    })
}

function toHostKey(host) {
  return String(host || '').trim().toLowerCase()
}

function getTunnelImportSignature(tunnelLike) {
  const mode = String(tunnelLike?.mode || 'local').trim().toLowerCase()
  const localHost = toHostKey(tunnelLike?.localHost || '127.0.0.1')
  const localPort = Number(tunnelLike?.localPort) || 0
  const remoteHost = mode === 'dynamic' ? '' : toHostKey(tunnelLike?.remoteHost)
  const remotePort = mode === 'dynamic' ? 0 : (Number(tunnelLike?.remotePort) || 0)
  const jumperKey = normalizeJumperIdList(tunnelLike?.jumperIds).join(',')
  return `${mode}|${localHost}|${localPort}|${remoteHost}|${remotePort}|${jumperKey}`
}

function getNextTunnelJumperCandidate(selectedIds = []) {
  if (!jumpers.value.length) return ''
  const selected = new Set(normalizeJumperIdList(selectedIds))
  const candidate = jumpers.value.find((item) => !selected.has(item.id))
  return candidate ? candidate.id : jumpers.value[0].id
}

function getTunnelJumperLabel(tunnel) {
  const ids = normalizeJumperIdList(tunnel?.jumperIds)
  if (!ids.length) return t('app.options.jumper.unknown')
  const names = ids.map((id) => getJumperName(id))
  return names.join(' -> ')
}

function normalizeTunnelFromBackend(tunnel) {
  const rawLatency = Number(tunnel?.latencyMs)
  const rawGroupId = Number(tunnel?.groupId)
  return {
    ...tunnel,
    groupId: Number.isInteger(rawGroupId) && rawGroupId > 0 ? rawGroupId : 0,
    jumperIds: normalizeJumperIdList(tunnel?.jumperIds),
    latencyMs: Number.isFinite(rawLatency) && rawLatency > 0 ? rawLatency : 0
  }
}

function logEvent(level, message) {
  logs.value.unshift({
    id: nextId(logs.value),
    level,
    time: nowLabel(),
    message
  })
}

function errorMessage(err, fallback = 'Operation failed.') {
  if (!err) return fallback
  if (typeof err === 'string') return err
  if (typeof err.message === 'string' && err.message) return err.message
  return fallback
}

function aiDebugErrorMessage(err, fallback = 'AI Debug failed.') {
  const message = errorMessage(err, fallback)
  if (/quota exceeded|daily quota/i.test(message)) {
    return t('app.aiDebug.quotaExceeded', { freeLimit: 10, proLimit: 200 })
  }
  if (/AI_PROVIDER_BUSY|rate limited|rate_limit_exceeded/i.test(message)) {
    return t('app.aiDebug.providerBusy')
  }
  if (/HTTP 5\d\d|cannot connect|connection refused|timeout|not configured|provider request failed|backend/i.test(message)) {
    return t('app.aiDebug.unavailable')
  }
  return message
}

function buildAIDebugReportBody({ targetType, result }) {
  const lines = [
    'Please describe why this AI output is inappropriate:',
    '',
    '[Your report]',
    '',
    '--- Context (auto-filled) ---',
    'Feature: AI Debug',
    `Target Type: ${clipText(targetType || 'unknown', 40)}`,
    `App Version: ${clipText(appMeta.version, 64)}`,
    `OS: ${clipText(typeof navigator !== 'undefined' ? navigator.platform || 'unknown' : 'unknown', 80)}`,
    `UI Locale: ${clipText(locale.value || 'ru', 24)}`,
    `Timestamp (UTC): ${new Date().toISOString()}`,
    '',
    '[AI Output]',
    `Reason: ${clipText(result?.reason)}`,
    `Summary: ${clipText(result?.summary)}`,
    `Steps: ${clipText(Array.isArray(result?.steps) ? result.steps.join(' | ') : '')}`
  ]
  return lines.join('\n').trim()
}

async function showAIDebugReportFallbackDialog(reportBody) {
  openActionDialog({
    mode: 'alert',
    message: t('app.aiDebug.reportOpenFailed'),
    confirmButtonClass: 'btn-primary',
    confirmLabel: t('app.aiDebug.copyReportBody'),
    onConfirm: async () => {
      const copied = await writeTextToClipboard(reportBody)
      setConfigMessage(copied ? t('app.aiDebug.copyBodyDone') : t('app.aiDebug.copyFailed'))
    },
    secondaryLabel: t('app.aiDebug.copySupportEmail'),
    secondaryButtonClass: 'btn-outline-secondary',
    onSecondary: async () => {
      const copied = await writeTextToClipboard(AI_REPORT_SUPPORT_EMAIL)
      setConfigMessage(copied ? t('app.aiDebug.copyEmailDone') : t('app.aiDebug.copyFailed'))
    }
  })
}

async function reportAIDebugContent(targetType, state) {
  const result = state?.result
  if (!result) return

  const body = buildAIDebugReportBody({ targetType, result })
  try {
    const openResult = await OpenReportEmail({
      subject: AI_REPORT_SUBJECT,
      body
    })
    if (!openResult?.success) {
      await showAIDebugReportFallbackDialog(body)
      return
    }
    setConfigMessage(t('app.aiDebug.reportDraftOpened'))
  } catch (_) {
    await showAIDebugReportFallbackDialog(body)
  }
}

async function loadStateFromBackend(options = {}) {
  const { silent = false } = options
  try {
    const state = await GetState()
    tunnelsLoaded.value = true
    jumpers.value = Array.isArray(state?.jumpers) ? state.jumpers : []
    tunnelGroups.value = Array.isArray(state?.groups) ? state.groups : []
    const backendTunnels = (Array.isArray(state?.tunnels) ? state.tunnels : []).map(normalizeTunnelFromBackend)
    const validTunnelIds = new Set(backendTunnels.map((item) => String(item.id)))
    Object.keys(tunnelErrorAiDebugStates).forEach((key) => {
      if (!validTunnelIds.has(key)) {
        delete tunnelErrorAiDebugStates[key]
      }
    })
    backendTunnels.forEach((item) => {
      if (item.status !== 'error' || !item.lastError) {
        delete tunnelErrorAiDebugStates[String(item.id)]
      }
    })

    if (pendingToggleTunnelIds.size === 0) {
      tunnels.value = backendTunnels
      return
    }

    const localBusyTunnelIds = new Set(
      tunnels.value
        .filter((item) => pendingToggleTunnelIds.has(item.id) && item.status === 'busy')
        .map((item) => item.id)
    )

    tunnels.value = backendTunnels.map((item) => {
      if (!localBusyTunnelIds.has(item.id)) return item
      return { ...item, status: 'busy', lastError: '', latencyMs: 0 }
    })
  } catch (err) {
    if (silent) return
    const message = errorMessage(err, 'Failed to load config from backend.')
    configMessage.value = message
    logEvent('error', message)
  }
}

function syncStateSilently() {
  if (stateSyncInFlight) return
  stateSyncInFlight = true
  loadStateFromBackend({ silent: true }).finally(() => {
    stateSyncInFlight = false
  })
}

function pushTrafficHistory(target, value) {
  const next = Math.max(0, Number(value) || 0)
  target.value = [...target.value, next].slice(-TRAFFIC_HISTORY_LEN)
}

async function syncTrafficSilently() {
  if (!trafficMonitorEnabled.value) return
  try {
    const stats = await GetTrafficStats()
    const upBps = Number(stats?.upBps) || 0
    const downBps = Number(stats?.downBps) || 0
    traffic.value = { upBps, downBps }
    pushTrafficHistory(trafficHistoryUp, upBps)
    pushTrafficHistory(trafficHistoryDown, downBps)
  } catch (_) {
    // best-effort background sync
  }
}

function stopTrafficSync() {
  if (trafficSyncTimer !== null) {
    window.clearInterval(trafficSyncTimer)
    trafficSyncTimer = null
  }
  traffic.value = { upBps: 0, downBps: 0 }
  trafficHistoryUp.value = []
  trafficHistoryDown.value = []
}

function startTrafficSync() {
  if (!trafficMonitorEnabled.value || trafficSyncTimer !== null) return
  void syncTrafficSilently()
  trafficSyncTimer = window.setInterval(syncTrafficSilently, TRAFFIC_SYNC_INTERVAL_MS)
}

function onTrafficMonitorChange(enabled) {
  trafficMonitorEnabled.value = !!enabled
  if (trafficMonitorEnabled.value) {
    startTrafficSync()
  } else {
    stopTrafficSync()
  }
}

function switchPage(pageKey) {
  activePage.value = pageKey
}

function setThemeBySwitch(enabled) {
  themePinned = true
  theme.value = enabled ? 'dark' : 'light'
  logEvent('info', `Theme switched to ${theme.value}`)
}

function setConfigMessage(msg) {
  configMessage.value = msg || ''
}

function hideConfigToast() {
  showConfigToast.value = false
  if (configToastTimer !== null) {
    window.clearTimeout(configToastTimer)
    configToastTimer = null
  }
}

function resetJumperValidation() {
  jumperValidationError.value = ''
}

function resetInlineJumperValidation() {
  inlineJumperValidationError.value = ''
}

function resetJumperTest() {
  jumperTest.status = 'idle'
  jumperTest.message = ''
  jumperTest.debuggable = false
}

function resetTunnelTest() {
  tunnelTest.status = 'idle'
  tunnelTest.message = ''
  tunnelTest.debuggable = false
}

function resetAIDebugState(state) {
  state.status = 'idle'
  state.error = ''
  state.result = null
}

function ensureTunnelErrorAIDebugState(tunnelId) {
  const key = String(tunnelId)
  if (!tunnelErrorAiDebugStates[key]) {
    tunnelErrorAiDebugStates[key] = defaultAIDebugState()
  }
  return tunnelErrorAiDebugStates[key]
}

function setAIDebugLoading(state) {
  state.status = 'analyzing'
  state.error = ''
  state.result = null
}

function setAIDebugResult(state, result) {
  state.status = 'success'
  state.error = ''
  state.result = result || null
}

function setAIDebugError(state, message) {
  state.status = 'error'
  state.error = message
  state.result = null
}

function buildTunnelPayloadForTest() {
  return {
    name: tunnelForm.name.trim(),
    groupId: Number(tunnelForm.groupId) || 0,
    mode: tunnelForm.mode,
    jumperIds: normalizeJumperIdList(tunnelForm.jumperIds),
    localHost: tunnelForm.localHost.trim(),
    localPort: Number(tunnelForm.localPort),
    remoteHost: tunnelForm.remoteHost.trim(),
    remotePort: Number(tunnelForm.remotePort),
    autoStart: !!tunnelForm.autoStart,
    status: 'stopped',
    description: tunnelForm.description.trim()
  }
}

function buildJumperPayload(form) {
  const payload = {
    name: form.name.trim(),
    host: form.host.trim(),
    port: Number(form.port),
    user: form.user.trim(),
    authType: form.authType,
    keyPath: form.keyPath.trim(),
    agentSocketPath: form.agentSocketPath.trim(),
    password: form.password,
    bypassHostVerification: !!form.bypassHostVerification,
    keepAliveIntervalMs: Number(form.keepAliveIntervalMs),
    timeoutMs: Number(form.timeoutMs),
    notes: form.notes.trim()
  }

  if (!authNeedsKeyFile(payload.authType)) payload.keyPath = ''
  if (!authShowsPassword(payload.authType)) payload.password = ''
  return payload
}

function validateJumperPayload(payload) {
  if (!payload.name) return 'Name is required.'
  if (nameUnits(payload.name) > JUMPER_LIMITS.name) return 'Name must be <= 20 chars or <= 10 Chinese chars.'
  if (!payload.host) return 'Host is required.'
  if (payload.host.length > JUMPER_LIMITS.host) return `Host length must be <= ${JUMPER_LIMITS.host}.`
  if (!payload.user) return 'User is required.'
  if (payload.user.length > JUMPER_LIMITS.user) return `User length must be <= ${JUMPER_LIMITS.user}.`
  if (!Number.isInteger(payload.port) || payload.port < 1 || payload.port > 65535) {
    return 'Port must be between 1 and 65535.'
  }
  if (payload.keyPath.length > JUMPER_LIMITS.keyPath) return `Key path length must be <= ${JUMPER_LIMITS.keyPath}.`
  if (payload.agentSocketPath.length > JUMPER_LIMITS.agentSocketPath) {
    return `Agent socket path length must be <= ${JUMPER_LIMITS.agentSocketPath}.`
  }
  if (payload.password.length > JUMPER_LIMITS.password) return `Password length must be <= ${JUMPER_LIMITS.password}.`
  if (payload.notes.length > JUMPER_LIMITS.notes) return `Notes length must be <= ${JUMPER_LIMITS.notes}.`
  if (authNeedsKeyFile(payload.authType) && !payload.keyPath) {
    return 'SSH Key mode requires selecting a key file.'
  }
  if (authNeedsPassword(payload.authType) && !payload.password) {
    return 'Current auth method requires a password.'
  }
  if (!Number.isInteger(payload.keepAliveIntervalMs) || payload.keepAliveIntervalMs > JUMPER_LIMITS.keepAliveIntervalMax) {
    return `KeepAlive interval(ms) must be 0 (disable) or between 1000 and ${JUMPER_LIMITS.keepAliveIntervalMax}.`
  }
  if (payload.keepAliveIntervalMs > 0 && payload.keepAliveIntervalMs < 1000) {
    return `KeepAlive interval(ms) must be 0 (disable) or between 1000 and ${JUMPER_LIMITS.keepAliveIntervalMax}.`
  }
  if (
    !Number.isInteger(payload.timeoutMs) ||
    payload.timeoutMs < JUMPER_LIMITS.timeoutMin ||
    payload.timeoutMs > JUMPER_LIMITS.timeoutMax
  ) {
    return `Timeout(ms) must be between ${JUMPER_LIMITS.timeoutMin} and ${JUMPER_LIMITS.timeoutMax}.`
  }
  return ''
}

function onJumperKeyFileChange(event) {
  const file = event.target.files && event.target.files[0]
  if (file) jumperForm.keyPath = file.name
}

function onInlineJumperKeyFileChange(event) {
  const file = event.target.files && event.target.files[0]
  if (file) inlineJumperForm.keyPath = file.name
}

function openNewJumper() {
  editingJumperId.value = null
  Object.assign(jumperForm, defaultJumperForm())
  showJumperBasic.value = true
  showJumperAdvanced.value = false
  resetJumperValidation()
  resetJumperTest()
  resetAIDebugState(jumperAiDebug)
  showJumperModal.value = true
}

async function loadImportJumperSources() {
  importJumperError.value = ''
  try {
    const sources = await GetSSHConfigImportSources()
    sshConfigSources.value = Array.isArray(sources) ? sources : []
    selectedImportJumperSourcePath.value = sshConfigSources.value[0]?.path || ''
  } catch (err) {
    sshConfigSources.value = []
    selectedImportJumperSourcePath.value = ''
    importJumperError.value = errorMessage(err, 'Failed to load SSH config sources')
  }
}

async function loadImportJumpers() {
  const targetPath = String(selectedImportJumperSourcePath.value || '').trim()
  if (!targetPath) return

  importJumperLoading.value = true
  importJumperError.value = ''
  importJumperHasLoaded.value = false
  try {
    const result = await LoadSSHConfigJumpersByPath(targetPath)
    sshConfigCandidates.value = Array.isArray(result?.candidates) ? result.candidates : []
    importJumperHasLoaded.value = true
  } catch (err) {
    sshConfigCandidates.value = []
    importJumperError.value = errorMessage(err, 'Failed to load SSH config jumpers')
  } finally {
    importJumperLoading.value = false
  }
}

function openImportJumper() {
  showImportJumperModal.value = true
  importJumperLoading.value = false
  importJumperError.value = ''
  importJumperHasLoaded.value = false
  sshConfigCandidates.value = []
  void loadImportJumperSources()
}

function closeImportJumper() {
  showImportJumperModal.value = false
  importJumperLoading.value = false
  importJumperError.value = ''
  importJumperHasLoaded.value = false
  sshConfigSources.value = []
  selectedImportJumperSourcePath.value = ''
  sshConfigCandidates.value = []
}

function editJumper(jumper) {
  editingJumperId.value = jumper.id
  Object.assign(jumperForm, defaultJumperForm(), jumper)
  showJumperBasic.value = true
  showJumperAdvanced.value = false
  resetJumperValidation()
  resetJumperTest()
  resetAIDebugState(jumperAiDebug)
  showJumperModal.value = true
}

function fillJumperFormFromJumper(jumper, nameOverride = null) {
  Object.assign(jumperForm, {
    name: nameOverride ?? jumper.name,
    host: jumper.host,
    port: jumper.port,
    user: jumper.user,
    authType: jumper.authType,
    keyPath: jumper.keyPath || '',
    agentSocketPath: jumper.agentSocketPath || '',
    password: jumper.password || '',
    bypassHostVerification: !!jumper.bypassHostVerification,
    keepAliveIntervalMs: jumper.keepAliveIntervalMs,
    timeoutMs: jumper.timeoutMs,
    notes: jumper.notes || ''
  })
}

function copyJumper(jumper) {
  editingJumperId.value = null
  fillJumperFormFromJumper(jumper, `copy-${jumper.name}`)
  showJumperBasic.value = true
  showJumperAdvanced.value = false
  resetJumperValidation()
  resetJumperTest()
  resetAIDebugState(jumperAiDebug)
  showJumperModal.value = true
}

async function saveJumper() {
  resetJumperValidation()
  const payload = buildJumperPayload(jumperForm)
  const error = validateJumperPayload(payload)
  if (error) {
    jumperValidationError.value = error
    return
  }

  try {
    if (editingJumperId.value) {
      await UpdateJumper(editingJumperId.value, payload)
      logEvent('info', `Jumper ${payload.name} updated`)
    } else {
      const created = await CreateJumper(payload)
      logEvent('info', `Jumper ${created.name} created`)
    }

    await loadStateFromBackend()
    showJumperModal.value = false
  } catch (err) {
    jumperValidationError.value = errorMessage(err)
  }
}

async function importJumpers(jumpersToImport) {
  try {
    importJumperError.value = ''
    let importedCount = 0
    let skippedCount = 0

    const existingSignatures = new Set(
      jumpers.value.map((item) => `${String(item.host || '').trim().toLowerCase()}|${String(item.user || '').trim()}|${Number(item.port) || 22}`)
    )

    for (const item of jumpersToImport) {
      const payload = {
        name: String(item.name || '').trim(),
        host: String(item.host || '').trim(),
        port: Number(item.port) || 22,
        user: String(item.user || '').trim(),
        authType: item.authType || 'ssh_agent',
        keyPath: String(item.keyPath || '').trim(),
        agentSocketPath: String(item.agentSocketPath || '').trim(),
        password: '',
        bypassHostVerification: !!item.bypassHostVerification,
        keepAliveIntervalMs: Number(item.keepAliveIntervalMs) || 5000,
        timeoutMs: Number(item.timeoutMs) || 5000,
        hostKeyAlgorithms: String(item.hostKeyAlgorithms || '').trim(),
        notes: `Imported from SSH config alias "${item.alias}" on ${new Date().toLocaleDateString()}`
      }

      const signature = `${payload.host.toLowerCase()}|${payload.user}|${payload.port}`
      if (existingSignatures.has(signature)) {
        skippedCount++
        continue
      }

      await CreateJumper(payload)
      existingSignatures.add(signature)
      importedCount++
      logEvent('info', `Jumper ${payload.name} imported from SSH config`)
    }

    await loadStateFromBackend()
    closeImportJumper()

    let message = `Successfully imported ${importedCount} jumper(s)`
    if (skippedCount > 0) {
      message = `${message}; skipped ${skippedCount} duplicate jumper(s)`
    }
    logEvent('info', message)
  } catch (err) {
    const message = errorMessage(err, 'Failed to import jumpers')
    importJumperError.value = message
    logEvent('error', message)
  }
}

async function testJumperConnection() {
  resetJumperValidation()
  resetAIDebugState(jumperAiDebug)
  const payload = buildJumperPayload(jumperForm)
  const error = validateJumperPayload(payload)
  if (error) {
    jumperTest.status = 'error'
    jumperTest.message = error
    jumperTest.debuggable = false
    return
  }

  resetJumperTest()
  jumperTest.status = 'testing'
  jumperTest.message = t('app.modals.jumper.testing')
  try {
    await TestJumperConnectionAPI(payload)
    jumperTest.status = 'success'
    jumperTest.message = 'Connection test passed.'
    jumperTest.debuggable = false
    logEvent('info', `Connection test passed for jumper ${payload.name}`)
  } catch (err) {
    jumperTest.status = 'error'
    jumperTest.message = errorMessage(err)
    jumperTest.debuggable = AI_DEBUG_ENABLED
    logEvent('error', `Connection test failed for jumper ${payload.name}: ${jumperTest.message}`)
  }
}

async function testTunnelConnection() {
  resetInlineJumperValidation()
  tunnelValidationError.value = ''
  resetAIDebugState(tunnelAiDebug)

  let inlinePayload = null
  let selectedJumperIds = normalizeJumperIdList(tunnelForm.jumperIds)

  if (tunnelForm.appendNewJumper) {
    inlinePayload = buildJumperPayload(inlineJumperForm)
    const inlineError = validateJumperPayload(inlinePayload)
    if (inlineError) {
      tunnelTest.status = 'error'
      tunnelTest.message = `[Jumper] ${inlineError}`
      tunnelTest.debuggable = false
      return
    }
  }

  if (!selectedJumperIds.length && !inlinePayload) {
    tunnelTest.status = 'error'
    tunnelTest.message = t('app.modals.tunnel.testSelectJumper')
    tunnelTest.debuggable = false
    return
  }

  const payload = buildTunnelPayloadForTest()
  payload.jumperIds = selectedJumperIds

  if (!payload.name || !payload.localHost || !payload.localPort) {
    tunnelTest.status = 'error'
    tunnelTest.message = t('app.modals.tunnel.testRequiredFields')
    tunnelTest.debuggable = false
    return
  }
  if (payload.mode !== 'dynamic' && (!payload.remoteHost || !payload.remotePort)) {
    tunnelTest.status = 'error'
    tunnelTest.message = t('app.modals.tunnel.testRequiredRemote')
    tunnelTest.debuggable = false
    return
  }

  resetTunnelTest()
  tunnelTest.status = 'testing'
  tunnelTest.message = t('app.modals.tunnel.testing')
  try {
    const result = await TestTunnelConnectionAPI(payload, inlinePayload)
    const latencyText = formatLatencyLabel(result?.latencyMs)
    tunnelTest.status = 'success'
    tunnelTest.message = t('app.modals.tunnel.testPassedWithLatency', { latency: latencyText })
    tunnelTest.debuggable = false
    logEvent('info', `Connection test passed for tunnel ${payload.name}; latency=${latencyText}`)
  } catch (err) {
    tunnelTest.status = 'error'
    tunnelTest.message = errorMessage(err)
    tunnelTest.debuggable = AI_DEBUG_ENABLED
    logEvent('error', `Connection test failed for tunnel ${payload.name}: ${tunnelTest.message}`)
  }
}

async function runJumperAIDebug() {
  if (!AI_DEBUG_ENABLED || !jumperTest.debuggable || !jumperTest.message) return
  const payload = buildJumperPayload(jumperForm)
  setAIDebugLoading(jumperAiDebug)
  try {
    const result = await DebugJumperFailureAPI(payload, jumperTest.message, locale.value)
    setAIDebugResult(jumperAiDebug, result)
    logEvent('info', `AI Debug completed for jumper ${payload.name}`)
  } catch (err) {
    const message = aiDebugErrorMessage(err, 'AI Debug failed for this jumper.')
    setAIDebugError(jumperAiDebug, message)
    logEvent('error', message)
  }
}

async function runTunnelAIDebug() {
  if (!AI_DEBUG_ENABLED || !tunnelTest.debuggable || !tunnelTest.message) return

  let inlinePayload = null
  if (tunnelForm.appendNewJumper) {
    inlinePayload = buildJumperPayload(inlineJumperForm)
  }
  const payload = buildTunnelPayloadForTest()
  setAIDebugLoading(tunnelAiDebug)
  try {
    const result = await DebugTunnelFailureAPI(payload, inlinePayload, tunnelTest.message, locale.value)
    setAIDebugResult(tunnelAiDebug, result)
    logEvent('info', `AI Debug completed for tunnel ${payload.name}`)
  } catch (err) {
    const message = aiDebugErrorMessage(err, 'AI Debug failed for this tunnel.')
    setAIDebugError(tunnelAiDebug, message)
    logEvent('error', message)
  }
}

async function runSavedTunnelAIDebug(tunnel) {
  if (!AI_DEBUG_ENABLED || !tunnel?.id || !tunnel?.lastError) return
  const state = ensureTunnelErrorAIDebugState(tunnel.id)
  setAIDebugLoading(state)
  try {
    const result = await DebugSavedTunnelFailureAPI(Number(tunnel.id), String(tunnel.lastError || ''), locale.value)
    setAIDebugResult(state, result)
    logEvent('info', `AI Debug completed for tunnel ${tunnel.name}`)
  } catch (err) {
    const message = aiDebugErrorMessage(err, `AI Debug failed for tunnel ${tunnel?.name || ''}`.trim())
    setAIDebugError(state, message)
    logEvent('error', message)
  }
}

function openSavedTunnelAIDebug(tunnel) {
  if (!AI_DEBUG_ENABLED || !tunnel?.id || !tunnel?.lastError) return
  selectedAIDebugTunnel.value = tunnel
  const state = ensureTunnelErrorAIDebugState(tunnel.id)
  if (state.status === 'idle') {
    void runSavedTunnelAIDebug(tunnel)
  }
}

function closeSavedTunnelAIDebug() {
  selectedAIDebugTunnel.value = null
}

function retrySavedTunnelAIDebug() {
  if (!selectedAIDebugTunnel.value) return
  void runSavedTunnelAIDebug(selectedAIDebugTunnel.value)
}

function retestSavedTunnelFromAIDebug() {
  if (!selectedAIDebugTunnel.value) return
  void toggleTunnel(selectedAIDebugTunnel.value)
}

function openNewTunnel() {
  editingTunnelId.value = null
  Object.assign(tunnelForm, defaultTunnelForm())
  Object.assign(inlineJumperForm, defaultInlineJumperForm())
  resetInlineJumperValidation()
  resetTunnelTest()
  resetAIDebugState(tunnelAiDebug)
  tunnelValidationError.value = ''
  tunnelForm.jumperIds = jumpers.value.length ? [jumpers.value[0].id] : []
  tunnelForm.nextJumperId = getNextTunnelJumperCandidate(tunnelForm.jumperIds)
  tunnelForm.appendNewJumper = jumpers.value.length === 0
  showTunnelModal.value = true
}

function openImportTunnel() {
  importTunnelError.value = ''
  showImportTunnelModal.value = true
}

function closeImportTunnel() {
  showImportTunnelModal.value = false
  importTunnelError.value = ''
}

async function importTunnels(tunnelsToImport) {
  try {
    importTunnelError.value = ''
    let importedCount = 0
    let skippedCount = 0
    let createdJumperCount = 0
    const existingSignatures = new Set(tunnels.value.map((item) => getTunnelImportSignature(item)))
    const createdJumperCache = new Map()
    
    for (const tunnelData of tunnelsToImport) {
      let jumperIds = []

      if (tunnelData.importJumper?.mode === 'existing') {
        const selectedJumperId = Number(tunnelData.importJumper.jumperId)
        const selectedJumper = jumpers.value.find((item) => item.id === selectedJumperId)
        if (!selectedJumper) {
          throw new Error('Selected existing jumper is missing. Please re-parse and try again.')
        }
        jumperIds = [selectedJumper.id]
      }

      if (tunnelData.importJumper?.mode === 'new' && tunnelData.importJumper?.payload) {
        const payload = tunnelData.importJumper.payload
        const existingJumper = jumpers.value.find((item) => {
          return (
            String(item.host || '').trim().toLowerCase() === String(payload.host || '').trim().toLowerCase() &&
            String(item.user || '').trim() === String(payload.user || '').trim() &&
            Number(item.port) === Number(payload.port)
          )
        })

        if (existingJumper) {
          jumperIds = [existingJumper.id]
        } else {
          const cacheKey = JSON.stringify({
            host: String(payload.host || '').trim().toLowerCase(),
            user: String(payload.user || '').trim(),
            port: Number(payload.port),
            authType: payload.authType,
            keyPath: String(payload.keyPath || '').trim(),
            agentSocketPath: String(payload.agentSocketPath || '').trim()
          })

          if (createdJumperCache.has(cacheKey)) {
            jumperIds = [createdJumperCache.get(cacheKey)]
          } else {
            const createdJumper = await CreateJumper(payload)
            jumperIds = [createdJumper.id]
            jumpers.value.push(createdJumper)
            createdJumperCache.set(cacheKey, createdJumper.id)
            createdJumperCount++
            logEvent('info', `Jumper ${createdJumper.name} created from import`)
          }
        }
      }

      // Backward-compatible fallback for old import payload.
      if (jumperIds.length === 0 && tunnelData.jumperConfig) {
        const config = tunnelData.jumperConfig
        const existingJumper = jumpers.value.find(
          (item) => item.host === config.host && item.user === config.user && item.port === config.port
        )

        if (existingJumper) {
          jumperIds = [existingJumper.id]
        } else {
          const fallbackPayload = {
            name: `${config.host.split('.')[0]}-import`,
            host: config.host,
            port: config.port,
            user: config.user,
            authType: config.keyPath ? 'ssh_key' : 'ssh_agent',
            keyPath: config.keyPath || '',
            agentSocketPath: '',
            password: '',
            bypassHostVerification: false,
            keepAliveIntervalMs: config.keepAliveIntervalMs || 5000,
            timeoutMs: 5000,
            notes: `Imported from SSH command on ${new Date().toLocaleDateString()}`
          }

          const createdJumper = await CreateJumper(fallbackPayload)
          jumperIds = [createdJumper.id]
          jumpers.value.push(createdJumper)
          createdJumperCount++
          logEvent('info', `Jumper ${createdJumper.name} created from import`)
        }
      }
      
      if (jumperIds.length === 0) {
        throw new Error(t('app.modals.importTunnel.errorMissingTarget'))
      }
      
      // Create the tunnel
      const payload = {
        name: tunnelData.name,
        mode: tunnelData.mode,
        jumperIds: jumperIds,
        localHost: tunnelData.localHost,
        localPort: tunnelData.localPort,
        remoteHost: tunnelData.remoteHost,
        remotePort: tunnelData.remotePort,
        autoStart: false,
        status: 'stopped',
        description: `Imported from SSH command on ${new Date().toLocaleDateString()}`
      }

      const signature = getTunnelImportSignature(payload)
      if (existingSignatures.has(signature)) {
        skippedCount++
        logEvent('warn', `Tunnel ${payload.name} skipped (duplicate)`)
        continue
      }
      
      await CreateTunnel(payload)
      existingSignatures.add(signature)
      importedCount++
      logEvent('info', `Tunnel ${payload.name} imported`)
    }
    
    await loadStateFromBackend()
    closeImportTunnel()
    
    let message = createdJumperCount > 0
      ? `Successfully imported ${importedCount} tunnel(s) and created ${createdJumperCount} jumper(s)`
      : `Successfully imported ${importedCount} tunnel(s)`
    if (skippedCount > 0) {
      message = `${message}; skipped ${skippedCount} duplicate tunnel(s)`
    }
    logEvent('info', message)
  } catch (err) {
    const message = errorMessage(err, 'Failed to import tunnels')
    importTunnelError.value = message
    logEvent('error', message)
  }
}

function fillTunnelFormFromTunnel(tunnel, nameOverride = null) {
  Object.assign(tunnelForm, {
    name: nameOverride ?? tunnel.name,
    groupId: Number(tunnel.groupId) || 0,
    mode: tunnel.mode,
    jumperIds: normalizeJumperIdList(tunnel.jumperIds),
    nextJumperId: getNextTunnelJumperCandidate(tunnel.jumperIds),
    appendNewJumper: false,
    localHost: tunnel.localHost,
    localPort: tunnel.localPort,
    remoteHost: tunnel.remoteHost,
    remotePort: tunnel.remotePort,
    autoStart: tunnel.autoStart,
    description: tunnel.description
  })
}

function editTunnel(tunnel) {
  editingTunnelId.value = tunnel.id
  fillTunnelFormFromTunnel(tunnel)
  Object.assign(inlineJumperForm, defaultInlineJumperForm())
  resetInlineJumperValidation()
  resetTunnelTest()
  resetAIDebugState(tunnelAiDebug)
  tunnelValidationError.value = ''
  showTunnelModal.value = true
}

function addJumperToTunnelChain(jumperId) {
  const id = Number(jumperId)
  if (!Number.isInteger(id) || id <= 0) return
  const ids = normalizeJumperIdList(tunnelForm.jumperIds)
  if (ids.includes(id)) return
  tunnelForm.jumperIds = [...ids, id]
  tunnelForm.nextJumperId = getNextTunnelJumperCandidate(tunnelForm.jumperIds)
}

function setPrimaryJumperForTunnelChain(jumperId) {
  const id = Number(jumperId)
  if (!Number.isInteger(id) || id <= 0) return
  const ids = normalizeJumperIdList(tunnelForm.jumperIds).filter((item) => item !== id)
  // When the tunnel currently only has one jumper, changing the primary hop
  // should replace that hop instead of preserving it as an unintended second hop.
  tunnelForm.jumperIds = ids.length > 0 ? [id, ...ids] : [id]
  tunnelForm.nextJumperId = getNextTunnelJumperCandidate(tunnelForm.jumperIds)
}

function removeJumperFromTunnelChain(index) {
  const ids = normalizeJumperIdList(tunnelForm.jumperIds)
  if (!Number.isInteger(index) || index < 0 || index >= ids.length) return
  ids.splice(index, 1)
  tunnelForm.jumperIds = ids
  tunnelForm.nextJumperId = getNextTunnelJumperCandidate(tunnelForm.jumperIds)
}

function moveJumperInTunnelChain(index, offset) {
  const ids = normalizeJumperIdList(tunnelForm.jumperIds)
  if (!Number.isInteger(index) || index < 0 || index >= ids.length) return
  const step = Number(offset)
  if (!Number.isInteger(step) || step === 0) return
  const target = index + step
  if (target < 0 || target >= ids.length) return
  const temp = ids[target]
  ids[target] = ids[index]
  ids[index] = temp
  tunnelForm.jumperIds = ids
  tunnelForm.nextJumperId = getNextTunnelJumperCandidate(tunnelForm.jumperIds)
}

function trimJumperChainToPrimary() {
  const ids = normalizeJumperIdList(tunnelForm.jumperIds)
  if (ids.length <= 1) return
  tunnelForm.jumperIds = [ids[0]]
  tunnelForm.nextJumperId = getNextTunnelJumperCandidate(tunnelForm.jumperIds)
}

function copyTunnel(tunnel) {
  editingTunnelId.value = null
  fillTunnelFormFromTunnel(tunnel, `copy-${tunnel.name}`)
  Object.assign(inlineJumperForm, defaultInlineJumperForm())
  resetInlineJumperValidation()
  resetTunnelTest()
  resetAIDebugState(tunnelAiDebug)
  tunnelValidationError.value = ''
  showTunnelModal.value = true
}

async function saveTunnel() {
  let selectedJumperIds = normalizeJumperIdList(tunnelForm.jumperIds)
  resetInlineJumperValidation()
  tunnelValidationError.value = ''

  try {
    if (tunnelForm.appendNewJumper) {
      const inlinePayload = buildJumperPayload(inlineJumperForm)
      const inlineError = validateJumperPayload(inlinePayload)
      if (inlineError) {
        inlineJumperValidationError.value = inlineError
        return
      }

      const createdJumper = await CreateJumper(inlinePayload)
      selectedJumperIds = [...selectedJumperIds, createdJumper.id]
      logEvent('info', `Jumper ${createdJumper.name} created from New Tunnel`)
    }

    const editingStatus = editingTunnelId.value
      ? tunnels.value.find((item) => item.id === editingTunnelId.value)?.status || 'stopped'
      : 'stopped'
    const payload = {
      name: tunnelForm.name.trim(),
      groupId: Number(tunnelForm.groupId) || 0,
      mode: tunnelForm.mode,
      jumperIds: selectedJumperIds,
      localHost: tunnelForm.localHost.trim(),
      localPort: Number(tunnelForm.localPort),
      remoteHost: tunnelForm.remoteHost.trim(),
      remotePort: Number(tunnelForm.remotePort),
      autoStart: tunnelForm.autoStart,
      status: editingStatus === 'busy' || editingStatus === 'reconnecting' ? 'stopped' : editingStatus,
      description: tunnelForm.description.trim()
    }

    if (!payload.name || !payload.jumperIds.length || !payload.localHost || !payload.localPort) return
    if (nameUnits(payload.name) > TUNNEL_LIMITS.name) {
      tunnelValidationError.value = 'Tunnel name must be <= 20 chars or <= 10 Chinese chars.'
      return
    }
    if (payload.mode !== 'dynamic' && (!payload.remoteHost || !payload.remotePort)) return

    if (editingTunnelId.value) {
      await UpdateTunnel(editingTunnelId.value, payload)
      logEvent('info', `Tunnel ${payload.name} updated`)
    } else {
      await CreateTunnel(payload)
      logEvent('info', `Tunnel ${payload.name} created`)
    }

    await loadStateFromBackend()
    showTunnelModal.value = false
  } catch (err) {
    tunnelValidationError.value = errorMessage(err)
  }
}

function readPortSwitchSkip() {
  try {
    return localStorage.getItem(PORT_SWITCH_SKIP_KEY) === '1'
  } catch {
    return false
  }
}

function writePortSwitchSkip() {
  try {
    localStorage.setItem(PORT_SWITCH_SKIP_KEY, '1')
  } catch {
    // Preference stays for this session only when storage is unavailable.
  }
}

function findRunningPortConflicts(tunnel) {
  return findPortConflicts(tunnel, tunnels.value)
}

async function runToggle(tunnel) {
  if (!tunnel) return
  if (tunnel.status === 'busy') {
    try {
      const updated = await ToggleTunnel(tunnel.id)
      pendingToggleTunnelIds.delete(tunnel.id)
      patchTunnelLocal(updated.id, updated)
      logEvent('warn', `Tunnel ${updated.name || tunnel.name} stopped`)
    } catch (err) {
      logEvent('error', errorMessage(err, `Failed to stop tunnel ${tunnel.name}`))
    }
    return
  }

  const previousStatus = tunnel.status
  const previousLastError = tunnel.lastError || ''
  const shouldShowBusy = previousStatus === 'stopped' || previousStatus === 'error'
  if (shouldShowBusy) {
    pendingToggleTunnelIds.add(tunnel.id)
    patchTunnelLocal(tunnel.id, { status: 'busy', lastError: '', latencyMs: 0 })
  }

  const startTime = Date.now()

  try {
    const updated = await ToggleTunnel(tunnel.id)
    patchTunnelLocal(updated.id, updated)

    if (updated.status === 'error') {
      const reason = updated.lastError ? `: ${updated.lastError}` : ''
      logEvent('error', `Tunnel ${updated.name} failed${reason}`)
      return
    }

    if (previousStatus === 'error') {
      logEvent('info', `Tunnel ${tunnel.name} retry triggered`)
      return
    }

    const action = updated.status === 'running' ? 'started' : 'stopped'
    if (updated.status === 'running') {
      const duration = Date.now() - startTime
      const durationText = duration < 1000 ? `${duration}ms` : `${(duration / 1000).toFixed(2)}s`
      logEvent('info', `Tunnel ${updated.name} started (took ${durationText})`)
    } else {
      logEvent('warn', `Tunnel ${updated.name} ${action}`)
    }
  } catch (err) {
    if (shouldShowBusy) {
      patchTunnelLocal(tunnel.id, { status: previousStatus, lastError: previousLastError })
    }
    logEvent('error', errorMessage(err, `Failed to toggle tunnel ${tunnel.name}`))
  } finally {
    pendingToggleTunnelIds.delete(tunnel.id)
  }
}

async function switchTunnelPort(tunnel, conflicts) {
  for (const conflict of conflicts) {
    const current = tunnels.value.find((item) => item.id === conflict.id) || conflict
    if (current.status !== 'running' && current.status !== 'busy' && current.status !== 'reconnecting') continue
    await runToggle(current)
  }
  const latest = tunnels.value.find((item) => item.id === tunnel.id) || tunnel
  if (latest.status === 'running' || latest.status === 'busy') return
  await runToggle(latest)
}

async function toggleTunnel(tunnel) {
  if (!tunnel) return
  const isStart = tunnel.status === 'stopped' || tunnel.status === 'error'
  if (!isStart) {
    await runToggle(tunnel)
    return
  }

  const conflicts = findRunningPortConflicts(tunnel)
  if (!conflicts.length) {
    await runToggle(tunnel)
    return
  }

  if (readPortSwitchSkip()) {
    await switchTunnelPort(tunnel, conflicts)
    return
  }

  // простой режим спрашивает прямо в компактном окне (SimpleMode), без перехода в расширенный
  if (windowMode.value === 'simple') {
    simplePortSwitchId.value = tunnel.id
    return
  }

  const oldName = conflicts.map((item) => item.name || t('app.options.jumper.unknown')).join('», «')
  openActionDialog({
    mode: 'confirm',
    message: t('app.tunnels.portSwitch.message', {
      oldName,
      newName: tunnel.name,
      port: Number(tunnel.localPort) || 0
    }),
    confirmLabel: t('app.tunnels.portSwitch.confirm'),
    confirmButtonClass: 'btn-primary',
    rememberLabel: t('app.tunnels.portSwitch.dontAsk'),
    onConfirm: (remember) => confirmPortSwitch(tunnel, remember)
  })
}

// «Переключить» в диалоге и в простом режиме: остановить туннели на этом порту и запустить выбранный.
// remember — «Больше не спрашивать» (общая настройка PORT_SWITCH_SKIP_KEY).
async function confirmPortSwitch(tunnel, remember) {
  if (remember) writePortSwitchSkip()
  await switchTunnelPort(tunnel, findRunningPortConflicts(tunnel))
}

const simplePortSwitchPending = computed(() => {
  const tunnel = simpleTunnel.value
  if (simplePortSwitchId.value === null || !tunnel || tunnel.id !== simplePortSwitchId.value) return false
  if (tunnel.status !== 'stopped' && tunnel.status !== 'error') return false
  return findRunningPortConflicts(tunnel).length > 0
})

watch(simplePortSwitchPending, (pending) => {
  if (!pending) simplePortSwitchId.value = null
})

watch(windowMode, () => {
  simplePortSwitchId.value = null
})

function cancelSimplePortSwitch() {
  simplePortSwitchId.value = null
}

async function confirmSimplePortSwitch(remember) {
  const tunnel = simplePortSwitchPending.value ? simpleTunnel.value : null
  simplePortSwitchId.value = null
  if (tunnel) await confirmPortSwitch(tunnel, remember)
}

function openActionDialog({
  mode = 'alert',
  message,
  confirmButtonClass = 'btn-primary',
  confirmLabel = '',
  onConfirm = null,
  secondaryLabel = '',
  secondaryButtonClass = 'btn-outline-primary',
  onSecondary = null,
  rememberLabel = ''
}) {
  actionDialog.mode = mode
  actionDialog.message = message
  actionDialog.confirmButtonClass = confirmButtonClass
  actionDialog.confirmLabel = confirmLabel
  actionDialog.onConfirm = onConfirm
  actionDialog.secondaryLabel = secondaryLabel
  actionDialog.secondaryButtonClass = secondaryButtonClass
  actionDialog.onSecondary = onSecondary
  actionDialog.rememberLabel = rememberLabel
  actionDialog.remember = false
  actionDialog.visible = true
}

function closeActionDialog() {
  actionDialog.visible = false
  actionDialog.confirmLabel = ''
  actionDialog.onConfirm = null
  actionDialog.secondaryLabel = ''
  actionDialog.onSecondary = null
  actionDialog.rememberLabel = ''
  actionDialog.remember = false
}

async function confirmActionDialog() {
  const handler = actionDialog.onConfirm
  const remember = actionDialog.remember
  closeActionDialog()
  if (typeof handler === 'function') {
    await handler(remember)
  }
}

async function secondaryActionDialog() {
  const handler = actionDialog.onSecondary
  closeActionDialog()
  if (typeof handler === 'function') {
    await handler()
  }
}

function deleteTunnel(tunnel) {
  openActionDialog({
    mode: 'confirm',
    message: t('app.confirmations.deleteTunnel', { name: tunnel.name }),
    confirmButtonClass: 'btn-danger',
    onConfirm: async () => {
      try {
        await DeleteTunnel(tunnel.id)
        await loadStateFromBackend()
        logEvent('warn', `Tunnel ${tunnel.name} deleted`)
      } catch (err) {
        logEvent('error', errorMessage(err, `Failed to delete tunnel ${tunnel.name}`))
      }
    }
  })
}

function openTunnelGroupModal(groupId = null) {
  tunnelGroupModalError.value = ''
  pendingTunnelGroupEditId.value = groupId
  showTunnelGroupModal.value = true
}

function closeTunnelGroupModal() {
  showTunnelGroupModal.value = false
  tunnelGroupModalError.value = ''
  pendingTunnelGroupEditId.value = null
}

async function createTunnelGroup(name) {
  try {
    tunnelGroupModalError.value = ''
    const trimmed = String(name || '').trim()
    if (!trimmed) {
      tunnelGroupModalError.value = t('app.tunnels.groups.nameRequired')
      return
    }
    if (nameUnits(trimmed) > TUNNEL_LIMITS.name) {
      tunnelGroupModalError.value = t('app.tunnels.groups.nameTooLong', {
        max: TUNNEL_LIMITS.name,
        half: Math.floor(TUNNEL_LIMITS.name / 2)
      })
      return
    }
    if (isTunnelGroupNameTaken(trimmed)) {
      tunnelGroupModalError.value = t('app.tunnels.groups.nameDuplicate')
      return
    }
    await CreateGroup({ name: trimmed })
    await loadStateFromBackend()
    logEvent('info', `Tunnel group ${name} created`)
  } catch (err) {
    const message = errorMessage(err, 'Failed to create tunnel group.')
    tunnelGroupModalError.value = /group name already exists/i.test(message)
      ? t('app.tunnels.groups.nameDuplicate')
      : message
  }
}

async function renameTunnelGroup({ id, name }) {
  try {
    tunnelGroupModalError.value = ''
    const trimmed = String(name || '').trim()
    if (!trimmed) {
      tunnelGroupModalError.value = t('app.tunnels.groups.nameRequired')
      return
    }
    if (nameUnits(trimmed) > TUNNEL_LIMITS.name) {
      tunnelGroupModalError.value = t('app.tunnels.groups.nameTooLong', {
        max: TUNNEL_LIMITS.name,
        half: Math.floor(TUNNEL_LIMITS.name / 2)
      })
      return
    }
    if (isTunnelGroupNameTaken(trimmed, id)) {
      tunnelGroupModalError.value = t('app.tunnels.groups.nameDuplicate')
      return
    }
    await UpdateGroup(id, { name: trimmed })
    await loadStateFromBackend()
    logEvent('info', `Tunnel group ${name} updated`)
  } catch (err) {
    const message = errorMessage(err, 'Failed to rename tunnel group.')
    tunnelGroupModalError.value = /group name already exists/i.test(message)
      ? t('app.tunnels.groups.nameDuplicate')
      : message
  }
}

function requestRenameTunnelGroup(group) {
  openTunnelGroupModal(group?.id ?? null)
}

function deleteTunnelGroup(group) {
  openActionDialog({
    mode: 'confirm',
    message: t('app.confirmations.deleteTunnelGroup', { name: group.name }),
    confirmButtonClass: 'btn-danger',
    onConfirm: async () => {
      try {
        tunnelGroupModalError.value = ''
        await DeleteGroup(group.id)
        await loadStateFromBackend()
        logEvent('warn', `Tunnel group ${group.name} deleted`)
      } catch (err) {
        const message = errorMessage(err, `Failed to delete tunnel group ${group.name}`)
        tunnelGroupModalError.value = message
        logEvent('error', message)
      }
    }
  })
}

async function reorderTunnelGroups(ids) {
  const orderedIds = (Array.isArray(ids) ? ids : []).map((id) => Number(id)).filter((id) => Number.isInteger(id) && id > 0)
  if (orderedIds.length === 0) return

  const currentIds = tunnelGroups.value.map((group) => Number(group.id))
  if (
    orderedIds.length === currentIds.length &&
    orderedIds.every((id, index) => id === currentIds[index])
  ) {
    return
  }

  try {
    tunnelGroupModalError.value = ''
    await ReorderGroups(orderedIds)
    await loadStateFromBackend()
    logEvent('info', 'Tunnel groups reordered')
  } catch (err) {
    const message = errorMessage(err, 'Failed to reorder tunnel groups.')
    tunnelGroupModalError.value = message
    logEvent('error', message)
    await loadStateFromBackend()
  }
}

async function moveTunnelToGroup({ tunnel, groupId }) {
  if (!tunnel?.id) return
  const normalizedGroupId = Number(groupId) || 0
  if (normalizedGroupId === (Number(tunnel.groupId) || 0)) return
  if (tunnel.status === 'busy') return

  const groupName = normalizedGroupId === 0
    ? t('app.tunnels.groups.ungrouped')
    : (tunnelGroups.value.find((group) => Number(group.id) === normalizedGroupId)?.name || String(normalizedGroupId))

  try {
    await MoveTunnelToGroup(tunnel.id, normalizedGroupId)
    await loadStateFromBackend()
    setConfigMessage(t('app.tunnels.groups.moved', { name: tunnel.name, group: groupName }))
    logEvent('info', `Tunnel ${tunnel.name} moved to group ${groupName}`)
  } catch (err) {
    const message = errorMessage(err, `Failed to move tunnel ${tunnel.name}`)
    setConfigMessage(message)
    logEvent('error', message)
  }
}

function deleteJumper(jumper) {
  const inUseBy = tunnels.value.filter((item) => normalizeJumperIdList(item.jumperIds).includes(jumper.id))
  if (inUseBy.length > 0) {
    openActionDialog({
      mode: 'alert',
      message: t('app.confirmations.deleteJumperBlocked', { name: jumper.name, count: inUseBy.length }),
      confirmButtonClass: 'btn-primary'
    })
    logEvent('warn', `Delete blocked for jumper ${jumper.name} (still in use)`)
    return
  }
  openActionDialog({
    mode: 'confirm',
    message: t('app.confirmations.deleteJumper', { name: jumper.name }),
    confirmButtonClass: 'btn-danger',
    onConfirm: async () => {
      try {
        await DeleteJumper(jumper.id)
        await loadStateFromBackend()
        logEvent('warn', `Jumper ${jumper.name} deleted`)
      } catch (err) {
        logEvent('error', errorMessage(err, `Failed to delete jumper ${jumper.name}`))
      }
    }
  })
}

const dialogOpen = computed(() => (
  actionDialog.visible
  || showImportTunnelModal.value
  || showImportJumperModal.value
  || showTunnelGroupModal.value
  || showTunnelModal.value
  || showJumperModal.value
  || !!selectedAIDebugTunnel.value
))

function dismissTopDialog() {
  if (actionDialog.visible) {
    closeActionDialog()
    return
  }
  if (showImportTunnelModal.value) {
    closeImportTunnel()
    return
  }
  if (showImportJumperModal.value) {
    closeImportJumper()
    return
  }
  if (showTunnelGroupModal.value) {
    closeTunnelGroupModal()
    return
  }
  if (showTunnelModal.value) {
    showTunnelModal.value = false
    resetAIDebugState(tunnelAiDebug)
    return
  }
  if (showJumperModal.value) {
    showJumperModal.value = false
    resetAIDebugState(jumperAiDebug)
    return
  }
  if (selectedAIDebugTunnel.value) {
    closeSavedTunnelAIDebug()
  }
}

// WebView2 на Windows не ставит event.shiftKey у Ctrl+Shift+буква (Escape при этом приходит
// как обычно) и может отдать пустой event.code в русской раскладке. Shift держим сами
// и сверяем с GetAsyncKeyState, буква M — и по code, и по символу (M / ь).
let shiftDown = false
let modeShortcutFromKeyDown = false
let modeShortcutLock = false

function isShiftKey(event) {
  return event.key === 'Shift' || event.code === 'ShiftLeft' || event.code === 'ShiftRight'
}

function isModeKey(event) {
  if (event.code === 'KeyM') return true
  const key = event.key
  return key === 'M' || key === 'm' || key === 'ь' || key === 'Ь'
}

function isModeChord(event) {
  return event.ctrlKey && !event.altKey && !event.metaKey && isModeKey(event)
}

async function shiftIsHeld(event) {
  if (event.shiftKey || event.getModifierState?.('Shift') || shiftDown) return true
  try {
    return (await ShiftDown()) === true
  } catch (_) {
    return false
  }
}

async function toggleWindowModeFromShortcut(event) {
  if (dialogOpen.value || modeShortcutLock) return
  modeShortcutLock = true
  try {
    if (!(await shiftIsHeld(event))) return
    event.preventDefault()
    await setWindowMode(windowMode.value === 'simple' ? 'advanced' : 'simple')
  } finally {
    modeShortcutLock = false
  }
}

function onWindowKeydown(event) {
  if (isShiftKey(event)) {
    shiftDown = true
    return
  }
  if (isModeChord(event)) {
    if (event.repeat) return
    modeShortcutFromKeyDown = true
    void toggleWindowModeFromShortcut(event)
    return
  }
  if (event.key !== 'Escape' || !dialogOpen.value) return
  if (event.target instanceof HTMLSelectElement) return
  event.preventDefault()
  dismissTopDialog()
}

function onWindowKeyup(event) {
  if (isShiftKey(event)) {
    shiftDown = false
    return
  }
  if (!isModeChord(event)) return
  if (modeShortcutFromKeyDown) {
    modeShortcutFromKeyDown = false
    return
  }
  void toggleWindowModeFromShortcut(event)
}

function onWindowBlur() {
  shiftDown = false
  modeShortcutFromKeyDown = false
}

// диалоги не помещаются в компактное окно (подтверждение смены порта простой режим
// показывает сам, см. simplePortSwitchId — сюда оно не попадает)
watch(dialogOpen, (open) => {
  if (open && windowMode.value === 'simple') void setWindowMode('advanced')
})

watch(dialogOpen, (open) => {
  if (!open || typeof document === 'undefined') return
  requestAnimationFrame(() => {
    const dialog = document.querySelector('.overlay .dialog-card')
    const field = dialog?.querySelector('input:not([type="hidden"]):not([type="file"]), select, textarea, .app-select-trigger')
    if (field instanceof HTMLElement) {
      field.focus()
      return
    }
    const closeButton = dialog?.querySelector('.dialog-close')
    if (closeButton instanceof HTMLElement) closeButton.focus()
  })
})

onMounted(async () => {
  if (!themePinned && typeof window !== 'undefined' && window.matchMedia) {
    const media = window.matchMedia('(prefers-color-scheme: dark)')
    const onOsTheme = () => {
      if (themePinned) return
      theme.value = media.matches ? 'dark' : 'light'
    }
    media.addEventListener('change', onOsTheme)
    onBeforeUnmount(() => media.removeEventListener('change', onOsTheme))
  }
  await detectCustomTitleBar()
  await ensureWindowOnScreen()
  if (windowMode.value === 'simple') {
    // окно стартует в размере из main.go — запоминаем его для возврата и сжимаем
    await enterSimpleWindow()
  }
  await loadStateFromBackend()
  try {
    await SaveUILocale(locale.value)
  } catch (_) {
    /* tray locale sync is best-effort */
  }
  try {
    trafficMonitorEnabled.value = await GetTrafficMonitorEnabled()
  } catch (_) {
    trafficMonitorEnabled.value = true
  }
  stateSyncTimer = window.setInterval(syncStateSilently, STATE_SYNC_INTERVAL_MS)
  startTrafficSync()
  window.addEventListener('keydown', onWindowKeydown, true)
  window.addEventListener('keyup', onWindowKeyup, true)
  window.addEventListener('blur', onWindowBlur)
  subscribeTrayEvents()
})

onBeforeUnmount(() => {
  if (stateSyncTimer !== null) {
    window.clearInterval(stateSyncTimer)
    stateSyncTimer = null
  }
  stopTrafficSync()
  if (configToastTimer !== null) {
    window.clearTimeout(configToastTimer)
    configToastTimer = null
  }
  window.removeEventListener('keydown', onWindowKeydown, true)
  window.removeEventListener('keyup', onWindowKeyup, true)
  window.removeEventListener('blur', onWindowBlur)
  offRuntimeEvents.forEach((off) => off?.())
})

watch(
  () => jumperForm.authType,
  (newType) => {
    if (!authShowsPassword(newType)) jumperForm.password = ''
    if (!authNeedsKeyFile(newType)) jumperForm.keyPath = ''
    resetJumperTest()
  }
)

watch(
  () => inlineJumperForm.authType,
  (newType) => {
    if (!authShowsPassword(newType)) inlineJumperForm.password = ''
    if (!authNeedsKeyFile(newType)) inlineJumperForm.keyPath = ''
  }
)
</script>


<template>
  <n-config-provider class="app-root" :theme="naiveTheme" :theme-overrides="themeOverrides" :locale="ruRU" :date-locale="dateRuRU">
  <n-global-style />
  <n-dialog-provider>
  <n-message-provider>
  <div
    class="app-frame"
    :class="[`app-frame--${windowMode}`, { 'app-frame--titlebar': customTitleBar }]"
  >
  <AppTitleBar
    v-if="customTitleBar"
    :mode="windowMode"
    :theme="theme"
    :status="norkaStatus"
    @minimise="minimiseWindow"
    @toggle-mode="setWindowMode(windowMode === 'simple' ? 'advanced' : 'simple')"
    @close="closeWindow"
  />
  <SimpleMode
    v-if="windowMode === 'simple'"
    :tunnels="tunnels"
    :tunnel="simpleTunnel"
    :theme="theme"
    :get-running-since="getTunnelRunningSince"
    :port-switch-pending="simplePortSwitchPending"
    @select="selectSimpleTunnel"
    @toggle="toggleTunnel"
    @port-switch-confirm="confirmSimplePortSwitch"
    @port-switch-cancel="cancelSimplePortSwitch"
    @manage="openTunnelsFromSimple"
  />
  <template v-else>
  <a class="skip-link" href="#page-main">{{ $t('app.common.skipToContent') }}</a>
  <n-layout class="app-shell" has-sider :inert="dialogOpen">
    <AppSidebar
      :pages="pages"
      :active-page="activePage"
      :app-version="appMeta.version"
      :collapsed="sidebarCollapsed"
      :theme="theme"
      :tunnel-status="norkaStatus"
      :traffic-monitor-enabled="trafficMonitorEnabled"
      :traffic="traffic"
      :traffic-history-up="trafficHistoryUp"
      :traffic-history-down="trafficHistoryDown"
      @switch-page="switchPage"
      @toggle-collapse="toggleSidebar"
    />

    <n-layout class="content-shell">
      <AppTopHeader
        :current-page="currentPage"
        :active-page="activePage"
        @import-jumper="openImportJumper"
        @new-jumper="openNewJumper"
        @new-tunnel="openNewTunnel"
        @import-tunnel="openImportTunnel"
      />

      <main id="page-main" tabindex="-1" class="page-body" :class="{ 'page-body-overview': activePage === 'overview' }">
        <OverviewPage
          v-if="activePage === 'overview'"
          :total-tunnels="totalTunnels"
          :running-tunnels="runningTunnels"
          :stopped-tunnels="stoppedTunnels"
          :auto-start-count="autoStartTunnels.length"
          :show-overview-active="showOverviewActive"
          :show-overview-activity="showOverviewActivity"
          :logs="logs"
          :get-tunnel-jumper-label="getTunnelJumperLabel"
          @toggle-overview-active="showOverviewActive = !showOverviewActive"
          @toggle-overview-activity="showOverviewActivity = !showOverviewActivity"
          @toggle-tunnel="toggleTunnel"
        />

        <JumpersPage
          v-if="activePage === 'jumpers'"
          :jumpers="filteredJumpers"
          :search-query="jumperSearchQuery"
          :get-auth-label="getAuthLabel"
          @update-search-query="jumperSearchQuery = $event"
          @copy-jumper="copyJumper"
          @edit-jumper="editJumper"
          @delete-jumper="deleteJumper"
        />

        <TunnelsPage
          v-if="activePage === 'tunnels'"
          :tunnels="filteredTunnels"
          :groups="tunnelGroups"
          :hide-empty-ungrouped="hideEmptyUngrouped"
          :search-query="tunnelSearchQuery"
          :mode-options="modeOptions"
          :tunnel-ai-debug-states="tunnelErrorAiDebugStates"
          :ai-debug-enabled="AI_DEBUG_ENABLED"
          :get-tunnel-jumper-label="getTunnelJumperLabel"
          @update-search-query="tunnelSearchQuery = $event"
          @toggle-tunnel="toggleTunnel"
          @copy-tunnel="copyTunnel"
          @edit-tunnel="editTunnel"
          @delete-tunnel="deleteTunnel"
          @ai-debug="openSavedTunnelAIDebug"
          @manage-groups="openTunnelGroupModal()"
          @rename-group="requestRenameTunnelGroup"
          @delete-group="deleteTunnelGroup"
          @move-tunnel-to-group="moveTunnelToGroup"
        />

        <LogsPage
          v-if="activePage === 'logs'"
          :selected-log-level="selectedLogLevel"
          :filtered-logs="filteredLogs"
          @set-log-level="selectedLogLevel = $event"
        />

        <ConfigPage
          v-if="activePage === 'config'"
          :theme="theme"
          :app-meta="appMeta"
          :window-mode="windowMode"
          :simple-on-top="simpleOnTop"
          @theme-change="setThemeBySwitch"
          @set-config-message="setConfigMessage"
          @reload-state="loadStateFromBackend"
          @confirm-action="openActionDialog"
          @traffic-monitor-change="onTrafficMonitorChange"
          @window-mode-change="setWindowMode"
          @simple-on-top-change="simpleOnTop = $event"
        />
      </main>

      <n-alert
        v-if="showConfigToast && configMessage"
        class="config-toast"
        type="info"
        closable
        @close="hideConfigToast"
      >
        {{ configMessage }}
      </n-alert>
    </n-layout>
  </n-layout>
  </template>
  </div>

  <n-modal
    :show="actionDialog.visible"
    preset="card"
    :title="$t('app.common.confirm')"
    style="width: min(480px, calc(100vw - 32px))"
    :mask-closable="true"
    @update:show="(open) => { if (!open) closeActionDialog() }"
  >
    <p class="action-dialog-message">{{ actionDialog.message }}</p>
    <n-checkbox v-if="actionDialog.rememberLabel" v-model:checked="actionDialog.remember">
      {{ actionDialog.rememberLabel }}
    </n-checkbox>
    <template #footer>
      <n-space justify="end">
        <n-button v-if="actionDialog.mode === 'confirm'" @click="closeActionDialog">
          {{ $t('app.common.cancel') }}
        </n-button>
        <n-button
          v-if="actionDialog.onSecondary"
          secondary
          @click="secondaryActionDialog"
        >
          {{ actionDialog.secondaryLabel }}
        </n-button>
        <n-button
          :type="actionDialog.confirmButtonClass.includes('danger') ? 'error' : 'primary'"
          @click="confirmActionDialog"
        >
          {{
            actionDialog.confirmLabel ||
              (actionDialog.mode === 'confirm' ? $t('app.common.delete') : $t('app.common.confirm'))
          }}
        </n-button>
      </n-space>
    </template>
  </n-modal>

  <AIDebugModal
    v-if="AI_DEBUG_ENABLED"
    :show="!!selectedAIDebugTunnel"
    :title="selectedAIDebugTunnelTitle"
    :subtitle="selectedAIDebugTunnelSubtitle"
    :raw-error="selectedAIDebugTunnel?.lastError || ''"
    :state="selectedAIDebugTunnelState"
    @close="closeSavedTunnelAIDebug"
    @retry-debug="retrySavedTunnelAIDebug"
    @test-again="retestSavedTunnelFromAIDebug"
    @report-content="reportAIDebugContent('saved_tunnel', selectedAIDebugTunnelState)"
  />

  <JumperModal
    :show="showJumperModal"
    :editing-jumper-id="editingJumperId"
    :jumper-form="jumperForm"
    :show-jumper-basic="showJumperBasic"
    :show-jumper-advanced="showJumperAdvanced"
    :auth-options="authOptions"
    :jumper-needs-key-file="jumperNeedsKeyFile"
    :jumper-needs-password="jumperNeedsPassword"
    :jumper-shows-password="jumperShowsPassword"
    :jumper-limits="JUMPER_LIMITS"
    :jumper-validation-error="jumperValidationError"
    :jumper-test="jumperTest"
    :jumper-ai-debug="jumperAiDebug"
    :ai-debug-enabled="AI_DEBUG_ENABLED"
    @close="showJumperModal = false; resetAIDebugState(jumperAiDebug)"
    @submit="saveJumper"
    @toggle-basic="showJumperBasic = !showJumperBasic"
    @toggle-advanced="showJumperAdvanced = !showJumperAdvanced"
    @key-file-change="onJumperKeyFileChange"
    @test-connection="testJumperConnection"
    @ai-debug="runJumperAIDebug"
    @report-ai-content="reportAIDebugContent('jumper', jumperAiDebug)"
  />

  <ImportJumperModal
    :show="showImportJumperModal"
    :candidates="sshConfigCandidates"
    :existing-jumpers="jumpers"
    :auth-options="authOptions"
    :jumper-limits="JUMPER_LIMITS"
    :sources="sshConfigSources"
    :selected-source-path="selectedImportJumperSourcePath"
    :loading="importJumperLoading"
    :load-error="importJumperError"
    :import-error="importJumperError"
    :has-loaded="importJumperHasLoaded"
    @close="closeImportJumper"
    @update:selected-source-path="selectedImportJumperSourcePath = $event"
    @load="loadImportJumpers"
    @import="importJumpers"
  />

  <TunnelModal
    :show="showTunnelModal"
    :editing-tunnel-id="editingTunnelId"
    :tunnel-form="tunnelForm"
    :tunnels="tunnels"
    :groups="tunnelGroups"
    :mode-options="modeOptions"
    :jumpers="jumpers"
    :inline-jumper-form="inlineJumperForm"
    :auth-options="authOptions"
    :inline-jumper-needs-key-file="inlineJumperNeedsKeyFile"
    :inline-jumper-needs-password="inlineJumperNeedsPassword"
    :inline-jumper-shows-password="inlineJumperShowsPassword"
    :jumper-limits="JUMPER_LIMITS"
    :inline-jumper-validation-error="inlineJumperValidationError"
    :tunnel-validation-error="tunnelValidationError"
    :tunnel-test="tunnelTest"
    :tunnel-ai-debug="tunnelAiDebug"
    :ai-debug-enabled="AI_DEBUG_ENABLED"
    @close="showTunnelModal = false; resetAIDebugState(tunnelAiDebug)"
    @submit="saveTunnel"
    @set-primary-jumper="setPrimaryJumperForTunnelChain"
    @add-jumper="addJumperToTunnelChain"
    @move-jumper="moveJumperInTunnelChain"
    @trim-jumpers-to-primary="trimJumperChainToPrimary"
    @remove-jumper="removeJumperFromTunnelChain"
    @inline-key-file-change="onInlineJumperKeyFileChange"
    @test-connection="testTunnelConnection"
    @ai-debug="runTunnelAIDebug"
    @report-ai-content="reportAIDebugContent('tunnel', tunnelAiDebug)"
  />

  <TunnelGroupModal
    :show="showTunnelGroupModal"
    :groups="tunnelGroups"
    :hide-empty-ungrouped="hideEmptyUngrouped"
    :initial-edit-group-id="pendingTunnelGroupEditId"
    :error-message="tunnelGroupModalError"
    :name-max-length="TUNNEL_LIMITS.name"
    @close="closeTunnelGroupModal"
    @create-group="createTunnelGroup"
    @rename-group="renameTunnelGroup"
    @delete-group="deleteTunnelGroup"
    @reorder-groups="reorderTunnelGroups"
    @update:hide-empty-ungrouped="setHideEmptyUngrouped"
  />

  <ImportTunnelModal
    :show="showImportTunnelModal"
    :jumpers="jumpers"
    :existing-tunnels="tunnels"
    :mode-options="modeOptions"
    :auth-options="authOptions"
    :jumper-limits="JUMPER_LIMITS"
    :import-error="importTunnelError"
    @close="closeImportTunnel"
    @import="importTunnels"
  />
  </n-message-provider>
  </n-dialog-provider>
  </n-config-provider>
</template>
