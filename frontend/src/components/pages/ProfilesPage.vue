<script setup>
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import NorkaStatusLogo from '../norka/NorkaStatusLogo.vue'
import {
  ActivateProfile,
  ClearActiveProfile,
  CreateProfile,
  DeleteProfile,
  SetProfileStopOthers,
  UpdateProfile
} from '../../../wailsjs/go/main/App'

const PROFILE_COLORS = ['#0ea5e9', '#22c55e', '#eab308', '#ef4444', '#a855f7', '#64748b']
const EMOJI_PRESETS = ['🏠', '🌱', '🚀', '🧪', '🔒', '☁️']

const props = defineProps({
  profiles: { type: Array, default: () => [] },
  tunnels: { type: Array, default: () => [] },
  activeProfileId: { type: Number, default: 0 },
  stopOthers: { type: Boolean, default: false },
  theme: { type: String, default: 'light' }
})

const emit = defineEmits(['changed', 'confirm-action', 'message'])

const { t } = useI18n()
const editingId = ref(0)
const creating = ref(false)
const busy = ref(false)
const formError = ref('')
const draft = ref(emptyDraft())

const editorOpen = computed(() => creating.value || editingId.value > 0)

function emptyDraft() {
  return { name: '', emoji: '', color: PROFILE_COLORS[0], tunnelIds: [] }
}

function profileError(err) {
  const text = String(err?.message || err || '')
  if (text.includes('name is required')) return t('app.profiles.nameRequired')
  if (text.includes('already exists')) return t('app.profiles.nameDuplicate')
  if (text.includes('too long')) return t('app.profiles.nameTooLong')
  if (text.includes('not found')) return t('app.profiles.notFound')
  return text
}

function startCreate() {
  creating.value = true
  editingId.value = 0
  formError.value = ''
  draft.value = emptyDraft()
}

function startEdit(profile) {
  creating.value = false
  editingId.value = profile.id
  formError.value = ''
  draft.value = {
    name: profile.name || '',
    emoji: profile.emoji || '',
    color: profile.color || PROFILE_COLORS[0],
    tunnelIds: [...(profile.tunnelIds || [])]
  }
}

function cancelEdit() {
  creating.value = false
  editingId.value = 0
  formError.value = ''
}

function toggleTunnel(id) {
  const ids = new Set(draft.value.tunnelIds)
  if (ids.has(id)) ids.delete(id)
  else ids.add(id)
  draft.value = { ...draft.value, tunnelIds: [...ids] }
}

async function save() {
  const payload = {
    name: draft.value.name.trim(),
    emoji: draft.value.emoji.trim(),
    color: draft.value.color,
    tunnelIds: draft.value.tunnelIds
  }
  if (!payload.name) {
    formError.value = t('app.profiles.nameRequired')
    return
  }
  busy.value = true
  formError.value = ''
  try {
    if (editingId.value) await UpdateProfile(editingId.value, payload)
    else await CreateProfile(payload)
    cancelEdit()
    emit('changed')
  } catch (err) {
    formError.value = profileError(err)
  } finally {
    busy.value = false
  }
}

async function onStopOthers(enabled) {
  try {
    await SetProfileStopOthers(!!enabled)
    emit('changed')
  } catch (err) {
    emit('message', profileError(err))
  }
}

function tunnelCount(profile) {
  return (profile.tunnelIds || []).length
}

function memberNames(profile) {
  const ids = new Set(profile.tunnelIds || [])
  return props.tunnels.filter((tunnel) => ids.has(tunnel.id)).map((tunnel) => tunnel.name)
}

async function activate(profile) {
  busy.value = true
  try {
    const result = await ActivateProfile(profile.id)
    emit('changed')
    emit('message', activationText(profile, result))
  } catch (err) {
    emit('message', profileError(err))
  } finally {
    busy.value = false
  }
}

function activationText(profile, result) {
  const lines = [t('app.profiles.activated', {
    name: profile.name,
    started: result?.started?.length || 0,
    stopped: result?.stopped?.length || 0
  })]
  for (const conflict of result?.conflicts || []) {
    lines.push(t(conflict.insideProfile ? 'app.profiles.conflictInside' : 'app.profiles.conflictOutside', {
      name: conflict.tunnelName,
      holder: conflict.holderName,
      port: conflict.port
    }))
  }
  return lines.join('\n')
}

