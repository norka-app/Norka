<script setup>
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { PreviewSSHCommands } from '../../../wailsjs/go/main/App'
import { BUTTON_GAP, SECTION_GAP, plainInputProps } from '../../theme/form-layout'

const props = defineProps({
  show: {
    type: Boolean,
    required: true,
  },
  modeOptions: {
    type: Array,
    required: true,
  },
  nameMax: {
    type: Number,
    default: 20,
  },
  importError: {
    type: String,
    default: '',
  },
})

const emit = defineEmits(['close', 'import'])
const { t } = useI18n()

const sshCommand = ref('')
const parseError = ref('')
const parsing = ref(false)
const warnings = ref([])
const hosts = ref([])
const tunnels = ref([])

const hasPreview = computed(() => tunnels.value.length > 0 || hosts.value.length > 0)
const selectableTunnels = computed(() => tunnels.value.filter((tunnel) => !tunnel.blocked))
const hasSelection = computed(() => selectableTunnels.value.some((tunnel) => tunnel.selected))

function resetFormState() {
  sshCommand.value = ''
  parseError.value = ''
  parsing.value = false
  warnings.value = []
  hosts.value = []
  tunnels.value = []
}

function handleClose() {
  resetFormState()
  emit('close')
}

function warningText(item) {
  const code = item?.code || 'unknown_option'
  return t(`app.modals.importTunnel.warnings.${code}`, {
    option: item?.option || '',
    detail: item?.detail || '',
  })
}

function nameUnits(text) {
  let units = 0
  for (const char of text || '') {
    units += /[\u3400-\u9fff\uf900-\ufaff]/.test(char) ? 2 : 1
  }
  return units
}

async function handleParse() {
  parseError.value = ''
  warnings.value = []
  hosts.value = []
  tunnels.value = []
  if (!sshCommand.value.trim()) {
    parseError.value = t('app.modals.importTunnel.parseEmpty')
    return
  }
  parsing.value = true
  try {
    const preview = await PreviewSSHCommands(sshCommand.value)
    warnings.value = Array.isArray(preview?.warnings) ? preview.warnings : []
    hosts.value = (Array.isArray(preview?.hosts) ? preview.hosts : []).map((host) => ({
      ...host,
      name: host.existingId ? (host.existingName || host.name) : (host.name || ''),
    }))
    tunnels.value = (Array.isArray(preview?.tunnels) ? preview.tunnels : []).map((tunnel, index) => ({
      ...tunnel,
      id: `ssh-${index}`,
      selected: !tunnel.blocked,
    }))
    if (tunnels.value.length === 0) {
      parseError.value = t('app.modals.importTunnel.nothing')
    }
  } catch (err) {
    const message = typeof err === 'string' ? err : err?.message
    parseError.value = message || t('app.modals.importTunnel.parseFailed')
  } finally {
    parsing.value = false
  }
}

function hostStatusKey(host) {
  if (!host.ready && !host.existingId) return 'app.modals.importTunnel.statusBlocked'
  if (host.existingId) return 'app.modals.importTunnel.statusReuse'
  return 'app.modals.importTunnel.statusCreate'
}

function getModeLabel(modeValue) {
  return props.modeOptions.find((mode) => mode.value === modeValue)?.label || modeValue
}

function routeTop(tunnel) {
  if (tunnel.mode === 'dynamic') return `${tunnel.localHost}:${tunnel.localPort}`
  if (tunnel.mode === 'remote') return `${tunnel.remoteHost}:${tunnel.remotePort}`
  return `${tunnel.localHost}:${tunnel.localPort}`
}

function routeBottom(tunnel) {
  if (tunnel.mode === 'dynamic') return 'SOCKS5'
  if (tunnel.mode === 'remote') return `${tunnel.localHost}:${tunnel.localPort}`
  return `${tunnel.remoteHost}:${tunnel.remotePort}`
}

function handleImport() {
  parseError.value = ''
  const selected = tunnels.value.filter((tunnel) => tunnel.selected && !tunnel.blocked)
  if (selected.length === 0) {
    parseError.value = t('app.modals.importTunnel.nothing')
    return
  }
  const names = []
  for (const host of hosts.value) {
    if (host.existingId) continue
    const name = String(host.name || '').trim()
    if (!name || nameUnits(name) > props.nameMax) {
      parseError.value = t('app.modals.importTunnel.nameInvalid', { max: props.nameMax })
      return
    }
    names.push(name.toLowerCase())
  }
  const tunnelNames = selected.map((tunnel) => String(tunnel.name || '').trim())
  if (tunnelNames.some((name) => !name || nameUnits(name) > props.nameMax)) {
    parseError.value = t('app.modals.importTunnel.nameInvalid', { max: props.nameMax })
    return
  }
  if (new Set(tunnelNames.map((name) => name.toLowerCase())).size !== tunnelNames.length) {
    parseError.value = t('app.modals.importTunnel.warnings.duplicate_batch')
    return
  }
  emit('import', {
    hosts: hosts.value.map((host) => ({ ...host, name: String(host.name || '').trim() })),
    tunnels: selected.map((tunnel) => ({ ...tunnel, name: String(tunnel.name || '').trim() })),
  })
}

watch(
  () => props.show,
  (visible) => {
    if (!visible) resetFormState()
  },
)
</script>

