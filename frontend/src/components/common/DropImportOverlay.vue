<script setup>
import { computed } from 'vue'
import { useThemeVars } from 'naive-ui'

defineProps({
  show: { type: Boolean, default: false }
})

const themeVars = useThemeVars()

const cssVars = computed(() => ({
  '--drop-mask': themeVars.value.modalColor,
  '--drop-border': themeVars.value.primaryColor,
  '--drop-text': themeVars.value.textColor1,
  '--drop-muted': themeVars.value.textColor3,
  '--drop-radius': themeVars.value.borderRadius
}))
</script>

<template>
  <transition name="drop-import-fade">
    <div v-if="show" class="drop-import-overlay" :style="cssVars" aria-live="polite">
      <div class="drop-import-panel">
        <i class="bi bi-box-arrow-in-down drop-import-icon" aria-hidden="true"></i>
        <p class="drop-import-title">{{ $t('config.dropTitle') }}</p>
        <p class="drop-import-hint">{{ $t('config.dropHint') }}</p>
      </div>
    </div>
  </transition>
</template>

<style scoped>
.drop-import-overlay {
  position: fixed;
  inset: 0;
  z-index: 4000;
  display: flex;
  padding: 16px;
  background: color-mix(in srgb, var(--drop-mask) 88%, transparent);
  pointer-events: none;
}

.drop-import-panel {
  flex: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 8px;
  border: 2px dashed var(--drop-border);
  border-radius: var(--drop-radius);
  color: var(--drop-text);
  text-align: center;
}

.drop-import-icon {
  font-size: 40px;
  color: var(--drop-border);
}

.drop-import-title {
  margin: 0;
  font-size: 16px;
  font-weight: 600;
}

.drop-import-hint {
  margin: 0;
  font-size: 13px;
  color: var(--drop-muted);
}

.drop-import-fade-enter-active,
.drop-import-fade-leave-active {
  transition: opacity 0.12s ease;
}

.drop-import-fade-enter-from,
.drop-import-fade-leave-to {
  opacity: 0;
}
</style>
