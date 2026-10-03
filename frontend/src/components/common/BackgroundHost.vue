<script setup>
import { onBeforeUnmount, onMounted } from 'vue'
import { useDialog, useMessage } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { StopBackgroundDaemon } from '../../../wailsjs/go/main/App'
import { EventsOff, EventsOn } from '../../../wailsjs/runtime/runtime'

const dialog = useDialog()
const message = useMessage()
const { t } = useI18n()

function showNotice(payload) {
  const key = payload && payload.key
  if (!key) return
  message.warning(t(key), { duration: 8000, keepAliveOnHover: true })
}

function showOffer() {
  dialog.warning({
    title: t('background.offerTitle'),
    content: t('background.offerBody'),
    positiveText: t('background.offerYes'),
    negativeText: t('background.offerNo'),
    onPositiveClick: () => StopBackgroundDaemon(),
  })
}

onMounted(() => {
  if (typeof window === 'undefined' || !window.runtime) return
  EventsOn('background:notice', showNotice)
  EventsOn('background:offer-stop', showOffer)
})

onBeforeUnmount(() => {
  if (typeof window === 'undefined' || !window.runtime) return
  EventsOff('background:notice')
  EventsOff('background:offer-stop')
})
</script>

<template>
  <span class="background-host" hidden></span>
</template>
