<script setup>
// Dynamic Island: одна скруглённая капсула. Взгляд — статус, клик — список.
// Создавать, импортировать и открывать настройки отсюда нельзя.
import { computed, onBeforeUnmount, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import NorkaIcon from '../norka/NorkaIcon.vue'
import {
  islandAddress,
  islandDotCount,
  islandEyes,
  islandFocusTunnel,
  islandPhase,
  islandSnapshot,
  islandSwitchOn,
  tunnelCountForm,
} from './island-state'
import { islandSize } from './island-metrics'

const props = defineProps({
  tunnels: { type: Array, default: () => [] },
  theme: { type: String, default: 'dark' },
  expanded: { type: Boolean, default: false },
  conflict: { type: Object, default: null },
  eyes: { type: String, default: '' },
  profilesEnabled: { type: Boolean, default: false },
  activeProfile: { type: Object, default: null },
  getRunningSince: { type: Function, default: () => undefined },
  fill: { type: Boolean, default: false },
  motionMs: { type: Number, default: 280 },
})

const emit = defineEmits(['expand', 'collapse', 'toggle', 'manage', 'connect-instead'])

const { t, locale } = useI18n()
const now = ref(Date.now())
const clock = setInterval(() => { now.value = Date.now() }, 1000)
onBeforeUnmount(() => clearInterval(clock))

const since = computed(() => ({ error: null, connecting: null }))
const snap = computed(() => islandSnapshot(props.tunnels, since.value))
const phase = computed(() => islandPhase({
  tunnels: props.tunnels,
  expanded: props.expanded,
  conflict: props.conflict,
}))
const box = computed(() => islandSize(phase.value))
const eyeStatus = computed(() => islandEyes(phase.value, props.eyes))
const focus = computed(() => islandFocusTunnel(phase.value, snap.value))
const profileName = computed(() => (
  props.profilesEnabled && props.activeProfile?.name ? String(props.activeProfile.name) : ''
))
const showAddress = computed(() => phase.value === 'one' && !profileName.value)
const dotCount = computed(() => islandDotCount(snap.value.running.length))

const countKey = computed(() => {
  const form = tunnelCountForm(snap.value.running.length, locale.value)
  if (form === 'one') return 'app.island.tunnelsOne'
  if (form === 'few') return 'app.island.tunnelsFew'
  return 'app.island.tunnelsMany'
})

function uptime(tunnel) {
  const sinceMs = props.getRunningSince?.(tunnel?.id)
  if (!sinceMs) return ''
  const total = Math.max(0, Math.floor((now.value - sinceMs) / 1000))
  const pad = (value) => String(value).padStart(2, '0')
  return `${pad(Math.floor(total / 3600))}:${pad(Math.floor((total % 3600) / 60))}:${pad(total % 60)}`
}

const focusTime = computed(() => (focus.value ? uptime(focus.value) : ''))
const conflictNames = computed(() => {
  const names = (props.conflict?.conflicts || []).map((item) => item.name).filter(Boolean)
  return names.join(', ')
})

function dotClass(status) {
  if (status === 'running') return 'is-on'
  if (status === 'busy' || status === 'reconnecting') return 'is-busy'
  if (status === 'error') return 'is-error'
  return 'is-off'
}

function onGlance() {
  if (phase.value === 'conflict' || phase.value === 'expanded') return
  emit('expand')
}
</script>

<template>
  <section
    class="island"
    :class="[`island--${phase}`, `island--${theme}`, { 'island--fill': fill }]"
    :style="fill ? { borderRadius: `${box.r}px`, '--island-ms': `${motionMs}ms` } : {
      width: `${box.w}px`,
      height: `${box.h}px`,
      borderRadius: `${box.r}px`,
      '--island-ms': `${motionMs}ms`,
    }"
    :data-phase="phase"
    :aria-label="t('app.island.region')"
  >
    <button
      v-if="phase !== 'expanded' && phase !== 'conflict'"
      type="button"
      class="island__hit"
      :aria-expanded="false"
      :aria-label="t('app.island.expand')"
      @click="onGlance"
    >
      <NorkaIcon
        class="island__mascot"
        :status="eyeStatus"
        variant="A"
        :theme="theme === 'light' ? 'light' : 'dark'"
        :size="phase === 'idle' ? 28 : 26"
        aria-hidden="true"
      />
      <template v-if="phase === 'one' || phase === 'connecting' || phase === 'error'">
        <span v-if="profileName" class="island__chip">{{ profileName }}</span>
        <span class="island__text" :class="{ 'is-alert': phase === 'error' }">
          <template v-if="phase === 'connecting'">{{ t('app.island.connecting', { name: focus?.name || '' }) }}</template>
          <template v-else-if="phase === 'error'">{{ t('app.island.dropped', { name: focus?.name || '' }) }}</template>
          <template v-else>
            {{ focus?.name }}
            <template v-if="showAddress"><span class="island__dim"> · {{ islandAddress(focus) }}</span></template>
          </template>
        </span>
        <span v-if="phase === 'one' && focusTime" class="island__time">{{ focusTime }}</span>
        <span v-else-if="phase === 'connecting'" class="island__spin" aria-hidden="true" />
        <span v-else-if="phase === 'error'" class="island__spin island__spin--alert" aria-hidden="true" />
      </template>
      <template v-else-if="phase === 'multi'">
        <span class="island__text">{{ t(countKey, { count: snap.running.length }) }}</span>
        <span class="island__dots" aria-hidden="true">
          <i v-for="index in dotCount" :key="index" />
        </span>
      </template>
    </button>

    <div v-else-if="phase === 'conflict'" class="island__conflict">
      <NorkaIcon
        class="island__mascot"
        :status="eyeStatus"
        variant="A"
        :theme="theme === 'light' ? 'light' : 'dark'"
        :size="28"
        aria-hidden="true"
      />
      <div class="island__conflict-copy">
        <div class="island__conflict-title">{{ conflict?.tunnel?.name }} :{{ conflict?.tunnel?.localPort }}</div>
        <div class="island__conflict-sub">{{ t((conflict?.conflicts?.length || 0) > 1 ? 'app.island.willStopMany' : 'app.island.willStop', { name: conflictNames, names: conflictNames }) }}</div>
      </div>
      <span class="island__badge">{{ t('app.simple.portBusyTag') }}</span>
      <button type="button" class="island__instead" @click="emit('connect-instead')">
        {{ t('app.island.connectInstead') }}
      </button>
    </div>

    <div v-else class="island__sheet">
      <button type="button" class="island__header" :aria-expanded="true" :aria-label="t('app.island.collapse')" @click="emit('collapse')">
        <NorkaIcon
          class="island__mascot"
          :status="eyeStatus"
          variant="A"
          :theme="theme === 'light' ? 'light' : 'dark'"
          :size="26"
          aria-hidden="true"
        />
        <span>{{ t('app.island.activeOf', { active: snap.active, total: snap.total }) }}</span>
      </button>
      <ul class="island__list">
        <li v-if="!snap.list.length" class="island__empty">{{ t('app.simple.noTunnels') }}</li>
        <li v-for="item in snap.list" :key="item.id" class="island__row">
          <i class="island__status" :class="dotClass(item.status)" aria-hidden="true" />
          <span class="island__name">{{ item.name }}</span>
          <span class="island__addr">{{ islandAddress(item) }}</span>
          <button
            type="button"
            class="island__switch"
            :class="{ 'is-on': islandSwitchOn(item.status) }"
            role="switch"
            :aria-checked="islandSwitchOn(item.status) ? 'true' : 'false'"
            :aria-label="item.name"
            @click="emit('toggle', item)"
          />
        </li>
      </ul>
      <button type="button" class="island__manage" @click="emit('manage')">
        {{ t('app.simple.manageTunnels') }}
      </button>
    </div>
  </section>
