<script setup>
defineProps({
  buttonClass: {
    type: String,
    default: 'btn-outline-secondary',
  },
  title: {
    type: String,
    required: true,
  },
  ariaLabel: {
    type: String,
    required: true,
  },
  iconClass: {
    type: String,
    required: true,
  },
  disabled: {
    type: Boolean,
    default: false,
  },
})

defineEmits(['click'])

function buttonType(buttonClass) {
  if (buttonClass.includes('danger')) return 'error'
  if (buttonClass.includes('warning')) return 'warning'
  if (buttonClass.includes('success')) return 'success'
  if (buttonClass.includes('primary')) return 'primary'
  return 'default'
}
</script>

<template>
  <n-tooltip>
    <template #trigger>
      <n-button
        size="small"
        quaternary
        :type="buttonType(buttonClass)"
        :aria-label="ariaLabel"
        :disabled="disabled"
        @click="$emit('click')"
      >
        <template #icon>
          <i class="bi action-icon" :class="iconClass" />
        </template>
      </n-button>
    </template>
    {{ title }}
  </n-tooltip>
</template>
