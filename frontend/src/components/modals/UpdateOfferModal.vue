<script setup>
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { renderSafeMarkdown } from '../../utils/safe-markdown'

const props = defineProps({
  show: {
    type: Boolean,
    default: false,
  },
  current: {
    type: String,
    default: '',
  },
  latest: {
    type: String,
    default: '',
  },
  notes: {
    type: String,
    default: '',
  },
  pageUrl: {
    type: String,
    default: '',
  },
  canApply: {
    type: Boolean,
    default: false,
  },
  phase: {
    type: String,
    default: '',
  },
  percent: {
    type: Number,
    default: 0,
  },
  error: {
    type: String,
    default: '',
  },
})

const emit = defineEmits(['close', 'download', 'skip', 'open-url'])
const { t } = useI18n()
const applyButton = ref(null)

const safeNotes = computed(() => renderSafeMarkdown(props.notes))
const showPageLink = computed(() => {
  const page = String(props.pageUrl || '').trim()
  if (!page) return false
  return !String(props.notes || '').includes(page)
})
const busy = computed(() => ['download', 'verify', 'install', 'restart'].includes(props.phase))
const progressValue = computed(() => {
  const value = Number(props.percent) || 0
  if (value < 0) return 0
  if (value > 100) return 100
  return value
})
const phaseLabel = computed(() => {
  if (props.phase === 'download') return t('app.update.downloading', { percent: progressValue.value })
  if (props.phase === 'verify') return t('app.update.verifying')
  if (props.phase === 'install') return t('app.update.installing')
  if (props.phase === 'restart') return t('app.update.restarting')
  return ''
})

function onShowChange(open) {
  if (!open) emit('close')
}

function focusApply() {
  window.setTimeout(() => {
    document.querySelectorAll('.update-notes-body a').forEach((node) => node.blur())
    const el = applyButton.value?.$el
    const button = el instanceof HTMLElement
      ? (el.tagName === 'BUTTON' ? el : el.querySelector('button'))
      : null
    if (!button || button.disabled) return
    try {
      button.focus({ focusVisible: true })
    } catch {
      button.focus()
    }
  }, 30)
}

watch(
  () => props.show,
  (open) => {
    if (open) focusApply()
  },
  { immediate: true }
)

function onNotesClick(event) {
  const link = event.target?.closest?.('a')
  if (!link) return
  event.preventDefault()
  const href = link.getAttribute('href')
  if (href) emit('open-url', href)
}
</script>

<template>
  <n-modal
    :show="show"
    preset="card"
    :title="t('app.update.title')"
    style="width: min(640px, calc(100vw - 32px))"
    :mask-closable="!busy"
    :closable="!busy || phase !== 'restart'"
    @update:show="onShowChange"
    @after-enter="focusApply"
  >
    <p class="update-offer-message">
      {{
        canApply || error
          ? t('app.update.message', { latest, current })
          : t('app.update.messageBrowser', { latest, current })
      }}
    </p>

    <section class="update-notes" :aria-label="t('app.update.whatsNew')">
      <h3 class="update-notes-title">{{ t('app.update.whatsNew') }}</h3>
      <div
        v-if="safeNotes"
        class="update-notes-body"
        @click="onNotesClick"
        v-html="safeNotes"
      />
      <p v-else class="update-notes-empty">{{ t('app.update.notesEmpty') }}</p>
      <button
        v-if="showPageLink"
        type="button"
        class="update-release-link"
        @click="emit('open-url', pageUrl)"
      >
        {{ t('app.update.releasePage') }}
      </button>
    </section>

    <div v-if="phaseLabel" class="update-progress">
      <div class="update-progress-label">{{ phaseLabel }}</div>
      <n-progress
        type="line"
        :percentage="progressValue"
        :show-indicator="false"
        :processing="phase !== 'download'"
        :height="8"
        :border-radius="999"
      />
    </div>
    <p v-if="error" class="update-error" role="alert">{{ error }}</p>

    <template #footer>
      <div class="update-actions">
        <n-button quaternary :disabled="phase === 'restart'" @click="emit('skip')">
          {{ t('app.update.skip') }}
        </n-button>
        <div class="update-actions-main">
          <n-button :disabled="phase === 'restart'" @click="emit('close')">
            {{ t('app.update.later') }}
          </n-button>
          <n-button
            ref="applyButton"
            type="primary"
            :loading="busy"
            :disabled="busy"
            @click="emit('download')"
          >
            {{ canApply ? t('app.update.restart') : t('app.update.download') }}
          </n-button>
        </div>
      </div>
    </template>
  </n-modal>
</template>

<style scoped>
.update-offer-message,
.update-notes-empty,
.update-error {
  margin: 0;
}

.update-notes {
  margin-top: 14px;
  padding: 12px 14px;
  border: 1px solid var(--lt-border);
  border-radius: var(--lt-radius-md);
  background: var(--lt-surface-soft);
  max-height: 260px;
  overflow: auto;
}

.update-notes-title {
  margin: 0 0 8px;
  font-size: 14px;
  font-weight: 600;
  color: var(--lt-ink);
}

.update-notes-empty {
  color: var(--lt-muted);
  font-size: 13px;
}

.update-notes-body :deep(h1),
.update-notes-body :deep(h2),
.update-notes-body :deep(h3) {
  margin: 0 0 8px;
  font-size: 14px;
  line-height: 1.4;
}

.update-notes-body :deep(p),
.update-notes-body :deep(ul),
.update-notes-body :deep(ol) {
  margin: 0 0 8px;
  font-size: 13px;
  line-height: 1.5;
  color: var(--lt-ink);
}

.update-notes-body :deep(ul),
.update-notes-body :deep(ol) {
  padding-left: 1.2rem;
}

.update-notes-body :deep(a) {
  color: var(--lt-brand);
}

.update-notes-body :deep(code) {
  font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
  font-size: 12px;
}

.update-notes-body :deep(pre) {
  margin: 0 0 8px;
  padding: 8px 10px;
  overflow: auto;
  border-radius: 6px;
  background: var(--lt-inline-bg);
}

.update-release-link {
  margin-top: 4px;
  padding: 0;
  border: 0;
  background: none;
  color: var(--lt-brand);
  font: inherit;
  font-size: 13px;
  cursor: pointer;
  text-decoration: underline;
  text-underline-offset: 2px;
}

.update-progress {
  margin-top: 14px;
}

.update-progress-label {
  margin-bottom: 6px;
  font-size: 13px;
  color: var(--lt-muted);
}

.update-error {
  margin-top: 12px;
  color: var(--lt-danger-ink, #991b1b);
  font-size: 13px;
}

.update-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  align-items: center;
  width: 100%;
}

.update-actions-main {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-left: auto;
}
</style>
