<!--
  NorkaIcon.vue - Norka ("burrow") app icon as an inline, themeable SVG (Vue 3.5+, uses useId()).

  <NorkaIcon status="connected" />                     variant A, dark, 64px
  <NorkaIcon status="connecting" variant="B" theme="light" :size="32" />

  status  : 'connected' (green) | 'connecting' (amber + CSS blink) | 'error' (red) | 'stopped' (eyes closed, asleep)
            | 'partial' (left eye green, right eye red: some tunnels up, some failed)  [ssh-client addition]
  variant : 'A' (tall arch) | 'B' (flat mound)
  theme   : 'dark' | 'light'
  statusLabel : optional text for the tooltip/aria-label instead of the raw status  [ssh-client addition]
  All colours are CSS custom properties, so the host can override them, e.g.
    .my-tray { --norka-bg: transparent; --norka-arch: currentColor; }

  Wink on successful connect (Lottie, vector JSON shipped next to this file):
    // npm i @lottiefiles/dotlottie-vue   (ThorVG/wasm renderer; verified to render norka-*-wink.json)
    <DotLottieVue v-if="justConnected" :src="winkUrl" :autoplay="true" :loop="false"
                  style="width:64px;height:64px" @complete="justConnected = false" />
    // or lottie-web (SVG renderer, verified frame-by-frame):
    //   import lottie from 'lottie-web'
    //   const a = lottie.loadAnimation({ container: el, renderer: 'svg', loop: false, autoplay: true,
    //                                    path: '/icons/norka-A-wink.json' })
    //   a.addEventListener('complete', () => { a.destroy(); showStaticIcon() })
    // Watch `status` -> when it changes from 'connecting' to 'connected', show the Lottie once
    // (3 s loop; the wink itself is at 0.83-1.7 s, there is a "wink" marker at frame 50),
    // then swap back to this static component. Frame 0 of the Lottie == the static icon.
-->
<script setup lang="ts">
import { computed, useId } from 'vue'

type Status = 'connected' | 'connecting' | 'error' | 'stopped' | 'partial'
type EyeStatus = Exclude<Status, 'partial'>
type Side = 'left' | 'right'
const props = withDefaults(defineProps<{
  status?: Status
  variant?: 'A' | 'B'
  theme?: 'dark' | 'light'
  size?: number | string
  title?: string
  statusLabel?: string
  // render the extra parts used by the idle behaviour (useNorkaIdle): open eyes under the closed ones
  idle?: boolean
}>(), { status: 'connected', variant: 'A', theme: 'dark', size: 64, title: 'Norka', idle: false })
// human-readable status for <title>/aria-label (e.g. localized); falls back to the raw status
const label = computed(() => `${props.title}: ${props.statusLabel || props.status}`)

const EYE: Record<EyeStatus, string> = {
  connected: '#3DDC84', connecting: '#F5C542', error: '#FF5A5F', stopped: '#8A9099',
}
const THEME = {
  dark:  { bg: '#1E2127', arch: '#F2EFEA', pupil: '#101215', lid: '#2B3631', glow: 0.55, shade: 0.28 },
  light: { bg: '#F6F4F0', arch: '#1E2127', pupil: '#14171B', lid: '#A3C4B0', glow: 0.22, shade: 0.20 },
} as const

