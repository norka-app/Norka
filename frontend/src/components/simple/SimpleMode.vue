<script setup>
// Простой режим (концепт A): компактное окно на один туннель.
// Вся логика запуска/остановки остаётся в App.vue — компонент только отображает и эмитит события.
import { computed, nextTick, onBeforeUnmount, onMounted, ref, useId, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { BrowserOpenURL, ClipboardSetText } from '../../../wailsjs/runtime/runtime'
import NorkaStatusLogo from '../norka/NorkaStatusLogo.vue'
import { findPortConflicts } from '../../utils/port-conflicts'

const COPIED_FEEDBACK_MS = 1500
const TYPEAHEAD_RESET_MS = 600

const props = defineProps({
  tunnels: {
    type: Array,
    default: () => []
  },
  tunnel: {
    type: Object,
    default: null
  },
  theme: {
    type: String,
    default: 'light'
  },
  // id туннеля → время (ms), с которого он в статусе running (для аптайма)
  getRunningSince: {
    type: Function,
    default: () => undefined
  },
  // App.vue ждёт подтверждения: локальный порт выбранного туннеля занят другим туннелем
  portSwitchPending: {
    type: Boolean,
    default: false
  }
})

const emit = defineEmits(['select', 'toggle', 'manage', 'port-switch-confirm', 'port-switch-cancel'])

const { t } = useI18n()
const now = ref(Date.now())
const copied = ref(false)
let uptimeTimer = null
let copiedTimer = null

const rawStatus = computed(() => props.tunnel?.status || 'stopped')

// состояние глаз Норки и цвет строки статуса
const state = computed(() => {
  switch (rawStatus.value) {
    case 'running':
      return 'connected'
    case 'busy':
    case 'reconnecting':
      return 'connecting'
    case 'error':
      return 'error'
    default:
      return 'stopped'
  }
})

const statusKey = computed(() => (
  ['running', 'busy', 'reconnecting', 'error'].includes(rawStatus.value) ? rawStatus.value : 'stopped'
))
const statusLabel = computed(() => (props.tunnel ? t(`app.simple.status.${statusKey.value}`) : t('app.simple.noTunnels')))

function formatUptime(ms) {
  const total = Math.max(0, Math.floor(ms / 1000))
  const days = Math.floor(total / 86400)
  const pad = (value) => String(value).padStart(2, '0')
  const hms = `${pad(Math.floor((total % 86400) / 3600))}:${pad(Math.floor((total % 3600) / 60))}:${pad(total % 60)}`
  return days > 0 ? t('app.simple.uptimeDays', { days, time: hms }) : hms
}

const statusDetail = computed(() => {
  if (!props.tunnel) return ''
  if (rawStatus.value === 'running') {
    const since = props.getRunningSince(props.tunnel.id)
    return since ? formatUptime(now.value - since) : ''
  }
  if (rawStatus.value === 'error') return props.tunnel.lastError || ''
  return ''
})

function displayHost(host) {
  const value = String(host || '').trim()
  if (!value || value === '127.0.0.1' || value === '0.0.0.0' || value === '::1' || value === 'localhost') return 'localhost'
  return value
}

const localAddress = computed(() => (
  props.tunnel ? `${displayHost(props.tunnel.localHost)}:${props.tunnel.localPort}` : ''
))

const route = computed(() => {
  const item = props.tunnel
  if (!item) return null
  const remote = `${item.remoteHost}:${item.remotePort}`
  if (item.mode === 'dynamic') return { from: localAddress.value, to: 'SOCKS5' }
  if (item.mode === 'remote') return { from: remote, to: localAddress.value }
  return { from: localAddress.value, to: remote }
})

const canCopy = computed(() => rawStatus.value === 'running' && props.tunnel?.mode !== 'remote')
const canOpen = computed(() => rawStatus.value === 'running' && props.tunnel?.mode === 'local')

const primary = computed(() => {
  switch (rawStatus.value) {
    case 'running':
    case 'reconnecting':
      return { label: t('app.simple.actions.disconnect'), icon: 'bi-power', variant: 'outline' }
    case 'busy':
      return { label: t('app.simple.actions.cancel'), icon: 'bi-x-lg', variant: 'outline' }
    case 'error':
      return { label: t('app.simple.actions.retry'), icon: 'bi-arrow-clockwise', variant: 'primary' }
    default:
      return { label: t('app.simple.actions.connect'), icon: 'bi-power', variant: 'primary' }
  }
})

// ── Занятый порт ────────────────────────────────────────────────────────────
// «Порт 3000 занят «…»» — для туннелей, чей порт держит другой работающий/подключающийся туннель
function portBusyText(item, conflicts) {
  const names = conflicts.map((other) => other.name || t('app.options.jumper.unknown')).join('», «')
  return t('app.simple.portBusy', { port: item.localPort, names })
}

const portBusyHints = computed(() => {
  const hints = new Map()
  for (const item of props.tunnels) {
    const conflicts = findPortConflicts(item, props.tunnels)
    if (conflicts.length) hints.set(item.id, portBusyText(item, conflicts))
  }
  return hints
})

// Подтверждение «Переключить / Отмена» прямо в окне (вместо диалога расширенного режима)
const portConflict = computed(() => (
  props.portSwitchPending && props.tunnel ? portBusyHints.value.get(props.tunnel.id) || '' : ''
))
const dontAsk = ref(false)
const switchBtnRef = ref(null)

watch(portConflict, async (text, previous) => {
  if (!text || previous) return
  dontAsk.value = false
  await nextTick()
  if (!listOpen.value) switchBtnRef.value?.focus({ preventScroll: true })
})

function confirmPortSwitch() {
  emit('port-switch-confirm', dontAsk.value)
}

function cancelPortSwitch() {
  emit('port-switch-cancel')
}

function onWindowKeydown(event) {
  if (event.key !== 'Escape' || !portConflict.value || listOpen.value) return
  event.preventDefault()
  cancelPortSwitch()
}


// ── Список туннелей ─────────────────────────────────────────────────────────
// Свой listbox вместо нативного <select>: системный список WebView2 вылезал за края
// маленького окна. Панель открывается поверх правой колонки и не выходит за окно;
// если пунктов много — прокручивается, «Управление туннелями…» закреплён снизу.
const listId = useId()
const listOpen = ref(false)
const activeIndex = ref(0)
const triggerRef = ref(null)
const listRef = ref(null)
let typeahead = ''
let typeaheadTimer = null

// последний «пункт» — действие «Управление туннелями…»
const manageIndex = computed(() => props.tunnels.length)
const optionId = (index) => `${listId}-opt-${index}`

function statusKeyOf(item) {
  return ['running', 'busy', 'reconnecting', 'error'].includes(item?.status) ? item.status : 'stopped'
}

function dotStateOf(item) {
  const key = statusKeyOf(item)
  if (key === 'running') return 'connected'
  if (key === 'busy' || key === 'reconnecting') return 'connecting'
  return key === 'error' ? 'error' : 'stopped'
}

function optionTitle(item) {
  const title = `${item.name} · ${displayHost(item.localHost)}:${item.localPort} — ${t(`app.simple.optionStatus.${statusKeyOf(item)}`)}`
  const busy = portBusyHints.value.get(item.id)
  return busy ? `${title}. ${busy}` : title
}

function scrollActiveIntoView() {
  const list = listRef.value
  if (!list) return
  // «Управление туннелями…» закреплён снизу (sticky) — для него просто докручиваем до конца
  if (activeIndex.value === manageIndex.value) {
    list.scrollTop = list.scrollHeight
    return
  }
  list.querySelector(`#${CSS.escape(optionId(activeIndex.value))}`)?.scrollIntoView({ block: 'nearest' })
}

function setActive(index) {
  activeIndex.value = Math.min(Math.max(index, 0), manageIndex.value)
  nextTick(scrollActiveIntoView)
}

async function openList() {
  if (listOpen.value) return
  const selected = props.tunnels.findIndex((item) => item.id === props.tunnel?.id)
  activeIndex.value = selected >= 0 ? selected : 0
  listOpen.value = true
  await nextTick()
  listRef.value?.focus({ preventScroll: true })
  scrollActiveIntoView()
}

function closeList(restoreFocus = true) {
  if (!listOpen.value) return
  listOpen.value = false
  if (restoreFocus) nextTick(() => triggerRef.value?.focus({ preventScroll: true }))
}

function chooseOption(index) {
  if (index === manageIndex.value) {
    closeList(false)
    emit('manage')
    return
  }
  const item = props.tunnels[index]
  closeList()
  if (item) emit('select', item.id)
}

function onTriggerKeydown(event) {
  if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    event.preventDefault()
    openList()
  }
}

