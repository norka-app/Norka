<script setup>
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AIDebugResultCard from '../common/AIDebugResultCard.vue'
import AppSelect from '../common/AppSelect.vue'
import JumperFields from './JumperFields.vue'
import {
  BUTTON_GAP,
  FIELD_GAP,
  FORM_COLS,
  SPAN_FULL,
  SPAN_HALF,
  SPAN_THIRD,
  plainInputProps,
  requiredInputProps,
} from '../../theme/form-layout'

const props = defineProps({
  show: {
    type: Boolean,
    required: true
  },
  editingTunnelId: {
    type: Number,
    default: null
  },
  tunnelForm: {
    type: Object,
    required: true
  },
  tunnels: {
    type: Array,
    default: () => []
  },
  groups: {
    type: Array,
    default: () => []
  },
  modeOptions: {
    type: Array,
    required: true
  },
  jumpers: {
    type: Array,
    required: true
  },
  inlineJumperForm: {
    type: Object,
    required: true
  },
  authOptions: {
    type: Array,
    required: true
  },
  inlineJumperNeedsKeyFile: {
    type: Boolean,
    required: true
  },
  inlineJumperNeedsPassword: {
    type: Boolean,
    required: true
  },
  inlineJumperShowsPassword: {
    type: Boolean,
    required: true
  },
  jumperLimits: {
    type: Object,
    required: true
  },
  inlineJumperValidationError: {
    type: String,
    default: ''
  },
  tunnelValidationError: {
    type: String,
    default: ''
  },
  tunnelTest: {
    type: Object,
    required: true
  },
  tunnelAiDebug: {
    type: Object,
    required: true
  },
  aiDebugEnabled: {
    type: Boolean,
    default: false
  },
  sshCommandEnabled: {
    type: Boolean,
    default: false
  }
})

const emit = defineEmits([
  'close',
  'submit',
  'add-jumper',
  'remove-jumper',
  'set-primary-jumper',
  'move-jumper',
  'trim-jumpers-to-primary',
  'inline-key-file-change',
  'test-connection',
  'ai-debug',
  'report-ai-content',
  'from-ssh-command'
])

const { t } = useI18n()
const showMoreJumpers = ref(false)

const showInlineTestResult = computed(() => {
  if (!props.tunnelTest?.message) return false
  if (props.aiDebugEnabled && props.tunnelTest.status === 'error') return false
  return true
})

const localPortConflictTunnels = computed(() => {
  const port = Number(props.tunnelForm?.localPort)
  if (!Number.isInteger(port) || port < 1) return []
  const editingId = Number(props.editingTunnelId)
  return (Array.isArray(props.tunnels) ? props.tunnels : []).filter((tunnel) => {
    const tunnelPort = Number(tunnel?.localPort)
    if (!Number.isInteger(tunnelPort) || tunnelPort !== port) return false
    if (Number.isInteger(editingId) && editingId > 0 && Number(tunnel?.id) === editingId) return false
    return true
  })
})

const localPortConflictWarning = computed(() => {
  const conflicts = localPortConflictTunnels.value
  if (conflicts.length === 0) return ''
  const names = conflicts
    .map((tunnel) => String(tunnel?.name || '').trim())
    .filter(Boolean)
    .join(', ')
  return t('app.modals.tunnel.localPortInUseWarning', {
    port: Number(props.tunnelForm.localPort),
    names: names || String(conflicts.length)
  })
})

const selectedJumperIds = computed(() => {
  const rawIds = Array.isArray(props.tunnelForm?.jumperIds) ? props.tunnelForm.jumperIds : []
  const ids = []
  for (const value of rawIds) {
    const id = Number(value)
    if (!Number.isInteger(id) || id <= 0 || ids.includes(id)) continue
    ids.push(id)
  }
  return ids
})

const primaryJumperId = computed(() => {
  return selectedJumperIds.value.length > 0 ? selectedJumperIds.value[0] : ''
})