function clearActive() {
  busy.value = true
  ClearActiveProfile()
    .then(() => emit('changed'))
    .catch((err) => emit('message', profileError(err)))
    .finally(() => { busy.value = false })
}

function remove(profile) {
  emit('confirm-action', {
    mode: 'confirm',
    message: t('app.profiles.deleteConfirm', { name: profile.name }),
    confirmLabel: t('app.common.delete'),
    confirmButtonClass: 'btn-danger',
    onConfirm: async () => {
      await DeleteProfile(profile.id)
      if (editingId.value === profile.id) cancelEdit()
      emit('changed')
    }
  })
}
</script>

<template>
  <div class="profiles">
    <section class="profiles-pref">
      <div>
        <div class="profiles-pref__name">{{ t('app.profiles.stopOthers') }}</div>
        <p class="profiles-pref__desc">{{ t('app.profiles.stopOthersDesc') }}</p>
      </div>
      <n-switch :value="stopOthers" @update:value="onStopOthers" />
    </section>

    <div class="profiles-toolbar">
      <n-button type="primary" :disabled="busy" @click="startCreate">{{ t('app.profiles.create') }}</n-button>
      <n-button v-if="activeProfileId" secondary :disabled="busy" @click="clearActive">{{ t('app.profiles.clear') }}</n-button>
    </div>

    <div v-if="!profiles.length && !editorOpen" class="profiles-empty">
      <NorkaStatusLogo :status="'stopped'" :theme="theme === 'dark' ? 'dark' : 'light'" :size="84" :title="t('app.title')" />
      <p>{{ t('app.profiles.empty') }}</p>
    </div>

    <div v-else class="profiles-grid">
      <article
        v-for="profile in profiles"
        :key="profile.id"
        class="profile-card"
        :class="{ 'is-active': profile.id === activeProfileId }"
        :style="profile.color ? { '--profile-color': profile.color } : undefined"
      >
        <header class="profile-card__head">
          <span class="profile-card__emoji">{{ profile.emoji || '•' }}</span>
          <div class="profile-card__id">
            <h2>{{ profile.name }}</h2>
            <p>{{ t('app.profiles.tunnelCount', { count: tunnelCount(profile) }) }}</p>
          </div>
          <span v-if="profile.id === activeProfileId" class="profile-card__badge">{{ t('app.profiles.active') }}</span>
        </header>
        <p class="profile-card__members">
          <template v-if="memberNames(profile).length">{{ memberNames(profile).join(' · ') }}</template>
          <template v-else>{{ t('app.profiles.noMembers') }}</template>
        </p>
        <footer class="profile-card__actions">
          <n-button size="small" type="primary" :disabled="busy" @click="activate(profile)">{{ t('app.profiles.activate') }}</n-button>
          <n-button size="small" secondary :disabled="busy" @click="startEdit(profile)">{{ t('app.profiles.edit') }}</n-button>
          <n-button size="small" quaternary :disabled="busy" @click="remove(profile)">{{ t('app.profiles.delete') }}</n-button>
        </footer>
      </article>
    </div>

    <section v-if="editorOpen" class="profile-editor">
      <h2>{{ editingId ? t('app.profiles.editTitle') : t('app.profiles.createTitle') }}</h2>
      <label class="profile-field">
        <span>{{ t('app.profiles.name') }}</span>
        <n-input v-model:value="draft.name" :placeholder="t('app.profiles.namePlaceholder')" maxlength="40" />
      </label>
      <div class="profile-field">
        <span>{{ t('app.profiles.emoji') }}</span>
        <div class="profile-emojis">
          <button
            v-for="emoji in EMOJI_PRESETS"
            :key="emoji"
            type="button"
            class="emoji-btn"
            :class="{ 'is-on': draft.emoji === emoji }"
            @click="draft.emoji = draft.emoji === emoji ? '' : emoji"
          >{{ emoji }}</button>
          <n-input v-model:value="draft.emoji" class="emoji-input" :placeholder="t('app.profiles.emojiPlaceholder')" maxlength="8" />
        </div>
      </div>
      <div class="profile-field">
        <span>{{ t('app.profiles.color') }}</span>
        <div class="profile-colors">
          <button
            v-for="color in PROFILE_COLORS"
            :key="color"
            type="button"
            class="color-btn"
            :class="{ 'is-on': draft.color === color }"
            :style="{ background: color }"
            :aria-label="color"
            :aria-pressed="draft.color === color ? 'true' : 'false'"
            @click="draft.color = color"
          />
        </div>
      </div>
      <div class="profile-field">
        <span>{{ t('app.profiles.tunnels') }}</span>
        <p v-if="!tunnels.length" class="profile-hint">{{ t('app.profiles.noTunnelsToPick') }}</p>
        <ul v-else class="tunnel-pick">
          <li v-for="tunnel in tunnels" :key="tunnel.id">
            <label>
              <input
                type="checkbox"
                :checked="draft.tunnelIds.includes(tunnel.id)"
                @change="toggleTunnel(tunnel.id)"
              >
              <span>{{ tunnel.name }}</span>
              <span class="tunnel-pick__port">{{ tunnel.localPort }}</span>
            </label>
          </li>
        </ul>
      </div>
      <p v-if="formError" class="profile-error" role="alert">{{ formError }}</p>
      <div class="profile-editor__actions">
        <n-button type="primary" :disabled="busy" @click="save">{{ t('app.common.save') }}</n-button>
        <n-button secondary :disabled="busy" @click="cancelEdit">{{ t('app.common.cancel') }}</n-button>
      </div>
    </section>
  </div>