// Geometry generated from the master SVGs (viewBox 0 0 512 512, eye paths in eye-local coords)
const GEOMETRY = {
  "A": {
    "arch": "M50 392L50 292C50 171.68 124.16 104 256 104C387.84 104 462 171.68 462 292L462 392C462 400.84 454.84 408 446 408L392 408C383.16 408 376 400.84 376 392L376 326C376 265.68 325.6 222 256 222C186.4 222 136 265.68 136 326L136 392C136 400.84 128.84 408 120 408L66 408C57.16 408 50 400.84 50 392Z",
    "glow": {
      "rx": 70,
      "ry": 56
    },
    "eyes": [
      {
        "side": "left",
        "cx": 200,
        "cy": 344,
        "almond": "M-45 -4C-36 -22.6 -27 -31 0 -31C27 -31 36 -18.6 45 0C36 18.6 27.9 31 0 31C-27.9 31 -36 14.6 -45 -4Z",
        "lid": "M-85.5 -186L85.5 -186L85.5 -3C28.5 1 -28.5 -10 -85.5 -14L-85.5 -186Z",
        "shade": "M-85.5 -186L85.5 -186L85.5 4C28.5 9.33 -28.5 -1.67 -85.5 -7L-85.5 -186Z",
        "pupil": "M-5 -21C-2.3 -21 2.5 -9.75 2.5 4C2.5 17.75 -2.3 29 -5 29C-7.7 29 -12.5 17.75 -12.5 4C-12.5 -9.75 -7.7 -21 -5 -21Z",
        "hl1": "M3 1.2C6.2 1.2 8.8 3.8 8.8 7C8.8 10.2 6.2 12.8 3 12.8C-0.2 12.8 -2.8 10.2 -2.8 7C-2.8 3.8 -0.2 1.2 3 1.2Z",
        "hl2": "M-9.4 12.4C-7.96 12.4 -6.8 13.56 -6.8 15C-6.8 16.44 -7.96 17.6 -9.4 17.6C-10.84 17.6 -12 16.44 -12 15C-12 13.56 -10.84 12.4 -9.4 12.4Z",
        "closed": "M-54 -2.79C-50.4 1.98 -45 5.39 -45 5.39C-24.75 20.4 16.65 23.81 41.4 6.76"
      },
      {
        "side": "right",
        "cx": 312,
        "cy": 344,
        "almond": "M0 31C-27.9 31 -36 18.6 -45 0C-36 -18.6 -27 -31 0 -31C27 -31 36 -22.6 45 -4C36 14.6 27.9 31 0 31Z",
        "lid": "M85.5 -14C28.5 -10 -28.5 1 -85.5 -3L-85.5 -186L85.5 -186L85.5 -14Z",
        "shade": "M85.5 -7C28.5 -1.67 -28.5 9.33 -85.5 4L-85.5 -186L85.5 -186L85.5 -7Z",
        "pupil": "M-5 -21C-2.3 -21 2.5 -9.75 2.5 4C2.5 17.75 -2.3 29 -5 29C-7.7 29 -12.5 17.75 -12.5 4C-12.5 -9.75 -7.7 -21 -5 -21Z",
        "hl1": "M3 1.2C6.2 1.2 8.8 3.8 8.8 7C8.8 10.2 6.2 12.8 3 12.8C-0.2 12.8 -2.8 10.2 -2.8 7C-2.8 3.8 -0.2 1.2 3 1.2Z",
        "hl2": "M-9.4 12.4C-7.96 12.4 -6.8 13.56 -6.8 15C-6.8 16.44 -7.96 17.6 -9.4 17.6C-10.84 17.6 -12 16.44 -12 15C-12 13.56 -10.84 12.4 -9.4 12.4Z",
        "closed": "M-41.4 6.76C-16.65 23.81 24.75 20.4 45 5.39C45 5.39 50.4 1.98 54 -2.79"
      }
    ]
  },
  "B": {
    "arch": "M55 420C48.92 420 44 415.08 44 409C44 391 64 397.2 64 360C64 233.46 146.56 138 256 138C365.44 138 448 233.46 448 360C448 397.2 468 391 468 409C468 415.08 463.08 420 457 420L357 420C352.03 420 348 415.97 348 411C348 395.4 362 400.24 362 368C362 313.12 315.36 270 256 270C196.64 270 150 313.12 150 368C150 400.24 164 395.4 164 411C164 415.97 159.97 420 155 420L55 420Z",
    "glow": {
      "rx": 58,
      "ry": 56
    },
    "eyes": [
      {
        "side": "left",
        "cx": 201,
        "cy": 360,
        "almond": "M-36 -4C-28.8 -23.8 -21.6 -33 0 -33C21.6 -33 28.8 -19.8 36 0C28.8 19.8 22.32 33 0 33C-22.32 33 -28.8 15.8 -36 -4Z",
        "lid": "M-68.4 -198L68.4 -198L68.4 -4C22.8 -1.33 -22.8 -19.33 -68.4 -22L-68.4 -198Z",
        "shade": "M-68.4 -198L68.4 -198L68.4 3C22.8 7 -22.8 -11 -68.4 -15L-68.4 -198Z",
        "pupil": "M-3 -24C-0.66 -24 3.5 -12.75 3.5 1C3.5 14.75 -0.66 26 -3 26C-5.34 26 -9.5 14.75 -9.5 1C-9.5 -12.75 -5.34 -24 -3 -24Z",
        "hl1": "M4 -13.6C7.09 -13.6 9.6 -11.09 9.6 -8C9.6 -4.91 7.09 -2.4 4 -2.4C0.91 -2.4 -1.6 -4.91 -1.6 -8C-1.6 -11.09 0.91 -13.6 4 -13.6Z",
        "hl2": "M-6.85 9.6C-5.52 9.6 -4.45 10.67 -4.45 12C-4.45 13.33 -5.52 14.4 -6.85 14.4C-8.18 14.4 -9.25 13.33 -9.25 12C-9.25 10.67 -8.18 9.6 -6.85 9.6Z",
        "closed": "M-43.2 -2.97C-40.32 2.11 -36 5.74 -36 5.74C-19.8 21.71 13.32 25.34 33.12 7.19"
      },
      {
        "side": "right",
        "cx": 311,
        "cy": 360,
        "almond": "M0 33C-22.32 33 -28.8 19.8 -36 0C-28.8 -19.8 -21.6 -33 0 -33C21.6 -33 28.8 -23.8 36 -4C28.8 15.8 22.32 33 0 33Z",
        "lid": "M68.4 -30C22.8 -30 -22.8 -29 -68.4 -29L-68.4 -198L68.4 -198L68.4 -30Z",
        "shade": "M68.4 -23C22.8 -21.67 -22.8 -20.67 -68.4 -22L-68.4 -198L68.4 -198L68.4 -23Z",
        "pupil": "M-3 -24C-0.66 -24 3.5 -12.75 3.5 1C3.5 14.75 -0.66 26 -3 26C-5.34 26 -9.5 14.75 -9.5 1C-9.5 -12.75 -5.34 -24 -3 -24Z",
        "hl1": "M4 -13.6C7.09 -13.6 9.6 -11.09 9.6 -8C9.6 -4.91 7.09 -2.4 4 -2.4C0.91 -2.4 -1.6 -4.91 -1.6 -8C-1.6 -11.09 0.91 -13.6 4 -13.6Z",
        "hl2": "M-6.85 9.6C-5.52 9.6 -4.45 10.67 -4.45 12C-4.45 13.33 -5.52 14.4 -6.85 14.4C-8.18 14.4 -9.25 13.33 -9.25 12C-9.25 10.67 -8.18 9.6 -6.85 9.6Z",
        "closed": "M-33.12 7.19C-13.32 25.34 19.8 21.71 36 5.74C36 5.74 40.32 2.11 43.2 -2.97"
      }
    ]
  }
} as const

