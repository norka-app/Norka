import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import { FIRST_LAUNCH_TOUR_FLAG, TOUR_STEPS, isLastTourStep, tourStep } from './tour-steps.js'

describe('first-launch tour steps', () => {
  it('walks welcome, adding a tunnel, window mode, then features', () => {
    assert.deepEqual(TOUR_STEPS.map((step) => step.id), ['welcome', 'add', 'mode', 'features'])
    assert.equal(TOUR_STEPS[1].page, 'tunnels')
    assert.equal(TOUR_STEPS[1].target, '[data-tour="add-tunnel"]')
    assert.equal(TOUR_STEPS[2].page, 'config')
    assert.equal(TOUR_STEPS[2].target, '[data-tour="window-mode"]')
    assert.equal(TOUR_STEPS[3].target, '[data-tour="features"]')
    assert.equal(TOUR_STEPS[0].target, '')
  })

  it('ends on the features step so Done can close the tour', () => {
    assert.equal(isLastTourStep(0), false)
    assert.equal(isLastTourStep(TOUR_STEPS.length - 1), true)
    assert.equal(tourStep(99).id, 'welcome')
  })

  it('uses the feature-flag id', () => {
    assert.equal(FIRST_LAUNCH_TOUR_FLAG, 'first_launch_tour')
  })
})
