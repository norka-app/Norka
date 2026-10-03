<script setup>
// Короткий тур первого запуска. Маскот — настоящая NorkaIcon со статусом
// connected: зелёные миндалевидные глаза и вертикальные зрачки, не круглая мордочка.
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import NorkaIcon from '../norka/NorkaIcon.vue'
import { TOUR_STEPS, isLastTourStep } from './tour-steps.js'

const props = defineProps({
  show: {
    type: Boolean,
    default: false,
  },
  theme: {
    type: String,
    default: 'light',
  },
})

const emit = defineEmits(['finish', 'goto-page'])

const { t } = useI18n()
const stepIndex = ref(0)
const rootRef = ref(null)
const cardRef = ref(null)
const hole = ref(null)
const cardStyle = ref({})

const CARD_WIDTH = 400
const GAP = 14
const MARGIN = 16
const HOLE_PAD = 8

const step = computed(() => TOUR_STEPS[stepIndex.value] || TOUR_STEPS[0])
const isLast = computed(() => isLastTourStep(stepIndex.value))
const isWelcome = computed(() => step.value.id === 'welcome')
const titleId = 'first-launch-tour-title'
const bodyId = 'first-launch-tour-body'

const holeStyle = computed(() => {
  if (!hole.value) return { display: 'none' }
  return {
    top: `${hole.value.top}px`,
    left: `${hole.value.left}px`,
    width: `${hole.value.width}px`,
    height: `${hole.value.height}px`,
  }
})

let placeToken = 0
let resizeObserver = null

function focusPrimary() {
  const button = cardRef.value?.querySelector('[data-tour-primary]')
  if (button instanceof HTMLElement) button.focus()
}

function focusable() {
  if (!cardRef.value) return []
  return [...cardRef.value.querySelectorAll('button:not(:disabled)')]
}

async function waitForTarget(selector) {
  for (let attempt = 0; attempt < 20; attempt += 1) {
    const node = document.querySelector(selector)
    if (node) return node
    await new Promise((resolve) => requestAnimationFrame(resolve))
  }
  return document.querySelector(selector)
}

function layout(cardHeight) {
  const root = rootRef.value
  if (!root) return
  const bounds = root.getBoundingClientRect()
  const cardWidth = Math.min(CARD_WIDTH, Math.max(240, bounds.width - MARGIN * 2))
  const maxHeight = Math.max(160, bounds.height - MARGIN * 2)
  const height = Math.min(cardHeight || 240, maxHeight)
  if (!hole.value) {
    cardStyle.value = {
      width: `${cardWidth}px`,
      maxHeight: `${maxHeight}px`,
      top: '50%',
      left: '50%',
      transform: 'translate(-50%, -50%)',
    }
    return
  }
  const belowTop = hole.value.top + hole.value.height + GAP
  const aboveTop = hole.value.top - GAP - height
  const belowFits = belowTop + height <= bounds.height - MARGIN
  const aboveFits = aboveTop >= MARGIN
  let top = belowFits || !aboveFits ? belowTop : aboveTop
  let left = hole.value.left
  if (left + cardWidth > bounds.width - MARGIN) left = hole.value.left + hole.value.width - cardWidth
  left = Math.min(Math.max(MARGIN, left), Math.max(MARGIN, bounds.width - MARGIN - cardWidth))
  top = Math.min(Math.max(MARGIN, top), Math.max(MARGIN, bounds.height - MARGIN - height))
  const coversHole = top < hole.value.top + hole.value.height + GAP && top + height > hole.value.top - GAP
  if (coversHole && belowFits) top = belowTop
  else if (coversHole && aboveFits) top = aboveTop
  cardStyle.value = {
    width: `${cardWidth}px`,
    maxHeight: `${maxHeight}px`,
    top: `${top}px`,
    left: `${left}px`,
    transform: 'none',
  }
}

