<script setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import AIDebugResultCard from '../common/AIDebugResultCard.vue'
import JumperFields from './JumperFields.vue'
import { BUTTON_GAP, FIELD_GAP } from '../../theme/form-layout'

const props = defineProps({
  show: {
    type: Boolean,
    required: true
  },
  editingJumperId: {
    type: Number,
    default: null
  },
  jumperForm: {
    type: Object,
    required: true
  },
  showJumperBasic: {
    type: Boolean,
    required: true
  },
  showJumperAdvanced: {
    type: Boolean,
    required: true
  },
  authOptions: {
    type: Array,
    required: true
  },
  jumperNeedsKeyFile: {
    type: Boolean,
    required: true
  },
  jumperNeedsPassword: {
    type: Boolean,
    required: true
  },
  jumperShowsPassword: {
    type: Boolean,
    required: true
  },
  jumperLimits: {
    type: Object,
    required: true
  },
  jumperValidationError: {
    type: String,
    default: ''
  },
  jumperTest: {
    type: Object,
    required: true
  },
  jumperAiDebug: {
    type: Object,
    required: true
  },
  aiDebugEnabled: {
    type: Boolean,
    default: false
  }
})

defineEmits(['close', 'submit', 'toggle-basic', 'toggle-advanced', 'key-file-change', 'test-connection', 'ai-debug', 'report-ai-content'])

const { t } = useI18n()
const keepsStoredSecret = computed(() => !!props.jumperForm?.hasSecret || Number(props.jumperForm?.secretSourceId) > 0)
const passwordPlaceholder = computed(() => {
  if (keepsStoredSecret.value) return t('app.modals.jumper.passwordKeepPlaceholder')
  if (props.jumperNeedsPassword) return t('app.modals.jumper.passwordPlaceholder')
  return t('app.modals.jumper.passwordOptionalPlaceholder')
})
</script>

