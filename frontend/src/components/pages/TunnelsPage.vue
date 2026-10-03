<script setup>
import { computed, h, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { NButton, NDropdown, NTag } from 'naive-ui'
import { formatDuration, sessionUptimeSeconds } from '../../utils/tunnel-stats'

const props = defineProps({
  tunnels: {
    type: Array,
    required: true
  },
  groups: {
    type: Array,
    default: () => []
  },
  hideEmptyUngrouped: {
    type: Boolean,
    default: true
  },
  searchQuery: {
    type: String,
    default: ''
  },
  focusTunnelId: {
    type: Number,
    default: 0
  },
  modeOptions: {
    type: Array,
    required: true
  },
  tunnelAiDebugStates: {
    type: Object,
    required: true
  },
  aiDebugEnabled: {
    type: Boolean,
    default: false
  },
  getTunnelJumperLabel: {
    type: Function,
    required: true
  },
  sshCommandEnabled: {
    type: Boolean,
    default: false
  },
  automationEnabled: {
    type: Boolean,
    default: false
  },
  statsEnabled: {
    type: Boolean,
    default: false
  },
  tunnelStats: {
    type: Array,
    default: () => []
  },
  statsNow: {
    type: Number,
    default: 0
  },
  diagnosticsEnabled: {
    type: Boolean,
    default: false
  }
})

const emit = defineEmits([
  'toggle-tunnel',
  'copy-tunnel',
  'edit-tunnel',
  'delete-tunnel',
  'update-search-query',
  'ai-debug',
  'manage-groups',
  'rename-group',
  'delete-group',
  'move-tunnel-to-group',
  'copy-ssh-command',
  'copy-link',
  'show-stats',
  'diagnose'
])
const { t } = useI18n()

const statsById = computed(() => {
  const map = new Map()
  for (const row of props.tunnelStats || []) {
    if (row?.id) map.set(row.id, row)
  }
  return map
})

function uptimeLabel(row, now) {
  if (!props.statsEnabled || row.status !== 'running') return ''
  const stat = statsById.value.get(row.id)
  const seconds = sessionUptimeSeconds(stat, now)
  if (!stat?.connected || !stat.sessionStartedUnix) return ''
  return formatDuration(seconds, t)
}

const UNGROUPED_SECTION_KEY = 'ungrouped'
const COLLAPSED_GROUPS_STORAGE_KEY = 'lt.tunnel-groups.collapsed'

const localSearchQuery = ref(props.searchQuery)
const expandedErrorIds = ref(new Set())
const copiedErrorIds = ref(new Set())
const copyResetTimers = new Map()
const collapsedSectionKeys = ref(new Set())
const showGroupedView = computed(() => Array.isArray(props.groups) && props.groups.length > 0)

const validGroupIds = computed(() => new Set(props.groups.map((group) => Number(group.id))))

const tunnelSections = computed(() => {
  if (!showGroupedView.value) return []

  const grouped = new Map()
  for (const group of props.groups) {
    grouped.set(Number(group.id), [])
  }

  const ungrouped = []
  for (const tunnel of props.tunnels) {
    const groupId = resolveTunnelGroupId(tunnel)
    if (groupId > 0) {
      const bucket = grouped.get(groupId)
      if (bucket) bucket.push(tunnel)
      else ungrouped.push(tunnel)
    } else {
      ungrouped.push(tunnel)
    }
  }

  const searching = Boolean(localSearchQuery.value.trim())
  const sections = []

  for (const group of props.groups) {
    const items = grouped.get(Number(group.id)) || []
    if (searching && items.length === 0) continue
    sections.push({
      key: String(group.id),
      groupId: Number(group.id),
      name: group.name,
      tunnels: items,
      isUngrouped: false
    })
  }

  if (ungrouped.length > 0 || (!searching && !props.hideEmptyUngrouped)) {
    sections.push({
      key: UNGROUPED_SECTION_KEY,
      groupId: 0,
      name: t('app.tunnels.groups.ungrouped'),
      tunnels: ungrouped,
      isUngrouped: true
    })
  }

  return sections
})

const visibleEntries = computed(() => {
  if (props.tunnels.length === 0) {
    return [{ kind: 'empty', key: 'empty' }]
  }

  if (!showGroupedView.value) {
    return props.tunnels.flatMap((tunnel) => [
      { kind: 'tunnel', key: `tunnel-${tunnel.id}`, tunnel },
      ...(isErrorExpanded(tunnel.id)
        ? [{ kind: 'error', key: `error-${tunnel.id}`, tunnel }]
        : [])
    ])
  }

  const entries = []
  for (const section of tunnelSections.value) {
    entries.push({ kind: 'group', key: `group-${section.key}`, section })
    if (isSectionCollapsed(section.key)) continue
    for (const tunnel of section.tunnels) {
      entries.push({ kind: 'tunnel', key: `tunnel-${tunnel.id}`, tunnel })
      if (isErrorExpanded(tunnel.id)) {
        entries.push({ kind: 'error', key: `error-${tunnel.id}`, tunnel })
      }
    }
  }
  return entries
})

watch(() => props.searchQuery, (newValue) => {
  localSearchQuery.value = newValue
})

watch(() => props.focusTunnelId, (id) => {
  const tunnelId = Number(id)
  if (!tunnelId) return
  const tunnel = props.tunnels.find((item) => Number(item.id) === tunnelId)
  if (tunnel && showGroupedView.value) {
    const groupId = resolveTunnelGroupId(tunnel)
    const key = groupId > 0 ? String(groupId) : UNGROUPED_SECTION_KEY
    if (collapsedSectionKeys.value.has(key)) {
      const next = new Set(collapsedSectionKeys.value)
      next.delete(key)
      collapsedSectionKeys.value = next
    }
  }
  void nextTick(() => {
    document.querySelector(`[data-tunnel-id="${tunnelId}"]`)?.scrollIntoView({ block: 'center', behavior: 'smooth' })
  })
})

function rowProps(row) {
  return { 'data-tunnel-id': String(row.id) }
}

function rowClassName(row) {
  return Number(row.id) === Number(props.focusTunnelId) ? 'tunnel-row-focus' : ''
}

watch(localSearchQuery, (newValue) => {
  emit('update-search-query', newValue)
  if (!newValue.trim()) return
  const next = new Set(collapsedSectionKeys.value)
  tunnelSections.value.forEach((section) => {
    if (section.tunnels.length > 0) {
      next.delete(section.key)
    }
  })
  collapsedSectionKeys.value = next
})

watch(
  () => props.tunnels,
  (nextTunnels) => {
    const validErrorIds = new Set(
      nextTunnels
        .filter((tunnel) => tunnel.status === 'error' && tunnel.lastError)
        .map((tunnel) => tunnel.id)
    )
    expandedErrorIds.value = new Set([...expandedErrorIds.value].filter((id) => validErrorIds.has(id)))
    copiedErrorIds.value = new Set([...copiedErrorIds.value].filter((id) => validErrorIds.has(id)))
    copyResetTimers.forEach((timerId, id) => {
      if (!validErrorIds.has(id)) {
        clearTimeout(timerId)
        copyResetTimers.delete(id)
      }
    })
  },
  { deep: true }
)

// На узком окне (≈1000px, размер по умолчанию) режим переезжает под имя, задержка — в колонку
// статуса, а гибкие колонки получают минимальную ширину — иначе маршрут и имя сжимаются до 3 символов.
const COMPACT_TABLE_MAX_WINDOW_WIDTH = 1180
const viewportWidth = ref(typeof window !== 'undefined' ? window.innerWidth : COMPACT_TABLE_MAX_WINDOW_WIDTH)
const compactTable = computed(() => viewportWidth.value < COMPACT_TABLE_MAX_WINDOW_WIDTH)

function onViewportResize() {
  viewportWidth.value = window.innerWidth
}

onBeforeUnmount(() => {
  copyResetTimers.forEach((timerId) => {
    clearTimeout(timerId)
  })
  copyResetTimers.clear()
  window.removeEventListener('resize', onViewportResize)
})

onMounted(() => {
  loadCollapsedSections()
  window.addEventListener('resize', onViewportResize, { passive: true })
})

function loadCollapsedSections() {
  if (typeof window === 'undefined') return
  try {
    const raw = window.localStorage.getItem(COLLAPSED_GROUPS_STORAGE_KEY)
    if (!raw) return
    const parsed = JSON.parse(raw)
    if (Array.isArray(parsed)) {
      collapsedSectionKeys.value = new Set(parsed.map((item) => String(item)))
    }
  } catch (_) {
    collapsedSectionKeys.value = new Set()
  }
}

function persistCollapsedSections() {
  if (typeof window === 'undefined') return
  window.localStorage.setItem(COLLAPSED_GROUPS_STORAGE_KEY, JSON.stringify([...collapsedSectionKeys.value]))
}

function resolveTunnelGroupId(tunnel) {
  const groupId = Number(tunnel?.groupId) || 0
  if (groupId > 0 && validGroupIds.value.has(groupId)) return groupId
  return 0
}

function isSectionCollapsed(sectionKey) {
  return collapsedSectionKeys.value.has(sectionKey)
}

function toggleSectionCollapsed(sectionKey) {
  const next = new Set(collapsedSectionKeys.value)
  if (next.has(sectionKey)) {
    next.delete(sectionKey)
  } else {
    next.add(sectionKey)
  }
  collapsedSectionKeys.value = next
  persistCollapsedSections()
}

function getMoveGroupOptions(tunnel) {
  const currentGroupId = resolveTunnelGroupId(tunnel)
  const options = props.groups
    .filter((group) => Number(group.id) !== currentGroupId)
    .map((group) => ({
      groupId: Number(group.id),
      label: group.name
    }))
  if (currentGroupId !== 0) {
    options.unshift({
      groupId: 0,
      label: t('app.tunnels.groups.ungrouped')
    })
  }
  return options
}

function onGroupMenu(key, section) {
  if (key === 'rename') {
    emit('rename-group', { id: section.groupId, name: section.name })
    return
  }
  if (key === 'delete') {
    emit('delete-group', { id: section.groupId, name: section.name })
  }
}

function groupMenuOptions() {
  return [
    { label: t('app.tunnels.groups.rename'), key: 'rename' },
    { type: 'divider', key: 'divider' },
    { label: t('app.tunnels.groups.delete'), key: 'delete' },
  ]
}

function getModeLabel(modeValue) {
  return props.modeOptions.find((mode) => mode.value === modeValue)?.label || modeValue
}

function getStatusBadgeClass(status) {
  return {
    running: status === 'running',
    busy: status === 'busy' || status === 'reconnecting',
    error: status === 'error',
    stopped: status !== 'running' && status !== 'busy' && status !== 'reconnecting' && status !== 'error'
  }
}

function getPrimaryActionButtonClass(status) {
  if (status === 'running' || status === 'busy' || status === 'reconnecting') return 'btn-outline-danger'
  if (status === 'error') return 'btn-outline-warning'
  return 'btn-outline-success'
}

function getPrimaryActionTitle(status) {
  if (status === 'running' || status === 'busy' || status === 'reconnecting') return 'app.tunnels.actions.stop'
  if (status === 'error') return 'app.tunnels.actions.retry'
  return 'app.tunnels.actions.start'
}

function getPrimaryActionIcon(status) {
  if (status === 'running' || status === 'busy' || status === 'reconnecting') return 'bi-pause-fill'
  if (status === 'error') return 'bi-arrow-repeat'
  return 'bi-power'
}

function getMenuToggleButtonClass(status) {
  if (status === 'running' || status === 'busy' || status === 'reconnecting') return 'btn-outline-danger'
  if (status === 'error') return 'btn-outline-warning'
  return 'btn-outline-success'
}

function getStatusLabelKey(status) {
  switch (status) {
    case 'running':
      return 'app.tunnels.status.running'
    case 'stopped':
      return 'app.tunnels.status.stopped'
    case 'busy':
      return 'app.tunnels.status.busy'
    case 'reconnecting':
      return 'app.tunnels.status.reconnecting'
    case 'error':
      return 'app.tunnels.status.error'
    default:
      return ''
  }
}

function getRouteLines(tunnel) {
  const local = `${tunnel.localHost}:${tunnel.localPort}`
  const remote = `${tunnel.remoteHost}:${tunnel.remotePort}`
  if (tunnel.mode === 'dynamic') {
    return {
      top: local,
      bottom: 'SOCKS5'
    }
  }
  if (tunnel.mode === 'remote') {
    return {
      top: remote,
      bottom: local
    }
  }
  return {
    top: local,
    bottom: remote
  }
}

function getRouteTop(tunnel) {
  return getRouteLines(tunnel).top
}

function getRouteBottom(tunnel) {
  return getRouteLines(tunnel).bottom
}

// Keep latency readable in a compact table cell by switching units automatically.
function formatLatencyLabel(latencyMs) {
  const ms = Number(latencyMs)
  if (!Number.isFinite(ms) || ms <= 0) return '--'
  if (ms < 1000) return `${Math.round(ms)} ms`
  if (ms < 60000) return `${(ms / 1000).toFixed(ms < 10000 ? 2 : 1)} s`
  if (ms < 3600000) return `${(ms / 60000).toFixed(ms < 600000 ? 2 : 1)} min`
  return `${(ms / 3600000).toFixed(2)} h`
}

// Show placeholder/measuring state when tunnel is not running or latency is not ready yet.
function getTunnelLatencyLabel(tunnel) {
  if (tunnel.status !== 'running') return '--'
  if (!tunnel.latencyMs) return t('app.tunnels.latency.measuring')
  return formatLatencyLabel(tunnel.latencyMs)
}

function canToggleErrorDetails(tunnel) {
  return tunnel.status === 'error' && Boolean(tunnel.lastError)
}

function isErrorExpanded(tunnelId) {
  return expandedErrorIds.value.has(tunnelId)
}

function toggleErrorDetails(tunnel) {
  if (!canToggleErrorDetails(tunnel)) return
  const next = new Set(expandedErrorIds.value)
  if (next.has(tunnel.id)) {
    next.delete(tunnel.id)
  } else {
    next.add(tunnel.id)
  }
  expandedErrorIds.value = next
}

function isErrorCopied(tunnelId) {
  return copiedErrorIds.value.has(tunnelId)
}

async function writeTextToClipboard(text) {
  if (navigator?.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(text)
      return true
    } catch (err) {
      // fallback below
    }
  }

  try {
    const fallbackInput = document.createElement('textarea')
    fallbackInput.value = text
    fallbackInput.setAttribute('readonly', '')
    fallbackInput.style.position = 'fixed'
    fallbackInput.style.top = '-9999px'
    fallbackInput.style.left = '-9999px'
    fallbackInput.style.opacity = '0'
    document.body.appendChild(fallbackInput)
    fallbackInput.focus()
    fallbackInput.select()
    const copied = document.execCommand('copy')
    document.body.removeChild(fallbackInput)
    return copied
  } catch (err) {
    return false
  }
}

