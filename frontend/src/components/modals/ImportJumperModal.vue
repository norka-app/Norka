<script setup>
import { computed, ref, watch } from 'vue'
import AppSelect from '../common/AppSelect.vue'
import { BUTTON_GAP, plainInputProps } from '../../theme/form-layout'
import { ChevronDown, ChevronUp } from '../../icons'

const props = defineProps({
  show: {
    type: Boolean,
    required: true
  },
  candidates: {
    type: Array,
    default: () => []
  },
  existingJumpers: {
    type: Array,
    default: () => []
  },
  authOptions: {
    type: Array,
    required: true
  },
  jumperLimits: {
    type: Object,
    required: true
  },
  sources: {
    type: Array,
    default: () => []
  },
  selectedSourcePath: {
    type: String,
    default: ''
  },
  loading: {
    type: Boolean,
    default: false
  },
  loadError: {
    type: String,
    default: ''
  },
  importError: {
    type: String,
    default: ''
  },
  hasLoaded: {
    type: Boolean,
    default: false
  }
})

const emit = defineEmits(['close', 'load', 'import', 'update:selectedSourcePath'])

const localValidationError = ref('')
const rows = ref([])
const expandedDetailIds = ref(new Set())

function toHostKey(host) {
  return String(host || '').trim().toLowerCase()
}

function getJumperSignature(item) {
  return `${toHostKey(item?.host)}|${String(item?.user || '').trim()}|${Number(item?.port) || 22}`
}

function nameUnits(text) {
  let units = 0
  for (const char of text || '') {
    units += /[\u3400-\u9fff\uf900-\ufaff]/.test(char) ? 2 : 1
  }
  return units
}

function getAuthLabel(authType) {
  return props.authOptions.find((item) => item.value === authType)?.label || authType
}

function buildRowWarnings(candidate) {
  const warnings = []
  if (Array.isArray(candidate.warnings) && candidate.warnings.includes('agent_fallback')) {
    warnings.push({ key: 'app.modals.importJumper.warningAgentFallback' })
  }
  return warnings
}

function rebuildRows() {
  const existingSignatures = new Set((Array.isArray(props.existingJumpers) ? props.existingJumpers : []).map((item) => getJumperSignature(item)))
  const seenBatch = new Set()
  const previousRows = new Map(rows.value.map((row) => [row.id, row]))

  rows.value = (Array.isArray(props.candidates) ? props.candidates : []).map((candidate, index) => {
    const signature = getJumperSignature(candidate)
    const duplicateExisting = existingSignatures.has(signature)
    const duplicateBatch = seenBatch.has(signature)
    seenBatch.add(signature)

    const importErrors = []
    if (!String(candidate?.user || '').trim()) {
      importErrors.push({ key: 'app.modals.importJumper.errorMissingUser' })
    }
    if (!String(candidate?.host || '').trim()) {
      importErrors.push({ key: 'app.modals.importJumper.errorMissingHost' })
    }
    if (String(candidate?.proxyJump || '').trim()) {
      importErrors.push({
        key: 'app.modals.importJumper.errorProxyJump',
        params: { proxyJump: candidate.proxyJump }
      })
    }
    if (duplicateExisting) {
      importErrors.push({ key: 'app.modals.importJumper.errorDuplicateExisting' })
    }
    if (duplicateBatch) {
      importErrors.push({ key: 'app.modals.importJumper.errorDuplicateBatch' })
    }

    const importWarnings = buildRowWarnings(candidate)
    const importStatus = importErrors.length > 0 ? 'error' : 'success'
    const rowID = `ssh-config-jumper-${index}-${candidate.alias}`
    const previousRow = previousRows.get(rowID)
    const defaultSelected = importStatus === 'success'
    const preservedSelected = previousRow && previousRow.importStatus === importStatus
      ? !!previousRow.selected
      : defaultSelected

    return {
      ...candidate,
      id: rowID,
      name: previousRow?.name ?? candidate.name,
      selected: preservedSelected,
      importStatus,
      importErrors,
      importWarnings
    }
  })
}