const uid = useId()
const g = computed(() => GEOMETRY[props.variant])
const t = computed(() => THEME[props.theme])
const asleep = computed(() => props.status === 'stopped')
// 'partial' = left eye connected (green), right eye error (red); every other status drives both eyes
const eyeStatus = (side: Side): EyeStatus =>
  props.status === 'partial' ? (side === 'left' ? 'connected' : 'error') : props.status
const eyeLid = (s: EyeStatus) => s === 'connected' ? t.value.lid : (props.theme === 'dark' ? '#30343B' : '#C9CCD1')
// per-eye resolved colours (also written as presentation attributes, so the SVG is correct even without CSS)
const eyeColors = computed(() => Object.fromEntries((['left', 'right'] as Side[]).map((side) => {
  const s = eyeStatus(side)
  return [side, { status: s, eye: EYE[s], lid: eyeLid(s) }]
})) as Record<Side, { status: EyeStatus, eye: string, lid: string }>)
const c = computed(() => ({
  eye: eyeColors.value.left.eye, bg: t.value.bg, arch: t.value.arch, pupil: t.value.pupil, lid: eyeColors.value.left.lid,
}))
const style = computed(() => ({
  '--norka-eye-default': c.value.eye, '--norka-bg-default': c.value.bg, '--norka-arch-default': c.value.arch,
  '--norka-pupil-default': c.value.pupil, '--norka-lid-default': c.value.lid,
}))
const eyeStyle = (side: Side) => ({
  '--norka-eye-default': eyeColors.value[side].eye, '--norka-lid-default': eyeColors.value[side].lid,
})
</script>

