<script setup>
// Короткий тур: подсветка реального элемента и карточка на переменных темы.
// Тяжёлой библиотеки нет. Если цель не видна, карточка встаёт по центру без пятна.
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import NorkaIcon from '../norka/NorkaIcon.vue'
import { featureEnabled } from '../../features/feature-store'
import {
  onboardingKeyAction,
  onboardingSteps,
  placeOnboardingCard,
  reduceOnboarding,
  spotlightBox,
} from '../../onboarding/onboarding.js'

const props = defineProps({
  theme: {
    type: String,
    default: 'light',
  },
})

const emit = defineEmits(['skip', 'done', 'step'])

const { t } = useI18n()
const index = ref(0)
const cardRef = ref(null)
const primaryRef = ref(null)
const cardStyle = ref({})
const spotStyle = ref(null)
let resizeObserver = null
let layoutFrame = 0

const steps = computed(() => onboardingSteps({
  ssh_command: featureEnabled('ssh_command'),
  quick_search: featureEnabled('quick_search'),
}))
const step = computed(() => steps.value[Math.min(index.value, steps.value.length - 1)] || steps.value[0])
const isFirst = computed(() => index.value <= 0)
const isLast = computed(() => index.value >= steps.value.length - 1)
const iconTheme = computed(() => (props.theme === 'dark' ? 'dark' : 'light'))

function currentStep() {
  return steps.value[index.value] || steps.value[0]
}

function apply(action) {
  const next = reduceOnboarding({
    index: index.value,
    count: steps.value.length,
    open: true,
    done: false,
    enabled: true,
  }, action)
  if (!next.open) {
    emit(action === 'skip' ? 'skip' : 'done')
    return
  }
  if (next.index === index.value) return
  index.value = next.index
}

function onKeydown(event) {
  const action = onboardingKeyAction(event, { index: index.value })
  if (!action) return
  event.preventDefault()
  event.stopPropagation()
  apply(action)
}

function viewport() {
  return { width: window.innerWidth, height: window.innerHeight }
}

function rectOf(element) {
  if (!element || typeof element.getBoundingClientRect !== 'function') return null
  const style = window.getComputedStyle(element)
  const rect = element.getBoundingClientRect()
  return {
    hidden: style.display === 'none' || style.visibility === 'hidden' || Number(style.opacity) === 0,
    left: rect.left,
    top: rect.top,
    width: rect.width,
    height: rect.height,
  }
}

function findTarget(id) {
  if (!id || typeof document === 'undefined') return null
  const safe = String(id).replace(/[^a-z-]/g, '')
  if (!safe) return null
  return document.querySelector(`[data-onboarding="${safe}"]`)
}

function watchTarget(element) {
  resizeObserver?.disconnect()
  resizeObserver = null
  if (!element || typeof ResizeObserver === 'undefined') return
  resizeObserver = new ResizeObserver(() => scheduleLayout())
  resizeObserver.observe(element)
}

async function layout(focusPrimary) {
  await nextTick()
  const element = findTarget(currentStep()?.target)
  let rect = rectOf(element)
  if (element && rect && !rect.hidden && !spotlightBox(rect, viewport())) {
    element.scrollIntoView({ block: 'center', inline: 'nearest', behavior: 'auto' })
    await nextTick()
    rect = rectOf(element)
  }
  watchTarget(element)
  const card = cardRef.value?.getBoundingClientRect()
  const box = spotlightBox(rect, viewport())
  const placed = placeOnboardingCard({
    target: box,
    card: { width: card?.width || 0, height: card?.height || 0 },
    viewport: viewport(),
  })
  cardStyle.value = {
    top: `${placed.top}px`,
    left: `${placed.left}px`,
    transform: 'none',
  }
  spotStyle.value = placed.spotlight && box
    ? {
      top: `${box.top}px`,
      left: `${box.left}px`,
      width: `${box.width}px`,
      height: `${box.height}px`,
    }
    : null
  if (focusPrimary) {
    await nextTick()
    primaryRef.value?.focus({ preventScroll: true })
  }
}

function scheduleLayout() {
  if (layoutFrame) return
  layoutFrame = requestAnimationFrame(() => {
    layoutFrame = 0
    void layout(false)
  })
}

watch(index, () => {
  emit('step', currentStep())
  void layout(true)
})

watch(steps, (list) => {
  if (index.value > list.length - 1) index.value = Math.max(0, list.length - 1)
  else void layout(false)
})

onMounted(() => {
  emit('step', currentStep())
  window.addEventListener('keydown', onKeydown, true)
  window.addEventListener('resize', scheduleLayout)
  void layout(true)
})

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown, true)
  window.removeEventListener('resize', scheduleLayout)
  resizeObserver?.disconnect()
  if (layoutFrame) cancelAnimationFrame(layoutFrame)
})
</script>