const selectedRows = computed(() => rows.value.filter((row) => row.selected && row.importStatus === 'success'))
const selectableRows = computed(() => rows.value.filter((row) => row.importStatus === 'success'))
const hasSelectableRows = computed(() => selectableRows.value.length > 0)
const hasRows = computed(() => rows.value.length > 0)
const allSelectableSelected = computed(() => hasSelectableRows.value && selectableRows.value.every((row) => row.selected))
const canLoad = computed(() => !!String(props.selectedSourcePath || '').trim() && !props.loading)
const footerSummary = computed(() => {
  if (!props.hasLoaded || props.loadError) return ''
  return `Success! ${rows.value.length} found, ${selectableRows.value.length} ready to import`
})

function getImportStatusLabelKey(row) {
  return row.importStatus === 'error'
    ? 'app.modals.importJumper.statusError'
    : 'app.modals.importJumper.statusReady'
}

function getImportStatusBadgeClass(row) {
  return row.importStatus === 'error' ? 'error' : 'running'
}

function canToggleDetails(row) {
  return (Array.isArray(row.importErrors) && row.importErrors.length > 0) ||
    (Array.isArray(row.importWarnings) && row.importWarnings.length > 0)
}

function isDetailExpanded(rowId) {
  return expandedDetailIds.value.has(rowId)
}

function toggleDetails(row) {
  if (!canToggleDetails(row)) return
  const next = new Set(expandedDetailIds.value)
  if (next.has(row.id)) {
    next.delete(row.id)
  } else {
    next.add(row.id)
  }
  expandedDetailIds.value = next
}

function handleImport() {
  localValidationError.value = ''

  if (selectedRows.value.length === 0) {
    localValidationError.value = 'Please select at least one jumper to import.'
    return
  }

  const names = new Set()
  for (const row of selectedRows.value) {
    const name = String(row.name || '').trim()
    if (!name) {
      localValidationError.value = 'Jumper name is required.'
      return
    }
    if (nameUnits(name) > props.jumperLimits.name) {
      localValidationError.value = `Jumper name must be <= ${props.jumperLimits.name} chars or <= ${Math.floor(props.jumperLimits.name / 2)} Chinese chars.`
      return
    }
    const key = name.toLowerCase()
    if (names.has(key)) {
      localValidationError.value = 'Duplicate jumper names found. Please make each name unique.'
      return
    }
    names.add(key)
  }

  emit('import', selectedRows.value.map((row) => ({
    ...row,
    name: String(row.name || '').trim()
  })))
}

function setAllSelected(checked) {
  for (const row of rows.value) {
    row.selected = checked && row.importStatus === 'success'
  }
}

function handleClose() {
  localValidationError.value = ''
  expandedDetailIds.value = new Set()
  emit('close')
}

watch(
  () => [props.candidates, props.existingJumpers],
  () => {
    const previousExpandedIds = new Set(expandedDetailIds.value)
    rebuildRows()
    const currentIds = new Set(rows.value.map((row) => row.id))
    expandedDetailIds.value = new Set(
      [...previousExpandedIds].filter((id) => currentIds.has(id))
    )
    localValidationError.value = ''
  },
  { deep: true, immediate: true }
)

watch(
  () => props.show,
  (visible) => {
    if (!visible) {
      localValidationError.value = ''
      expandedDetailIds.value = new Set()
    }
  }
)
</script>

