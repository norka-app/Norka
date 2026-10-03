<!--
  Кнопки окна для вида «без рамки» на Windows. Окно не меняет размер,
  поэтому «Развернуть» нет: остаются «Свернуть» и «Закрыть».
  Логика та же, что у AppTitleBar: события minimise / close.
  Шапка страницы — зона перетаскивания, кнопки из неё исключены.
-->
<script setup>
import { onBeforeUnmount, ref } from 'vue'
import { NIcon } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { CloseOutline, Remove } from '@vicons/ionicons5'

defineProps({
  theme: { type: String, default: 'light' },
})
const emit = defineEmits(['minimise', 'close'])
const { t } = useI18n()

const TIP_DELAY_MS = 500
const tip = ref(null)
let tipTimer = 0

function showTip(id, text) {
  clearTimeout(tipTimer)
  tipTimer = setTimeout(() => { tip.value = { id, text } }, TIP_DELAY_MS)
}

function hideTip() {
  clearTimeout(tipTimer)
  tip.value = null
}

function press(event) {
  hideTip()
  emit(event)
}

onBeforeUnmount(() => clearTimeout(tipTimer))
</script>

<template>
  <div class="flc flc--win" :class="theme === 'dark' ? 'flc--dark' : 'flc--light'">
    <button
      type="button"
      class="flc-btn"
      :aria-label="t('app.titlebar.minimise')"
      @mouseenter="showTip('min', t('app.titlebar.minimise'))"
      @mouseleave="hideTip"
      @click="press('minimise')"
    >
      <n-icon :component="Remove" :size="14" />
    </button>
    <button
      type="button"
      class="flc-btn flc-btn--close"
      :aria-label="t('app.titlebar.close')"
      @mouseenter="showTip('close', t('app.titlebar.close'))"
      @mouseleave="hideTip"
      @click="press('close')"
    >
      <n-icon :component="CloseOutline" :size="16" />
    </button>
    <div v-if="tip" class="flc-tip" :class="`flc-tip--${tip.id}`" role="tooltip">{{ tip.text }}</div>
  </div>
</template>

<style scoped>
.flc {
  position: absolute;
  z-index: 1000;
  top: 0;
  right: 0;
  display: flex;
  height: 40px;
  --wails-draggable: no-drag;
  color: var(--lt-ink, #18181b);
  font-family: var(--norka-font-family);
}

.flc--dark { color: #e8eaed; }

.flc-btn {
  --wails-draggable: no-drag;
  width: 46px;
  height: 40px;
  display: grid;
  place-items: center;
  padding: 0;
  border: 0;
  background: transparent;
  color: inherit;
  cursor: default;
}

.flc-btn:hover { background: var(--fl-hover, rgba(24, 24, 27, 0.045)); }
.flc-btn--close:hover { background: #c42b1c; color: #fff; }
.flc-btn--close { border-top-right-radius: var(--win-r, 8px); }
.flc-btn:focus-visible { outline: 2px solid #2f7cf6; outline-offset: -2px; }

.flc-tip {
  position: absolute;
  top: calc(100% + 6px);
  padding: 5px 9px;
  border-radius: 6px;
  font-size: 12px;
  line-height: 16px;
  white-space: nowrap;
  color: var(--lt-ink, #18181b);
  background: #fff;
  border: 1px solid rgba(0, 0, 0, 0.08);
  box-shadow: 0 4px 14px rgba(0, 0, 0, 0.14);
  pointer-events: none;
}

.flc--dark .flc-tip {
  color: #e8eaed;
  background: #2b2d33;
  border-color: rgba(255, 255, 255, 0.1);
}

.flc-tip--min { right: 46px; }
.flc-tip--close { right: 6px; }
</style>