</template>

<style scoped>
.profiles { display: flex; flex-direction: column; gap: 16px; }
.profiles-pref, .profile-editor, .profile-card, .profiles-empty {
  border: 1px solid var(--lt-border);
  border-radius: 12px;
  background: var(--lt-surface);
}
.profiles-pref {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 16px;
  padding: 14px 16px;
}
.profiles-pref__name, .profile-field > span { font-weight: 650; }
.profiles-pref__desc, .profile-hint, .profile-card__id p, .profile-card__members {
  margin: 4px 0 0;
  color: var(--lt-muted);
  font-size: 13px;
}
.profiles-toolbar { display: flex; gap: 8px; }
.profiles-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  padding: 28px 16px;
  text-align: center;
  color: var(--lt-muted);
}
.profiles-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
  gap: 12px;
}
.profile-card { padding: 14px; display: flex; flex-direction: column; gap: 10px; box-shadow: inset 3px 0 0 var(--profile-color, transparent); }
.profile-card.is-active { outline: 1px solid color-mix(in srgb, var(--profile-color, var(--lt-brand)) 55%, transparent); }
.profile-card__head { display: flex; align-items: center; gap: 10px; }
.profile-card__emoji {
  width: 36px;
  height: 36px;
  display: grid;
  place-items: center;
  border-radius: 10px;
  background: var(--lt-surface-soft);
  font-size: 18px;
}
.profile-card__id { flex: 1; min-width: 0; }
.profile-card__id h2, .profile-editor h2 { margin: 0; font-size: 16px; }
.profile-card__id p { margin: 0; }
.profile-card__badge {
  padding: 2px 8px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--lt-brand) 16%, transparent);
  color: var(--lt-brand);
  font-size: 12px;
  font-weight: 650;
}
.profile-card__members { min-height: 18px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
.profile-card__actions { display: flex; flex-wrap: wrap; gap: 6px; }
.profile-editor { padding: 16px; display: flex; flex-direction: column; gap: 14px; }
.profile-field { display: flex; flex-direction: column; gap: 8px; }
.profile-emojis, .profile-colors, .profile-editor__actions { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; }
.emoji-btn, .color-btn {
  width: 32px;
  height: 32px;
  border: 1px solid var(--lt-border);
  border-radius: 8px;
  background: var(--lt-surface-soft);
  cursor: pointer;
}
.emoji-btn.is-on, .color-btn.is-on { outline: 2px solid var(--lt-brand); outline-offset: 1px; }
.color-btn { border: 0; }
.emoji-input { width: 120px; }
.tunnel-pick { margin: 0; padding: 0; list-style: none; display: flex; flex-direction: column; gap: 4px; max-height: 220px; overflow: auto; }
.tunnel-pick label { display: flex; align-items: center; gap: 8px; padding: 4px 2px; cursor: pointer; }
.tunnel-pick__port { margin-left: auto; color: var(--lt-muted); font-variant-numeric: tabular-nums; }
.profile-error { margin: 0; color: var(--lt-danger-ink, #991b1b); }
</style>