async function copyErrorDetails(tunnel) {
  if (!tunnel.lastError) return
  try {
    const copied = await writeTextToClipboard(tunnel.lastError)
    if (!copied) return
    const next = new Set(copiedErrorIds.value)
    next.add(tunnel.id)
    copiedErrorIds.value = next
    const oldTimer = copyResetTimers.get(tunnel.id)
    if (oldTimer) {
      clearTimeout(oldTimer)
    }
    const timerId = setTimeout(() => {
      const nextCopied = new Set(copiedErrorIds.value)
      nextCopied.delete(tunnel.id)
      copiedErrorIds.value = nextCopied
      copyResetTimers.delete(tunnel.id)
    }, 1800)
    copyResetTimers.set(tunnel.id, timerId)
  } catch (err) {
    console.warn('Failed to copy tunnel error details:', err)
  }
}

function getErrorToggleLabelKey(tunnel) {
  if (!canToggleErrorDetails(tunnel)) return ''
  return isErrorExpanded(tunnel.id) ? 'app.tunnels.actions.collapseError' : 'app.tunnels.actions.expandError'
}

function getErrorCopyLabelKey(tunnelId) {
  return isErrorCopied(tunnelId) ? 'app.tunnels.actions.copied' : 'app.tunnels.actions.copyError'
}

