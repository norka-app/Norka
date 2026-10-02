<!--
  NorkaIcon.vue - Norka ("burrow") app icon as an inline, themeable SVG (Vue 3.5+, uses useId()).

  <NorkaIcon status="connected" />                     variant A, dark, 64px
  <NorkaIcon status="connecting" variant="B" theme="light" :size="32" />

  status  : 'connected' (green) | 'connecting' (amber + CSS blink) | 'error' (red) | 'stopped' (eyes closed, asleep)
            both eyes always share one status (the latest tunnel event, see utils/norka-status.js)  [ssh-client]
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
import { KNOCKOUT_VARIANTS } from './norka-knockout'

type Status = 'connected' | 'connecting' | 'error' | 'stopped'
type EyeStatus = Status
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
  // easter egg (useNorkaKnockout): knockout variant being played, its fade-out, static reduced-motion version
  knockout?: string | null
  knockoutLeaving?: boolean
  knockoutStill?: boolean
}>(), { status: 'connected', variant: 'A', theme: 'dark', size: 64, title: 'Norka', idle: false,
  knockout: null, knockoutLeaving: false, knockoutStill: false })
// human-readable status for <title>/aria-label (e.g. localized); falls back to the raw status
const label = computed(() => `${props.title}: ${props.statusLabel || props.status}`)