<template>
  <div class="onboarding">
    <div class="onboarding-blocker" :class="{ 'is-dim': !spotStyle }" />
    <div v-if="spotStyle" class="onboarding-spot" :style="spotStyle" />
    <div
      ref="cardRef"
      class="onboarding-card"
      role="dialog"
      aria-modal="true"
      aria-labelledby="onboarding-title"
      :style="cardStyle"
    >
      <div class="onboarding-scroll">
        <div v-if="step.showMascot" class="onboarding-hello">
          <NorkaIcon
            status="connected"
            :theme="iconTheme"
            :size="64"
            :title="t('app.title')"
            :status-label="t('onboarding.welcomeTitle')"
          />
          <div class="onboarding-copy">
            <h2 id="onboarding-title" class="onboarding-title">{{ t(step.titleKey) }}</h2>
            <p v-for="key in step.bodyKeys" :key="key" class="onboarding-body">{{ t(key) }}</p>
          </div>
        </div>
        <template v-else>
          <h2 id="onboarding-title" class="onboarding-title">{{ t(step.titleKey) }}</h2>
          <p v-for="key in step.bodyKeys" :key="key" class="onboarding-body">{{ t(key) }}</p>
        </template>
        <ul v-if="step.legend" class="onboarding-legend">
          <li v-for="item in step.legend" :key="item.status">
            <NorkaIcon
              :status="item.status"
              :theme="iconTheme"
              :size="40"
              :title="t('app.title')"
              :status-label="t(item.labelKey)"
            />
            <span>{{ t(item.labelKey) }}</span>
          </li>
        </ul>
      </div>
      <div class="onboarding-actions">
        <span class="onboarding-progress">{{ t('onboarding.progress', { current: index + 1, total: steps.length }) }}</span>
        <button type="button" class="onboarding-btn onboarding-btn--ghost" @click="apply('skip')">
          {{ t('onboarding.skip') }}
        </button>
        <button type="button" class="onboarding-btn" :disabled="isFirst" @click="apply('back')">
          {{ t('onboarding.back') }}
        </button>
        <button
          ref="primaryRef"
          type="button"
          class="onboarding-btn onboarding-btn--primary"
          data-onboarding-primary
          @click="apply(isLast ? 'done' : 'next')"
        >
          {{ isLast ? t('onboarding.done') : t('onboarding.next') }}
        </button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.onboarding-blocker,
.onboarding-spot,
.onboarding-card {
  position: fixed;
}

.onboarding-blocker {
  inset: 0;
  z-index: 4200;
  background: transparent;
}

.onboarding-blocker.is-dim {
  background: var(--lt-overlay-bg);
}

.onboarding-spot {
  z-index: 4201;
  pointer-events: none;
  border-radius: 14px;
  box-shadow:
    0 0 0 2px var(--lt-brand),
    0 0 0 9999px var(--lt-overlay-bg);
}

.onboarding-card {
  z-index: 4202;
  left: 50%;
  top: 50%;
  transform: translate(-50%, -50%);
  display: flex;
  flex-direction: column;
  width: min(360px, calc(100vw - 16px));
  max-height: calc(100vh - 16px);
  overflow: hidden;
  padding: 14px 14px 10px;
  background: var(--lt-surface);
  color: var(--lt-ink);
  border: 1px solid var(--lt-border);
  border-radius: var(--lt-radius-lg, 12px);
  box-shadow: var(--lt-dialog-shadow);
}

.onboarding-scroll {
  flex: 1 1 auto;
  min-height: 0;
  overflow: auto;
}

.onboarding-hello {
  display: flex;
  align-items: flex-start;
  gap: 12px;
}

.onboarding-hello :deep(svg) {
  flex: none;
  border-radius: 14px;
}

.onboarding-copy {
  min-width: 0;
}

.onboarding-title {
  margin: 0 0 6px;
  font-size: 16px;
  line-height: 1.3;
  font-weight: 650;
}

.onboarding-body {
  margin: 0;
  color: var(--lt-muted);
  font-size: 13.5px;
  line-height: 1.45;
}

.onboarding-body + .onboarding-body {
  margin-top: 8px;
}

.onboarding-legend {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 12px;
  margin: 10px 0 0;
  padding: 0;
  list-style: none;
}

.onboarding-legend li {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
  color: var(--lt-ink);
  font-size: 12.5px;
  line-height: 1.3;
}

.onboarding-legend :deep(svg) {
  flex: none;
  border-radius: 8px;
}

.onboarding-actions {
  display: flex;
  flex: 0 0 auto;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
  margin-top: 12px;
  padding-top: 8px;
  background: var(--lt-surface);
}

.onboarding-progress {
  margin-right: auto;
  color: var(--lt-muted);
  font-size: 12px;
}

.onboarding-btn {
  font: inherit;
  font-size: 13px;
  line-height: 1.2;
  border-radius: 8px;
  padding: 7px 12px;
  cursor: pointer;
  border: 1px solid var(--lt-border-strong);
  background: var(--lt-surface);
  color: var(--lt-ink);
}

.onboarding-btn:disabled {
  opacity: 0.45;
  cursor: default;
}

.onboarding-btn--primary {
  background: var(--lt-primary);
  color: var(--lt-on-primary);
  border-color: transparent;
}

.onboarding-btn--ghost {
  background: transparent;
  border-color: transparent;
  color: var(--lt-muted);
}

.onboarding-btn:focus-visible {
  outline: 2px solid var(--lt-focus);
  outline-offset: 2px;
}

@media (max-height: 220px) {
  .onboarding-card {
    padding: 8px 10px;
  }

  .onboarding-hello :deep(svg) {
    width: 40px;
    height: 40px;
  }

  .onboarding-title {
    font-size: 14px;
  }

  .onboarding-body,
  .onboarding-btn {
    font-size: 12.5px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .onboarding-card {
    transition: none;
  }
}
</style>
