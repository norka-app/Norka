<script setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { formatBytes, formatDuration, formatWhen, liveConnectedSeconds, sessionUptimeSeconds } from '../../utils/tunnel-stats'

const props = defineProps({
  show: {
    type: Boolean,
    required: true,
  },
  tunnel: {
    type: Object,
    default: null,
  },
  stat: {
    type: Object,
    default: null,
  },
  now: {
    type: Number,
    default: 0,
  },
})

const emit = defineEmits(['close', 'reset'])
const { t, locale } = useI18n()

const name = computed(() => props.tunnel?.name || '')
const uptime = computed(() => formatDuration(sessionUptimeSeconds(props.stat, props.now), t))
const todayConnected = computed(() => formatDuration(
  liveConnectedSeconds(props.stat?.todayConnectedSec, props.stat, props.now),
  t,
))
const totalConnected = computed(() => formatDuration(
  liveConnectedSeconds(props.stat?.totalConnectedSec, props.stat, props.now),
  t,
))
const errorWhen = computed(() => formatWhen(props.stat?.lastErrorUnix, locale.value))

function row(label, value) {
  return { label, value }
}

const sections = computed(() => [
  {
    title: t('app.tunnels.stats.session'),
    rows: [
      row(t('app.tunnels.stats.uptime'), uptime.value),
      row(t('app.tunnels.stats.sent'), formatBytes(props.stat?.sessionBytesUp)),
      row(t('app.tunnels.stats.received'), formatBytes(props.stat?.sessionBytesDown)),
    ],
  },
  {
    title: t('app.tunnels.stats.today'),
    rows: [
      row(t('app.tunnels.stats.connected'), todayConnected.value),
      row(t('app.tunnels.stats.reconnects'), String(props.stat?.reconnectsToday || 0)),
    ],
  },
  {
    title: t('app.tunnels.stats.total'),
    rows: [
      row(t('app.tunnels.stats.connected'), totalConnected.value),
      row(t('app.tunnels.stats.reconnects'), String(props.stat?.reconnectsTotal || 0)),
      row(t('app.tunnels.stats.sent'), formatBytes(props.stat?.totalBytesUp)),
      row(t('app.tunnels.stats.received'), formatBytes(props.stat?.totalBytesDown)),
    ],
  },
])
</script>

<template>
  <n-modal
    :show="show"
    preset="card"
    class="app-modal tunnel-stats-dialog"
    :title="$t('app.tunnels.stats.title', { name })"
    :style="{ width: 'min(440px, calc(100vw - 32px))' }"
    :mask-closable="true"
    :segmented="{ content: true, footer: 'soft' }"
    @update:show="(visible) => { if (!visible) emit('close') }"
  >
    <div class="tunnel-stats">
      <section v-for="section in sections" :key="section.title" class="tunnel-stats-section">
        <h3>{{ section.title }}</h3>
        <dl>
          <div v-for="item in section.rows" :key="item.label" class="tunnel-stats-row">
            <dt>{{ item.label }}</dt>
            <dd>{{ item.value }}</dd>
          </div>
        </dl>
      </section>
      <section class="tunnel-stats-section">
        <h3>{{ $t('app.tunnels.stats.lastError') }}</h3>
        <p v-if="stat?.lastError" class="tunnel-stats-error">{{ stat.lastError }}</p>
        <p v-else class="tunnel-stats-quiet">{{ $t('app.tunnels.stats.noError') }}</p>
        <p v-if="errorWhen" class="tunnel-stats-when">{{ errorWhen }}</p>
      </section>
    </div>
    <template #footer>
      <n-space justify="end">
        <n-button secondary @click="emit('reset')">
          {{ $t('app.tunnels.stats.reset') }}
        </n-button>
      </n-space>
    </template>
  </n-modal>
</template>

<style scoped>
.tunnel-stats-section + .tunnel-stats-section {
  margin-top: 14px;
  padding-top: 12px;
  border-top: 1px solid var(--lt-border);
}

.tunnel-stats-section h3 {
  margin: 0 0 8px;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--lt-muted);
}

.tunnel-stats-section dl {
  margin: 0;
}

.tunnel-stats-row {
  display: flex;
  justify-content: space-between;
  align-items: baseline;
  gap: 16px;
  padding: 3px 0;
  font-size: 14px;
}

.tunnel-stats-row dt {
  color: var(--lt-muted);
  font-weight: 400;
}

.tunnel-stats-row dd {
  margin: 0;
  font-variant-numeric: tabular-nums;
  text-align: right;
}

.tunnel-stats-error,
.tunnel-stats-quiet,
.tunnel-stats-when {
  margin: 0;
}

.tunnel-stats-error {
  color: var(--lt-danger-ink);
  font-size: 13px;
  line-height: 1.4;
  word-break: break-word;
}

.tunnel-stats-quiet,
.tunnel-stats-when {
  color: var(--lt-muted);
  font-size: 13px;
}

.tunnel-stats-when {
  margin-top: 4px;
  font-variant-numeric: tabular-nums;
}
</style>