function runTypeahead(char) {
  typeahead += char.toLocaleLowerCase()
  if (typeaheadTimer !== null) clearTimeout(typeaheadTimer)
  typeaheadTimer = setTimeout(() => {
    typeahead = ''
    typeaheadTimer = null
  }, TYPEAHEAD_RESET_MS)
  const count = props.tunnels.length
  // при повторе одной буквы — к следующему совпадению, иначе ищем с текущего пункта
  const start = typeahead.length === 1 ? activeIndex.value + 1 : activeIndex.value
  for (let step = 0; step < count; step += 1) {
    const index = (start + step) % count
    if (String(props.tunnels[index].name || '').toLocaleLowerCase().startsWith(typeahead)) {
      setActive(index)
      return
    }
  }
}

function onListKeydown(event) {
  switch (event.key) {
    case 'ArrowDown':
      setActive(activeIndex.value + 1)
      break
    case 'ArrowUp':
      setActive(activeIndex.value - 1)
      break
    case 'Home':
    case 'PageUp':
      setActive(0)
      break
    case 'End':
    case 'PageDown':
      setActive(manageIndex.value)
      break
    case 'Enter':
    case ' ':
      chooseOption(activeIndex.value)
      break
    case 'Escape':
      closeList()
      break
    case 'Tab':
      closeList(false)
      return
    default:
      if (event.key.length === 1 && !event.ctrlKey && !event.metaKey && !event.altKey) {
        runTypeahead(event.key)
        break
      }
      return
  }
  event.preventDefault()
  event.stopPropagation()
}

