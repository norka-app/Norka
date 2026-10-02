<script setup>
defineProps({
  currentPage: {
    type: Object,
    default: null,
  },
  activePage: {
    type: String,
    required: true,
  },
})

defineEmits(['new-jumper', 'new-tunnel', 'import-tunnel', 'import-jumper'])
</script>

<template>
  <n-layout-header class="top-header" bordered>
    <div>
      <h1 class="page-title">{{ currentPage?.title }}</h1>
      <p class="page-subtitle">{{ currentPage?.subtitle }}</p>
    </div>
    <div class="header-actions">
      <div v-if="activePage === 'jumpers'" class="header-actions-group">
        <n-button secondary @click="$emit('import-jumper')">
          <template #icon>
            <i class="bi bi-file-earmark-plus" />
          </template>
          {{ $t('app.header.importTunnel') }}
        </n-button>
        <n-button type="primary" @click="$emit('new-jumper')">
          {{ $t('app.header.newJumper') }}
        </n-button>
      </div>
      <div v-if="activePage === 'tunnels'" class="header-actions-group">
        <n-button secondary @click="$emit('import-tunnel')">
          <template #icon>
            <i class="bi bi-file-earmark-plus" />
          </template>
          {{ $t('app.header.importTunnel') }}
        </n-button>
        <n-button type="primary" @click="$emit('new-tunnel')">
          {{ $t('app.header.newTunnel') }}
        </n-button>
      </div>
    </div>
  </n-layout-header>
</template>
