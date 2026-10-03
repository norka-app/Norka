<script setup>
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  GetAutoRunEnabled,
  GetFeatures,
  GetQuickSearchSettings,
  SetAutoRunEnabled,
  SetFeature,
  SetQuickSearchSettings,
  ExportConfigWithDialog,
  SelectImportFile,
  ImportConfig,
  OpenConfigDir,
  GetConfigLocationInfo,
  GetConfigDirTargetConflict,
  SelectConfigDirectory,
  SetConfigDirectory,
  ResetConfigDirectoryToDefault,
  QuitApplication,
  GetNotificationSettings,
  SetNotificationSettings,
  GetSecretsStatus,
  CheckForUpdateNow,
  SaveDiagnostics
} from '../../../wailsjs/go/main/App'
import { applyFeatureViews, featureEnabled, featureState, setFeatureEnabled, useFeature } from '../../features/feature-store'
import { hotkeyChord, matchesKey } from '../../utils/keyboard'

const props = defineProps({
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
  },
  language: {
    type: String,
    default: 'auto'
  }
})

const emit = defineEmits([
  'theme-change',
  'set-config-message',
  'reload-state',
  'confirm-action',
  'window-mode-change',
  'simple-on-top-change',
  'update-offer',
  'language-change',
  'restart-onboarding',
  'diagnostics-saved'
])

const { t } = useI18n()
const notificationsOn = useFeature('notifications')
const quickSearchOn = useFeature('quick_search')
const autoUpdateOn = useFeature('auto_update')
const onboardingOn = useFeature('onboarding')
const diagnosticsOn = useFeature('diagnostics')
const autoRunEnabled = ref(false)
const configBusy = ref('')
const configLocationInfo = ref(null)
const includePasswords = ref(false)
const includeServerAddresses = ref(false)
const diagnosticsBusy = ref(false)
const secretsStatus = ref({ keychainAvailable: true, mode: 'keychain' })
const notifications = ref({
  enabled: false,
  dropped: true,
  reconnected: true,
  gaveUp: true,
  connectFailed: true,
  connected: false
})
const updateChecking = ref(false)
const updateStatus = ref('')
const quickSearch = ref(null)
const capturingHotkey = ref(false)
const hotkeyButtonRef = ref(null)

onMounted(async () => {
  try {
    autoRunEnabled.value = await GetAutoRunEnabled()
  } catch (_) {
    autoRunEnabled.value = false
  }
  try {
    applyFeatureViews(await GetFeatures())
  } catch (_) {
    /* каталог уже показан */
  }
  await loadConfigLocation()
  await loadNotificationSettings()
  await loadSecretsStatus()
  await loadQuickSearch()
})

async function loadNotificationSettings() {
  try {
    const settings = await GetNotificationSettings()
    if (settings) notifications.value = { ...notifications.value, ...settings }
  } catch (_) {
    /* keep defaults */
  }
}

async function loadSecretsStatus() {
  try {
    const status = await GetSecretsStatus()
    if (status) secretsStatus.value = status
  } catch (_) {
    secretsStatus.value = { keychainAvailable: false, mode: 'config' }
  }
}

async function saveNotifications(next) {
  const previous = { ...notifications.value }
  const payload = { ...next, enabled: featureEnabled('notifications') }
  notifications.value = payload
  try {
    await SetNotificationSettings(payload)
  } catch (err) {
    notifications.value = previous
    emit('set-config-message', String(err))
  }
}

function onNotificationToggle(key, checked) {
  void saveNotifications({ ...notifications.value, [key]: !!checked })
}

function formatHotkey(spec, platform) {
  if (!spec) return ''
  const alt = platform === 'darwin' ? 'Option' : 'Alt'
  const meta = platform === 'darwin' ? 'Cmd' : 'Win'
  const names = { ctrl: 'Ctrl', alt, option: 'Option', shift: 'Shift', meta, space: 'Space', escape: 'Esc', enter: 'Enter', tab: 'Tab' }
  return String(spec).split('+').filter(Boolean).map((part) => names[part] || part.toUpperCase()).join('+')
}

