<script setup>
import { computed, h, onBeforeUnmount, onMounted, ref, useId } from 'vue'
import { useI18n } from 'vue-i18n'
import { NIcon } from 'naive-ui'
import { ArrowDown, ArrowUp, ChevronBack, ChevronForward } from '../../icons'
import NorkaStatusLogo from '../norka/NorkaStatusLogo.vue'
import { useFeature } from '../../features/feature-store'

const SPARKLINE_WIDTH = 200
const SPARKLINE_HEIGHT = 28
// не больше стольких последних замеров (история в App.vue — 40 точек раз в секунду)
const SPARKLINE_MAX_POINTS = 60

const props = defineProps({
  pages: {
    type: Array,
    required: true
  },
  activePage: {
    type: String,
    required: true
  },
  appVersion: {
    type: String,
    required: true
  },
  collapsed: {
    type: Boolean,
    default: false
  },
  trafficMonitorEnabled: {
    type: Boolean,
    default: true
  },
  traffic: {
    type: Object,
    default: () => ({ upBps: 0, downBps: 0 })
  },
  trafficHistoryUp: {
    type: Array,
    default: () => []
  },
  trafficHistoryDown: {
    type: Array,
    default: () => []
  },
  theme: {
    type: String,
    default: 'light'
  },
  // сводный статус туннелей: connected | connecting | error | stopped (null — ещё не загружен)
  tunnelStatus: {
    type: String,
    default: null
  },
  // вариант логотипа Norka: 'A' — высокая арка, 'B' — плоский холм
  logoVariant: {
    type: String,
    default: 'A'
  }
})

const emit = defineEmits(['switch-page', 'toggle-collapse'])

const { t } = useI18n()
const mascotOn = useFeature('mascot')
const norkaStatusLabel = computed(() => t(`app.sidebar.norkaStatus.${props.tunnelStatus || 'stopped'}`))

const menuOptions = computed(() => props.pages.map((page) => ({
  key: page.key,
  label: page.title,
  icon: () => h(NIcon, { class: 'nav-item-icon', component: page.icon, 'aria-hidden': 'true' }),
})))

const sparkId = useId()

// Высота нижней панели (трафик, версия) — меняется, когда появляется график. Содержимое панели
// отступает на неё снизу, чтобы Норка стояла прямо над разделителем.
const bottomRef = ref(null)
const bottomHeight = ref(0)
let bottomObserver = null
const siderStyle = computed(() => (bottomHeight.value ? { '--sidebar-bottom-h': `${bottomHeight.value}px` } : undefined))

onMounted(() => {
  if (typeof ResizeObserver === 'undefined' || !bottomRef.value) return
  bottomObserver = new ResizeObserver(() => {
    bottomHeight.value = bottomRef.value?.offsetHeight || 0
  })
  bottomObserver.observe(bottomRef.value)
})

onBeforeUnmount(() => {
  bottomObserver?.disconnect()
  bottomObserver = null
})

