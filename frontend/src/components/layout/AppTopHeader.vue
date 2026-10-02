<script setup>
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

defineProps({
  currentPage: {
    type: Object,
    default: null,
  },
  activePage: {
    type: String,
    required: true,
  },
  activeProfile: {
    type: Object,
    default: null,
  },
  sshCommandEnabled: {
    type: Boolean,
    default: false,
  },
})

const emit = defineEmits(['new-jumper', 'new-tunnel', 'import-tunnel', 'import-jumper', 'open-profiles'])
const { t } = useI18n()

const importMenuOptions = computed(() => [
  { label: t('app.header.fromSSHCommand'), key: 'ssh' },
])

function onImportMenu(key) {
  if (key === 'ssh') emit('import-tunnel')
}
</script>

<template>
  <n-layout-header class="top-header" bordered>
    <div>
      <h1 class="page-title">{{ currentPage?.title }}</h1>
      <p class="page-subtitle">{{ currentPage?.subtitle }}</p>
    </div>
    <button
      v-if="activeProfile"
      type="button"
      class="profile-chip"
      @click="$emit('open-profiles')"
    >
      <span class="profile-chip__mark" :style="{ background: activeProfile.color || 'var(--lt-brand)' }" />
      <span v-if="activeProfile.emoji">{{ activeProfile.emoji }}</span>
      <span>{{ activeProfile.name }}</span>
      <span class="profile-chip__state">{{ $t('app.profiles.active') }}</span>
    </button>
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
        <n-dropdown
          v-if="sshCommandEnabled"
          trigger="click"
          :options="importMenuOptions"
          @select="onImportMenu"
        >
          <n-button secondary>
            <template #icon>
              <i class="bi bi-file-earmark-plus" />
            </template>
            {{ $t('app.header.importTunnel') }}
          </n-button>
        </n-dropdown>
        <n-button type="primary" @click="$emit('new-tunnel')">
          {{ $t('app.header.newTunnel') }}
        </n-button>
      </div>
    </div>
  </n-layout-header>
</template>

<style scoped>
.profile-chip {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  max-width: 260px;
  margin-left: auto;
  padding: 4px 10px 4px 8px;
  border: 1px solid var(--lt-border);
  border-radius: 999px;
  background: var(--lt-surface);
  color: var(--lt-ink);
  font: inherit;
  font-size: 13px;
  cursor: pointer;
}
.profile-chip__mark { width: 8px; height: 8px; border-radius: 50%; flex: none; }
.profile-chip__state { color: var(--lt-brand); font-size: 12px; }
</style>