const quickSearchLabel = computed(() => formatHotkey(quickSearch.value?.effectiveHotkey || quickSearch.value?.hotkey, quickSearch.value?.platform))

const quickSearchStatus = computed(() => {
  const settings = quickSearch.value
  if (!settings) return ''
  if (!settings.enabled) return t('config.quickSearchDisabled')
  if (settings.errorCode === 'unsupported') return t('config.quickSearchUnsupported')
  if (settings.errorCode === 'invalid') return t('config.quickSearchInvalid')
  if (settings.errorCode === 'register_failed') {
    return t('config.quickSearchFailed', { detail: settings.errorDetail || quickSearchLabel.value })
  }
  if (settings.registered) return t('config.quickSearchRegistered', { hotkey: quickSearchLabel.value })
  return t('config.quickSearchDefault', { hotkey: quickSearchLabel.value })
})

async function loadQuickSearch() {
  try {
    quickSearch.value = await GetQuickSearchSettings()
  } catch (_) {
    quickSearch.value = {
      enabled: true,
      effectiveHotkey: 'ctrl+alt+space',
      platform: 'linux',
      registered: false,
      errorCode: 'unsupported',
      errorDetail: ''
    }
  }
}

async function saveQuickSearch(patch) {
  const current = quickSearch.value || { enabled: featureEnabled('quick_search'), hotkey: '' }
  try {
    quickSearch.value = await SetQuickSearchSettings({
      ...current,
      ...patch,
      enabled: featureEnabled('quick_search'),
    })
  } catch (err) {
    emit('set-config-message', String(err))
    await loadQuickSearch()
  }
}

async function beginHotkeyCapture() {
  capturingHotkey.value = true
  await nextTick()
  const el = hotkeyButtonRef.value?.$el || hotkeyButtonRef.value
  el?.focus?.()
}

function chordFromEvent(event) {
  return hotkeyChord(event, { altName: quickSearch.value?.platform === 'darwin' ? 'option' : 'alt' })
}

function onHotkeyKeydown(event) {
  if (!capturingHotkey.value) return
  event.preventDefault()
  event.stopPropagation()
  if (matchesKey(event, 'escape') && !event.ctrlKey && !event.altKey && !event.metaKey && !event.shiftKey) {
    capturingHotkey.value = false
    return
  }
  const chord = chordFromEvent(event)
  if (!chord) return
  capturingHotkey.value = false
  void saveQuickSearch({ hotkey: chord, enabled: true })
}

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

async function onFeatureToggle(id, checked) {
  const previous = featureEnabled(id)
  setFeatureEnabled(id, !!checked)
  try {
    applyFeatureViews(await SetFeature(id, !!checked))
  } catch (err) {
    setFeatureEnabled(id, previous)
    emit('set-config-message', String(err))
  }
}

watch(quickSearchOn, () => {
  void loadQuickSearch()
})

watch(notificationsOn, (on) => {
  notifications.value = { ...notifications.value, enabled: !!on }
})

async function doExport(withPasswords) {
  configBusy.value = 'export'
  try {
    await ExportConfigWithDialog(!!withPasswords)
    emit('set-config-message', t('config.exportSuccess'))
  } catch (err) {
    if (String(err).toLowerCase().includes('export cancelled')) return
    emit('set-config-message', String(err))
  } finally {
    configBusy.value = ''
  }
}