function formatBytesRate(bps) {
  const value = Math.max(0, Number(bps) || 0)
  if (value < 1024) return `${Math.round(value)} B/s`
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(2)} KB/s`
  return `${(value / (1024 * 1024)).toFixed(2)} MB/s`
}

function historyPeak(history) {
  const data = Array.isArray(history) ? history : []
  if (data.length === 0) return 0
  return Math.max(...data.map((value) => Math.max(0, Number(value) || 0)), 0)
}

// Плавная линия без выбросов: монотонная кубическая интерполяция (Steffen, как d3.curveMonotoneX).
// Кривая между соседними замерами не выходит за их значения, поэтому никогда не уходит ниже нуля.
// Возвращает d для линии и для заливки под ней — один <path> на серию, DOM не пересоздаётся.
function buildSparklinePath(history, width, height, scaleMax) {
  const data = (Array.isArray(history) ? history : []).slice(-SPARKLINE_MAX_POINTS)
  const max = Math.max(scaleMax, 1)
  const ys = data.map((value) => height - (Math.max(0, Number(value) || 0) / max) * (height - 2) - 1)
  const f = (value) => value.toFixed(1)
  if (ys.length < 2) {
    const y = f(ys.length ? ys[0] : height - 1)
    const line = `M0,${y}H${width}`
    return { line, area: `${line}V${height}H0Z` }
  }
  const n = ys.length
  const step = width / (n - 1)
  const secants = []
  for (let i = 0; i < n - 1; i += 1) secants.push((ys[i + 1] - ys[i]) / step)
  const tangents = new Array(n)
  tangents[0] = secants[0]
  tangents[n - 1] = secants[n - 2]
  for (let i = 1; i < n - 1; i += 1) {
    const s0 = secants[i - 1]
    const s1 = secants[i]
    tangents[i] = s0 * s1 <= 0
      ? 0
      : Math.sign(s0) * Math.min(2 * Math.abs(s0), 2 * Math.abs(s1), Math.abs(s0 + s1) / 2)
  }
  const h3 = step / 3
  let line = `M0,${f(ys[0])}`
  for (let i = 0; i < n - 1; i += 1) {
    const x0 = step * i
    const x1 = x0 + step
    line += `C${f(x0 + h3)},${f(ys[i] + tangents[i] * h3)} ${f(x1 - h3)},${f(ys[i + 1] - tangents[i + 1] * h3)} ${f(x1)},${f(ys[i + 1])}`
  }
  return { line, area: `${line}V${height}H0Z` }
}

const hasSparkline = computed(() => historyPeak(props.trafficHistoryUp) > 0 || historyPeak(props.trafficHistoryDown) > 0)

const chartScaleMax = computed(() => {
  const upPeak = historyPeak(props.trafficHistoryUp)
  const downPeak = historyPeak(props.trafficHistoryDown)
  return Math.max(upPeak, downPeak, 1)
})

const uploadSpark = computed(() => buildSparklinePath(
  props.trafficHistoryUp,
  SPARKLINE_WIDTH,
  SPARKLINE_HEIGHT,
  chartScaleMax.value
))
const downloadSpark = computed(() => buildSparklinePath(
  props.trafficHistoryDown,
  SPARKLINE_WIDTH,
  SPARKLINE_HEIGHT,
  chartScaleMax.value
))
</script>

<template>
  <n-layout-sider
    class="sidebar-panel"
    :class="{ collapsed }"
    bordered
    collapse-mode="width"
    :collapsed="collapsed"
    :collapsed-width="72"
    :width="264"
    :native-scrollbar="false"
    :style="siderStyle"
  >
    <div class="sidebar-main">
      <div class="brand-logo-wrap">
        <div v-if="!collapsed" class="brand-identity">
          <div class="brand-meta">
            <div class="brand-title">{{ $t('app.title') }}</div>
            <div class="brand-subtitle">{{ $t('app.sidebar.subtitle') }}</div>
          </div>
        </div>
        <n-button
          quaternary
          circle
          class="sidebar-collapse-btn"
          :title="collapsed ? $t('app.sidebar.expand') : $t('app.sidebar.collapse')"
          :aria-label="collapsed ? $t('app.sidebar.expand') : $t('app.sidebar.collapse')"
          @click="emit('toggle-collapse')"
        >
          <n-icon aria-hidden="true" :component="collapsed ? ChevronForward : ChevronBack" />
        </n-button>
      </div>

      <n-menu
        :value="activePage"
        :collapsed="collapsed"
        :collapsed-width="72"
        :collapsed-icon-size="22"
        :options="menuOptions"
        @update:value="emit('switch-page', $event)"
      />
    </div>

    <!-- Норка со статусом над нижней панелью: занимает оставшуюся высоту, по ширине — вся панель -->
    <div class="sidebar-mascot">
      <div class="sidebar-mascot__logo" data-onboarding="mascot">
        <NorkaStatusLogo
          :status="tunnelStatus"
          :variant="logoVariant"
          :theme="theme === 'dark' ? 'dark' : 'light'"
          size="100%"
          :title="$t('app.title')"
          :status-label="norkaStatusLabel"
          :idle="mascotOn"
          :easter-egg="mascotOn"
        />
      </div>
    </div>

    <div ref="bottomRef" class="sidebar-bottom" :class="{ collapsed }">
      <div
        v-if="trafficMonitorEnabled && !collapsed && hasSparkline"
        class="traffic-chart-wrap"
        aria-hidden="true"
      >
        <svg
          class="traffic-sparkline"
          :viewBox="`0 0 ${SPARKLINE_WIDTH} ${SPARKLINE_HEIGHT}`"
          preserveAspectRatio="none"
          aria-hidden="true"
        >
          <defs>
            <linearGradient :id="`${sparkId}-down`" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0" class="traffic-stop-down" stop-opacity="0.22" />
              <stop offset="1" class="traffic-stop-down" stop-opacity="0" />
            </linearGradient>
            <linearGradient :id="`${sparkId}-up`" x1="0" y1="0" x2="0" y2="1">
              <stop offset="0" class="traffic-stop-up" stop-opacity="0.18" />
              <stop offset="1" class="traffic-stop-up" stop-opacity="0" />
            </linearGradient>
          </defs>
          <path class="traffic-area" :d="downloadSpark.area" :fill="`url(#${sparkId}-down)`" />
          <path class="traffic-area" :d="uploadSpark.area" :fill="`url(#${sparkId}-up)`" />
          <path class="traffic-line-down" :d="downloadSpark.line" />
          <path class="traffic-line-up" :d="uploadSpark.line" />
        </svg>
      </div>

      <div class="sidebar-meta">
        <div
          v-if="trafficMonitorEnabled"
          class="traffic-rates"
          :aria-label="$t('app.sidebar.traffic')"
        >
          <div class="traffic-rate-row traffic-rate-up">
            <n-icon aria-hidden="true" class="traffic-rate-icon" :component="ArrowUp" />
            <span class="traffic-rate-current" :title="$t('app.sidebar.upload')">
              {{ formatBytesRate(traffic.upBps) }}
            </span>
          </div>
          <div class="traffic-rate-row traffic-rate-down">
            <n-icon aria-hidden="true" class="traffic-rate-icon" :component="ArrowDown" />
            <span class="traffic-rate-current" :title="$t('app.sidebar.download')">
              {{ formatBytesRate(traffic.downBps) }}
            </span>
          </div>
        </div>

        <div class="sidebar-footer">
        <span class="sidebar-version" :title="`v${appVersion}`">v{{ appVersion }}</span>
        </div>
      </div>
    </div>
  </n-layout-sider>
</template>
