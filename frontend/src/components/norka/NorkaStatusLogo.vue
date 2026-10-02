<script setup>
// Логотип Norka со статусом туннелей: статичная SVG-иконка (NorkaIcon) +
// однократное подмигивание (Lottie) при переходе сводного статуса в 'connected'
// и, если включено idle, поведение в простое (useNorkaIdle). С easterEgg клики по логотипу
// копят пасхалку «нокаут» (useNorkaKnockout) — других действий по клику нет.
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import NorkaIcon from './NorkaIcon.vue'
import { useNorkaIdle } from './useNorkaIdle'
import { useNorkaKnockout } from './useNorkaKnockout'

const props = defineProps({
  // connected | connecting | error | stopped; null — состояние ещё не загружено
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
  },
  // пасхалка «нокаут»: 7 кликов за 2 секунды
  easterEgg: {
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

const { knockout, leaving: knockoutLeaving, still: knockoutStill, press } = useNorkaKnockout(() => props.easterEgg)

// нокаут прерывает подмигивание
watch(knockout, (name) => {
  if (name) stopWink()
})

watch(() => props.status, (next, prev) => {
  if (next === 'connected' && prev && prev !== 'connected' && !knockout.value) {
    void playWink()
  } else if (next !== 'connected') {
    stopWink()
  }
})

watch(() => [props.variant, props.theme], stopWink)

onBeforeUnmount(stopWink)

// пока играет подмигивание или нокаут, поведение в простое ждёт
useNorkaIdle(root, () => iconStatus.value, () => props.idle && !winking.value && !knockout.value)
</script>

<template>
  <span
    ref="root"
    class="norka-status-logo"
    :class="[`norka-status-logo--${theme}`, { 'is-winking': winking, 'norka-status-logo--egg': easterEgg }]"
    :style="{ width: boxSize, height: boxSize }"
    @click="press"
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
      :knockout="knockout"
      :knockout-leaving="knockoutLeaving"
      :knockout-still="knockoutStill"
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

/* клики копят пасхалку: курсор обычный, лёгкое нажатие */
.norka-status-logo--egg {
  cursor: default;
  user-select: none;
  -webkit-user-select: none;
  transition: transform 0.12s ease;
}

.norka-status-logo--egg:active {
  transform: scale(0.97);
}

@media (prefers-reduced-motion: reduce) {
  .norka-status-logo--egg { transition: none; }
  .norka-status-logo--egg:active { transform: none; }
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