async function place({ scroll = false } = {}) {
  const token = ++placeToken
  emit('goto-page', step.value.page || '')
  await nextTick()
  if (token !== placeToken || !props.show) return
  const target = step.value.target ? await waitForTarget(step.value.target) : null
  if (token !== placeToken || !props.show) return
  if (target instanceof HTMLElement) {
    if (scroll) target.scrollIntoView({ block: 'center', inline: 'nearest' })
    await nextTick()
    if (token !== placeToken) return
    const root = rootRef.value
    const rect = target.getBoundingClientRect()
    const origin = root ? root.getBoundingClientRect() : { top: 0, left: 0 }
    hole.value = {
      top: rect.top - origin.top - HOLE_PAD,
      left: rect.left - origin.left - HOLE_PAD,
      width: Math.max(rect.width, 24) + HOLE_PAD * 2,
      height: Math.max(rect.height, 24) + HOLE_PAD * 2,
    }
  } else {
    hole.value = null
  }
  layout(0)
  await nextTick()
  if (token !== placeToken) return
  layout(cardRef.value?.offsetHeight || 240)
  focusPrimary()
}

function go(next) {
  const index = Math.min(Math.max(next, 0), TOUR_STEPS.length - 1)
  if (index === stepIndex.value) {
    void place({ scroll: true })
    return
  }
  stepIndex.value = index
}

function onNext() {
  if (isLast.value) {
    emit('finish')
    return
  }
  go(stepIndex.value + 1)
}

function onKeydown(event) {
  if (event.key !== 'Tab') return
  const nodes = focusable()
  if (nodes.length === 0) return
  const active = document.activeElement
  const inside = cardRef.value?.contains(active)
  const first = nodes[0]
  const last = nodes[nodes.length - 1]
  if (!inside) {
    event.preventDefault()
    first.focus()
    return
  }
  if (event.shiftKey && active === first) {
    event.preventDefault()
    last.focus()
  } else if (!event.shiftKey && active === last) {
    event.preventDefault()
    first.focus()
  }
}

function onResize() {
  void place({ scroll: false })
}

watch(stepIndex, () => {
  void place({ scroll: true })
})

watch(() => props.show, (open) => {
  if (!open) return
  if (stepIndex.value !== 0) {
    stepIndex.value = 0
    return
  }
  void place({ scroll: true })
})

onMounted(() => {
  window.addEventListener('keydown', onKeydown, true)
  window.addEventListener('resize', onResize)
  if (typeof ResizeObserver !== 'undefined' && rootRef.value) {
    resizeObserver = new ResizeObserver(() => {
      void place()
    })
    resizeObserver.observe(rootRef.value)
  }
  void place({ scroll: true })
})

onBeforeUnmount(() => {
  placeToken += 1
  window.removeEventListener('keydown', onKeydown, true)
  window.removeEventListener('resize', onResize)
  resizeObserver?.disconnect()
  resizeObserver = null
})
</script>

