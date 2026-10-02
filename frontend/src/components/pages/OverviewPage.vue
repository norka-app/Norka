<script setup>
import { computed, h, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { NButton, NTag } from 'naive-ui'

const props = defineProps({
  totalTunnels: {
    type: Number,
    required: true
  },
  runningTunnels: {
    type: Array,
    required: true
  },
  stoppedTunnels: {
    type: Array,
    required: true
  },
  autoStartCount: {
    type: Number,
    required: true
  },
  showOverviewActive: {
    type: Boolean,
    required: true
  },
  showOverviewActivity: {
    type: Boolean,
    required: true
  },
  logs: {
    type: Array,
    required: true
  },
  getTunnelJumperLabel: {
    type: Function,
    required: true
  }
})

const emit = defineEmits(['toggle-overview-active', 'toggle-overview-activity', 'toggle-tunnel'])
const { t } = useI18n()

function logTagType(level) {
  if (level === 'error') return 'error'
  if (level === 'warn') return 'warning'
  if (level === 'info') return 'info'
  return 'default'
}

const activeColumns = computed(() => [
  {
    title: t('app.overview.table.name'),
    key: 'name',
    ellipsis: { tooltip: true },
    render: (row) => (row.status === 'reconnecting'
      ? [row.name, ' ', h(NTag, { size: 'small', type: 'warning' }, { default: () => t('app.tunnels.status.reconnecting') })]
      : row.name),
  },
  {
    title: t('app.overview.table.route'),
    key: 'route',
    ellipsis: { tooltip: true },
    render: (row) => getOverviewRoute(row),
  },
  {
    title: t('app.overview.table.jumper'),
    key: 'jumper',
    ellipsis: { tooltip: true },
    render: (row) => props.getTunnelJumperLabel(row),
  },
  {
    title: t('app.overview.table.action'),
    key: 'action',
    align: 'right',
    width: 110,
    render: (row) => h(NButton, {
      size: 'small',
      quaternary: true,
      type: 'error',
      title: t('app.overview.actions.stopTunnel'),
      onClick: () => emit('toggle-tunnel', row),
    }, { icon: () => h('i', { class: 'bi bi-pause' }) }),
  },
])

const ACTIVE_PAGE_SIZE = 5
const activeTunnelsPage = ref(1)

const activeTunnelsTotalPages = computed(() => {
  return Math.max(1, Math.ceil(props.runningTunnels.length / ACTIVE_PAGE_SIZE))
})

const pagedRunningTunnels = computed(() => {
  const start = (activeTunnelsPage.value - 1) * ACTIVE_PAGE_SIZE
  return props.runningTunnels.slice(start, start + ACTIVE_PAGE_SIZE)
})

const showActiveTunnelsPagination = computed(() => props.runningTunnels.length > ACTIVE_PAGE_SIZE)

watch(
  () => props.runningTunnels.length,
  () => {
    if (!showActiveTunnelsPagination.value) {
      activeTunnelsPage.value = 1
      return
    }
    if (activeTunnelsPage.value > activeTunnelsTotalPages.value) {
      activeTunnelsPage.value = activeTunnelsTotalPages.value
    }
  }
)

function goPrevActiveTunnelsPage() {
  if (activeTunnelsPage.value <= 1) return
  activeTunnelsPage.value -= 1
}

function goNextActiveTunnelsPage() {
  if (activeTunnelsPage.value >= activeTunnelsTotalPages.value) return
  activeTunnelsPage.value += 1
}

function getOverviewRoute(tunnel) {
  return `${tunnel.localHost}:${tunnel.localPort} -> ${tunnel.remoteHost}:${tunnel.remotePort}`
}
</script>

<template>
  <n-space vertical :size="16">
    <n-grid :cols="4" :x-gap="12" :y-gap="12" responsive="screen" item-responsive>
      <n-gi span="2 m:1">
        <n-card size="small">
          <n-statistic :label="$t('app.overview.totalTunnels')" :value="totalTunnels" />
        </n-card>
      </n-gi>
      <n-gi span="2 m:1">
        <n-card size="small">
          <n-statistic :label="$t('app.overview.running')" :value="runningTunnels.length" />
        </n-card>
      </n-gi>
      <n-gi span="2 m:1">
        <n-card size="small">
          <n-statistic :label="$t('app.overview.stopped')" :value="stoppedTunnels.length" />
        </n-card>
      </n-gi>
      <n-gi span="2 m:1">
        <n-card size="small">
          <n-statistic :label="$t('app.overview.autoStart')" :value="autoStartCount" />
        </n-card>
      </n-gi>
    </n-grid>

    <n-grid :cols="12" :x-gap="12" :y-gap="12" responsive="screen" item-responsive>
      <n-gi span="12 l:7">
        <n-card size="small" :title="$t('app.overview.activeTunnels')">
          <template #header-extra>
            <n-button
              quaternary
              size="small"
              :aria-label="showOverviewActive ? $t('app.overview.collapsePanel') : $t('app.overview.expandPanel')"
              :aria-expanded="showOverviewActive"
              @click="$emit('toggle-overview-active')"
            >
              <template #icon>
                <i class="bi" :class="showOverviewActive ? 'bi-chevron-up' : 'bi-chevron-down'" />
              </template>
            </n-button>
          </template>
          <template v-if="showOverviewActive">
            <n-data-table
              v-if="runningTunnels.length > 0"
              size="small"
              :columns="activeColumns"
              :data="pagedRunningTunnels"
              :bordered="false"
              :row-key="(row) => row.id"
            />
            <n-empty v-else :description="$t('app.overview.noRunningTunnels')" />
            <n-space v-if="showActiveTunnelsPagination" justify="end" align="center" class="overview-pagination">
              <n-button size="small" :disabled="activeTunnelsPage === 1" :aria-label="$t('app.overview.pagination.prev')" @click="goPrevActiveTunnelsPage">
                <template #icon><i class="bi bi-chevron-left" /></template>
              </n-button>
              <span>
                {{ $t('app.overview.pagination.pageInfo', { current: activeTunnelsPage, total: activeTunnelsTotalPages }) }}
              </span>
              <n-button size="small" :disabled="activeTunnelsPage === activeTunnelsTotalPages" :aria-label="$t('app.overview.pagination.next')" @click="goNextActiveTunnelsPage">
                <template #icon><i class="bi bi-chevron-right" /></template>
              </n-button>
            </n-space>
          </template>
        </n-card>
      </n-gi>
      <n-gi span="12 l:5">
        <n-card size="small" :title="$t('app.overview.recentActivity')">
          <template #header-extra>
            <n-button
              quaternary
              size="small"
              :aria-label="showOverviewActivity ? $t('app.overview.collapsePanel') : $t('app.overview.expandPanel')"
              :aria-expanded="showOverviewActivity"
              @click="$emit('toggle-overview-activity')"
            >
              <template #icon>
                <i class="bi" :class="showOverviewActivity ? 'bi-chevron-up' : 'bi-chevron-down'" />
              </template>
            </n-button>
          </template>
          <n-list v-if="showOverviewActivity" bordered>
            <n-list-item v-for="entry in logs.slice(0, 6)" :key="entry.id">
              <n-space align="center" :size="8">
                <n-text depth="3">{{ entry.time }}</n-text>
                <n-tag size="small" :type="logTagType(entry.level)">{{ entry.level.toUpperCase() }}</n-tag>
                <span>{{ entry.message }}</span>
              </n-space>
            </n-list-item>
          </n-list>
        </n-card>
      </n-gi>
    </n-grid>
  </n-space>
</template>