<template>
  <n-modal
    :show="show"
    preset="card"
    class="app-modal import-jumper-dialog"
    :title="$t('app.modals.importJumper.title')"
    :style="{ width: 'min(1020px, calc(100vw - 32px))' }"
    :mask-closable="false"
    :segmented="{ content: true, footer: 'soft' }"
    :content-style="{ maxHeight: 'calc(100vh - 180px)', overflow: 'auto' }"
    @update:show="(visible) => { if (!visible) handleClose() }"
  >
      <n-form class="kit-form" label-placement="top" :show-feedback="false" :show-require-mark="false">
        <n-form-item :label="$t('app.modals.importJumper.source')" :show-feedback="false">
          <n-input-group>
            <AppSelect
              :model-value="selectedSourcePath"
              :options="sources.map((source) => ({ value: source.path, label: source.label }))"
              @update:model-value="$emit('update:selectedSourcePath', $event)"
            />
            <n-button :disabled="!canLoad" :loading="loading" @click="$emit('load')">
              {{ loading ? $t('app.modals.importJumper.loading') : $t('app.modals.importJumper.load') }}
            </n-button>
          </n-input-group>
        </n-form-item>

        <n-alert v-if="loadError" type="error" :show-icon="false">{{ loadError }}</n-alert>
        <n-alert v-if="importError && importError !== loadError" type="error" :show-icon="false">{{ importError }}</n-alert>
        <n-alert v-if="localValidationError" type="error" :show-icon="false">{{ localValidationError }}</n-alert>

        <n-space v-if="loading" align="center" :size="BUTTON_GAP">
          <n-spin size="small" />
          <span>{{ $t('app.modals.importJumper.loading') }}</span>
        </n-space>

        <template v-else-if="hasLoaded">
          <div v-if="!hasRows" class="import-jumper-empty-state">
            <div class="import-jumper-empty-title">{{ $t('app.modals.importJumper.emptyTitle') }}</div>
            <n-text depth="3" class="kit-note">{{ $t('app.modals.importJumper.emptyDesc') }}</n-text>
          </div>

          <div v-else class="kit-section">
            <n-text strong>{{ $t('app.modals.importJumper.loadResult') }}</n-text>
            <div class="table-responsive parsed-tunnels-table">
            <table class="table align-middle mb-0 tunnels-table import-jumpers-table">
              <thead>
                <tr>
                  <th class="text-center import-select-col">
                    <n-checkbox :checked="allSelectableSelected" @update:checked="setAllSelected" />
                  </th>
                  <th class="import-jumper-name-col">{{ $t('app.modals.importJumper.jumperName') }}</th>
                  <th class="import-jumper-conn-col">{{ $t('app.jumpers.table.connection') }}</th>
                  <th class="import-jumper-auth-col">{{ $t('app.jumpers.table.auth') }}</th>
                  <th class="import-jumper-status-col">{{ $t('app.tunnels.table.status') }}</th>
                </tr>
              </thead>
              <tbody>
                <template v-for="row in rows" :key="row.id">
                  <tr :class="{ 'import-row-error': row.importStatus === 'error' }">
                    <td class="text-center">
                      <n-checkbox v-model:checked="row.selected" :disabled="row.importStatus === 'error'" />
                    </td>
                    <td class="import-jumper-name-col">
                      <n-input
                        v-model:value="row.name"
                        :disabled="row.importStatus === 'error'"
                        :input-props="plainInputProps"
                      />
                      <n-text depth="3" class="kit-note">
                        {{ $t('app.modals.importJumper.aliasLabel', { alias: row.alias }) }}
                      </n-text>
                    </td>
                    <td class="import-jumper-conn-col">
                      <div class="cell-ellipsis" :title="`${row.user}@${row.host}:${row.port}`">
                        {{ row.user || '--' }}@{{ row.host || '--' }}:{{ row.port || '--' }}
                      </div>
                      <div v-if="row.sourcePath" class="text-muted small cell-ellipsis" :title="row.sourcePath">
                        {{ row.sourcePath }}
                      </div>
                    </td>
                    <td class="import-jumper-auth-col">
                      <div class="cell-ellipsis" :title="getAuthLabel(row.authType)">{{ getAuthLabel(row.authType) }}</div>
                      <div v-if="row.keyPath" class="text-muted small cell-ellipsis" :title="row.keyPath">
                        {{ row.keyPath }}
                      </div>
                      <div v-else-if="row.agentSocketPath" class="text-muted small cell-ellipsis" :title="row.agentSocketPath">
                        {{ row.agentSocketPath }}
                      </div>
                      <n-text v-else-if="row.importWarnings.length > 0" depth="3" class="kit-note">
                        {{ $t(row.importWarnings[0].key, row.importWarnings[0].params || {}) }}
                      </n-text>
                    </td>
                    <td class="import-jumper-status-col">
                      <div class="tunnel-status-wrap">
                        <span
                          class="status-badge"
                          :class="[getImportStatusBadgeClass(row), { 'status-badge-expandable': canToggleDetails(row) }]"
                          :role="canToggleDetails(row) ? 'button' : undefined"
                          :tabindex="canToggleDetails(row) ? 0 : undefined"
                          :aria-expanded="canToggleDetails(row) ? isDetailExpanded(row.id) : undefined"
                          @click="toggleDetails(row)"
                          @keydown.enter.prevent="toggleDetails(row)"
                          @keydown.space.prevent="toggleDetails(row)"
                        >
                          <span>{{ $t(getImportStatusLabelKey(row)) }}</span>
                          <n-icon aria-hidden="true"
                            v-if="canToggleDetails(row)"
                            class="status-badge-toggle-icon"
                            :component="isDetailExpanded(row.id) ? ChevronUp : ChevronDown"
                          />
                        </span>
                      </div>
                    </td>
                  </tr>

                  <tr v-if="isDetailExpanded(row.id)" class="tunnel-error-detail-row">
                    <td colspan="5">
                      <div class="tunnel-error-detail">
                        <div class="tunnel-error-detail-content">
                          <div class="tunnel-error-detail-label">{{ $t('app.modals.importJumper.detailLabel') }}</div>
                          <div class="tunnel-error-detail-message">
                            <div v-for="(item, index) in row.importErrors" :key="`row-error-${row.id}-${index}`">
                              {{ $t(item.key, item.params || {}) }}
                            </div>
                            <div v-for="(item, index) in row.importWarnings" :key="`row-warning-${row.id}-${index}`">
                              {{ $t(item.key, item.params || {}) }}
                            </div>
                          </div>
                        </div>
                      </div>
                    </td>
                  </tr>
                </template>
              </tbody>
            </table>
            </div>
          </div>
        </template>
      </n-form>
    <template #footer>
      <n-space justify="space-between" align="center" :size="BUTTON_GAP" :wrap="true" style="width: 100%">
        <n-text v-if="footerSummary" depth="3">{{ footerSummary }}</n-text>
        <span v-else />
        <n-space :size="BUTTON_GAP">
          <n-button @click="handleClose">
            {{ $t('app.common.cancel') }}
          </n-button>
          <n-button
            type="primary"
            :disabled="loading || !hasSelectableRows || selectedRows.length === 0"
            @click="handleImport"
          >
            {{ $t('app.modals.importJumper.importBtn', { count: selectedRows.length }) }}
          </n-button>
        </n-space>
      </n-space>
    </template>
  </n-modal>
</template>

<style scoped>
.import-jumper-dialog {
  max-width: 1020px;
}

.import-jumper-empty-state {
  padding: 1.2rem 0.4rem 0.8rem;
  text-align: center;
}

.import-jumper-empty-title {
  font-size: 0.92rem;
  font-weight: 700;
  color: var(--lt-ink-strong);
}

.import-jumpers-table .import-select-col {
  width: 40px;
  max-width: 40px;
}

.import-jumpers-table .import-jumper-name-col {
  width: 220px;
  max-width: 220px;
}

.import-jumpers-table .import-jumper-conn-col {
  width: auto;
}

.import-jumpers-table .import-jumper-auth-col {
  width: 180px;
  max-width: 180px;
}

.import-jumpers-table .import-jumper-status-col {
  width: 124px;
  max-width: 124px;
}
</style>
