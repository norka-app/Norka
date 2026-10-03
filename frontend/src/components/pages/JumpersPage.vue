<script setup>
import { computed, h, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { NButton, NEllipsis } from 'naive-ui'
import { CopyOutline, OptionsOutline, Search, TrashOutline, renderIcon } from '../../icons'

const props = defineProps({
  jumpers: {
    type: Array,
    required: true
  },
  searchQuery: {
    type: String,
    default: ''
  },
  getAuthLabel: {
    type: Function,
    required: true
  }
})

const emit = defineEmits(['copy-jumper', 'edit-jumper', 'delete-jumper', 'update-search-query'])
const { t } = useI18n()

function actionButton(icon, type, label, onClick) {
  return h(NButton, {
    size: 'small',
    quaternary: true,
    type,
    title: label,
    'aria-label': label,
    onClick,
  }, { icon: renderIcon(icon) })
}

const columns = computed(() => [
  { title: t('app.jumpers.table.name'), key: 'name', ellipsis: { tooltip: true } },
  {
    title: t('app.jumpers.table.connection'),
    key: 'connection',
    render: (row) => h('div', [
      h('div', row.user),
      h(NEllipsis, { style: 'opacity: 0.7' }, { default: () => `${row.host}:${row.port}` }),
    ]),
  },
  {
    title: t('app.jumpers.table.auth'),
    key: 'auth',
    render: (row) => h('div', [
      h('div', props.getAuthLabel(row.authType)),
      row.keyPath ? h(NEllipsis, { style: 'opacity: 0.7' }, { default: () => row.keyPath }) : null,
    ]),
  },
  {
    title: t('app.jumpers.table.notes'),
    key: 'notes',
    ellipsis: { tooltip: true },
    render: (row) => row.notes || '-',
  },
  {
    title: t('app.jumpers.table.action'),
    key: 'action',
    align: 'right',
    width: 140,
    render: (row) => h('div', { style: 'display:flex;justify-content:flex-end;gap:4px' }, [
      actionButton(CopyOutline, 'primary', t('app.jumpers.actions.copy'), () => emit('copy-jumper', row)),
      actionButton(OptionsOutline, 'default', t('app.jumpers.actions.edit'), () => emit('edit-jumper', row)),
      actionButton(TrashOutline, 'error', t('app.jumpers.actions.delete'), () => emit('delete-jumper', row)),
    ]),
  },
])

const localSearchQuery = ref(props.searchQuery)

watch(() => props.searchQuery, (newValue) => {
  localSearchQuery.value = newValue
})

watch(localSearchQuery, (newValue) => {
  emit('update-search-query', newValue)
})
</script>

<template>
  <n-card size="small" :title="$t('app.jumpers.title')">
    <template #header-extra>
      <n-input
        v-model:value="localSearchQuery"
        clearable
        :placeholder="$t('app.common.searchPlaceholder')"
        :aria-label="$t('app.common.searchJumpers')"
        style="width: 240px"
      >
        <template #prefix>
          <n-icon aria-hidden="true" :component="Search" />
        </template>
      </n-input>
    </template>
    <n-data-table
      v-if="jumpers.length > 0"
      size="small"
      :columns="columns"
      :data="jumpers"
      :bordered="false"
      :row-key="(row) => row.id"
    />
    <n-empty v-else :description="$t('app.jumpers.noJumpers')" />
  </n-card>
</template>
