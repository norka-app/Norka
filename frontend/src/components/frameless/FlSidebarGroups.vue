<!-- Список групп в боковой панели вида «без рамки»: точка статуса и счётчик. -->
<script setup>
import { computed } from 'vue'
import { NIcon } from 'naive-ui'
import { useI18n } from 'vue-i18n'
import { Add, FolderOutline } from '../../icons'

const props = defineProps({
  groups: { type: Array, default: () => [] },
  tunnels: { type: Array, default: () => [] },
})
const emit = defineEmits(['select', 'add'])
const { t } = useI18n()

function dotFor(list) {
  if (list.some((tunnel) => tunnel.status === 'error')) return 'error'
  if (list.some((tunnel) => tunnel.status === 'busy' || tunnel.status === 'reconnecting')) return 'busy'
  if (list.some((tunnel) => tunnel.status === 'running')) return 'running'
  return 'stopped'
}

const items = computed(() => props.groups.map((group) => {
  const id = Number(group.id)
  const list = props.tunnels.filter((tunnel) => Number(tunnel.groupId) === id)
  return {
    id,
    name: group.name,
    count: list.length,
    on: list.filter((tunnel) => tunnel.status === 'running').length,
    dot: dotFor(list),
  }
}))
</script>

<template>
  <div class="fl-groups">
    <div class="fl-groups__head">
      <span>{{ t('app.sidebar.groups') }}</span>
      <button
        type="button"
        class="fl-groups__add"
        :title="t('app.sidebar.addGroup')"
        :aria-label="t('app.sidebar.addGroup')"
        @click="emit('add')"
      >
        <n-icon :component="Add" :size="14" />
      </button>
    </div>
    <button
      v-for="group in items"
      :key="group.id"
      type="button"
      class="fl-group"
      @click="emit('select', group.id)"
    >
      <n-icon class="fl-group__icon" :component="FolderOutline" :size="15" />
      <span class="fl-group__name">{{ group.name }}</span>
      <span class="fl-group__dot" :class="`is-${group.dot}`" />
      <span class="fl-group__count">{{ group.on }}/{{ group.count }}</span>
    </button>
  </div>
</template>