<template>
  <svg class="norka" :class="[`norka--${status}`]" viewBox="0 0 512 512" :width="size" :height="size"
       role="img" :aria-label="label" :style="style">
    <title>{{ label }}</title>
    <defs>
      <radialGradient v-for="e in g.eyes" :key="e.side" :id="`${uid}-${e.side}-glow`" :style="eyeStyle(e.side)"
                      cx="0.5" cy="0.5" r="0.5">
        <stop offset="0" class="norka-glow-stop" :stop-color="eyeColors[e.side].eye" :stop-opacity="t.glow" />
        <stop offset="0.45" class="norka-glow-stop" :stop-color="eyeColors[e.side].eye" :stop-opacity="t.glow * 0.45" />
        <stop offset="1" class="norka-glow-stop" :stop-color="eyeColors[e.side].eye" stop-opacity="0" />
      </radialGradient>
      <mask v-for="e in g.eyes" :key="e.side" :id="`${uid}-${e.side}-vis`"
            maskUnits="userSpaceOnUse" x="-120" y="-120" width="240" height="240">
        <path :d="e.almond" fill="#fff" />
        <path :d="e.lid" fill="#000" />
      </mask>
    </defs>
    <rect class="norka-bg" width="512" height="512" rx="115" :fill="c.bg" />
    <path class="norka-arch" :d="g.arch" :fill="c.arch" />
    <g v-for="e in g.eyes" :key="e.side" :id="`eye-${e.side}`" :data-side="e.side"
       :transform="`translate(${e.cx} ${e.cy})`" :style="eyeStyle(e.side)">
      <g v-if="!asleep || idle" class="norka-eye"
         :class="[`norka-eye--${eyeColors[e.side].status}`, { 'norka-eye--peek': asleep }]">
        <ellipse cx="0" cy="2" :rx="g.glow.rx" :ry="g.glow.ry" :fill="`url(#${uid}-${e.side}-glow)`" />
        <path class="norka-lid" :d="e.almond" :fill="eyeColors[e.side].lid" />
        <g :mask="`url(#${uid}-${e.side}-vis)`">
          <path class="norka-iris" :d="e.almond" :fill="eyeColors[e.side].eye" />
          <path :d="e.shade" fill="#000" :fill-opacity="t.shade" />
          <g class="norka-look">
            <path class="norka-pupil" :d="e.pupil" :fill="c.pupil" />
            <path class="norka-hl" :d="e.hl1" fill="#fff" />
            <path class="norka-hl" :d="e.hl2" fill="#fff" fill-opacity="0.75" />
          </g>
        </g>
      </g>
      <path v-if="asleep" class="norka-closed" :d="e.closed" fill="none" :stroke="c.eye" stroke-width="8"
            stroke-linecap="round" stroke-linejoin="round" />
    </g>
  </svg>
</template>

<style scoped>
.norka { display: inline-block; vertical-align: middle; }
.norka-bg     { fill: var(--norka-bg, var(--norka-bg-default)); }
.norka-arch   { fill: var(--norka-arch, var(--norka-arch-default)); }
.norka-iris   { fill: var(--norka-eye, var(--norka-eye-default)); }
.norka-glow-stop { stop-color: var(--norka-eye, var(--norka-eye-default)); }
.norka-lid    { fill: var(--norka-lid, var(--norka-lid-default)); }
.norka-pupil  { fill: var(--norka-pupil, var(--norka-pupil-default)); }
.norka-hl     { fill: var(--norka-highlight, #fff); }
.norka-closed { stroke: var(--norka-eye, var(--norka-eye-default)); }

/* idle (useNorkaIdle): while asleep the open eye waits, hidden, under the closed one */
.norka-eye--peek { opacity: 0; transform: scaleY(0.02); }
/* connecting: slow, "thinking" blink (both eyes squash toward the lower lid) */
.norka-eye { transform-box: fill-box; transform-origin: 50% 70%; transition: transform .15s ease; }
.norka--connecting .norka-eye { animation: norka-blink 1.6s ease-in-out infinite; }
/* error (and the red eye of 'partial'): a nervous little glow pulse */
.norka-eye--error ellipse { animation: norka-pulse 0.9s ease-in-out infinite; }
@keyframes norka-blink {
  0%, 38%, 62%, 100% { transform: scaleY(1); }
  48%, 52%           { transform: scaleY(0.08); }
}
@keyframes norka-pulse { 0%, 100% { opacity: 1; } 50% { opacity: .45; } }
@media (prefers-reduced-motion: reduce) {
  .norka-eye, .norka-eye ellipse { animation: none !important; }
}
</style>
