<script setup>
// Три вида одного приложения. Карточка острова скрыта, пока флаг выключен:
// полный и минималистичный вид остаются сегодняшними.
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useFeature } from '../../features/feature-store'

const props = defineProps({
  mode: { type: String, default: 'advanced' },
})
const emit = defineEmits(['change'])
const { t } = useI18n()
const islandOn = useFeature('dynamic_island')

const cards = computed(() => {
  const all = [
    {
      id: 'advanced',
      title: t('config.appViewFull'),
      text: t('config.appViewFullDesc'),
      art: 'full',
    },
    {
      id: 'simple',
      title: t('config.appViewMinimal'),
      text: t('config.appViewMinimalDesc'),
      art: 'minimal',
    },
    {
      id: 'island',
      title: t('config.appViewIsland'),
      text: t('config.appViewIslandDesc'),
      art: 'island',
    },
  ]
  return islandOn.value ? all : all.filter((card) => card.id !== 'island')
})

function choose(id) {
  if (id === props.mode) return
  emit('change', id)
}
</script>

<template>
  <section class="view-picker" :aria-label="t('config.appView')">
    <header class="view-picker__head">
      <h2>{{ t('config.appView') }}</h2>
      <p>{{ t('config.appViewDesc') }}</p>
    </header>
    <div class="view-picker__grid" :class="{ 'is-two': cards.length < 3 }" role="radiogroup" :aria-label="t('config.appView')">
      <button
        v-for="card in cards"
        :key="card.id"
        type="button"
        class="view-card"
        :class="{ 'is-current': mode === card.id }"
        role="radio"
        :aria-checked="mode === card.id ? 'true' : 'false'"
        @click="choose(card.id)"
      >
        <span class="view-card__art" :class="`is-${card.art}`" aria-hidden="true">
          <span v-if="card.art === 'full'" class="art-full">
            <i /><i /><i />
          </span>
          <span v-else-if="card.art === 'minimal'" class="art-minimal">
            <i />
          </span>
          <span v-else class="art-island"><i /></span>
        </span>
        <span class="view-card__title">
          <i class="view-card__radio" aria-hidden="true" />
          {{ card.title }}
          <em v-if="mode === card.id">{{ t('config.appViewCurrent') }}</em>
        </span>
        <span class="view-card__text">{{ card.text }}</span>
      </button>
    </div>
  </section>
</template>

<style scoped>
.view-picker { display: flex; flex-direction: column; gap: 14px; }
.view-picker__head h2 {
  margin: 0;
  font-size: 16px;
  font-weight: 700;
  letter-spacing: -0.02em;
}
.view-picker__head p { margin: 4px 0 0; color: var(--lt-muted); font-size: 13px; }
.view-picker__grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 12px;
}
.view-picker__grid.is-two { grid-template-columns: repeat(2, minmax(0, 1fr)); }
.view-card {
  display: flex;
  flex-direction: column;
  align-items: flex-start;
  gap: 8px;
  min-width: 0;
  padding: 10px;
  border-radius: 14px;
  border: 1px solid var(--lt-border);
  background: var(--lt-surface);
  color: var(--lt-ink);
  text-align: left;
  font: inherit;
  cursor: pointer;
}
.view-card.is-current {
  border-color: #3b82f6;
  box-shadow: inset 0 0 0 1px #3b82f6;
}
.view-card:focus-visible { outline: 2px solid var(--lt-focus); outline-offset: 2px; }
.view-card__art {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 92px;
  border-radius: 10px;
  background: color-mix(in srgb, var(--lt-ink) 6%, var(--lt-surface-soft));
}
.art-full, .art-minimal, .art-island { display: block; }
.art-full {
  width: 78%;
  height: 58px;
  border-radius: 8px;
  background: color-mix(in srgb, var(--lt-ink) 14%, var(--lt-surface));
  padding: 10px 12px;
  box-sizing: border-box;
}
.art-full i {
  display: block;
  height: 6px;
  margin-bottom: 6px;
  border-radius: 99px;
  background: color-mix(in srgb, var(--lt-ink) 28%, transparent);
}
.art-minimal {
  width: 46%;
  height: 28px;
  border-radius: 7px;
  background: color-mix(in srgb, var(--lt-ink) 16%, var(--lt-surface));
  position: relative;
}
.art-minimal i {
  position: absolute;
  left: 8px;
  top: 11px;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #3ddc84;
}
.art-island {
  width: 72%;
  height: 16px;
  border-radius: 99px;
  background: color-mix(in srgb, var(--lt-ink) 72%, #0b0d12);
  position: relative;
}
:root[data-theme="light"] .art-island { background: #fff; box-shadow: 0 4px 12px rgba(15, 23, 42, 0.12); }
.art-island i {
  position: absolute;
  left: 18%;
  top: 5px;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: #3ddc84;
}
.view-card__title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 14px;
  font-weight: 700;
}
.view-card__radio {
  width: 14px;
  height: 14px;
  border-radius: 50%;
  border: 2px solid var(--lt-border-strong);
  box-sizing: border-box;
}
.is-current .view-card__radio { border: 4px solid #3b82f6; }
.view-card__title em {
  margin-left: 2px;
  padding: 1px 6px;
  border-radius: 999px;
  background: #3b82f6;
  color: #fff;
  font-style: normal;
  font-size: 11px;
  font-weight: 650;
}
.view-card__text { color: var(--lt-muted); font-size: 12px; line-height: 1.35; }
@media (max-width: 860px) {
  .view-picker__grid, .view-picker__grid.is-two { grid-template-columns: 1fr; }
}
</style>
