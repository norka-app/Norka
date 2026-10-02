<!--
  Собственная строка заголовка для окна без системной рамки (Windows, Frameless в main.go).
  Прозрачная полоса 32px поверх интерфейса: слева маленькая Norka (сводный статус) и «Norka»,
  справа кнопки 46×32 — «Свернуть», переключатель режима окна, «Закрыть».
  Полоса перетаскивает окно (--wails-draggable: drag), кнопки из этой области исключены (no-drag).
-->
<script setup>
import { computed, onBeforeUnmount, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import NorkaIcon from '../norka/NorkaIcon.vue'

const props = defineProps({
  // текущий режим окна: 'advanced' | 'simple'
  mode: { type: String, default: 'advanced' },
  theme: { type: String, default: 'light' },
  // сводный статус туннелей для маленькой иконки
  status: { type: String, default: null }
})
const emit = defineEmits(['minimise', 'toggle-mode', 'close'])
const { t } = useI18n()

const toSimple = computed(() => props.mode !== 'simple')
const modeLabel = computed(() => t(toSimple.value ? 'app.titlebar.toSimple' : 'app.titlebar.toAdvanced'))

// Своя подсказка: системный title у WebView2 появляется с задержкой и без темы.
const TIP_DELAY_MS = 500
const tip = ref(null) // { id, text }
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
  <header class="titlebar" :class="[`titlebar--${theme}`, `titlebar--${mode}`]">
    <div class="titlebar__drag">
      <NorkaIcon
        class="titlebar__icon"
        :status="status || 'stopped'"
        :theme="theme === 'dark' ? 'dark' : 'light'"
        :size="16"
        aria-hidden="true"
      />
      <span class="titlebar__title">{{ $t('app.title') }}</span>
    </div>
    <div class="titlebar__controls">
      <button
        type="button"
        class="titlebar__btn"
        :aria-label="$t('app.titlebar.minimise')"
        @mouseenter="showTip('min', $t('app.titlebar.minimise'))"
        @mouseleave="hideTip"
        @click="press('minimise')"
      >
        <svg viewBox="0 0 10 10" width="10" height="10" aria-hidden="true"><path d="M0 5.5h10" /></svg>
      </button>
      <button
        type="button"
        class="titlebar__btn titlebar__btn--mode"
        :aria-label="`${modeLabel} (Ctrl+Shift+M)`"
        @mouseenter="showTip('mode', `${modeLabel} · Ctrl+Shift+M`)"
        @mouseleave="hideTip"
        @click="press('toggle-mode')"
      >
        <!-- значок показывает режим, в который переключит кнопка -->
        <svg v-if="toSimple" viewBox="0 0 14 14" width="14" height="14" aria-hidden="true">
          <rect x="0.5" y="3.5" width="13" height="7" rx="1.5" />
          <circle cx="4" cy="7" r="1.5" />
          <path d="M7 6h4.5M7 8h3" />
        </svg>
        <svg v-else viewBox="0 0 14 14" width="14" height="14" aria-hidden="true">
          <rect x="0.5" y="1.5" width="13" height="11" rx="1.5" />
          <path d="M4.5 1.5v11M7 4.5h4.5M7 6.5h4.5M7 8.5h3" />
        </svg>
      </button>
      <button
        type="button"
        class="titlebar__btn titlebar__btn--close"
        :aria-label="$t('app.titlebar.close')"
        @mouseenter="showTip('close', $t('app.titlebar.close'))"
        @mouseleave="hideTip"
        @click="press('close')"
      >
        <svg viewBox="0 0 10 10" width="10" height="10" aria-hidden="true"><path d="M0.5 0.5l9 9M9.5 0.5l-9 9" /></svg>
      </button>
    </div>
    <div v-if="tip" class="titlebar__tip" :class="`titlebar__tip--${tip.id}`" role="tooltip">{{ tip.text }}</div>
  </header>
</template>

<style scoped>
.titlebar {
  --tb-h: 32px;
  --tb-fg: #1f2329;
  --tb-muted: #5f6672;
  --tb-hover: rgba(0, 0, 0, 0.055);
  --tb-press: rgba(0, 0, 0, 0.09);
  --tb-tip-bg: #ffffff;
  --tb-tip-line: rgba(0, 0, 0, 0.08);
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  /* ниже модальных окон и выпадающих списков naive-ui (z-index от 2000) */
  z-index: 1000;
  display: flex;
  align-items: stretch;
  height: var(--tb-h);
  color: var(--tb-fg);
  background: transparent;
  user-select: none;
  -webkit-user-select: none;
  --wails-draggable: drag;
  font-family: "Segoe UI Variable Text", "Segoe UI", system-ui, sans-serif;
}

.titlebar--dark {
  --tb-fg: #e8eaed;
  --tb-muted: #a3a9b3;
  --tb-hover: rgba(255, 255, 255, 0.07);
  --tb-press: rgba(255, 255, 255, 0.11);
  --tb-tip-bg: #2b2d33;
  --tb-tip-line: rgba(255, 255, 255, 0.1);
}

.titlebar__drag {
  flex: 1;
  min-width: 0;
  display: flex;
  align-items: center;
  gap: 8px;
  padding-left: 10px;
}

.titlebar__icon {
  flex: none;
  display: block;
}

.titlebar__title {
  font-size: 12px;
  line-height: 1;
  color: var(--tb-muted);
  white-space: nowrap;
}

.titlebar__controls {
  display: flex;
  flex: none;
  --wails-draggable: no-drag;
}

.titlebar__btn {
  --wails-draggable: no-drag;
  width: 46px;
  height: var(--tb-h);
  display: grid;
  place-items: center;
  padding: 0;
  border: 0;
  border-radius: 0;
  color: var(--tb-fg);
  background: transparent;
  cursor: default;
  outline: none;
  transition: background-color 0.08s linear, color 0.08s linear;
}

.titlebar__btn:hover { background: var(--tb-hover); }
.titlebar__btn:active { background: var(--tb-press); }
.titlebar__btn--close:hover { background: #c42b1c; color: #fff; }
.titlebar__btn--close:active { background: #b3271a; color: #fff; }
.titlebar__btn:focus-visible { box-shadow: inset 0 0 0 2px #2f7cf6; }

.titlebar__btn svg {
  fill: none;
  stroke: currentColor;
  stroke-width: 1;
  overflow: visible;
}

.titlebar__tip {
  position: absolute;
  top: calc(var(--tb-h) + 6px);
  padding: 5px 9px;
  border-radius: 6px;
  font-size: 12px;
  line-height: 16px;
  white-space: nowrap;
  color: var(--tb-fg);
  background: var(--tb-tip-bg);
  border: 1px solid var(--tb-tip-line);
  box-shadow: 0 4px 14px rgba(0, 0, 0, 0.14);
  pointer-events: none;
}

.titlebar--dark .titlebar__tip { box-shadow: 0 4px 14px rgba(0, 0, 0, 0.45); }
.titlebar__tip--min { right: 92px; }
.titlebar__tip--mode { right: 46px; }
.titlebar__tip--close { right: 6px; }

@media (prefers-reduced-motion: reduce) {
  .titlebar__btn { transition: none; }
}
</style>
