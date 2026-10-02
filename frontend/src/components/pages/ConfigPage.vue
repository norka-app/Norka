<script setup>
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  GetAutoRunEnabled,
  GetTrafficMonitorEnabled,
  SetAutoRunEnabled,
  SetTrafficMonitorEnabled,
  ExportConfigWithDialog,
  SelectImportFile,
  ImportConfig,
  OpenConfigDir,
  GetConfigLocationInfo,
  GetConfigDirTargetConflict,
  SelectConfigDirectory,
  SetConfigDirectory,
  ResetConfigDirectoryToDefault,
  QuitApplication
} from '../../../wailsjs/go/main/App'

defineProps({
  theme: {
    type: String,
    required: true
  },
  appMeta: {
    type: Object,
    required: true
  },
  windowMode: {
    type: String,
    default: 'advanced'
  },
  simpleOnTop: {
    type: Boolean,
    default: false
  }
})

const emit = defineEmits([
  'theme-change',
  'set-config-message',
  'reload-state',
  'confirm-action',
  'traffic-monitor-change',
  'window-mode-change',
  'simple-on-top-change'
])

const { t } = useI18n()
const autoRunEnabled = ref(false)
const trafficMonitorEnabled = ref(true)
const configBusy = ref('')
const configLocationInfo = ref(null)

onMounted(async () => {
  try {
    autoRunEnabled.value = await GetAutoRunEnabled()
  } catch (_) {
    autoRunEnabled.value = false
  }
  try {
    trafficMonitorEnabled.value = await GetTrafficMonitorEnabled()
  } catch (_) {
    trafficMonitorEnabled.value = true
  }
  await loadConfigLocation()
})

async function loadConfigLocation() {
  try {
    configLocationInfo.value = await GetConfigLocationInfo()
  } catch (_) {
    configLocationInfo.value = null
  }
}

function hasConfigDirFileConflict(conflict) {
  return !!(conflict?.hasConfigToml || conflict?.hasUILocale)
}

async function getConfigDirTargetConflictSafe(dir) {
  try {
    return { ok: true, conflict: await GetConfigDirTargetConflict(dir) }
  } catch (e) {
    return { ok: false, error: String(e) }
  }
}

/** Копирование в целевой каталог: проверка конфликта → подтверждение перезаписи или сохранения → выполнение → обновление пути → выход (в собранной версии — автоперезапуск) */
function emitConfirmApplyConfigDirWrite({ dir, conflict, busyKey, messageNoConflict, apply }) {
  const hasConflict = hasConfigDirFileConflict(conflict)
  const message = hasConflict
    ? t('config.configDirConflictPrompt', { dir })
    : messageNoConflict

  const run = async (overwrite) => {
    configBusy.value = busyKey
    try {
      await apply(overwrite)
      await loadConfigLocation()
      emit('set-config-message', t('config.configDirRelocateQuitHint'))
      await QuitApplication()
    } catch (err) {
      emit('set-config-message', String(err))
    } finally {
      configBusy.value = ''
    }
  }

  emit('confirm-action', {
    mode: 'confirm',
    message,
    confirmButtonClass: 'btn-warning',
    confirmLabel: hasConflict ? t('config.configDirConflictOverwrite') : t('app.common.confirm'),
    secondaryLabel: hasConflict ? t('config.configDirConflictUseExisting') : '',
    secondaryButtonClass: 'btn-outline-primary',
    onSecondary: hasConflict ? () => run(false) : null,
    onConfirm: async () => run(true)
  })
}

/**
 * Общий конвейер «выбор каталога / каталог по умолчанию → GetConfigDirTargetConflict → подтверждение → apply(overwrite)».
 * Пустая строка из resolveDir значит отмену или уже показанное уведомление — завершаем без сообщения.
 */
async function runConfigDirTargetFlow({ resolveDir, busyKey, messageNoConflict, apply }) {
  let dir
  try {
    dir = await resolveDir()
  } catch (e) {
    emit('set-config-message', String(e))
    return
  }
  dir = typeof dir === 'string' ? dir.trim() : ''
  if (!dir) return

  const res = await getConfigDirTargetConflictSafe(dir)
  if (!res.ok) {
    emit('set-config-message', res.error)
    return
  }

  emitConfirmApplyConfigDirWrite({
    dir,
    conflict: res.conflict,
    busyKey,
    messageNoConflict: messageNoConflict(dir),
    apply: (overwrite) => apply(overwrite, dir)
  })
}

async function onChooseConfigDataDir() {
  try {
    await runConfigDirTargetFlow({
      resolveDir: () => SelectConfigDirectory(),
      busyKey: 'relocate',
      messageNoConflict: (dir) => t('config.configDirRelocateConfirm', { dir }),
      apply: (overwrite, dir) => SetConfigDirectory(dir, overwrite)
    })
  } catch (err) {
    emit('set-config-message', String(err))
  }
}

async function onResetConfigDataDir() {
  await runConfigDirTargetFlow({
    resolveDir: async () => {
      let info = configLocationInfo.value
      try {
        if (!info?.implicitConfigDir) {
          info = await GetConfigLocationInfo()
          configLocationInfo.value = info
        }
      } catch (e) {
        emit('set-config-message', String(e))
        return ''
      }
      const implicitDir = (info.implicitConfigDir ?? '').trim()
      if (!implicitDir) {
        emit('set-config-message', t('config.configDirResetImplicitMissing'))
        return ''
      }
      return implicitDir
    },
    busyKey: 'resetdir',
    messageNoConflict: () => t('config.configDirResetConfirm'),
    apply: (overwrite) => ResetConfigDirectoryToDefault(overwrite)
  })
}

