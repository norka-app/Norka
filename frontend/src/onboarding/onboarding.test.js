import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import {
  onboardingKeyAction,
  onboardingSteps,
  placeOnboardingCard,
  reduceOnboarding,
  shouldShowOnboarding,
  spotlightBox,
  visibleTargetRect,
} from './onboarding.js'

const ALL_FLAGS = { ssh_command: true, quick_search: true }

describe('onboarding step filtering', () => {
  it('keeps five steps and mentions ssh and quick search when those flags are on', () => {
    const steps = onboardingSteps(ALL_FLAGS)
    assert.deepEqual(steps.map((step) => step.id), ['welcome', 'add-tunnel', 'eyes', 'tray', 'features'])
    assert.ok(steps.find((step) => step.id === 'add-tunnel').bodyKeys.includes('onboarding.addSsh'))
    assert.ok(steps.find((step) => step.id === 'tray').bodyKeys.includes('onboarding.trayQuick'))
    assert.equal(steps.find((step) => step.id === 'eyes').legend.length, 3)
  })

  it('drops the ssh hint and the shortcut line when those flags are off', () => {
    const steps = onboardingSteps({ ssh_command: false, quick_search: false })
    const add = steps.find((step) => step.id === 'add-tunnel')
    const tray = steps.find((step) => step.id === 'tray')
    assert.deepEqual(add.bodyKeys, ['onboarding.addBody'])
    assert.deepEqual(tray.bodyKeys, ['onboarding.trayBody'])
    assert.equal(steps.length, 5)
  })

  it('does not treat unrelated flags as tour steps', () => {
    const steps = onboardingSteps({ profiles: true, notifications: true, automation: true })
    assert.equal(steps.find((step) => step.id === 'add-tunnel').bodyKeys.includes('onboarding.addSsh'), false)
    assert.equal(steps.find((step) => step.id === 'tray').bodyKeys.includes('onboarding.trayQuick'), false)
  })
})

describe('onboarding show, skip and done', () => {
  it('shows only on an empty list before the tour is finished', () => {
    assert.equal(shouldShowOnboarding({ enabled: true, ready: true, tunnelCount: 0, done: false }), true)
    assert.equal(shouldShowOnboarding({ enabled: false, ready: true, tunnelCount: 0, done: false }), false)
    assert.equal(shouldShowOnboarding({ enabled: true, ready: true, tunnelCount: 2, done: false }), false)
    assert.equal(shouldShowOnboarding({ enabled: true, ready: true, tunnelCount: 0, done: true }), false)
    assert.equal(shouldShowOnboarding({ enabled: true, ready: false, tunnelCount: 0, done: false }), false)
  })

  it('replays even when tunnels exist, and never when the flag is off', () => {
    assert.equal(shouldShowOnboarding({ enabled: true, ready: true, tunnelCount: 3, done: true, replay: true }), true)
    assert.equal(shouldShowOnboarding({ enabled: false, ready: true, tunnelCount: 0, done: false, replay: true }), false)
  })

  it('advances, goes back, and finishes on the last step', () => {
    const start = { index: 0, count: 5, open: true, done: false, enabled: true }
    const second = reduceOnboarding(start, 'next')
    assert.equal(second.index, 1)
    assert.equal(second.open, true)
    assert.equal(second.done, false)
    const back = reduceOnboarding(second, 'back')
    assert.equal(back.index, 0)
    assert.equal(reduceOnboarding(start, 'back').index, 0)
    const last = reduceOnboarding({ ...start, index: 4 }, 'next')
    assert.equal(last.open, false)
    assert.equal(last.done, true)
    assert.equal(last.index, 4)
  })

  it('marks the tour done on skip and on the done action', () => {
    const open = { index: 2, count: 5, open: true, done: false, enabled: true }
    for (const action of ['skip', 'done']) {
      const next = reduceOnboarding(open, action)
      assert.equal(next.open, false, action)
      assert.equal(next.done, true, action)
      assert.equal(next.index, 2, action)
    }
  })

  it('restarts from the first step only while the flag is on', () => {
    const again = reduceOnboarding({ index: 3, count: 5, open: false, done: true, enabled: true }, 'restart')
    assert.equal(again.open, true)
    assert.equal(again.done, false)
    assert.equal(again.index, 0)
    assert.equal(again.replay, true)
    const blocked = reduceOnboarding({ index: 0, count: 5, open: false, done: false, enabled: false }, 'restart')
    assert.equal(blocked.open, false)
    assert.equal(blocked.replay, false)
  })
})

describe('onboarding keyboard', () => {
  it('uses event.code so a Russian layout still skips and moves', () => {
    assert.equal(onboardingKeyAction({ code: 'Escape', key: 'Escape' }, { index: 1 }), 'skip')
    assert.equal(onboardingKeyAction({ code: 'ArrowRight', key: 'ArrowRight' }, { index: 0 }), 'next')
    assert.equal(onboardingKeyAction({ code: 'ArrowLeft', key: 'л' }, { index: 2 }), 'back')
    assert.equal(onboardingKeyAction({ code: 'ArrowLeft', key: 'ArrowLeft' }, { index: 0 }), '')
    assert.equal(onboardingKeyAction({ code: 'KeyK', key: 'л' }, { index: 1 }), '')
    assert.equal(onboardingKeyAction({ code: '', key: 'ArrowRight' }, { index: 0 }), 'next')
  })
})

describe('onboarding placement', () => {
  const viewport = { width: 800, height: 600 }

  it('centers without a spotlight when the target is missing or off-screen', () => {
    const card = { width: 200, height: 100 }
    const missing = placeOnboardingCard({ target: null, card, viewport })
    assert.equal(missing.spotlight, false)
    assert.equal(missing.left, 300)
    assert.equal(missing.top, 250)
    assert.equal(visibleTargetRect({ left: -200, top: 10, width: 40, height: 40, hidden: false }, viewport), null)
    assert.equal(visibleTargetRect({ left: 10, top: 10, width: 40, height: 40, hidden: true }, viewport), null)
    assert.equal(spotlightBox({ left: 10, top: 10, width: 0, height: 40 }, viewport), null)
  })

  it('places the card beside a visible target and keeps it inside the window', () => {
    const target = visibleTargetRect({ left: 16, top: 420, width: 100, height: 80 }, viewport)
    const placed = placeOnboardingCard({ target, card: { width: 320, height: 180 }, viewport })
    assert.equal(placed.spotlight, true)
    assert.ok(placed.top >= 12)
    assert.ok(placed.left >= 12)
    assert.ok(placed.top + 180 <= 588)
    assert.ok(placed.left + 320 <= 788)
    const box = spotlightBox({ left: -4, top: 20, width: 80, height: 40 }, viewport)
    assert.equal(box.left, 0)
    assert.ok(box.width > 8)
  })
})