const additionalSelectedJumperIds = computed(() => {
  return selectedJumperIds.value.slice(1)
})

const availableAdditionalJumpers = computed(() => {
  const selectedSet = new Set(selectedJumperIds.value)
  return (Array.isArray(props.jumpers) ? props.jumpers : []).filter((jumper) => !selectedSet.has(Number(jumper.id)))
})

function jumperOption(jumper) {
  const label = `${jumper.name} (${jumper.user}@${jumper.host})`
  return { value: jumper.id, label, title: label }
}

const groupSelectOptions = computed(() => [
  { value: 0, label: t('app.tunnels.groups.ungrouped') },
  ...(Array.isArray(props.groups) ? props.groups : []).map((group) => ({ value: group.id, label: group.name })),
])

const jumperSelectOptions = computed(() => (Array.isArray(props.jumpers) ? props.jumpers : []).map(jumperOption))

const additionalJumperOptions = computed(() => availableAdditionalJumpers.value.map(jumperOption))

const showJumperChainEditor = computed(() => showMoreJumpers.value)

watch(
  () => props.show,
  (visible) => {
    if (!visible) {
      showMoreJumpers.value = false
      return
    }
    showMoreJumpers.value = additionalSelectedJumperIds.value.length > 0
  },
  { immediate: true }
)

watch(showMoreJumpers, (enabled) => {
  if (enabled) return
  if (!props.show) return
  if (selectedJumperIds.value.length <= 1) return
  emit('trim-jumpers-to-primary')
})

function getJumperById(jumperId) {
  const id = Number(jumperId)
  if (!Number.isInteger(id) || id <= 0) return null
  return (Array.isArray(props.jumpers) ? props.jumpers : []).find((item) => Number(item.id) === id) || null
}

function getJumperDisplayName(jumperId) {
  const jumper = getJumperById(jumperId)
  if (!jumper) return `#${jumperId}`
  return String(jumper.name || '').trim() || `#${jumperId}`
}

function getJumperConnectionLabel(jumperId) {
  const jumper = getJumperById(jumperId)
  if (!jumper) return ''
  return `${jumper.user}@${jumper.host}`
}

function getJumperTooltipLabel(jumperId) {
  const jumper = getJumperById(jumperId)
  if (!jumper) return `#${jumperId}`
  return `${jumper.name} (${jumper.user}@${jumper.host})`
}

function onPrimaryJumperChange(value) {
  const nextId = Number(value)
  if (!Number.isInteger(nextId) || nextId <= 0) return
  emit('set-primary-jumper', nextId)
}
</script>

