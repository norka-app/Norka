<script setup>
// Логотип Norka со статусом туннелей: статичная SVG-иконка (NorkaIcon) +
// однократное подмигивание (Lottie) при переходе сводного статуса в 'connected'
// и, если включено idle, поведение в простое (useNorkaIdle).
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import NorkaIcon from './NorkaIcon.vue'
import { useNorkaIdle } from './useNorkaIdle'

const props = defineProps({
  // connected | connecting | error | partial | stopped; null — состояние ещё не загружено
  status: {
    type: String,
    default: null
  },
  // 'A' — высокая арка, 'B' — плоский холм
  variant: {
    type: String,
    default: 'A'
  },
  theme: {
    type: String,
    default: 'dark'
  },
  // px (число) или CSS-длина, например '100%' — тогда размер задаёт контейнер
  size: {
    type: [Number, String],
    default: 36
  },
  title: {
    type: String,
    default: 'Norka'
  },
  // подпись статуса для подсказки и aria-label
  statusLabel: {
    type: String,
    default: ''
  },
  // моргание, взгляды и засыпание в простое — для крупных экземпляров иконки
  idle: {
    type: Boolean,
    default: false
  }
})

// Lottie-анимации грузятся лениво (отдельные чанки), только когда нужно подмигнуть
const winkAnimations = import.meta.glob('../../assets/norka/*.json')

const iconStatus = computed(() => props.status || 'stopped')
const boxSize = computed(() => (typeof props.size === 'number' ? `${props.size}px` : props.size))
const root = ref(null)
const winkHost = ref(null)
const winking = ref(false)
let winkPlayer = null
let winkToken = 0

function prefersReducedMotion() {
  return typeof window !== 'undefined'
    && typeof window.matchMedia === 'function'
    && window.matchMedia('(prefers-reduced-motion: reduce)').matches
}

function stopWink() {
  winkToken += 1
  if (winkPlayer) {
    winkPlayer.destroy()
    winkPlayer = null
  }
  winking.value = false
}

async function playWink() {
  stopWink()
  if (prefersReducedMotion() || !winkHost.value) return
  const token = winkToken
  const file = `../../assets/norka/norka-${props.variant === 'B' ? 'B' : 'A'}-wink${props.theme === 'light' ? '-light' : ''}.json`
  const loadData = winkAnimations[file]
  if (!loadData) return
  try {
    const [{ default: lottie }, { default: animationData }] = await Promise.all([
      import('lottie-web/build/player/lottie_light'),
      loadData()
    ])
    if (token !== winkToken || !winkHost.value) return
    winkPlayer = lottie.loadAnimation({
      container: winkHost.value,
      renderer: 'svg',
      loop: false,
      autoplay: true,
      animationData
    })
    winkPlayer.addEventListener('DOMLoaded', () => {
      if (token === winkToken) winking.value = true
    })
    winkPlayer.addEventListener('complete', () => {
      if (token === winkToken) stopWink()
    })
  } catch (_) {
    // анимация необязательна — остаётся статичная иконка
    if (token === winkToken) stopWink()
  }
}

watch(() => props.status, (next, prev) => {
  if (next === 'connected' && prev && prev !== 'connected') {
    void playWink()
  } else if (next !== 'connected') {
    stopWink()
  }
})

watch(() => [props.variant, props.theme], stopWink)

onBeforeUnmount(stopWink)

// пока играет подмигивание, статичная иконка скрыта — поведение в простое ждёт
useNorkaIdle(root, () => iconStatus.value, () => props.idle && !winking.value)
</script>

<template>
  <span
    ref="root"
    class="norka-status-logo"
    :class="[`norka-status-logo--${theme}`, { 'is-winking': winking }]"
    :style="{ width: boxSize, height: boxSize }"
  >
    <NorkaIcon
      class="norka-status-logo__icon"
      :status="iconStatus"
      :variant="variant"
      :theme="theme"
      :size="size"
      :title="title"
      :status-label="statusLabel"
      :idle="idle"
    />
    <span ref="winkHost" class="norka-status-logo__wink" aria-hidden="true" />
  </span>
</template>

<style scoped>
.norka-status-logo {
  position: relative;
  display: inline-flex;
  flex: 0 0 auto;
  border-radius: 22.5%;
  line-height: 0;
}

/* светлая плитка почти сливается с белой панелью — тонкий контур */
.norka-status-logo--light {
  box-shadow: 0 0 0 1px var(--lt-border, #e4e4e7);
}

.norka-status-logo__icon {
  display: block;
}

.norka-status-logo__wink {
  position: absolute;
  inset: 0;
  pointer-events: none;
}

.norka-status-logo__wink :deep(svg) {
  display: block;
}

.norka-status-logo.is-winking .norka-status-logo__icon {
  visibility: hidden;
}
</style>