function getTunnelAiDebugState(tunnelId) {
  return props.tunnelAiDebugStates?.[tunnelId] || { status: 'idle', error: '', result: null }
}

function getAIDebugActionLabel(tunnelId) {
  const state = getTunnelAiDebugState(tunnelId)
  if (state.status === 'analyzing') return t('app.aiDebug.analyzing')
  if (state.status === 'success' || state.status === 'error') return t('app.aiDebug.viewResult')
  return t('app.aiDebug.action')
}

function statusTagType(status) {
  if (status === 'running') return 'success'
  if (status === 'busy') return 'info'
  if (status === 'reconnecting') return 'warning'
  if (status === 'error') return 'error'
  return 'default'
}

function actionButtonType(status) {
  if (status === 'running' || status === 'busy' || status === 'reconnecting') return 'error'
  if (status === 'error') return 'warning'
  return 'success'
}

function tunnelMenuOptions(tunnel) {
  const options = [
    { label: t('app.tunnels.actions.edit'), key: 'edit', disabled: tunnel.status === 'busy' },
    { label: t('app.tunnels.actions.copy'), key: 'copy' },
  ]
  if (props.sshCommandEnabled) {
    options.push({ label: t('app.tunnels.actions.copySSH'), key: 'copy-ssh' })
  }
  if (props.automationEnabled) {
    options.push({ label: t('app.tunnels.actions.copyLink'), key: 'copy-link' })
  }
  if (props.statsEnabled) {
    options.push({ label: t('app.tunnels.actions.stats'), key: 'stats' })
  }
  if (props.diagnosticsEnabled) {
    options.push({ label: t('app.tunnels.actions.diagnostics'), key: 'diagnostics' })
  }
  const moves = showGroupedView.value ? getMoveGroupOptions(tunnel) : []
  if (moves.length > 0) {
    options.push({ type: 'divider', key: 'move-divider' })
    options.push({
      label: t('app.tunnels.groups.moveTo'),
      key: 'move',
      disabled: tunnel.status === 'busy',
      children: moves.map((option) => ({
        label: option.label,
        key: `move:${option.groupId}`,
        disabled: tunnel.status === 'busy',
      })),
    })
  }
  options.push({ type: 'divider', key: 'delete-divider' })
  options.push({ label: t('app.tunnels.actions.delete'), key: 'delete', disabled: tunnel.status === 'busy' })
  return options
}