async function onExportConfig() {
  if (includePasswords.value) {
    emit('confirm-action', {
      mode: 'confirm',
      message: t('config.includePasswordsWarning'),
      confirmButtonClass: 'btn-warning',
      confirmLabel: t('config.exportConfigBtn'),
      onConfirm: () => doExport(true)
    })
    return
  }
  await doExport(false)
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

async function onCheckUpdates() {
  updateChecking.value = true
  try {
    const info = await CheckForUpdateNow()
    if (info?.available && info.url) {
      updateStatus.value = t('config.updateAvailable', { latest: info.latest })
      emit('update-offer', info)
      return
    }
    updateStatus.value = t('config.upToDate', { version: info?.current || props.appMeta.version })
  } catch (_) {
    updateStatus.value = t('config.checkFailed')
  } finally {
    updateChecking.value = false
  }
}

async function onOpenConfigDir() {
  try {
    await OpenConfigDir()
  } catch (err) {
    emit('set-config-message', String(err))
  }
}

async function onCollectDiagnostics() {
  if (!diagnosticsOn.value) return
  diagnosticsBusy.value = true
  try {
    const result = await SaveDiagnostics(!!includeServerAddresses.value, props.theme)
    if (!result || result.cancelled || !result.path) return
    emit('diagnostics-saved', result)
  } catch (err) {
    const message = String(err)
    if (message.toLowerCase().includes('cancelled')) return
    emit('set-config-message', message)
  } finally {
    diagnosticsBusy.value = false
  }
}


</script>

<template>
  <n-grid :cols="2" :x-gap="16" :y-gap="16" responsive="screen" item-responsive>
    <n-gi span="2">
      <div data-onboarding="features">
      <n-card size="small" :title="t('features.title')">
        <n-space vertical :size="16">
          <n-space
            v-for="item in featureState.items"
            :key="item.id"
            class="settings-row feature-row"
            justify="space-between"
            align="center"
            :wrap="false"
          >
            <div class="settings-label">
              <div class="config-name">{{ t(item.titleKey) }}</div>
              <div class="config-desc">{{ t(item.descriptionKey) }}</div>
            </div>
            <n-switch
              :value="item.enabled"
              :aria-label="t(item.titleKey)"
              @update:value="(checked) => onFeatureToggle(item.id, checked)"
            />
          </n-space>
          <button
            v-if="onboardingOn"
            type="button"
            class="onboarding-replay"
            @click="emit('restart-onboarding')"
          >
            {{ t('onboarding.replay') }}
          </button>
        </n-space>
      </n-card>
      </div>
    </n-gi>
    <n-gi span="2 l:1">
      <n-card size="small" :title="t('config.general')">
        <n-space vertical :size="16">
          <n-space class="settings-row" justify="space-between" align="center" :wrap="true">
            <div class="settings-label">
              <div class="config-name">{{ t('config.language') }}</div>
              <div class="config-desc">{{ t('config.languageDesc') }}</div>
            </div>
            <n-radio-group
              class="language-options"
              :value="language"
              size="small"
              @update:value="$emit('language-change', $event)"
            >
              <n-radio-button value="auto">{{ t('config.languageAuto') }}</n-radio-button>
              <n-radio-button value="ru">{{ t('config.languageRu') }}</n-radio-button>
              <n-radio-button value="en">{{ t('config.languageEn') }}</n-radio-button>
            </n-radio-group>
          </n-space>
          <n-space class="settings-row" justify="space-between" align="center" :wrap="true">
            <div class="settings-label">
              <div class="config-name">{{ t('config.theme') }}</div>
              <div class="config-desc">{{ t('config.themeDesc') }}</div>
            </div>
            <n-switch :value="theme === 'dark'" @update:value="$emit('theme-change', $event)" />
          </n-space>
          <n-space class="settings-row" justify="space-between" align="center" :wrap="true">
            <div class="settings-label">
              <div class="config-name">{{ t('config.windowMode') }}</div>
              <div class="config-desc">{{ t('config.windowModeDesc') }}</div>
            </div>
            <n-radio-group :value="windowMode" size="small" @update:value="$emit('window-mode-change', $event)">
              <n-radio-button value="advanced">{{ t('config.windowModeAdvanced') }}</n-radio-button>
              <n-radio-button value="simple">{{ t('config.windowModeSimple') }}</n-radio-button>
            </n-radio-group>
          </n-space>
          <n-space class="settings-row" justify="space-between" align="center" :wrap="true">
            <div class="settings-label">
              <div class="config-name">{{ t('config.simpleOnTop') }}</div>
              <div class="config-desc">{{ t('config.simpleOnTopDesc') }}</div>
            </div>
            <n-switch :value="simpleOnTop" @update:value="$emit('simple-on-top-change', $event)" />
          </n-space>
          <n-space class="settings-row" justify="space-between" align="center" :wrap="true">
            <div class="settings-label">
              <div class="config-name">{{ t('config.autoRun') }}</div>
              <div class="config-desc">{{ t('config.autoRunDesc') }}</div>
            </div>
            <n-switch :value="autoRunEnabled" @update:value="onAutoRunChange" />
          </n-space>
          <n-space class="settings-row" justify="space-between" align="center" :wrap="true">
            <div class="settings-label">
              <div class="config-name">{{ t('config.quickSearch') }}</div>
              <div class="config-desc">{{ t('config.quickSearchDesc') }}</div>
              <div class="config-desc" :class="{ 'config-desc--warn': quickSearch?.errorCode && quickSearchOn }" role="status">
                {{ quickSearchStatus }}
              </div>
            </div>
            <n-space align="center">
              <n-button
                ref="hotkeyButtonRef"
                size="small"
                :disabled="!quickSearchOn"
                :data-hotkey-capture="capturingHotkey ? '1' : undefined"
                :type="capturingHotkey ? 'primary' : 'default'"
                @click="beginHotkeyCapture"
                @keydown="onHotkeyKeydown"
              >
                {{ capturingHotkey ? t('config.quickSearchPress') : (quickSearchLabel || t('config.quickSearchCapture')) }}
              </n-button>
              <n-button size="small" quaternary :disabled="!quickSearchOn" @click="saveQuickSearch({ hotkey: '' })">{{ t('config.quickSearchReset') }}</n-button>
            </n-space>
          </n-space>
          <n-space class="settings-row" justify="space-between" align="center" :wrap="true">
            <div class="settings-label">
              <div class="config-name">{{ t('config.manageConfig') }}</div>
              <div class="config-desc">{{ t('config.manageConfigDesc') }}</div>
            </div>
            <n-space vertical align="end" :size="8">
              <n-space>
                <n-button size="small" :disabled="configBusy !== ''" @click="onImportConfig">{{ t('config.importConfigBtn') }}</n-button>
                <n-button size="small" :disabled="configBusy !== ''" @click="onExportConfig">{{ t('config.exportConfigBtn') }}</n-button>
                <n-button size="small" @click="onOpenConfigDir">{{ t('config.openConfigDirBtn') }}</n-button>
              </n-space>
              <div>
                <n-checkbox v-model:checked="includePasswords">{{ t('config.includePasswords') }}</n-checkbox>
                <div class="config-desc">{{ t('config.includePasswordsDesc') }}</div>
                <div v-if="includePasswords" class="config-desc config-warning">{{ t('config.includePasswordsWarning') }}</div>
              </div>
            </n-space>
          </n-space>
          <div class="settings-row">
            <div class="config-name">{{ t('config.secretsTitle') }}</div>
            <n-alert v-if="secretsStatus.keychainAvailable" class="config-secret-alert" type="success" :show-icon="true">
              {{ t('config.secretsOk') }}
            </n-alert>
            <n-alert v-else class="config-secret-alert" type="warning" :show-icon="true">
              {{ t('config.secretsUnavailable') }}
            </n-alert>
          </div>
          <n-space class="settings-row" justify="space-between" align="start" :wrap="true">
            <div class="settings-label">
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
      <div class="config-stack">
        <n-card size="small" :title="t('config.notifications')">
          <n-space vertical :size="16">
            <div class="config-desc">{{ t('config.notificationsDesc') }}</div>
            <n-space class="settings-row" justify="space-between" align="center" :wrap="true">
              <div class="settings-label">
                <div class="config-name">{{ t('config.notifyDropped') }}</div>
                <div class="config-desc">{{ t('config.notifyDroppedDesc') }}</div>
              </div>
              <n-switch :value="notifications.dropped" :disabled="!notificationsOn" @update:value="(checked) => onNotificationToggle('dropped', checked)" />
            </n-space>
            <n-space class="settings-row" justify="space-between" align="center" :wrap="true">
              <div class="settings-label">
                <div class="config-name">{{ t('config.notifyReconnected') }}</div>
                <div class="config-desc">{{ t('config.notifyReconnectedDesc') }}</div>
              </div>
              <n-switch :value="notifications.reconnected" :disabled="!notificationsOn" @update:value="(checked) => onNotificationToggle('reconnected', checked)" />
            </n-space>
            <n-space class="settings-row" justify="space-between" align="center" :wrap="true">
              <div class="settings-label">
                <div class="config-name">{{ t('config.notifyGaveUp') }}</div>
                <div class="config-desc">{{ t('config.notifyGaveUpDesc') }}</div>
              </div>
              <n-switch :value="notifications.gaveUp" :disabled="!notificationsOn" @update:value="(checked) => onNotificationToggle('gaveUp', checked)" />
            </n-space>
            <n-space class="settings-row" justify="space-between" align="center" :wrap="true">
              <div class="settings-label">
                <div class="config-name">{{ t('config.notifyConnectFailed') }}</div>
                <div class="config-desc">{{ t('config.notifyConnectFailedDesc') }}</div>
              </div>
              <n-switch :value="notifications.connectFailed" :disabled="!notificationsOn" @update:value="(checked) => onNotificationToggle('connectFailed', checked)" />
            </n-space>
            <n-space class="settings-row" justify="space-between" align="center" :wrap="true">
              <div class="settings-label">
                <div class="config-name">{{ t('config.notifyConnected') }}</div>
                <div class="config-desc">{{ t('config.notifyConnectedDesc') }}</div>
              </div>
              <n-switch :value="notifications.connected" :disabled="!notificationsOn" @update:value="(checked) => onNotificationToggle('connected', checked)" />
            </n-space>
          </n-space>
        </n-card>
        <n-card size="small" :title="t('config.advancedSettings')">
          <div class="config-name">{{ t('config.currentVersion') }}</div>
          <div class="config-desc">{{ appMeta.version }}</div>
          <n-space justify="space-between" align="center" style="margin-top: 16px">
            <div>
              <div class="config-name">{{ t('config.checkUpdates') }}</div>
              <div class="config-desc">{{ updateStatus || t('config.checkUpdatesDesc') }}</div>
            </div>
            <n-button size="small" :disabled="!autoUpdateOn" :loading="updateChecking" @click="onCheckUpdates">
              {{ t('config.checkUpdatesBtn') }}
            </n-button>
          </n-space>
        </n-card>
        <n-card v-if="diagnosticsOn" id="diagnostics-support" size="small" :title="t('config.support')">
          <div class="config-name">{{ t('config.diagnosticsCollect') }}</div>
          <div class="config-desc">{{ t('config.diagnosticsCollectDesc') }}</div>
          <div class="diagnostics-option">
            <n-checkbox v-model:checked="includeServerAddresses">{{ t('config.diagnosticsIncludeHosts') }}</n-checkbox>
            <div class="config-desc">{{ t('config.diagnosticsIncludeHostsDesc') }}</div>
          </div>
          <n-button id="collect-diagnostics" size="small" :loading="diagnosticsBusy" @click="onCollectDiagnostics">
            {{ t('config.diagnosticsCollect') }}
          </n-button>
        </n-card>
      </div>
    </n-gi>
  </n-grid>
</template>

<style scoped>
.config-desc--warn { color: var(--lt-warning-ink, #92400e); }

.feature-row {
  width: 100%;
  flex-wrap: nowrap;
}

.feature-row .settings-label {
  flex: 1 1 auto;
  min-width: 0;
}

.feature-row :deep(.n-switch) {
  flex-shrink: 0;
}

.onboarding-replay {
  align-self: flex-start;
  margin: 0;
  padding: 0;
  border: 0;
  background: none;
  color: var(--lt-brand);
  font: inherit;
  font-size: 13px;
  line-height: 1.4;
  text-decoration: underline;
  text-underline-offset: 2px;
  cursor: pointer;
}

.onboarding-replay:focus-visible {
  outline: 2px solid var(--lt-focus);
  outline-offset: 2px;
  border-radius: 4px;
}

.diagnostics-option {
  margin: 12px 0;
}
</style>
