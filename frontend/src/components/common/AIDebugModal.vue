<script setup>
import AIDebugResultCard from './AIDebugResultCard.vue'
import { BUTTON_GAP, FIELD_GAP } from '../../theme/form-layout'

defineProps({
  show: {
    type: Boolean,
    required: true
  },
  title: {
    type: String,
    required: true
  },
  subtitle: {
    type: String,
    default: ''
  },
  rawError: {
    type: String,
    default: ''
  },
  state: {
    type: Object,
    required: true
  }
})

defineEmits(['close', 'retry-debug', 'test-again', 'report-content'])
</script>

<template>
  <n-modal
    :show="show"
    preset="card"
    class="app-modal"
    :title="title"
    :style="{ width: 'min(720px, calc(100vw - 32px))' }"
    :segmented="{ content: true, footer: 'soft' }"
    :content-style="{ maxHeight: 'calc(100vh - 180px)', overflow: 'auto' }"
    @update:show="(visible) => { if (!visible) $emit('close') }"
  >
    <n-text v-if="subtitle" depth="3">{{ subtitle }}</n-text>
    <n-alert v-if="rawError" type="error" :style="{ marginTop: subtitle ? FIELD_GAP + 'px' : 0 }">
      <div>{{ $t('app.aiDebug.sourceError') }}</div>
      <div>{{ rawError }}</div>
    </n-alert>
    <AIDebugResultCard
      :state="state"
      show-actions
      @retry-debug="$emit('retry-debug')"
      @test-again="$emit('test-again')"
      @report-content="$emit('report-content')"
    />
    <template #footer>
      <n-space justify="end" :size="BUTTON_GAP" style="width: 100%">
        <n-button @click="$emit('close')">
          {{ $t('app.common.close') }}
        </n-button>
      </n-space>
    </template>
  </n-modal>
</template>
