<script setup>
import { computed } from 'vue'

const props = defineProps({
  modelValue: {
    type: [String, Number],
    default: '',
  },
  options: {
    type: Array,
    default: () => [],
  },
  disabled: {
    type: Boolean,
    default: false,
  },
  placeholder: {
    type: String,
    default: '',
  },
  id: {
    type: String,
    default: undefined,
  },
})

const emit = defineEmits(['update:modelValue'])

const selectOptions = computed(() => props.options.map((option) => ({
  label: option.label,
  value: option.value,
  disabled: Boolean(option.disabled),
})))
</script>

<template>
  <n-select
    :value="modelValue"
    :options="selectOptions"
    :disabled="disabled"
    :placeholder="placeholder || undefined"
    :input-props="id ? { id } : undefined"
    @update:value="emit('update:modelValue', $event)"
  />
</template>