function onTunnelMenu(key, tunnel) {
  if (key === 'edit') {
    emit('edit-tunnel', tunnel)
    return
  }
  if (key === 'copy') {
    emit('copy-tunnel', tunnel)
    return
  }
  if (key === 'copy-ssh') {
    emit('copy-ssh-command', tunnel)
    return
  }
  if (key === 'copy-link') {
    emit('copy-link', tunnel)
    return
  }
  if (key === 'stats') {
    emit('show-stats', tunnel)
    return
  }
  if (key === 'diagnostics') {
    emit('diagnose', tunnel)
    return
  }
  if (key === 'delete') {
    emit('delete-tunnel', tunnel)
    return
  }
  if (String(key).startsWith('move:')) {
    emit('move-tunnel-to-group', { tunnel, groupId: Number(String(key).slice(5)) })
  }
}

const expandedRowKeys = computed(() => [...expandedErrorIds.value])

function onExpandedRows(keys) {
  expandedErrorIds.value = new Set(keys)
}

const expandedSectionNames = computed(() => tunnelSections.value
  .filter((section) => !isSectionCollapsed(section.key))
  .map((section) => section.key))

function onExpandedSections(names) {
  const expanded = new Set(names)
  collapsedSectionKeys.value = new Set(
    tunnelSections.value.map((section) => section.key).filter((key) => !expanded.has(key))
  )
  persistCollapsedSections()
}