function onListFocusOut(event) {
  const next = event.relatedTarget
  if (next && (event.currentTarget.contains(next) || next === triggerRef.value)) return
  closeList(false)
}

function toggle() {
  if (props.tunnel) emit('toggle', props.tunnel)
}

async function copyAddress() {
  if (!canCopy.value) return
  try {
    const ok = await ClipboardSetText(localAddress.value)
    if (ok === false) throw new Error('clipboard')
  } catch (_) {
    try {
      await navigator.clipboard.writeText(localAddress.value)
    } catch (__) {
      return
    }
  }
  copied.value = true
  if (copiedTimer !== null) clearTimeout(copiedTimer)
  copiedTimer = setTimeout(() => {
    copied.value = false
    copiedTimer = null
  }, COPIED_FEEDBACK_MS)
}

function openInBrowser() {
  if (!canOpen.value) return
  const url = `http://localhost:${props.tunnel.localPort}`
  try {
    BrowserOpenURL(url)
  } catch (_) {
    window.open(url, '_blank', 'noopener')
  }
}

watch(state, (value) => {
  if (value === 'connected' && uptimeTimer === null) {
    now.value = Date.now()
    uptimeTimer = setInterval(() => { now.value = Date.now() }, 1000)
  } else if (value !== 'connected' && uptimeTimer !== null) {
    clearInterval(uptimeTimer)
    uptimeTimer = null
  }
}, { immediate: true })

onMounted(() => {
  window.addEventListener('keydown', onWindowKeydown)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onWindowKeydown)
  if (uptimeTimer !== null) clearInterval(uptimeTimer)
  if (copiedTimer !== null) clearTimeout(copiedTimer)
  if (typeaheadTimer !== null) clearTimeout(typeaheadTimer)
})
</script>

