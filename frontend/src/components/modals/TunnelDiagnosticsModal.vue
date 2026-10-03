<script setup>
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'

const props = defineProps({
  show: {
    type: Boolean,
    required: true,
  },
  tunnel: {
    type: Object,
    default: null,
  },
  report: {
    type: Object,
    default: null,
  },
  loading: {
    type: Boolean,
    default: false,
  },
  error: {
    type: String,
    default: '',
  },
})

const emit = defineEmits(['close', 'retry'])
const { t } = useI18n()
const openDetails = ref({})

watch(() => props.report, () => {
  openDetails.value = {}
})

const name = computed(() => props.tunnel?.name || '')
const checks = computed(() => (Array.isArray(props.report?.checks) ? props.report.checks : []))

function sentence(check) {
  const key = `app.tunnels.diagnostics.checks.${check?.code || ''}`
  const translated = check?.code ? t(key, check?.params || {}) : ''
  if (translated && translated !== key) return translated
  if (check?.status === 'skipped') return t('app.tunnels.diagnostics.skipped')
  return check?.detail || check?.code || ''
}

function titleOf(check) {
  const key = `app.tunnels.diagnostics.titles.${check?.id || ''}`
  const translated = t(key)
  return translated === key ? (check?.id || '') : translated
}

const summary = computed(() => {
  const code = props.report?.code
  if (!code) return ''
  const key = `app.tunnels.diagnostics.${code}`
  const translated = t(key, props.report?.params || {})
  return translated === key ? '' : translated
})

const tone = computed(() => {
  if (props.report?.code === 'summary_password') return 'neutral'
  if (props.report?.status === 'error') return 'bad'
  if (props.report?.status === 'ok') return 'ok'
  return 'neutral'
})

function iconClass(status) {
  if (status === 'ok') return 'bi-check-circle-fill'
  if (status === 'error') return 'bi-x-circle-fill'
  return 'bi-dash-circle'
}

function toggleDetail(id) {
  openDetails.value = { ...openDetails.value, [id]: !openDetails.value[id] }
}
</script>

<template>
  <n-modal
    :show="show"
    preset="card"
    class="app-modal tunnel-diagnostics-dialog"
    :title="name ? $t('app.tunnels.diagnostics.title', { name }) : $t('app.tunnels.actions.diagnostics')"
    :style="{ width: 'min(480px, calc(100vw - 32px))' }"
    :mask-closable="true"
    :segmented="{ content: true, footer: 'soft' }"
    @update:show="(visible) => { if (!visible) emit('close') }"
  >
    <div class="diag-body" :aria-busy="loading ? 'true' : 'false'">
      <p v-if="loading" class="diag-running" role="status">
        <n-spin size="small" />
        {{ $t('app.tunnels.diagnostics.running') }}
      </p>
      <p v-else-if="error" class="diag-failed" role="alert">{{ error }}</p>
      <template v-else-if="report">
        <p class="diag-summary" :class="`diag-summary--${tone}`" role="status">{{ summary }}</p>
        <ul class="diag-list">
          <li v-for="check in checks" :key="check.id" class="diag-row" :class="`diag-row--${check.status}`">
            <i class="bi diag-icon" :class="iconClass(check.status)" aria-hidden="true" />
            <div class="diag-copy">
              <div class="diag-title">{{ titleOf(check) }}</div>
              <p class="diag-sentence">{{ sentence(check) }}</p>
              <n-button
                v-if="check.detail"
                text
                size="small"
                :aria-expanded="openDetails[check.id] ? 'true' : 'false'"
                @click="toggleDetail(check.id)"
              >
                {{ openDetails[check.id] ? $t('app.tunnels.diagnostics.hideDetails') : $t('app.tunnels.diagnostics.details') }}
              </n-button>
              <p v-if="check.detail && openDetails[check.id]" class="diag-detail">{{ check.detail }}</p>
            </div>
          </li>
        </ul>
        <p class="diag-hint">{{ $t('app.tunnels.diagnostics.hint') }}</p>
      </template>
    </div>
    <template #footer>
      <n-space justify="end" :size="8">
        <n-button secondary :loading="loading" :disabled="loading" @click="emit('retry')">
          {{ $t('app.tunnels.diagnostics.retry') }}
        </n-button>
      </n-space>
    </template>
  </n-modal>
</template>

<style scoped>
.diag-body {
  max-height: min(640px, calc(100vh - 140px));
  overflow: auto;
}

.diag-running,
.diag-failed,
.diag-summary,
.diag-sentence,
.diag-detail,
.diag-hint {
  margin: 0;
}

.diag-running {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--lt-muted);
  font-size: 14px;
}

.diag-failed {
  color: var(--lt-danger-ink);
  font-size: 14px;
  line-height: 1.45;
}

.diag-summary {
  padding: 10px 12px;
  border: 1px solid var(--lt-border);
  border-radius: var(--lt-radius-md);
  background: var(--lt-neutral-bg);
  color: var(--lt-ink);
  font-size: 14px;
  line-height: 1.45;
}

.diag-summary--ok {
  background: var(--lt-positive-bg);
  border-color: var(--lt-positive-border);
  color: var(--lt-positive-ink);
}

.diag-summary--bad {
  background: var(--lt-danger-bg);
  border-color: var(--lt-danger-border);
  color: var(--lt-danger-ink);
}

.diag-list {
  list-style: none;
  margin: 12px 0 0;
  padding: 0;
}

.diag-row {
  display: flex;
  gap: 10px;
  padding: 10px 0;
  border-top: 1px solid var(--lt-border);
}

.diag-icon {
  flex: none;
  margin-top: 1px;
  font-size: 15px;
}

.diag-row--ok .diag-icon {
  color: var(--lt-positive-ink);
}

.diag-row--error .diag-icon {
  color: var(--lt-danger-ink);
}

.diag-row--skipped .diag-icon {
  color: var(--lt-muted);
}

.diag-copy {
  min-width: 0;
}

.diag-title {
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--lt-muted);
}

.diag-sentence {
  margin-top: 2px;
  font-size: 14px;
  line-height: 1.45;
  color: var(--lt-ink);
}

.diag-more {
  margin-top: 4px;
  padding: 0;
  border: 0;
  background: none;
  color: var(--lt-muted);
  font: inherit;
  font-size: 12px;
  text-decoration: underline;
  text-underline-offset: 2px;
  cursor: pointer;
}

.diag-more:focus-visible {
  outline: 2px solid var(--lt-focus);
  outline-offset: 2px;
}

.diag-detail {
  margin-top: 6px;
  padding: 8px 10px;
  border-radius: var(--lt-radius-md);
  background: var(--lt-inline-bg);
  border: 1px solid var(--lt-inline-border);
  color: var(--lt-muted);
  font-size: 12px;
  line-height: 1.4;
  word-break: break-word;
}

.diag-hint {
  margin-top: 12px;
  color: var(--lt-muted);
  font-size: 12px;
  line-height: 1.4;
}
</style>