const tunnelColumns = computed(() => {
  const clock = props.statsNow
  return [
  {
    type: 'expand',
    expandable: (row) => canToggleErrorDetails(row),
    renderExpand: (row) => h('div', { style: 'display:flex;justify-content:space-between;gap:12px' }, [
      h('div', [
        h('div', { style: 'font-weight: 600' }, t('app.tunnels.errorReason')),
        h('div', row.lastError),
      ]),
      h(NButton, {
        size: 'small',
        quaternary: true,
        title: t(getErrorCopyLabelKey(row.id)),
        onClick: () => copyErrorDetails(row),
      }, { icon: () => h('i', { class: isErrorCopied(row.id) ? 'bi bi-check2' : 'bi bi-copy' }) }),
    ]),
  },
  compactTable.value
    ? {
        title: t('app.tunnels.table.name'),
        key: 'name',
        minWidth: 120,
        render: (row) => h('div', { title: `${row.name} · ${getModeLabel(row.mode)}` }, [
          h('div', { class: 'tunnel-cell-line' }, row.name),
          h('div', { class: 'tunnel-cell-line', style: 'opacity: 0.7' }, getModeLabel(row.mode)),
        ]),
      }
    : { title: t('app.tunnels.table.name'), key: 'name', ellipsis: { tooltip: true } },
  ...(compactTable.value ? [] : [{
    title: t('app.tunnels.table.mode'),
    key: 'mode',
    width: 120,
    render: (row) => getModeLabel(row.mode),
  }]),
  {
    title: t('app.tunnels.table.route'),
    key: 'route',
    minWidth: 130,
    render: (row) => h('div', { title: `${getRouteTop(row)} → ${getRouteBottom(row)}` }, [
      h('div', { class: 'tunnel-cell-line' }, getRouteTop(row)),
      h('div', { class: 'tunnel-cell-line', style: 'opacity: 0.7' }, getRouteBottom(row)),
    ]),
  },
  {
    title: t('app.tunnels.table.jumper'),
    key: 'jumper',
    width: compactTable.value ? 104 : undefined,
    ellipsis: { tooltip: true },
    render: (row) => props.getTunnelJumperLabel(row),
  },
  {
    title: t('app.tunnels.table.status'),
    key: 'status',
    width: props.statsEnabled ? 210 : 140,
    render: (row) => {
      const tag = h(NTag, {
        size: 'small',
        type: statusTagType(row.status),
        style: canToggleErrorDetails(row) ? 'cursor: pointer' : undefined,
        onClick: () => toggleErrorDetails(row),
      }, { default: () => (getStatusLabelKey(row.status) ? t(getStatusLabelKey(row.status)) : row.status) })
      const uptime = uptimeLabel(row, clock)
      const showLatency = compactTable.value && row.status === 'running'
      if (!uptime && !showLatency) return tag
      return h('div', { class: 'tunnel-status-cell' }, [
        tag,
        uptime
          ? h('span', { class: 'tunnel-status-uptime', title: t('app.tunnels.stats.uptime') }, uptime)
          : null,
        showLatency
          ? h('span', { class: 'tunnel-status-latency', title: t('app.tunnels.table.latency') }, getTunnelLatencyLabel(row))
          : null,
      ].filter(Boolean))
    },
  },
  ...(compactTable.value ? [] : [{
    title: t('app.tunnels.table.latency'),
    key: 'latency',
    width: 110,
    render: (row) => getTunnelLatencyLabel(row),
  }]),
  {
    title: t('app.tunnels.table.action'),
    key: 'action',
    align: 'right',
    width: compactTable.value ? 84 : 110,
    render: (row) => h('div', { style: 'display:flex;justify-content:flex-end;gap:4px' }, [
      h(NButton, {
        size: 'small',
        quaternary: true,
        type: actionButtonType(row.status),
        title: t(getPrimaryActionTitle(row.status)),
        onClick: () => emit('toggle-tunnel', row),
      }, { icon: () => h('i', { class: ['bi', getPrimaryActionIcon(row.status)] }) }),
      h(NDropdown, {
        trigger: 'click',
        options: tunnelMenuOptions(row),
        onSelect: (key) => onTunnelMenu(key, row),
      }, {
        default: () => h(NButton, {
          size: 'small',
          quaternary: true,
          title: t('app.tunnels.actions.more'),
        }, { icon: () => h('i', { class: 'bi bi-three-dots' }) }),
      }),
    ]),
  },
  ]
})

