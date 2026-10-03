<script setup>
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { findPortConflicts } from '../../utils/port-conflicts'
import { bestFuzzyScore } from '../../utils/fuzzy'
import { matchesKey } from '../../utils/keyboard'
import { plainInputProps } from '../../theme/form-layout'
import { Search } from '../../icons'

const props = defineProps({
  open: { type: Boolean, default: false },
  tunnels: { type: Array, default: () => [] },
  profiles: { type: Array, default: () => [] },
  activeProfileId: { type: Number, default: 0 }
})

const emit = defineEmits(['close', 'toggle-tunnel', 'activate-profile'])

const { t } = useI18n()
const searchInputProps = {
  ...plainInputProps,
  autocomplete: 'off',
  role: 'combobox',
  'aria-autocomplete': 'list',
  'aria-controls': 'quick-search-list',
}
const query = ref('')
const activeIndex = ref(0)
const inputRef = ref(null)
const listRef = ref(null)

function displayHost(host) {
  const value = String(host || '').trim()
  if (!value || value === '127.0.0.1' || value === '0.0.0.0' || value === '::1' || value === 'localhost') return 'localhost'
  return value
}

function dotOf(status) {
  if (status === 'running') return 'connected'
  if (status === 'busy' || status === 'reconnecting') return 'connecting'
  if (status === 'error') return 'error'
  return 'stopped'
}

const results = computed(() => {
  const q = query.value
  const profiles = props.profiles.map((profile) => {
    const score = bestFuzzyScore(q, [profile.emoji || '', profile.name || ''])
    return {
      kind: 'profile',
      id: profile.id,
      score,
      title: profile.name,
      emoji: profile.emoji || '',
      color: profile.color || '',
      active: profile.id === props.activeProfileId,
      subtitle: t('app.quickSearch.profileMeta', { count: (profile.tunnelIds || []).length }),
      portConflict: ''
    }
  }).filter((item) => item.score > 0)

  const tunnels = props.tunnels.map((tunnel) => {
    const address = `${displayHost(tunnel.localHost)}:${tunnel.localPort}`
    const remote = tunnel.mode === 'dynamic' ? 'SOCKS5' : `${tunnel.remoteHost || ''}:${tunnel.remotePort || ''}`
    const score = bestFuzzyScore(q, [tunnel.name, address, remote, String(tunnel.localPort || '')])
    const conflicts = findPortConflicts(tunnel, props.tunnels)
    const holder = conflicts.map((item) => item.name).filter(Boolean).join('», «')
    return {
      kind: 'tunnel',
      id: tunnel.id,
      score,
      title: tunnel.name,
      emoji: '',
      color: '',
      active: false,
      status: tunnel.status,
      dot: dotOf(tunnel.status),
      subtitle: tunnel.mode === 'remote' ? `${remote} → ${address}` : `${address} → ${remote}`,
      portConflict: holder ? t('app.simple.portBusy', { port: tunnel.localPort, names: holder }) : '',
      tunnel
    }
  }).filter((item) => item.score > 0)

  const ranked = [...profiles, ...tunnels].sort((a, b) => b.score - a.score || a.title.localeCompare(b.title, 'ru'))
  return q.trim() ? ranked : [...profiles, ...tunnels]
})

const selected = computed(() => results.value[activeIndex.value] || null)

const actionLabel = computed(() => {
  const item = selected.value
  if (!item) return ''
  if (item.kind === 'profile') return t('app.quickSearch.activateProfile')
  if (item.portConflict && item.status !== 'running' && item.status !== 'busy' && item.status !== 'reconnecting') {
    return t('app.quickSearch.switchPort')
  }
  if (item.status === 'running' || item.status === 'reconnecting' || item.status === 'busy') return t('app.quickSearch.disconnect')
  return t('app.quickSearch.connect')
})

watch(() => props.open, async (open) => {
  if (!open) return
  query.value = ''
  activeIndex.value = 0
  await nextTick()
  inputRef.value?.focus()
})

watch(results, () => {
  if (activeIndex.value >= results.value.length) activeIndex.value = 0
})

watch(activeIndex, async () => {
  await nextTick()
  const node = listRef.value?.querySelector('[data-active="true"]')
  node?.scrollIntoView({ block: 'nearest' })
})

function close() {
  emit('close')
}

function runSelected() {
  const item = selected.value
  if (!item) return
  if (item.kind === 'profile') {
    emit('activate-profile', item.id)
    return
  }
  emit('toggle-tunnel', item.tunnel)
}

function onKeydown(event) {
  if (matchesKey(event, 'escape')) {
    event.preventDefault()
    close()
    return
  }
  if (matchesKey(event, 'arrowdown')) {
    event.preventDefault()
    if (results.value.length) activeIndex.value = (activeIndex.value + 1) % results.value.length
    return
  }
  if (matchesKey(event, 'arrowup')) {
    event.preventDefault()
    if (results.value.length) activeIndex.value = (activeIndex.value - 1 + results.value.length) % results.value.length
    return
  }
  if (matchesKey(event, 'enter')) {
    event.preventDefault()
    runSelected()
  }
}
</script>