</template>

<style scoped>
.island {
  --island-bg: #16181f;
  --island-ink: #f5f5f4;
  --island-dim: #a1a1aa;
  --island-line: rgba(255, 255, 255, 0.08);
  --island-green: #3ddc84;
  --island-amber: #f5c542;
  --island-red: #ff5a5f;
  --island-link: #9eb6ff;
  --island-chip: #2563eb;
  --island-switch: #3b82f6;
  --island-switch-off: #3f3f46;
  position: relative;
  display: block;
  box-sizing: border-box;
  overflow: hidden;
  color: var(--island-ink);
  background: var(--island-bg);
  box-shadow: 0 10px 28px rgba(0, 0, 0, 0.28), inset 0 0 0 1px var(--island-line);
  font-family: var(--norka-font-family, system-ui, sans-serif);
  user-select: none;
  transition:
    width var(--island-ms, 280ms) cubic-bezier(0.22, 1.15, 0.36, 1),
    height var(--island-ms, 280ms) cubic-bezier(0.22, 1.15, 0.36, 1),
    border-radius var(--island-ms, 280ms) ease,
    box-shadow 200ms ease;
}
.island--fill { width: 100%; height: 100%; transition: border-radius 200ms ease, box-shadow 200ms ease; }
.island--light {
  --island-bg: #ffffff;
  --island-ink: #18181b;
  --island-dim: #52525b;
  --island-line: rgba(15, 23, 42, 0.08);
  --island-link: #1d4ed8;
  --island-switch-off: #d4d4d8;
  box-shadow: 0 10px 28px rgba(15, 23, 42, 0.14), inset 0 0 0 1px var(--island-line);
}
.island--error {
  box-shadow: 0 0 0 1px rgba(255, 70, 80, 0.9), 0 0 22px rgba(255, 40, 50, 0.55);
}
.island--connecting { box-shadow: 0 10px 28px rgba(0, 0, 0, 0.28), inset 0 0 0 1px rgba(245, 197, 66, 0.35); }
.island__hit, .island__sheet, .island__conflict {
  width: 100%;
  height: 100%;
  box-sizing: border-box;
}
.island__hit {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 12px 0 6px;
  border: 0;
  background: transparent;
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
}
.island--idle .island__hit { justify-content: center; padding: 0; }
.island__mascot { flex: none; }
.island__text {
  min-width: 0;
  flex: 1;
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
  font-size: 13px;
  font-weight: 600;
  letter-spacing: -0.01em;
}
.island__dim { color: var(--island-dim); font-weight: 500; }
.island__text.is-alert { color: var(--island-red); }
.island__time {
  flex: none;
  color: var(--island-green);
  font-size: 12px;
  font-variant-numeric: tabular-nums;
  font-weight: 650;
}
.island__chip {
  flex: none;
  max-width: 84px;
  overflow: hidden;
  text-overflow: ellipsis;
  padding: 1px 7px;
  border-radius: 7px;
  background: var(--island-chip);
  color: #fff;
  font-size: 11px;
  font-weight: 650;
  line-height: 16px;
}
.island__dots { display: flex; gap: 4px; flex: none; }
.island__dots i, .island__status {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--island-green);
  box-shadow: 0 0 0 2px color-mix(in srgb, var(--island-green) 25%, transparent);
}
.island__status.is-off { background: #71717a; box-shadow: none; }
.island__status.is-busy { background: var(--island-amber); box-shadow: 0 0 0 2px color-mix(in srgb, var(--island-amber) 28%, transparent); }
.island__status.is-error { background: var(--island-red); box-shadow: 0 0 0 2px color-mix(in srgb, var(--island-red) 28%, transparent); }
.island__spin {
  width: 14px;
  height: 14px;
  flex: none;
  border-radius: 50%;
  border: 2px solid color-mix(in srgb, var(--island-amber) 35%, transparent);
  border-top-color: var(--island-amber);
  animation: island-spin 0.8s linear infinite;
}
.island__spin--alert {
  border-color: color-mix(in srgb, var(--island-red) 35%, transparent);
  border-top-color: var(--island-red);
}
.island--connecting .island__hit::after {
  content: "";
  position: absolute;
  inset: 0;
  background: linear-gradient(100deg, transparent 20%, rgba(245, 197, 66, 0.16), transparent 80%);
  transform: translateX(-60%);
  animation: island-shimmer 1.1s ease-in-out infinite;
  pointer-events: none;
}
.island__conflict {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0 8px 0 6px;
}
.island__conflict-copy { min-width: 0; flex: 1; }
.island__conflict-title, .island__conflict-sub {
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}
.island__conflict-title { font-size: 13px; font-weight: 650; }
.island__conflict-sub { font-size: 11px; color: var(--island-dim); }
.island__badge {
  flex: none;
  padding: 2px 7px;
  border-radius: 999px;
  background: #5c4314;
  color: #f6d48a;
  font-size: 11px;
  font-weight: 650;
}
.island--light .island__badge { background: #f6e7c1; color: #7a4b00; }
.island__instead {
  flex: none;
  border: 0;
  border-radius: 999px;
  padding: 5px 10px;
  background: #2f6fed;
  color: #fff;
  font: inherit;
  font-size: 12px;
  font-weight: 650;
  cursor: pointer;
}
.island__instead:hover { background: #1d4ed8; }
.island__sheet { display: flex; flex-direction: column; padding: 8px 8px 6px; }
.island__header {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  border: 0;
  background: transparent;
  color: inherit;
  font: inherit;
  font-size: 13px;
  font-weight: 650;
  text-align: left;
  cursor: pointer;
  padding: 0 4px;
}
.island__list {
  list-style: none;
  margin: 6px 0 0;
  padding: 0 4px;
  flex: 1;
  min-height: 0;
  overflow: auto;
}
.island__row {
  display: grid;
  grid-template-columns: 10px minmax(0, 1fr) auto auto;
  align-items: center;
  gap: 8px;
  min-height: 32px;
}
.island__name, .island__addr {
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
  font-size: 13px;
}
.island__addr { color: var(--island-dim); font-size: 12px; }
.island__empty { color: var(--island-dim); font-size: 13px; padding: 12px 0; }
.island__switch {
  width: 36px;
  height: 20px;
  border: 0;
  border-radius: 999px;
  background: var(--island-switch-off);
  position: relative;
  cursor: pointer;
  padding: 0;
}
.island__switch::after {
  content: "";
  position: absolute;
  top: 2px;
  left: 2px;
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background: #fff;
  transition: transform 160ms ease;
}
.island__switch.is-on { background: var(--island-switch); }
.island__switch.is-on::after { transform: translateX(16px); }
.island__manage {
  margin-top: 4px;
  border: 0;
  background: transparent;
  color: var(--island-link);
  font: inherit;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  padding: 6px 4px 4px;
}
.island__hit:focus-visible, .island__header:focus-visible, .island__manage:focus-visible, .island__instead:focus-visible, .island__switch:focus-visible {
  outline: 2px solid var(--island-link);
  outline-offset: 2px;
}
@keyframes island-spin { to { transform: rotate(360deg); } }
@keyframes island-shimmer { to { transform: translateX(60%); } }
@media (prefers-reduced-motion: reduce) {
  .island, .island__switch::after, .island__spin, .island--connecting .island__hit::after {
    transition: none;
    animation: none;
  }
}
</style>