// ниже этой ширины таблица прокручивается по горизонтали, а не сжимает колонки
const tableMinWidth = computed(() => tunnelColumns.value
  .reduce((sum, column) => sum + (column.width || column.minWidth || 48), 0))
</script>

<template>
  <n-card size="small" :title="$t('app.tunnels.title')">
    <template #header-extra>
      <div class="tunnel-card-toolbar">
        <n-button size="small" secondary @click="emit('manage-groups')">
          <template #icon><i class="bi bi-folder2" /></template>
          {{$t('app.tunnels.groups.manage')}}
        </n-button>
        <n-input
          v-model:value="localSearchQuery"
          size="small"
          clearable
          :placeholder="$t('app.common.searchPlaceholder')"
          :aria-label="$t('app.common.searchTunnels')"
        >
          <template #prefix><i class="bi bi-search" /></template>
        </n-input>
      </div>
    </template>
    <n-empty v-if="tunnels.length === 0" :description="$t('app.tunnels.noTunnels')" />
    <n-data-table
      v-else-if="!showGroupedView"
      size="small"
      :columns="tunnelColumns"
      :data="tunnels"
      :bordered="false"
      :scroll-x="tableMinWidth"
      :row-key="(row) => row.id"
      :row-props="rowProps"
      :row-class-name="rowClassName"
      :expanded-row-keys="expandedRowKeys"
      @update:expanded-row-keys="onExpandedRows"
    />
    <n-collapse
      v-else
      :expanded-names="expandedSectionNames"
      @update:expanded-names="onExpandedSections"
    >
      <n-collapse-item
        v-for="section in tunnelSections"
        :key="section.key"
        :name="section.key"
        :title="section.name + ' (' + section.tunnels.length + ')'"
      >
        <template v-if="!section.isUngrouped" #header-extra>
          <n-dropdown
            trigger="click"
            :options="groupMenuOptions()"
            @select="(key) => onGroupMenu(key, section)"
            @click.stop
          >
            <n-button size="tiny" quaternary @click.stop>
              <template #icon><i class="bi bi-three-dots" /></template>
            </n-button>
          </n-dropdown>
        </template>
        <n-data-table
          size="small"
          :columns="tunnelColumns"
          :data="section.tunnels"
          :bordered="false"
          :scroll-x="tableMinWidth"
          :row-key="(row) => row.id"
          :row-props="rowProps"
          :row-class-name="rowClassName"
          :expanded-row-keys="expandedRowKeys"
          @update:expanded-row-keys="onExpandedRows"
        />
      </n-collapse-item>
    </n-collapse>
  </n-card>
</template>

<style scoped>
:deep(.tunnel-cell-line) {
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

:deep(.tunnel-status-cell) {
  display: flex;
  align-items: center;
  gap: 6px;
  white-space: nowrap;
}

:deep(.tunnel-status-latency),
:deep(.tunnel-status-uptime) {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  color: var(--lt-muted);
  font-size: 12px;
  font-variant-numeric: tabular-nums;
}

.tunnel-card-toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
}

.tunnel-card-toolbar :deep(.n-button),
.tunnel-card-toolbar :deep(.n-input) {
  height: 32px;
}

.tunnel-card-toolbar :deep(.n-input) {
  width: 240px;
}

:deep(.tunnel-row-focus td) {
  background: color-mix(in srgb, var(--lt-brand) 18%, transparent);
}
</style>