const EYE: Record<EyeStatus, string> = {
  connected: '#3DDC84', connecting: '#F5C542', error: '#FF5A5F', stopped: '#8A9099',
}
// ko: stars and sparks of the knockout easter egg
const THEME = {
  dark:  { bg: '#1E2127', arch: '#F2EFEA', pupil: '#101215', lid: '#2B3631', glow: 0.55, shade: 0.28, ko: '#F5C542' },
  light: { bg: '#F6F4F0', arch: '#1E2127', pupil: '#14171B', lid: '#A3C4B0', glow: 0.22, shade: 0.20, ko: '#D99A00' },
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
// both eyes always show the same status — there is no mixed (green + red) state
const eyeStatus = (_side: Side): EyeStatus => props.status
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
// ── knockout easter egg (shapes in eye-local / icon coordinates) ──
const polar = (r: number, a: number) => `${(r * Math.cos(a)).toFixed(1)} ${(r * Math.sin(a)).toFixed(1)}`
// 5-point star, outer radius 24
const KO_STAR = Array.from({ length: 10 }, (_, i) =>
  `${i ? 'L' : 'M'}${polar(i % 2 ? 10 : 24, -Math.PI / 2 + i * Math.PI / 5)}`).join('') + 'Z'
// Archimedean spiral, 2.5 turns out to r = 26
const KO_SPIRAL = Array.from({ length: 61 }, (_, i) =>
  `${i ? 'L' : 'M'}${polar(2 + 24 * i / 60, i / 60 * 5 * Math.PI)}`).join('')
const KO_SPARKLE = 'M0 -16Q2.5 -2.5 16 0Q2.5 2.5 0 16Q-2.5 2.5 -16 0Q-2.5 -2.5 0 -16Z'
const KO_SPARKS = [[78, 150, 0], [436, 172, 0.35], [44, 300, 0.7], [468, 290, 0.2], [256, 64, 0.55], [150, 92, 0.9], [366, 96, 1.1]]
const ko = computed(() => props.knockout && props.knockout in KNOCKOUT_VARIANTS ? props.knockout : null)
// orbit centre above the arch
const koTop = computed(() => (props.variant === 'B' ? 92 : 62))
const koClass = computed(() => ko.value
  ? ['norka--ko', `norka--ko-${ko.value}`, { 'norka--ko-out': props.knockoutLeaving, 'norka--ko-still': props.knockoutStill }]
  : [])
const svgStyle = computed(() => ({
  ...style.value,
  '--norka-ko-default': t.value.ko,
  ...(ko.value ? { '--ko-dur': `${KNOCKOUT_VARIANTS[ko.value as keyof typeof KNOCKOUT_VARIANTS]}ms` } : {}),
}))

const eyeStyle = (side: Side) => ({
  '--norka-eye-default': eyeColors.value[side].eye, '--norka-lid-default': eyeColors.value[side].lid,
})
</script>

<template>
  <svg class="norka" :class="[`norka--${status}`, koClass]" viewBox="0 0 512 512" :width="size" :height="size"
       role="img" :aria-label="label" :style="svgStyle">
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
    <g class="norka-figure">
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
          <!-- knockout «birds»: the real eyes stay, the upper lid droops -->
          <g v-if="ko === 'birds'" class="ko-droop">
            <path :d="e.shade" fill="#000" :fill-opacity="t.shade" />
            <path :d="e.lid" :fill="eyeColors[e.side].lid" />
          </g>
        </g>
      </g>
      <path v-if="asleep" class="norka-closed" :d="e.closed" fill="none" :stroke="c.eye" stroke-width="8"
            stroke-linecap="round" stroke-linejoin="round" />
      <!-- knockout eyes: replace the normal ones while the easter egg plays -->
      <g v-if="ko && ko !== 'birds'" class="norka-ko-layer norka-ko-eye" fill="none" :stroke="c.arch" stroke-linecap="round"
         stroke-linejoin="round">
        <g v-if="ko === 'xEyes'" class="ko-x">
          <path d="M-21 -21L21 21M21 -21L-21 21" stroke-width="11" />
        </g>
        <g v-else-if="ko === 'spiral'" :class="['ko-spiral', `ko-spiral--${e.side}`]">
          <path :d="KO_SPIRAL" stroke-width="6" :transform="e.side === 'right' ? 'scale(-1 1)' : undefined" />
        </g>
        <g v-else-if="ko === 'stars'" class="ko-daze">
          <path :d="e.closed" stroke-width="9" />
        </g>
        <path v-else-if="ko === 'wobble'" :d="e.side === 'left' ? 'M-18 -17L15 0L-18 17' : 'M18 -17L-15 0L18 17'"
              stroke-width="9" />
      </g>
    </g>
    </g>
    <!-- knockout overlays above the arch -->
    <g v-if="ko && !knockoutStill && ko !== 'xEyes' && ko !== 'spiral'" class="norka-ko-layer">
      <g v-if="ko === 'stars' || ko === 'birds'" :transform="`translate(256 ${koTop})`">
        <g v-for="i in 3" :key="i" class="ko-orb-x" :style="{ '--k': i - 1 }">
          <g class="ko-orb-y">
            <path v-if="ko === 'stars'" class="ko-star" :d="KO_STAR" />
            <path v-else class="ko-bird" d="M-27 -1Q-14 -19 0 0Q14 -19 27 -1" fill="none" :stroke="c.arch"
                  stroke-width="8" stroke-linecap="round" stroke-linejoin="round" />
          </g>
        </g>
      </g>
      <g v-else-if="ko === 'wobble'">
        <g v-for="([x, y, delay], i) in KO_SPARKS" :key="i" :transform="`translate(${x} ${y})`">
          <path class="ko-spark" :d="KO_SPARKLE" :style="{ animationDelay: `${delay}s` }" />
        </g>
      </g>
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
.norka-eye { transform-box: fill-box; transform-origin: 50% 70%; transition: transform .15s ease, opacity .3s ease; }
.norka-closed { transition: opacity .3s ease; }
.norka--connecting .norka-eye { animation: norka-blink 1.6s ease-in-out infinite; }
/* error: a nervous little glow pulse */
.norka-eye--error ellipse { animation: norka-pulse 0.9s ease-in-out infinite; }
@keyframes norka-blink {
  0%, 38%, 62%, 100% { transform: scaleY(1); }
  48%, 52%           { transform: scaleY(0.08); }
}
@keyframes norka-pulse { 0%, 100% { opacity: 1; } 50% { opacity: .45; } }
@media (prefers-reduced-motion: reduce) {
  .norka-eye, .norka-eye ellipse { animation: none !important; }
}

/* ── knockout easter egg (useNorkaKnockout); duration in --ko-dur ── */
.norka--ko:not(.norka--ko-out):not(.norka--ko-birds) .norka-eye,
.norka--ko:not(.norka--ko-out) .norka-closed { opacity: 0; }
.norka-ko-layer { animation: norka-ko-in .2s ease-out backwards; transition: opacity .4s ease; }
.norka--ko-out .norka-ko-layer { opacity: 0; }
.ko-star, .ko-spark { fill: var(--norka-ko, var(--norka-ko-default)); }
/* SVG transforms below rotate/scale around the local origin (eye centre, orbit centre) */
.norka-ko-layer g, .norka-ko-layer path, .norka-figure { transform-box: view-box; }

/* stars / birds: x and y swing a quarter period apart -> an ellipse; nearer = bigger */
.ko-orb-x, .ko-orb-y {
  --ko-orb: 0.75s;
  animation: ko-orb-x var(--ko-orb) cubic-bezier(.37, 0, .63, 1) infinite alternate;
  animation-delay: calc(var(--k) * var(--ko-orb) * -2 / 3);
}
.ko-orb-y {
  animation-name: ko-orb-y;
  animation-delay: calc(var(--k) * var(--ko-orb) * -2 / 3 - var(--ko-orb) / 2);
}
.norka--ko-birds .ko-orb-x, .norka--ko-birds .ko-orb-y { --ko-orb: 0.95s; }
@keyframes ko-orb-x { from { transform: translateX(-112px); } to { transform: translateX(112px); } }
@keyframes ko-orb-y {
  from { transform: translateY(-22px) scale(.7); opacity: .6; }
  to   { transform: translateY(22px) scale(1.15); opacity: 1; }
}
.ko-star { animation: ko-spin 1.2s linear infinite; }
.ko-bird { animation: ko-flap .22s ease-in-out infinite alternate; }
@keyframes ko-flap { to { transform: scaleY(.4); } }
.ko-daze { animation: ko-bob .5s ease-in-out infinite alternate; }
@keyframes ko-bob { from { transform: translateY(-3px); } to { transform: translateY(4px); } }
/* birds: the real eyes stay; slit pupils roll and drift on their own, a bit cross-eyed, lids droop */
.norka--ko-birds:not(.norka--ko-out) .norka-eye--peek { opacity: 1; transform: none; }
.norka--ko-birds [data-side="left"] .norka-look { animation: ko-roll-l var(--ko-dur) linear both; }
.norka--ko-birds [data-side="right"] .norka-look { animation: ko-roll-r var(--ko-dur) linear both; }
.norka--ko-birds [data-side="left"] .ko-droop { animation: ko-droop-l var(--ko-dur) linear both; }
.norka--ko-birds [data-side="right"] .ko-droop { animation: ko-droop-r var(--ko-dur) linear both; }
@keyframes ko-roll-l {
  0% { transform: translate(0.0px, 0.0px) rotate(0.0deg); }
  3.125% { transform: translate(3.2px, 1.0px) rotate(0.7deg); }
  6.25% { transform: translate(8.2px, 4.3px) rotate(3.9deg); }
  9.375% { transform: translate(9.4px, 8.2px) rotate(8.8deg); }
  12.5% { transform: translate(5.6px, 9.5px) rotate(11.6deg); }
  15.62% { transform: translate(1.2px, 8.4px) rotate(12.0deg); }
  18.75% { transform: translate(-1.5px, 6.5px) rotate(11.0deg); }
  21.88% { transform: translate(-1.9px, 4.2px) rotate(8.9deg); }
  25% { transform: translate(0.1px, 1.8px) rotate(5.8deg); }
  28.12% { transform: translate(4.0px, 0.0px) rotate(2.1deg); }
  31.25% { transform: translate(9.0px, -0.9px) rotate(-1.9deg); }
  34.38% { transform: translate(14.0px, -0.8px) rotate(-5.6deg); }
  37.5% { transform: translate(17.9px, 0.2px) rotate(-8.7deg); }
  40.62% { transform: translate(19.9px, 1.9px) rotate(-10.9deg); }
  43.75% { transform: translate(19.5px, 3.7px) rotate(-11.9deg); }
  46.88% { transform: translate(16.8px, 5.3px) rotate(-11.7deg); }
  50% { transform: translate(12.4px, 6.1px) rotate(-10.1deg); }
  53.12% { transform: translate(7.3px, 6.1px) rotate(-7.5deg); }
  56.25% { transform: translate(2.5px, 5.1px) rotate(-4.1deg); }
  59.38% { transform: translate(-0.8px, 3.3px) rotate(-0.2deg); }
  62.5% { transform: translate(-2.0px, 1.2px) rotate(3.7deg); }
  65.62% { transform: translate(-0.8px, -0.9px) rotate(7.2deg); }
  68.75% { transform: translate(2.5px, -2.3px) rotate(9.9deg); }
  71.88% { transform: translate(7.3px, -2.7px) rotate(11.6deg); }
  75% { transform: translate(12.4px, -2.1px) rotate(12.0deg); }
  78.12% { transform: translate(16.8px, -0.3px) rotate(11.1deg); }
  81.25% { transform: translate(19.2px, 2.1px) rotate(8.9deg); }
  84.38% { transform: translate(16.6px, 4.1px) rotate(5.0deg); }
  87.5% { transform: translate(10.5px, 4.4px) rotate(1.3deg); }
  90.62% { transform: translate(4.4px, 2.9px) rotate(-0.5deg); }
  93.75% { transform: translate(0.9px, 1.0px) rotate(-0.5deg); }
  96.88% { transform: translate(0.0px, 0.0px) rotate(-0.0deg); }
  100% { transform: translate(0.0px, 0.0px) rotate(-0.0deg); }
}
@keyframes ko-roll-r {
  0% { transform: translate(-0.0px, 0.0px) rotate(-0.0deg); }
  3.125% { transform: translate(-1.0px, 1.4px) rotate(-1.8deg); }
  6.25% { transform: translate(-0.7px, 3.3px) rotate(-3.4deg); }
  9.375% { transform: translate(0.9px, 2.6px) rotate(-0.6deg); }
  12.5% { transform: translate(-0.4px, -0.3px) rotate(5.1deg); }
  15.62% { transform: translate(-4.9px, -2.5px) rotate(10.0deg); }
  18.75% { transform: translate(-10.8px, -2.9px) rotate(13.2deg); }
  21.88% { transform: translate(-16.1px, -1.5px) rotate(14.0deg); }
  25% { transform: translate(-18.9px, 1.4px) rotate(12.2deg); }
  28.12% { transform: translate(-18.1px, 4.6px) rotate(8.3deg); }
  31.25% { transform: translate(-14.0px, 7.2px) rotate(2.9deg); }
  34.38% { transform: translate(-8.2px, 8.3px) rotate(-3.0deg); }
  37.5% { transform: translate(-2.6px, 7.7px) rotate(-8.4deg); }
  40.62% { transform: translate(0.6px, 5.7px) rotate(-12.3deg); }
  43.75% { transform: translate(0.4px, 3.2px) rotate(-14.0deg); }
  46.88% { transform: translate(-3.2px, 1.2px) rotate(-13.2deg); }
  50% { transform: translate(-8.8px, 0.6px) rotate(-10.0deg); }
  53.12% { transform: translate(-14.6px, 1.7px) rotate(-5.1deg); }
  56.25% { transform: translate(-18.3px, 4.2px) rotate(0.8deg); }
  59.38% { transform: translate(-18.7px, 7.2px) rotate(6.5deg); }
  62.5% { transform: translate(-15.6px, 9.6px) rotate(11.0deg); }
  65.62% { transform: translate(-10.1px, 10.6px) rotate(13.6deg); }
  68.75% { transform: translate(-4.2px, 9.6px) rotate(13.7deg); }
  71.88% { transform: translate(-0.1px, 7.1px) rotate(11.4deg); }
  75% { transform: translate(0.9px, 3.6px) rotate(7.1deg); }
  78.12% { transform: translate(-1.7px, 0.4px) rotate(1.4deg); }
  81.25% { transform: translate(-6.8px, -1.6px) rotate(-4.4deg); }
  84.38% { transform: translate(-10.8px, -1.5px) rotate(-8.0deg); }
  87.5% { transform: translate(-10.3px, -0.1px) rotate(-7.6deg); }
  90.62% { transform: translate(-6.0px, 0.8px) rotate(-4.4deg); }
  93.75% { transform: translate(-1.6px, 0.5px) rotate(-1.2deg); }
  96.88% { transform: translate(-0.0px, 0.0px) rotate(-0.0deg); }
  100% { transform: translate(-0.0px, 0.0px) rotate(0.0deg); }
}
@keyframes ko-droop-l {
  0% { transform: translateY(0.0px); }
  6.25% { transform: translateY(2.7px); }
  12.5% { transform: translateY(5.9px); }
  18.75% { transform: translateY(5.9px); }
  25% { transform: translateY(5.2px); }
  31.25% { transform: translateY(4.0px); }
  37.5% { transform: translateY(2.8px); }
  43.75% { transform: translateY(2.1px); }
  50% { transform: translateY(2.1px); }
  56.25% { transform: translateY(2.8px); }
  62.5% { transform: translateY(4.0px); }
  68.75% { transform: translateY(5.2px); }
  75% { transform: translateY(5.9px); }
  81.25% { transform: translateY(5.8px); }
  87.5% { transform: translateY(3.0px); }
  93.75% { transform: translateY(0.4px); }
  100% { transform: translateY(0.0px); }
}
@keyframes ko-droop-r {
  0% { transform: translateY(0.0px); }
  6.25% { transform: translateY(3.7px); }
  12.5% { transform: translateY(5.5px); }
  18.75% { transform: translateY(4.1px); }
  25% { transform: translateY(3.5px); }
  31.25% { transform: translateY(3.8px); }
  37.5% { transform: translateY(5.0px); }
  43.75% { transform: translateY(6.5px); }
  50% { transform: translateY(7.9px); }
  56.25% { transform: translateY(8.5px); }
  62.5% { transform: translateY(8.2px); }
  68.75% { transform: translateY(7.0px); }
  75% { transform: translateY(5.5px); }
  81.25% { transform: translateY(4.1px); }
  87.5% { transform: translateY(2.1px); }
  93.75% { transform: translateY(0.4px); }
  100% { transform: translateY(0.0px); }
}

/* xEyes: hit -> shake, eyes pop in */
.norka--ko-xEyes .norka-figure { animation: ko-hit .5s ease-out; }
@keyframes ko-hit {
  0%, 100% { transform: translateX(0); }
  15% { transform: translateX(-14px) rotate(-2deg); } 35% { transform: translateX(11px); }
  55% { transform: translateX(-7px); } 75% { transform: translateX(4px); }
}
.ko-x { animation: ko-pop .4s cubic-bezier(.34, 1.56, .64, 1) backwards, ko-tilt 1.1s .4s ease-in-out infinite alternate; }
@keyframes ko-pop { from { transform: scale(0); } }
@keyframes ko-tilt { from { transform: rotate(-8deg); } to { transform: rotate(8deg); } }

/* spiral: eyes spin in opposite directions, the head leans */
.ko-spiral--left { animation: ko-spin .9s linear infinite; }
.ko-spiral--right { animation: ko-spin .9s linear infinite reverse; }
.norka--ko-spiral .norka-figure { transform-origin: 256px 408px; animation: ko-lean var(--ko-dur) ease-in-out; }
@keyframes ko-lean { 0%, 100% { transform: rotate(0); } 30% { transform: rotate(-4deg); } 65% { transform: rotate(4deg); } }

/* wobble: dizzy sway that settles, sparks pop around */
.norka--ko-wobble .norka-figure { transform-origin: 256px 408px; animation: ko-wobble var(--ko-dur) ease-in-out; }
@keyframes ko-wobble {
  0%, 92%, 100% { transform: rotate(0); }
  10% { transform: rotate(-9deg); } 22% { transform: rotate(8deg); } 34% { transform: rotate(-7deg); }
  46% { transform: rotate(5.5deg); } 58% { transform: rotate(-4deg); } 70% { transform: rotate(2.5deg); }
  82% { transform: rotate(-1deg); }
}
.ko-spark { opacity: 0; animation: ko-spark .9s ease-out infinite backwards; }
@keyframes ko-spark {
  0% { transform: scale(0) rotate(0); opacity: 0; }
  30% { transform: scale(1.1) rotate(25deg); opacity: 1; }
  60%, 100% { transform: scale(0) rotate(60deg); opacity: 0; }
}
@keyframes ko-spin { to { transform: rotate(360deg); } }

/* prefers-reduced-motion: static X eyes only, no movement or fades */
.norka--ko-still *, .norka--ko-still { animation: none !important; transition: none !important; }
</style>