<template>
  <div v-if="open" class="qs-root" @keydown="onKeydown">
    <button type="button" class="qs-backdrop" :aria-label="t('app.common.close')" @click="close" />
    <div class="qs-card" role="dialog" aria-modal="true" :aria-label="t('app.quickSearch.title')">
      <div class="qs-search">
        <n-input
          ref="inputRef"
          v-model:value="query"
          :placeholder="t('app.quickSearch.placeholder')"
          :input-props="{
            ...searchInputProps,
            'aria-label': t('app.quickSearch.placeholder'),
            'aria-expanded': results.length ? 'true' : 'false',
            'aria-activedescendant': selected ? `qs-opt-${activeIndex}` : undefined,
          }"
          @keydown="onKeydown"
        >
          <template #prefix>
            <n-icon aria-hidden="true" :component="Search" />
          </template>
          <template #suffix>
            <n-tag size="small" :bordered="true">esc</n-tag>
          </template>
        </n-input>
      </div>
      <ul v-if="results.length" id="quick-search-list" ref="listRef" class="qs-list" role="listbox">
        <li
          v-for="(item, index) in results"
          :id="`qs-opt-${index}`"
          :key="`${item.kind}-${item.id}`"
          role="option"
          :aria-selected="index === activeIndex ? 'true' : 'false'"
          :data-active="index === activeIndex ? 'true' : 'false'"
          class="qs-item"
          :class="{ 'is-active': index === activeIndex }"
          @mousemove="activeIndex = index"
          @click="runSelected"
        >
          <span v-if="item.kind === 'profile'" class="qs-emoji" :style="item.color ? { boxShadow: `inset 0 0 0 2px ${item.color}` } : undefined">{{ item.emoji || '•' }}</span>
          <span v-else class="qs-dot" :class="`d-${item.dot}`" aria-hidden="true" />
          <span class="qs-copy">
            <span class="qs-title">
              {{ item.title }}
              <span v-if="item.active" class="qs-badge">{{ t('app.profiles.active') }}</span>
              <span v-if="item.portConflict" class="qs-tag">{{ t('app.simple.portBusyTag') }}</span>
            </span>
            <span class="qs-sub">{{ item.portConflict || item.subtitle }}</span>
          </span>
          <span class="qs-kind">{{ item.kind === 'profile' ? t('app.quickSearch.kindProfile') : t('app.quickSearch.kindTunnel') }}</span>
        </li>
      </ul>
      <div v-else class="qs-empty">{{ t('app.quickSearch.empty') }}</div>
      <div class="qs-foot">
        <span>{{ t('app.quickSearch.hint') }}</span>
        <span v-if="actionLabel" class="qs-action">Enter · {{ actionLabel }}</span>
      </div>
    </div>
  </div>
</template>

<style scoped>
.qs-root {
  position: fixed;
  inset: 0;
  z-index: 4000;
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding: 12vh 16px 16px;
}
.qs-backdrop {
  position: absolute;
  inset: 0;
  border: 0;
  background: var(--lt-overlay-bg);
  cursor: default;
}
.qs-card {
  position: relative;
  width: min(560px, 100%);
  overflow: hidden;
  border: 1px solid var(--lt-border);
  border-radius: 14px;
  background: var(--lt-surface);
  color: var(--lt-ink);
  box-shadow: var(--lt-dialog-shadow);
}
.qs-search {
  padding: var(--kit-field-gap);
  border-bottom: 1px solid var(--lt-border);
}
.qs-list {
  max-height: min(360px, 50vh);
  margin: 0;
  padding: 6px;
  overflow: auto;
  list-style: none;
}
.qs-item {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 48px;
  padding: 6px 8px;
  border-radius: 8px;
  cursor: pointer;
}
.qs-item.is-active { background: var(--lt-accent-active); }
.qs-emoji, .qs-dot { flex: none; }
.qs-emoji {
  width: 28px;
  height: 28px;
  display: grid;
  place-items: center;
  border-radius: 8px;
  background: var(--lt-surface-soft);
  font-size: 16px;
}
.qs-dot {
  width: 10px;
  height: 10px;
  margin: 0 9px;
  border-radius: 50%;
  background: var(--lt-border-strong);
}
.qs-dot.d-connected { background: #22c55e; box-shadow: 0 0 0 3px color-mix(in srgb, #22c55e 22%, transparent); }
.qs-dot.d-connecting { background: #f5a524; box-shadow: 0 0 0 3px color-mix(in srgb, #f5a524 22%, transparent); }
.qs-dot.d-error { background: #ef4444; box-shadow: 0 0 0 3px color-mix(in srgb, #ef4444 22%, transparent); }
.qs-copy { flex: 1; min-width: 0; display: flex; flex-direction: column; }
.qs-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-weight: 650;
  white-space: nowrap;
}
.qs-sub {
  color: var(--lt-muted);
  font-size: 12px;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.qs-badge, .qs-tag {
  padding: 0 6px;
  border-radius: 999px;
  font-size: 11px;
  font-weight: 600;
  line-height: 18px;
}
.qs-badge { color: var(--lt-brand); background: color-mix(in srgb, var(--lt-brand) 16%, transparent); }
.qs-tag { color: var(--lt-warning-ink, #92400e); background: var(--lt-warning-bg, #fffbeb); }
:root[data-theme="dark"] .qs-tag { color: #fbbf24; background: color-mix(in srgb, #fbbf24 16%, transparent); }
.qs-kind { flex: none; color: var(--lt-muted); font-size: 11px; }
.qs-empty { padding: 22px 16px; color: var(--lt-muted); text-align: center; }
.qs-foot {
  display: flex;
  justify-content: space-between;
  gap: 12px;
  padding: 8px 14px;
  border-top: 1px solid var(--lt-border);
  color: var(--lt-muted);
  font-size: 12px;
}
.qs-action { color: var(--lt-ink); }
</style>
