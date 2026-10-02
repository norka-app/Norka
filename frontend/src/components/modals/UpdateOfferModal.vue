<script setup>
defineProps({
  show: {
    type: Boolean,
    default: false
  },
  current: {
    type: String,
    default: ''
  },
  latest: {
    type: String,
    default: ''
  }
})

const emit = defineEmits(['close', 'download'])

function onShowChange(open) {
  if (!open) emit('close')
}
</script>

<template>
  <n-modal
    :show="show"
    preset="card"
    :title="$t('app.update.title')"
    style="width: min(480px, calc(100vw - 32px))"
    :mask-closable="true"
    @update:show="onShowChange"
  >
    <p class="update-offer-message">{{ $t('app.update.message', { latest, current }) }}</p>
    <template #footer>
      <n-space justify="end">
        <n-button @click="emit('close')">{{ $t('app.update.later') }}</n-button>
        <n-button type="primary" @click="emit('download')">{{ $t('app.update.download') }}</n-button>
      </n-space>
    </template>
  </n-modal>
</template>

<style scoped>
.update-offer-message {
  margin: 0;
}
</style>