<template>
  <div class="simple" :class="`simple--${theme === 'dark' ? 'dark' : 'light'}`">
    <!-- Норка только показывает статус; клики копят пасхалку «нокаут», подключение — кнопкой справа -->
    <div class="simple-icon">
      <NorkaStatusLogo
        :key="tunnel ? tunnel.id : 'none'"
        :status="tunnel ? state : 'stopped'"
        :theme="theme === 'dark' ? 'dark' : 'light'"
        :size="117"
        :title="$t('app.title')"
        :status-label="statusLabel"
        idle
        easter-egg
      />
    </div>

    <div class="simple-col">
      <button
        ref="triggerRef"
        type="button"
        class="simple-select"
        aria-haspopup="listbox"
        :aria-expanded="listOpen ? 'true' : 'false'"
        :aria-controls="listId"
        :aria-label="`${$t('app.simple.selectTunnel')}: ${tunnel ? tunnel.name : $t('app.simple.noTunnels')}`"
        @click="listOpen ? closeList() : openList()"
        @keydown="onTriggerKeydown"
      >
        <i class="bi bi-diagram-3 simple-select__lead" aria-hidden="true" />
        <span class="simple-select__name">{{ tunnel ? tunnel.name : $t('app.simple.noTunnels') }}</span>
        <span v-if="tunnel" class="simple-select__port">· {{ tunnel.localPort }}</span>
        <i class="bi bi-chevron-down simple-select__chev" aria-hidden="true" />
      </button>

      <div v-if="!tunnel" class="simple-info simple-empty">{{ $t('app.simple.noTunnelsHint') }}</div>
      <div v-else-if="portConflict" class="simple-info">
        <div class="simple-status s-warning" role="alert">
          <i class="bi bi-exclamation-triangle-fill simple-warn-icon" aria-hidden="true" />
          <span class="simple-detail simple-warn-text" :title="portConflict">{{ portConflict }}</span>
        </div>
        <label class="simple-dontask">
          <input v-model="dontAsk" type="checkbox">{{ $t('app.tunnels.portSwitch.dontAsk') }}
        </label>
      </div>
      <div v-else class="simple-info">
        <div class="simple-status" :class="`s-${state}`" role="status">
          <span v-if="state === 'connecting'" class="simple-spin" aria-hidden="true" />
          <span v-else class="simple-dot" aria-hidden="true" />
          <b>{{ statusLabel }}</b>
          <template v-if="statusDetail">
            <span class="simple-sep" aria-hidden="true">·</span>
            <span class="simple-detail" :title="statusDetail">{{ statusDetail }}</span>
          </template>
        </div>
        <div class="simple-addr" :title="`${route.from} → ${route.to}`">
          {{ route.from }}<span class="simple-arrow">→</span>{{ route.to }}
        </div>
      </div>

      <div v-if="portConflict" class="simple-acts">
        <button ref="switchBtnRef" type="button" class="simple-btn simple-btn--primary" @click="confirmPortSwitch">
          <i class="bi bi-arrow-left-right" aria-hidden="true" />{{ $t('app.tunnels.portSwitch.confirm') }}
        </button>
        <button type="button" class="simple-btn simple-btn--outline simple-btn--secondary" @click="cancelPortSwitch">
          {{ $t('app.common.cancel') }}
        </button>
      </div>
      <div v-else class="simple-acts">
        <button
          type="button"
          class="simple-btn"
          :class="`simple-btn--${primary.variant}`"
          :disabled="!tunnel"
          @click="toggle"
        >
          <i class="bi" :class="primary.icon" aria-hidden="true" />{{ primary.label }}
        </button>
        <button
          type="button"
          class="simple-ibtn"
          :disabled="!canCopy"
          :title="copied ? $t('app.simple.copied') : $t('app.simple.copyAddress')"
          :aria-label="$t('app.simple.copyAddress')"
          @click="copyAddress"
        >
          <i class="bi" :class="copied ? 'bi-check2' : 'bi-copy'" aria-hidden="true" />
        </button>
        <button
          type="button"
          class="simple-ibtn"
          :disabled="!canOpen"
          :title="$t('app.simple.openBrowser')"
          :aria-label="$t('app.simple.openBrowser')"
          @click="openInBrowser"
        >
          <i class="bi bi-box-arrow-up-right" aria-hidden="true" />
        </button>
      </div>
    </div>

    <div v-if="listOpen" class="simple-backdrop" aria-hidden="true" @pointerdown.prevent="closeList()" />
    <Transition name="simple-pop">
      <div v-if="listOpen" class="simple-panel" @focusout="onListFocusOut">
        <ul
          :id="listId"
          ref="listRef"
          class="simple-list"
          role="listbox"
          tabindex="-1"
          :aria-label="$t('app.simple.selectTunnel')"
          :aria-activedescendant="optionId(activeIndex)"
          @keydown="onListKeydown"
        >
          <li
            v-for="(item, index) in tunnels"
            :id="optionId(index)"
            :key="item.id"
            role="option"
            class="simple-option"
            :class="{ 'is-active': index === activeIndex, 'is-selected': item.id === tunnel?.id }"
            :aria-selected="item.id === tunnel?.id ? 'true' : 'false'"
            :title="optionTitle(item)"
            @mousemove="activeIndex = index"
            @click="chooseOption(index)"
          >
            <span class="simple-option__dot" :class="`d-${dotStateOf(item)}`" aria-hidden="true" />
            <span class="simple-option__name">{{ item.name }}</span>
            <span
              v-if="portBusyHints.has(item.id)"
              class="simple-option__tag"
              :title="portBusyHints.get(item.id)"
            >{{ $t('app.simple.portBusyTag') }}</span>
            <span class="simple-option__port">{{ item.localPort }}</span>
            <span class="visually-hidden">{{ $t(`app.simple.optionStatus.${statusKeyOf(item)}`) }}</span>
            <span v-if="portBusyHints.has(item.id)" class="visually-hidden">{{ portBusyHints.get(item.id) }}</span>
          </li>
          <li v-if="!tunnels.length" class="simple-list__empty" role="presentation">{{ $t('app.simple.noTunnels') }}</li>
          <li
            :id="optionId(manageIndex)"
            role="option"
            aria-selected="false"
            class="simple-option simple-option--manage"
            :class="{ 'is-active': activeIndex === manageIndex }"
            @mousemove="activeIndex = manageIndex"
            @click="chooseOption(manageIndex)"
          >
            <i class="bi bi-sliders2" aria-hidden="true" />
            <span class="simple-option__name">{{ $t('app.simple.manageTunnels') }}</span>
          </li>
        </ul>
      </div>
    </Transition>
  </div>
