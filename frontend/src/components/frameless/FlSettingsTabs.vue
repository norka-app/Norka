<!-- Вкладки настроек вида «без рамки»: Общие | Функции | Уведомления. -->
<script setup>
import { useI18n } from 'vue-i18n'

defineProps({
  modelValue: { type: String, default: 'general' },
})
defineEmits(['update:modelValue'])
const { t } = useI18n()
</script>

<template>
  <div class="fl-settings-tabs">
    <n-radio-group :value="modelValue" size="medium" @update:value="$emit('update:modelValue', $event)">
      <n-radio-button value="general">{{ t('config.general') }}</n-radio-button>
      <n-radio-button value="features">{{ t('features.title') }}</n-radio-button>
      <n-radio-button value="notify">{{ t('config.notifications') }}</n-radio-button>
    </n-radio-group>
  </div>
</template>