<template>
  <n-modal
    :show="show"
    preset="card"
    class="app-modal tunnel-dialog"
    :title="editingTunnelId ? $t('app.modals.tunnel.editTitle') : $t('app.modals.tunnel.newTitle')"
    :style="{ width: 'min(760px, calc(100vw - 32px))' }"
    :mask-closable="false"
    :segmented="{ content: true, footer: 'soft' }"
    :content-style="{ maxHeight: 'calc(100vh - 180px)', overflow: 'auto' }"
    @update:show="(visible) => { if (!visible) $emit('close') }"
  >
    <n-form
      id="tunnel-form"
      class="kit-form"
      label-placement="top"
      :show-require-mark="false"
      :show-feedback="false"
      @submit.prevent="$emit('submit')"
    >
      <n-grid :cols="FORM_COLS" :x-gap="FIELD_GAP" :y-gap="FIELD_GAP" item-responsive>
        <n-form-item-gi :span="groups.length > 0 ? SPAN_THIRD : SPAN_HALF" :label="$t('app.modals.tunnel.name')" :show-feedback="false">
          <n-input v-model:value="tunnelForm.name" :maxlength="20" :input-props="requiredInputProps" />
        </n-form-item-gi>
        <n-form-item-gi v-if="groups.length > 0" :span="SPAN_THIRD" :label="$t('app.modals.tunnel.group')" :show-feedback="false">
          <AppSelect v-model="tunnelForm.groupId" :options="groupSelectOptions" />
        </n-form-item-gi>
        <n-form-item-gi :span="groups.length > 0 ? SPAN_THIRD : SPAN_HALF" :label="$t('app.modals.tunnel.mode')" :show-feedback="false">
          <AppSelect v-model="tunnelForm.mode" :options="modeOptions" />
        </n-form-item-gi>
        <n-form-item-gi :span="SPAN_FULL" :show-label="false" :show-feedback="false">
          <n-grid cols="1 520:3" :x-gap="FIELD_GAP" :y-gap="FIELD_GAP" item-responsive>
            <n-form-item-gi span="1 520:2" :label="$t('app.modals.tunnel.localHost')" :show-feedback="false">
              <n-input v-model:value="tunnelForm.localHost" :input-props="requiredInputProps" />
            </n-form-item-gi>
            <n-form-item-gi
              span="1 520:1"
              :label="$t('app.modals.tunnel.localPort')"
              :show-feedback="!!localPortConflictWarning"
              :feedback="localPortConflictWarning"
              :validation-status="localPortConflictWarning ? 'warning' : undefined"
            >
              <n-input-number
                v-model:value="tunnelForm.localPort"
                :min="1"
                :show-button="false"
                :status="localPortConflictWarning ? 'warning' : undefined"
                style="width: 100%"
              />
            </n-form-item-gi>
            <n-form-item-gi span="1 520:2" :label="$t('app.modals.tunnel.remoteHost')" :show-feedback="false">
              <n-input
                v-model:value="tunnelForm.remoteHost"
                :disabled="tunnelForm.mode === 'dynamic'"
                :input-props="tunnelForm.mode === 'dynamic' ? plainInputProps : requiredInputProps"
              />
            </n-form-item-gi>
            <n-form-item-gi span="1 520:1" :label="$t('app.modals.tunnel.remotePort')" :show-feedback="false">
              <n-input-number
                v-model:value="tunnelForm.remotePort"
                :min="1"
                :show-button="false"
                :disabled="tunnelForm.mode === 'dynamic'"
                style="width: 100%"
              />
            </n-form-item-gi>
          </n-grid>
        </n-form-item-gi>
        <n-form-item-gi :span="SPAN_FULL" :label="$t('app.modals.tunnel.jumpers')" :show-feedback="false">
          <div class="kit-inline">
            <n-tag>#1</n-tag>
            <AppSelect
              class="kit-grow"
              :model-value="primaryJumperId"
              :options="jumperSelectOptions"
              :disabled="jumpers.length === 0"
              :placeholder="$t('app.modals.tunnel.selectPrimaryJumperPlaceholder')"
              @update:model-value="onPrimaryJumperChange"
            />
          </div>
          <n-text depth="3" class="kit-note">{{ $t('app.modals.tunnel.primaryJumperHint') }}</n-text>
          <div class="kit-switch-line" :style="{ marginTop: FIELD_GAP + 'px' }">
            <span>{{ $t('app.modals.tunnel.addMoreJumpers') }}</span>
            <n-switch
              v-model:value="showMoreJumpers"
              :disabled="jumpers.length === 0"
              :aria-expanded="showJumperChainEditor"
            />
          </div>
        </n-form-item-gi>
        <n-gi v-if="showJumperChainEditor" :span="SPAN_FULL">
          <n-card size="small" embedded>
            <n-space align="center" :size="BUTTON_GAP">
              <span>{{ $t('app.modals.tunnel.selectedJumpers') }}</span>
              <n-tooltip>
                <template #trigger>
                  <n-button text size="tiny" aria-label="?">?</n-button>
                </template>
                {{ $t('app.modals.tunnel.selectedJumpersOrderTooltip') }}
              </n-tooltip>
            </n-space>
            <n-input-group :style="{ marginTop: FIELD_GAP + 'px' }">
              <AppSelect
                v-model="tunnelForm.nextJumperId"
                :options="additionalJumperOptions"
                :disabled="availableAdditionalJumpers.length === 0"
                :placeholder="$t('app.modals.tunnel.selectAdditionalJumperPlaceholder')"
              />
              <n-button
                :disabled="!tunnelForm.nextJumperId"
                :aria-label="$t('app.modals.tunnel.addJumper')"
                @click="$emit('add-jumper', tunnelForm.nextJumperId)"
              >
                <template #icon>
                  <i class="bi bi-plus-lg" />
                </template>
              </n-button>
            </n-input-group>
            <n-text depth="3" class="kit-note">{{ $t('app.modals.tunnel.jumpersHint') }}</n-text>
            <n-text v-if="selectedJumperIds.length === 0" depth="3" class="kit-note">
              {{ $t('app.modals.tunnel.noSelectedJumpers') }}
            </n-text>
            <n-list v-else bordered class="kit-jumper-list" :style="{ marginTop: FIELD_GAP + 'px' }">
              <n-list-item v-for="(jumperId, index) in selectedJumperIds" :key="`selected-jumper-${index}-${jumperId}`">
                <n-space align="center" :size="BUTTON_GAP" :wrap="false">
                  <n-tag round :bordered="false">{{ index + 1 }}</n-tag>
                  <div :title="getJumperTooltipLabel(jumperId)">
                    <div>
                      {{
                        getJumperById(jumperId)
                          ? getJumperDisplayName(jumperId)
                          : `${$t('app.options.jumper.unknown')} (#${jumperId})`
                      }}
                    </div>
                    <n-text v-if="getJumperById(jumperId)" depth="3">{{ getJumperConnectionLabel(jumperId) }}</n-text>
                  </div>
                </n-space>
                <template #suffix>
                  <n-button-group>
                    <n-button
                      :disabled="index === 0"
                      :aria-label="$t('app.modals.tunnel.moveJumperUp')"
                      @click="$emit('move-jumper', index, -1)"
                    >
                      <template #icon><i class="bi bi-arrow-up" /></template>
                    </n-button>
                    <n-button
                      :disabled="index === selectedJumperIds.length - 1"
                      :aria-label="$t('app.modals.tunnel.moveJumperDown')"
                      @click="$emit('move-jumper', index, 1)"
                    >
                      <template #icon><i class="bi bi-arrow-down" /></template>
                    </n-button>
                    <n-button
                      type="error"
                      quaternary
                      :aria-label="$t('app.modals.tunnel.removeJumper')"
                      @click="$emit('remove-jumper', index)"
                    >
                      <template #icon><i class="bi bi-trash3" /></template>
                    </n-button>
                  </n-button-group>
                </template>
              </n-list-item>
            </n-list>
          </n-card>
        </n-gi>
        <n-gi :span="SPAN_FULL">
          <div class="kit-switch-line">
            <span>{{ $t('app.modals.tunnel.appendNewJumper') }}</span>
            <n-switch v-model:value="tunnelForm.appendNewJumper" />
          </div>
        </n-gi>
      </n-grid>

      <n-card v-if="tunnelForm.appendNewJumper" size="small" :title="$t('app.modals.tunnel.quickCreateJumper')">
        <JumperFields
          :form="inlineJumperForm"
          :limits="jumperLimits"
          :auth-options="authOptions"
          :needs-key-file="inlineJumperNeedsKeyFile"
          :shows-password="inlineJumperShowsPassword"
          :password-placeholder="inlineJumperNeedsKeyFile || !inlineJumperNeedsPassword ? $t('app.modals.jumper.passwordOptionalPlaceholder') : $t('app.modals.jumper.passwordPlaceholder')"
          :password-required="inlineJumperNeedsPassword"
          @key-file-change="$emit('inline-key-file-change', $event)"
        />
        <div class="kit-switch-line" :style="{ marginTop: FIELD_GAP + 'px' }">
          <span>{{ $t('app.modals.jumper.bypassHostCheck') }}</span>
          <n-switch v-model:value="inlineJumperForm.bypassHostVerification" />
        </div>
        <n-alert v-if="inlineJumperValidationError" type="error" :show-icon="false" :style="{ marginTop: FIELD_GAP + 'px' }">
          {{ inlineJumperValidationError }}
        </n-alert>
      </n-card>

      <n-grid :cols="FORM_COLS" :x-gap="FIELD_GAP" :y-gap="FIELD_GAP" item-responsive>
        <n-form-item-gi :span="SPAN_FULL" :label="$t('app.modals.tunnel.description')" :show-feedback="false">
          <n-input
            v-model:value="tunnelForm.description"
            type="textarea"
            :rows="2"
            :input-props="plainInputProps"
          />
        </n-form-item-gi>
        <n-gi :span="SPAN_FULL">
          <div class="kit-switch-line">
            <span>{{ $t('app.modals.tunnel.autoStart') }}</span>
            <n-switch v-model:value="tunnelForm.autoStart" />
          </div>
        </n-gi>
      </n-grid>

      <n-alert v-if="tunnelValidationError" type="error" :show-icon="false">{{ tunnelValidationError }}</n-alert>

      <n-card v-if="aiDebugEnabled && tunnelTest.message && tunnelTest.status === 'error'" size="small">
        <n-space justify="space-between" align="start" :size="FIELD_GAP" :wrap="true">
          <div>
            <div>{{ $t('app.aiDebug.connectionFailed') }}</div>
            <n-text type="error">{{ tunnelTest.message }}</n-text>
            <n-text v-if="tunnelTest.debuggable" depth="3" class="kit-note">{{ $t('app.aiDebug.inlineHint') }}</n-text>
          </div>
          <n-button
            v-if="tunnelTest.debuggable"
            :disabled="tunnelAiDebug.status === 'analyzing'"
            @click="$emit('ai-debug')"
          >
            <template #icon>
              <i class="bi" :class="tunnelAiDebug.status === 'analyzing' ? 'bi-hourglass-split' : 'bi-magic'" />
            </template>
            {{ tunnelAiDebug.status === 'analyzing' ? $t('app.aiDebug.analyzing') : $t('app.aiDebug.action') }}
          </n-button>
        </n-space>
        <AIDebugResultCard
          v-if="tunnelAiDebug.status !== 'idle'"
          :state="tunnelAiDebug"
          show-actions
          @retry-debug="$emit('ai-debug')"
          @test-again="$emit('test-connection')"
          @report-content="$emit('report-ai-content')"
        />
      </n-card>
    </n-form>
    <template #footer>
      <n-space justify="space-between" align="center" :size="BUTTON_GAP" :wrap="true" style="width: 100%">
        <n-space :size="BUTTON_GAP" align="center" :wrap="true">
          <n-button
            v-if="sshCommandEnabled && !editingTunnelId"
            quaternary
            @click="$emit('from-ssh-command')"
          >
            {{ $t('app.header.fromSSHCommand') }}
          </n-button>
          <n-button
            :disabled="tunnelTest.status === 'testing'"
            @click="$emit('test-connection')"
          >
            {{ tunnelTest.status === 'testing' ? $t('app.modals.tunnel.testing') : $t('app.modals.tunnel.testConnection') }}
          </n-button>
          <n-text
            v-if="showInlineTestResult"
            :type="tunnelTest.status === 'error' ? 'error' : tunnelTest.status === 'success' ? 'success' : undefined"
          >
            {{ tunnelTest.message }}
          </n-text>
        </n-space>
        <n-space :size="BUTTON_GAP">
          <n-button @click="$emit('close')">{{ $t('app.common.cancel') }}</n-button>
          <n-button type="primary" attr-type="submit" form="tunnel-form">{{ $t('app.common.save') }}</n-button>
        </n-space>
      </n-space>
    </template>
  </n-modal>
</template>