<template>
  <n-modal
    :show="show"
    preset="card"
    class="app-modal jumper-dialog"
    :title="editingJumperId ? $t('app.modals.jumper.editTitle') : $t('app.modals.jumper.newTitle')"
    :style="{ width: 'min(720px, calc(100vw - 32px))' }"
    :mask-closable="false"
    :segmented="{ content: true, footer: 'soft' }"
    :content-style="{ maxHeight: 'calc(100vh - 180px)', overflow: 'auto' }"
    @update:show="(visible) => { if (!visible) $emit('close') }"
  >
    <n-form
      id="jumper-form"
      class="kit-form"
      label-placement="top"
      :show-require-mark="false"
      :show-feedback="false"
      @submit.prevent="$emit('submit')"
    >
      <div class="kit-section">
        <n-button text class="kit-disclosure" :aria-expanded="showJumperBasic" @click="$emit('toggle-basic')">
          <template #icon>
            <i class="bi bi-chevron-right kit-chevron" :class="{ 'is-open': showJumperBasic }" aria-hidden="true" />
          </template>
          {{ $t('app.modals.jumper.basicSettings') }}
        </n-button>
        <JumperFields
          v-if="showJumperBasic"
          :form="jumperForm"
          :limits="jumperLimits"
          :auth-options="authOptions"
          :needs-key-file="jumperNeedsKeyFile"
          :shows-password="jumperShowsPassword"
          :password-placeholder="passwordPlaceholder"
          :password-required="jumperNeedsPassword && !keepsStoredSecret"
          :show-stored-hint="keepsStoredSecret"
          show-notes
          @key-file-change="$emit('key-file-change', $event)"
        />
      </div>

      <n-space align="center" :size="BUTTON_GAP">
        <div class="kit-switch-line">
          <span>{{ $t('app.modals.jumper.bypassHostCheck') }}</span>
          <n-switch v-model:value="jumperForm.bypassHostVerification" />
        </div>
        <n-tooltip>
          <template #trigger>
            <n-button text size="tiny">i</n-button>
          </template>
          {{ $t('app.modals.jumper.bypassTooltip') }}
        </n-tooltip>
      </n-space>

      <div class="kit-section">
        <n-button text class="kit-disclosure" :aria-expanded="showJumperAdvanced" @click="$emit('toggle-advanced')">
          <template #icon>
            <i class="bi bi-chevron-right kit-chevron" :class="{ 'is-open': showJumperAdvanced }" aria-hidden="true" />
          </template>
          {{ $t('app.modals.jumper.advancedOptions') }}
        </n-button>
        <n-grid v-if="showJumperAdvanced" cols="1 640:2" :x-gap="FIELD_GAP" :y-gap="FIELD_GAP" item-responsive>
          <n-form-item-gi span="1 640:1" :show-feedback="false">
            <template #label>
              <n-space align="center" :size="BUTTON_GAP" :wrap="false">
                <span>{{ $t('app.modals.jumper.keepAliveInterval') }}</span>
                <n-tooltip>
                  <template #trigger>
                    <n-button text size="tiny" aria-label="?">?</n-button>
                  </template>
                  {{ $t('app.modals.jumper.keepAliveIntervalTooltip') }}
                </n-tooltip>
              </n-space>
            </template>
            <n-input-number
              v-model:value="jumperForm.keepAliveIntervalMs"
              :min="jumperLimits.keepAliveIntervalMin"
              :max="jumperLimits.keepAliveIntervalMax"
              :step="1000"
              :show-button="false"
              style="width: 100%"
            />
          </n-form-item-gi>
          <n-form-item-gi span="1 640:1" :label="$t('app.modals.jumper.timeout')" :show-feedback="false">
            <n-input-number
              v-model:value="jumperForm.timeoutMs"
              :min="jumperLimits.timeoutMin"
              :max="jumperLimits.timeoutMax"
              :step="100"
              :show-button="false"
              style="width: 100%"
            />
          </n-form-item-gi>
        </n-grid>
      </div>

      <n-alert v-if="jumperValidationError" type="error" :show-icon="false">{{ jumperValidationError }}</n-alert>
      <n-text
        v-if="(!aiDebugEnabled && jumperTest.message) || (aiDebugEnabled && jumperTest.message && jumperTest.status !== 'error')"
        :type="jumperTest.status === 'error' ? 'error' : jumperTest.status === 'success' ? 'success' : undefined"
      >
        {{ jumperTest.message }}
      </n-text>
      <n-card v-if="aiDebugEnabled && jumperTest.message && jumperTest.status === 'error'" size="small">
        <n-space justify="space-between" align="start" :size="FIELD_GAP" :wrap="true">
          <div>
            <div>{{ $t('app.aiDebug.connectionFailed') }}</div>
            <n-text type="error">{{ jumperTest.message }}</n-text>
            <n-text v-if="jumperTest.debuggable" depth="3" class="kit-note">{{ $t('app.aiDebug.inlineHint') }}</n-text>
          </div>
          <n-button
            v-if="jumperTest.debuggable"
            :disabled="jumperAiDebug.status === 'analyzing'"
            @click="$emit('ai-debug')"
          >
            <template #icon>
              <i class="bi" :class="jumperAiDebug.status === 'analyzing' ? 'bi-hourglass-split' : 'bi-magic'" />
            </template>
            {{ jumperAiDebug.status === 'analyzing' ? $t('app.aiDebug.analyzing') : $t('app.aiDebug.action') }}
          </n-button>
        </n-space>
        <AIDebugResultCard
          v-if="jumperAiDebug.status !== 'idle'"
          :state="jumperAiDebug"
          show-actions
          @retry-debug="$emit('ai-debug')"
          @test-again="$emit('test-connection')"
          @report-content="$emit('report-ai-content')"
        />
      </n-card>
    </n-form>
    <template #footer>
      <n-space justify="space-between" align="center" :size="BUTTON_GAP" :wrap="true" style="width: 100%">
        <n-button
          :disabled="jumperTest.status === 'testing'"
          @click="$emit('test-connection')"
        >
          {{ jumperTest.status === 'testing' ? $t('app.modals.jumper.testing') : $t('app.modals.jumper.testConnection') }}
        </n-button>
        <n-space :size="BUTTON_GAP">
          <n-button @click="$emit('close')">{{ $t('app.common.cancel') }}</n-button>
          <n-button type="primary" attr-type="submit" form="jumper-form">{{ $t('app.common.save') }}</n-button>
        </n-space>
      </n-space>
    </template>
  </n-modal>
</template>
