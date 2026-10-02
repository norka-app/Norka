<script setup>
import { computed, h } from 'vue'
import { useI18n } from 'vue-i18n'
import { NTag } from 'naive-ui'

const props = defineProps({
  selectedLogLevel: {
    type: String,
    required: true
  },
  filteredLogs: {
    type: Array,
    required: true
  }
})

defineEmits(['set-log-level'])
const { t } = useI18n()

function logTagType(level) {
  if (level === 'error') return 'error'
  if (level === 'warn') return 'warning'
  if (level === 'info') return 'info'
  return 'default'
}

const columns = computed(() => [
  { title: t('app.logs.table.time'), key: 'time', width: 180, ellipsis: { tooltip: true } },
  {
    title: t('app.logs.table.level'),
    key: 'level',
    width: 110,
    render: (row) => h(NTag, { size: 'small', type: logTagType(row.level) }, { default: () => String(row.level || '').toUpperCase() }),
  },
  { title: t('app.logs.table.message'), key: 'message' },
])
</script>

<template>
  <n-card size="small" :title="$t('app.logs.title')">
    <template #header-extra>
      <n-radio-group :value="selectedLogLevel" size="small" @update:value="$emit('set-log-level', $event)">
        <n-radio-button value="all">{{ $t('app.logs.levels.all') }}</n-radio-button>
        <n-radio-button value="info">{{ $t('app.logs.levels.info') }}</n-radio-button>
        <n-radio-button value="warn">{{ $t('app.logs.levels.warn') }}</n-radio-button>
        <n-radio-button value="error">{{ $t('app.logs.levels.error') }}</n-radio-button>
      </n-radio-group>
    </template>
    <n-data-table
      v-if="filteredLogs.length > 0"
      size="small"
      :columns="columns"
      :data="filteredLogs"
      :bordered="false"
      :row-key="(row) => row.id"
    />
    <n-empty v-else :description="$t('app.logs.noLogs')" />
  </n-card>
</template>
