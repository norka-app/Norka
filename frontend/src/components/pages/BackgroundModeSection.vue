<script setup>
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  GetBackgroundStatus,
  SetBackgroundLoginStart,
  StopBackgroundDaemon,
} from '../../../wailsjs/go/main/App'
import { EventsOff, EventsOn } from '../../../wailsjs/runtime/runtime'

const { t } = useI18n()
const status = ref({
  attached: false,
  running: false,
  pid: 0,
  version: '',
  loginStart: false,
  notice: '',
  hostingLocal: false,
})
const busy = ref(false)
const errorText = ref('')

async function reload() {
  try {
    const next = await GetBackgroundStatus()
    if (next) status.value = next
  } catch (err) {
    errorText.value = String(err?.message || err)
  }
}

async function onStop() {
  busy.value = true
  errorText.value = ''
  try {
    await StopBackgroundDaemon()
    await reload()
  } catch (err) {
    errorText.value = String(err?.message || err)
  } finally {
    busy.value = false
  }
}

async function onLogin(checked) {
  busy.value = true
  errorText.value = ''
  try {
    await SetBackgroundLoginStart(!!checked)
    await reload()
  } catch (err) {
    errorText.value = String(err?.message || err)
    await reload()
  } finally {
    busy.value = false
  }
}

function onRefresh() {
  void reload()
}

onMounted(() => {
  void reload()
  if (typeof window !== 'undefined' && window.runtime) {
    EventsOn('background:status', onRefresh)
  }
})

onBeforeUnmount(() => {
  if (typeof window !== 'undefined' && window.runtime) {
    EventsOff('background:status')
  }
})
</script>

<template>
  <section class="background-mode kit-section" :aria-label="t('background.title')">
    <div>
      <div class="config-name">{{ t('background.title') }}</div>
      <div class="config-desc">
        <template v-if="status.running">
          {{ t('background.running') }}
          <template v-if="status.pid"> · {{ t('background.pid', { pid: status.pid }) }}</template>
          <template v-if="status.version"> · {{ t('background.version', { version: status.version }) }}</template>
        </template>
        <template v-else>{{ t('background.stopped') }}</template>
      </div>
      <div v-if="status.hostingLocal" class="config-desc">{{ t('background.hostingLocal') }}</div>
      <div v-if="status.notice" class="config-desc" role="status">{{ t(status.notice) }}</div>
      <div v-if="errorText" class="config-desc" role="alert">{{ errorText }}</div>
    </div>
    <div class="kit-inline">
      <n-button size="small" :loading="busy" :disabled="busy || (!status.running && !status.attached)" @click="onStop">
        {{ t('background.stop') }}
      </n-button>
    </div>
    <div class="kit-switch-line">
      <span>
        {{ t('background.loginStart') }}
        <span class="kit-note">{{ t('background.loginStartDesc') }}</span>
      </span>
      <n-switch
        :value="!!status.loginStart"
        :disabled="busy"
        :aria-label="t('background.loginStart')"
        @update:value="onLogin"
      />
    </div>
  </section>
</template>