async function onAutoRunChange(checked) {
  try {
    await SetAutoRunEnabled(!!checked)
    autoRunEnabled.value = !!checked
  } catch (_) {
    autoRunEnabled.value = !checked
  }
}

async function onTrafficMonitorChange(checked) {
  try {
    await SetTrafficMonitorEnabled(!!checked)
    trafficMonitorEnabled.value = !!checked
    emit('traffic-monitor-change', !!checked)
  } catch (_) {
    trafficMonitorEnabled.value = !checked
  }
}

async function onExportConfig() {
  configBusy.value = 'export'
  try {
    await ExportConfigWithDialog()
    emit('set-config-message', t('config.exportSuccess'))
  } catch (err) {
    emit('set-config-message', String(err))
  } finally {
    configBusy.value = ''
  }
}

async function onImportConfig() {
  configBusy.value = 'import'
  try {
    const srcPath = await SelectImportFile()
    if (!srcPath) {
      configBusy.value = ''
      return
    }
    emit('confirm-action', {
      mode: 'confirm',
      message: t('config.importConfirm'),
      confirmButtonClass: 'btn-warning',
      confirmLabel: t('app.common.confirm'),
      onConfirm: async () => {
        try {
          await ImportConfig(srcPath)
          emit('set-config-message', t('config.importSuccess'))
          emit('reload-state')
        } catch (err) {
          emit('set-config-message', String(err))
        } finally {
          configBusy.value = ''
        }
      }
    })
  } catch (err) {
    emit('set-config-message', String(err))
    configBusy.value = ''
  }
}

async function onOpenConfigDir() {
  try {
    await OpenConfigDir()
  } catch (err) {
    emit('set-config-message', String(err))
  }
}


</script>

<template>
  <n-grid :cols="2" :x-gap="16" :y-gap="16" responsive="screen" item-responsive>
    <n-gi span="2 l:1">
      <n-card size="small" :title="t('config.general')">
        <n-space vertical :size="16">
          <n-space justify="space-between" align="center">
            <div>
              <div class="config-name">{{ t('config.theme') }}</div>
              <div class="config-desc">{{ t('config.themeDesc') }}</div>
            </div>
            <n-switch :value="theme === 'dark'" @update:value="$emit('theme-change', $event)" />
          </n-space>
          <n-space justify="space-between" align="center">
            <div>
              <div class="config-name">{{ t('config.windowMode') }}</div>
              <div class="config-desc">{{ t('config.windowModeDesc') }}</div>
            </div>
            <n-radio-group :value="windowMode" size="small" @update:value="$emit('window-mode-change', $event)">
              <n-radio-button value="advanced">{{ t('config.windowModeAdvanced') }}</n-radio-button>
              <n-radio-button value="simple">{{ t('config.windowModeSimple') }}</n-radio-button>
            </n-radio-group>
          </n-space>
          <n-space justify="space-between" align="center">
            <div>
              <div class="config-name">{{ t('config.simpleOnTop') }}</div>
              <div class="config-desc">{{ t('config.simpleOnTopDesc') }}</div>
            </div>
            <n-switch :value="simpleOnTop" @update:value="$emit('simple-on-top-change', $event)" />
          </n-space>
          <n-space justify="space-between" align="center">
            <div>
              <div class="config-name">{{ t('config.autoRun') }}</div>
              <div class="config-desc">{{ t('config.autoRunDesc') }}</div>
            </div>
            <n-switch :value="autoRunEnabled" @update:value="onAutoRunChange" />
          </n-space>
          <n-space justify="space-between" align="center">
            <div>
              <div class="config-name">{{ t('config.trafficMonitor') }}</div>
              <div class="config-desc">{{ t('config.trafficMonitorDesc') }}</div>
            </div>
            <n-switch :value="trafficMonitorEnabled" @update:value="onTrafficMonitorChange" />
          </n-space>
          <n-space justify="space-between" align="center">
            <div>
              <div class="config-name">{{ t('config.manageConfig') }}</div>
              <div class="config-desc">{{ t('config.manageConfigDesc') }}</div>
            </div>
            <n-space>
              <n-button size="small" :disabled="configBusy !== ''" @click="onImportConfig">{{ t('config.importConfigBtn') }}</n-button>
              <n-button size="small" :disabled="configBusy !== ''" @click="onExportConfig">{{ t('config.exportConfigBtn') }}</n-button>
              <n-button size="small" @click="onOpenConfigDir">{{ t('config.openConfigDirBtn') }}</n-button>
            </n-space>
          </n-space>
          <n-space justify="space-between" align="start">
            <div>
              <div class="config-name">{{ t('config.configDataDir') }}</div>
              <div class="config-desc">
                <span v-if="configLocationInfo?.effectiveConfigDir">{{ t('config.configDirCurrentPathPrefix') }}{{ configLocationInfo.effectiveConfigDir }}</span>
                <template v-else>{{ t('config.configDirUnavailable') }}</template>
              </div>
            </div>
            <n-space>
              <n-button size="small" :disabled="configBusy !== ''" @click="onChooseConfigDataDir">{{ t('config.chooseConfigDirBtn') }}</n-button>
              <n-button
                v-if="configLocationInfo?.isCustomConfigDir"
                size="small"
                secondary
                :disabled="configBusy !== ''"
                @click="onResetConfigDataDir"
              >
                {{ t('config.resetConfigDirBtn') }}
              </n-button>
            </n-space>
          </n-space>
        </n-space>
      </n-card>
    </n-gi>
    <n-gi span="2 l:1">
      <n-card size="small" :title="t('config.advancedSettings')">
        <div class="config-name">{{ t('config.currentVersion') }}</div>
        <div class="config-desc">{{ appMeta.version }}</div>
      </n-card>
    </n-gi>
  </n-grid>
</template>