</template>

<style scoped>
/* цвета и размеры — из макета концепта A (norka/simple/tool/style.css) */
.simple--light {
  --s-bg: #ffffff; --s-text: #1f2329; --s-muted: #646b76; --s-faint: #9aa1ac;
  --s-ctrl: #ffffff; --s-ctrl-b: #d9dce1; --s-ctrl-bb: #c4c8cf; --s-ctrl-h: #f3f4f6;
  --s-accent: #2563eb; --s-accent-h: #1d4ed8; --s-on-accent: #fff;
  --s-green: #15803d; --s-green-dot: #22c55e; --s-amber: #b45309; --s-amber-dot: #f5a524;
  --s-red: #c81e1e; --s-red-dot: #ef4444; --s-grey-dot: #b4bac3; --s-icon-ring: transparent;
  --s-panel-shadow: rgba(15, 23, 42, 0.16);
}
.simple--dark {
  --s-bg: #202227; --s-text: #e8eaed; --s-muted: #a3a9b3; --s-faint: #6e7580;
  --s-ctrl: #2b2e35; --s-ctrl-b: #3a3e47; --s-ctrl-bb: #454a54; --s-ctrl-h: #33363e;
  /* в тёмной теме — общий акцент приложения (как «Создать туннель», переключатели, меню) */
  --s-accent: var(--lt-brand, #7dd3fc); --s-accent-h: var(--lt-brand-hover, #bae6fd); --s-on-accent: var(--lt-on-brand, #000);
  --s-green: #4ade80; --s-green-dot: #3ddc84; --s-amber: #fbbf24; --s-amber-dot: #f5c542;
  --s-red: #f87171; --s-red-dot: #ff5a5f; --s-grey-dot: #6b7280; --s-icon-ring: rgba(255, 255, 255, 0.07);
  --s-panel-shadow: rgba(0, 0, 0, 0.5);
}

.simple {
  position: relative;
  width: 100vw;
  height: 100vh;
  min-width: 413px;
  min-height: 149px;
  overflow: hidden;
  background: var(--s-bg);
  color: var(--s-text);
  font-family: "Segoe UI Variable Text", "Segoe UI", system-ui, sans-serif;
  -webkit-font-smoothing: antialiased;
}

.simple-icon {
  position: absolute;
  left: 16px;
  top: 16px;
  width: 117px;
  height: 117px;
  border-radius: 26px;
  line-height: 0;
  box-shadow: 0 0 0 1px var(--s-icon-ring);
}

.simple-col {
  position: absolute;
  left: 149px;
  right: 16px;
  top: 16px;
  bottom: 16px;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
  min-width: 0;
}

.simple-select {
  position: relative;
  width: 100%;
  height: 32px;
  color: var(--s-text);
  font-family: inherit;
  text-align: left;
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 9px 0 11px;
  border: 1px solid var(--s-ctrl-b);
  border-bottom-color: var(--s-ctrl-bb);
  border-radius: 6px;
  background: var(--s-ctrl);
  font-size: 13px;
  cursor: pointer;
}
.simple-select:hover { background: var(--s-ctrl-h); }
.simple-select:focus-visible,
.simple-select[aria-expanded='true'] {
  outline: none;
  border-color: var(--s-accent);
  box-shadow: inset 0 -1px 0 var(--s-accent);
}
.simple-select__lead { color: var(--s-text); font-size: 15px; }
.simple-select__name { font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; min-width: 0; }
.simple-select__port { color: var(--s-muted); white-space: nowrap; }
.simple-select__chev { margin-left: auto; color: var(--s-muted); font-size: 13px; }

.simple-status {
  display: flex;
  align-items: center;
  gap: 7px;
  height: 18px;
  padding-left: 3px;
  font-size: 12.5px;
  white-space: nowrap;
  overflow: hidden;
}
.simple-status b { font-weight: 600; flex: none; }
.simple-status.s-connected b { color: var(--s-green); }
.simple-status.s-connecting b,
.simple-status.s-connecting .simple-spin { color: var(--s-amber); }
.simple-status.s-error b { color: var(--s-red); }
.simple-status.s-stopped b { color: var(--s-muted); }
.simple-sep { color: var(--s-faint); flex: none; }
.simple-status.s-warning { color: var(--s-amber); }
.simple-warn-icon { flex: none; font-size: 12px; }
.simple-detail.simple-warn-text { color: var(--s-amber); font-weight: 600; }
.simple-detail { color: var(--s-muted); overflow: hidden; text-overflow: ellipsis; min-width: 0; font-variant-numeric: tabular-nums; }

.simple-dot { width: 8px; height: 8px; border-radius: 50%; flex: none; background: var(--s-grey-dot); }
.s-connected .simple-dot { background: var(--s-green-dot); box-shadow: 0 0 0 3px color-mix(in srgb, var(--s-green-dot) 22%, transparent); }
.s-error .simple-dot { background: var(--s-red-dot); box-shadow: 0 0 0 3px color-mix(in srgb, var(--s-red-dot) 22%, transparent); }
.simple-spin {
  width: 11px;
  height: 11px;
  flex: none;
  border: 2px solid currentColor;
  border-right-color: transparent;
  border-radius: 50%;
  animation: simple-spin 1s linear infinite;
}
@keyframes simple-spin { to { transform: rotate(360deg); } }

.simple-addr {
  height: 17px;
  line-height: 17px;
  margin-top: 2px;
  padding-left: 18px;
  color: var(--s-muted);
  font-family: "Cascadia Mono", "Cascadia Code", Consolas, monospace;
  font-size: 11.5px;
  letter-spacing: -0.1px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.simple-dontask {
  display: flex;
  align-items: center;
  gap: 6px;
  width: fit-content;
  max-width: 100%;
  height: 17px;
  margin-top: 2px;
  padding-left: 3px;
  color: var(--s-muted);
  font-size: 12px;
  white-space: nowrap;
  cursor: pointer;
  user-select: none;
}
.simple-dontask input {
  width: 13px;
  height: 13px;
  margin: 0;
  flex: none;
  accent-color: var(--s-accent);
  cursor: pointer;
}
.simple-empty { padding-left: 3px; color: var(--s-muted); font-size: 12.5px; line-height: 18px; }
.simple-arrow { color: var(--s-faint); margin: 0 3px; font-family: "Segoe UI", system-ui, sans-serif; }

.simple-acts { display: flex; gap: 6px; }
.simple-btn,
.simple-ibtn {
  height: 32px;
  border-radius: 6px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  font-family: inherit;
  cursor: pointer;
  color: var(--s-text);
  background: var(--s-ctrl);
  border: 1px solid var(--s-ctrl-b);
  border-bottom-color: var(--s-ctrl-bb);
  transition: background-color 0.12s ease;
}
.simple-btn {
  flex: 1;
  min-width: 0;
  gap: 7px;
  padding: 0 12px;
  font-size: 13px;
  font-weight: 600;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.simple-btn .bi { font-size: 14px; }
.simple-btn--secondary { flex: none; padding: 0 16px; }
.simple-btn--primary { background: var(--s-accent); border-color: var(--s-accent-h); color: var(--s-on-accent); }
.simple-btn--primary:hover:not(:disabled) { background: var(--s-accent-h); }
.simple-btn--outline:hover:not(:disabled),
.simple-ibtn:hover:not(:disabled) { background: var(--s-ctrl-h); }
.simple-ibtn { width: 32px; flex: none; padding: 0; font-size: 15px; }
.simple-btn:disabled,
.simple-ibtn:disabled { opacity: 0.42; cursor: default; }
.simple-btn:focus-visible,
.simple-ibtn:focus-visible {
  outline: 2px solid var(--s-accent);
  outline-offset: 1px;
}

/* панель списка: по высоте содержимого, но не выше окна (8px сверху и снизу) */
.simple-backdrop { position: absolute; inset: 0; z-index: 10; }
.simple-panel {
  position: absolute;
  left: 141px;
  right: 8px;
  top: 8px;
  z-index: 11;
  display: flex;
  flex-direction: column;
  max-height: calc(100% - 16px);
  border-radius: 8px;
  background: var(--s-ctrl);
  box-shadow: 0 0 0 1px var(--s-ctrl-b), 0 8px 24px var(--s-panel-shadow);
  overflow: hidden;
}
.simple-list {
  flex: 0 1 auto;
  min-width: 0;
  min-height: 0;
  margin: 0;
  padding: 4px 4px 0;
  list-style: none;
  overflow-y: auto;
  overscroll-behavior: contain;
  scroll-padding: 4px 0 34px;
  outline: none;
  scrollbar-width: thin;
}
.simple-option {
  position: relative;
  display: flex;
  align-items: center;
  gap: 8px;
  height: 28px;
  padding: 0 10px 0 12px;
  border-radius: 5px;
  font-size: 13px;
  cursor: pointer;
  user-select: none;
}
.simple-option.is-active { background: var(--s-ctrl-h); }
.simple-option.is-selected { background: color-mix(in srgb, var(--s-accent) 11%, transparent); }
.simple-option.is-selected.is-active { background: color-mix(in srgb, var(--s-accent) 17%, transparent); }
.simple-option.is-selected::before {
  content: '';
  position: absolute;
  left: 2px;
  top: 7px;
  bottom: 7px;
  width: 3px;
  border-radius: 2px;
  background: var(--s-accent);
}
.simple-list:focus-visible .simple-option.is-active { box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--s-accent) 55%, transparent); }
.simple-option__dot { width: 8px; height: 8px; border-radius: 50%; flex: none; background: var(--s-grey-dot); }
.simple-option__dot.d-connected { background: var(--s-green-dot); box-shadow: 0 0 0 2px color-mix(in srgb, var(--s-green-dot) 22%, transparent); }
.simple-option__dot.d-connecting { background: var(--s-amber-dot); box-shadow: 0 0 0 2px color-mix(in srgb, var(--s-amber-dot) 22%, transparent); }
.simple-option__dot.d-error { background: var(--s-red-dot); box-shadow: 0 0 0 2px color-mix(in srgb, var(--s-red-dot) 22%, transparent); }
.simple-option__name { flex: 1; min-width: 0; font-weight: 600; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.simple-option__tag {
  flex: none;
  height: 18px;
  padding: 0 7px;
  border-radius: 9px;
  background: color-mix(in srgb, var(--s-muted) 14%, transparent);
  color: var(--s-muted);
  font-size: 11px;
  line-height: 18px;
}
.simple-option__port { flex: none; color: var(--s-muted); font-variant-numeric: tabular-nums; }
.simple-list__empty { padding: 6px 12px; color: var(--s-muted); font-size: 12.5px; }
.simple-option--manage {
  position: sticky;
  bottom: 0;
  height: 32px;
  margin: 4px -4px 0;
  padding: 0 14px;
  border-radius: 0;
  border-top: 1px solid var(--s-ctrl-b);
  background: var(--s-ctrl);
  color: var(--s-accent);
}
.simple-option--manage.is-active { background: var(--s-ctrl-h); }
.simple-option--manage .bi { font-size: 14px; }
.visually-hidden {
  position: absolute;
  width: 1px;
  height: 1px;
  overflow: hidden;
  clip: rect(0 0 0 0);
  white-space: nowrap;
}
.simple-pop-enter-active,
.simple-pop-leave-active { transition: opacity 0.12s ease, transform 0.12s ease; }
.simple-pop-enter-from,
.simple-pop-leave-to { opacity: 0; transform: translateY(-4px); }

@media (prefers-reduced-motion: reduce) {
  .simple-pop-enter-active,
  .simple-pop-leave-active { transition: none; }
  .simple-spin { animation: none; border-right-color: currentColor; opacity: 0.7; }
}
</style>