<template>
  <div
    ref="rootRef"
    class="tour"
    :class="{ 'tour--spot': hole, 'tour--welcome': isWelcome }"
    role="dialog"
    aria-modal="true"
    :aria-labelledby="titleId"
    :aria-describedby="bodyId"
  >
    <div class="tour-blocker" />
    <div class="tour-hole" :style="holeStyle" aria-hidden="true" />
    <div ref="cardRef" class="tour-card" :style="cardStyle">
      <div class="tour-card-main">
        <NorkaIcon
          class="tour-mascot"
          status="connected"
          variant="A"
          :theme="theme === 'dark' ? 'dark' : 'light'"
          :size="isWelcome ? 96 : 64"
          title="Norka"
          :status-label="t('app.tour.mascotLabel')"
        />
        <div class="tour-copy">
          <p class="tour-progress">{{ t('app.tour.progress', { current: stepIndex + 1, total: TOUR_STEPS.length }) }}</p>
          <h2 :id="titleId" class="tour-title">{{ t(`app.tour.steps.${step.id}.title`) }}</h2>
          <p :id="bodyId" class="tour-body">{{ t(`app.tour.steps.${step.id}.body`) }}</p>
        </div>
      </div>
      <div class="tour-dots" role="group" :aria-label="t('app.tour.progress', { current: stepIndex + 1, total: TOUR_STEPS.length })">
        <button
          v-for="(item, index) in TOUR_STEPS"
          :key="item.id"
          type="button"
          class="tour-dot"
          :class="{ 'is-active': index === stepIndex }"
          :aria-current="index === stepIndex ? 'step' : undefined"
          :aria-label="t(`app.tour.steps.${item.id}.title`)"
          @click="go(index)"
        />
      </div>
      <div class="tour-actions">
        <button v-if="!isLast" type="button" class="btn btn-outline-secondary" @click="emit('finish')">
          {{ t('app.tour.skip') }}
        </button>
        <span v-else />
        <div class="tour-actions-end">
          <button v-if="stepIndex > 0" type="button" class="btn btn-outline-secondary" @click="go(stepIndex - 1)">
            {{ t('app.tour.back') }}
          </button>
          <button type="button" class="btn btn-primary" data-tour-primary @click="onNext">
            {{ isLast ? t('app.tour.done') : t('app.tour.next') }}
          </button>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.tour {
  position: fixed;
  inset: 0;
  z-index: 2600;
}

.tour-blocker {
  position: absolute;
  inset: 0;
  background: var(--lt-overlay-bg);
}

.tour--spot .tour-blocker {
  background: transparent;
}

.tour-hole {
  position: absolute;
  border-radius: 12px;
  box-shadow: 0 0 0 9999px var(--lt-overlay-bg);
  outline: 2px solid #3ddc84;
  pointer-events: none;
}

.tour-card {
  position: absolute;
  display: flex;
  flex-direction: column;
  gap: 16px;
  padding: 18px 18px 14px;
  overflow: auto;
  border: 1px solid var(--lt-border);
  border-radius: var(--lt-radius-xl);
  background: var(--lt-surface);
  box-shadow: var(--lt-dialog-shadow);
  color: var(--lt-ink);
}

.tour-card-main {
  display: flex;
  align-items: flex-start;
  gap: 14px;
  min-width: 0;
}

.tour--welcome .tour-card-main {
  flex-direction: column;
  align-items: center;
  text-align: center;
}

.tour-mascot {
  flex: none;
  border-radius: 22px;
}

.tour-copy {
  min-width: 0;
}

.tour-progress {
  margin: 0 0 4px;
  color: var(--lt-muted);
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

.tour-title {
  margin: 0;
  color: var(--lt-ink);
  font-size: 20px;
  font-weight: 600;
  letter-spacing: -0.02em;
  line-height: 1.25;
  text-wrap: balance;
}

.tour-body {
  margin: 8px 0 0;
  color: var(--lt-muted);
  font-size: 14px;
  line-height: 1.5;
  text-wrap: pretty;
}

.tour-dots {
  display: flex;
  justify-content: center;
  gap: 2px;
}

.tour-dot {
  width: 28px;
  height: 28px;
  padding: 0;
  border: 0;
  border-radius: 999px;
  background: transparent;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
}

.tour-dot::before {
  content: '';
  width: 8px;
  height: 8px;
  border-radius: 999px;
  background: var(--lt-border-strong);
}

.tour-dot.is-active::before {
  width: 18px;
  background: #3ddc84;
}

.tour-dot:focus-visible {
  outline: 2px solid var(--lt-focus);
  outline-offset: 2px;
}

.tour-actions {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.tour-actions-end {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-left: auto;
}

.tour-actions .btn:focus-visible {
  outline: 2px solid var(--lt-focus);
  outline-offset: 2px;
}

@media (prefers-reduced-motion: reduce) {
  .tour-card,
  .tour-hole,
  .tour-dot::before {
    transition: none;
  }
}
</style>