<template>
  <n-modal
    :show="show"
    preset="card"
    class="app-modal import-tunnel-dialog"
    :title="$t('app.header.fromSSHCommand')"
    :style="{ width: 'min(860px, calc(100vw - 32px))' }"
    :mask-closable="false"
    :segmented="{ content: true, footer: 'soft' }"
    :content-style="{ maxHeight: 'calc(100vh - 180px)', overflow: 'auto' }"
    @update:show="(visible) => { if (!visible) handleClose() }"
  >
    <n-form class="kit-form" label-placement="top" :show-feedback="false" :show-require-mark="false">
      <n-form-item :label="$t('app.modals.importTunnel.sshCommand')" :show-feedback="false">
        <n-input
          v-model:value="sshCommand"
          class="kit-mono"
          type="textarea"
          :rows="4"
          :placeholder="$t('app.modals.importTunnel.sshCommandPlaceholder')"
          :input-props="plainInputProps"
        />
        <n-text depth="3" class="kit-note">{{ $t('app.modals.importTunnel.hint') }}</n-text>
      </n-form-item>
      <n-space justify="end" :size="BUTTON_GAP">
        <n-button secondary :loading="parsing" @click="handleParse">
          {{ $t('app.modals.importTunnel.parseCmd') }}
        </n-button>
      </n-space>

      <n-alert v-if="parseError" type="error" :show-icon="false">{{ parseError }}</n-alert>
      <n-alert v-if="props.importError" type="error" :show-icon="false">{{ props.importError }}</n-alert>
      <n-alert v-if="warnings.length > 0" type="warning" :show-icon="false">
        <div v-for="(item, index) in warnings" :key="`warn-${index}`">
          {{ warningText(item) }}
        </div>
      </n-alert>

      <div v-if="hosts.length > 0" class="kit-section" :style="{ marginTop: SECTION_GAP + 'px' }">
        <n-text strong>{{ $t('app.modals.importTunnel.hostsTitle') }}</n-text>
        <div class="table-responsive parsed-tunnels-table">
          <table class="table align-middle mb-0 tunnels-table import-tunnels-table">
            <thead>
              <tr>
                <th>{{ $t('app.modals.importTunnel.hostName') }}</th>
                <th>{{ $t('app.modals.importTunnel.route') }}</th>
                <th>{{ $t('app.modals.importTunnel.hostStatus') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="host in hosts" :key="host.key">
                <td>
                  <n-input
                    v-if="!host.existingId"
                    v-model:value="host.name"
                    :maxlength="nameMax"
                    :input-props="plainInputProps"
                  />
                  <span v-else>{{ host.existingName || host.name }}</span>
                  <n-text v-if="host.alias" depth="3" class="kit-note">{{ $t('app.modals.importTunnel.alias', { alias: host.alias }) }}</n-text>
                </td>
                <td>
                  <div>{{ host.user }}@{{ host.host }}:{{ host.port }}</div>
                  <n-text v-if="host.keyPath" depth="3" class="kit-note">{{ $t('app.modals.importTunnel.keyPath', { path: host.keyPath }) }}</n-text>
                </td>
                <td>{{ $t(hostStatusKey(host)) }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>

      <div v-if="tunnels.length > 0" class="kit-section" :style="{ marginTop: SECTION_GAP + 'px' }">
        <n-text strong>{{ $t('app.modals.importTunnel.parsedTunnels') }}</n-text>
        <div class="table-responsive parsed-tunnels-table">
          <table class="table align-middle mb-0 tunnels-table import-tunnels-table">
            <thead>
              <tr>
                <th class="import-select-col" />
                <th>{{ $t('app.modals.importTunnel.tunnelName') }}</th>
                <th>{{ $t('app.modals.importTunnel.mode') }}</th>
                <th>{{ $t('app.modals.importTunnel.route') }}</th>
                <th>{{ $t('app.modals.importTunnel.chain') }}</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="tunnel in tunnels" :key="tunnel.id" :class="{ 'import-row-error': tunnel.blocked }">
                <td class="text-center">
                  <n-checkbox v-model:checked="tunnel.selected" :disabled="tunnel.blocked" />
                </td>
                <td>
                  <n-input
                    v-model:value="tunnel.name"
                    :maxlength="nameMax"
                    :disabled="tunnel.blocked"
                    :input-props="plainInputProps"
                  />
                </td>
                <td class="tunnel-mode-cell">{{ getModeLabel(tunnel.mode) }}</td>
                <td>
                  <div>{{ routeTop(tunnel) }}</div>
                  <n-text depth="3" class="kit-note">{{ routeBottom(tunnel) }}</n-text>
                </td>
                <td>{{ tunnel.chainLabel }}</td>
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </n-form>
    <template #footer>
      <n-space justify="end" :size="BUTTON_GAP" style="width: 100%">
        <n-button @click="handleClose">{{ $t('app.common.cancel') }}</n-button>
        <n-button type="primary" :disabled="!hasPreview || !hasSelection" @click="handleImport">
          {{ $t('app.modals.importTunnel.importBtn') }}
        </n-button>
      </n-space>
    </template>
  </n-modal>
</template>

<style scoped>
.parsed-tunnels-table {
  max-height: 220px;
  overflow: auto;
}

.import-tunnels-table .import-select-col {
  width: 40px;
}

.import-parse-error {
  margin-top: 0.2rem;
  padding: 0.48rem 0.62rem;
  border: 1px solid var(--lt-danger-border);
  border-radius: 7px;
  background: var(--lt-danger-bg);
  font-size: 0.76rem;
  line-height: 1.35;
}

.import-parse-warnings {
  border: 1px solid var(--lt-warning-border);
  border-radius: 7px;
  background: var(--lt-warning-bg);
  color: var(--lt-warning-ink);
  padding: 0.45rem 0.62rem;
  font-size: 0.75rem;
}

.import-parse-warning-item + .import-parse-warning-item {
  margin-top: 0.18rem;
}

.import-row-error td {
  background: rgba(239, 200, 141, 0.16);
}
</style>
