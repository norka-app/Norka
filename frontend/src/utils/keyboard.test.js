import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import { hotkeyChord, isQuickSearchChord, matchesKey, physicalKey } from './keyboard.js'

describe('physicalKey', () => {
  it('prefers KeyK over the Russian letter', () => {
    assert.equal(physicalKey({ code: 'KeyK', key: 'л' }), 'k')
  })

  it('maps a Russian letter when code is empty', () => {
    assert.equal(physicalKey({ code: '', key: 'л' }), 'k')
    assert.equal(physicalKey({ code: '', key: 'Ь' }), 'm')
    assert.equal(physicalKey({ key: 'K' }), 'k')
  })
})

describe('isQuickSearchChord', () => {
  it('matches Ctrl/Cmd+K on a Russian layout', () => {
    assert.equal(isQuickSearchChord({ code: 'KeyK', key: 'л', ctrlKey: true }), true)
    assert.equal(isQuickSearchChord({ code: '', key: 'л', ctrlKey: true }), true)
    assert.equal(isQuickSearchChord({ code: 'KeyK', key: 'k', metaKey: true }), true)
  })

  it('ignores alt, shift, and other letters', () => {
    assert.equal(isQuickSearchChord({ code: 'KeyK', key: 'k', ctrlKey: true, altKey: true }), false)
    assert.equal(isQuickSearchChord({ code: 'KeyK', key: 'k', ctrlKey: true, shiftKey: true }), false)
    assert.equal(isQuickSearchChord({ code: 'KeyM', key: 'ь', ctrlKey: true }), false)
    assert.equal(isQuickSearchChord({ code: 'KeyK', key: 'k' }), false)
  })
})

describe('hotkeyChord', () => {
  it('saves a Russian Ctrl+Alt+K as ctrl+alt+k', () => {
    assert.equal(hotkeyChord({ code: 'KeyK', key: 'л', ctrlKey: true, altKey: true }), 'ctrl+alt+k')
    assert.equal(hotkeyChord({ code: '', key: 'Л', ctrlKey: true, altKey: true }), 'ctrl+alt+k')
  })

  it('uses option on macOS and keeps other chords', () => {
    assert.equal(hotkeyChord({ code: 'Space', key: ' ', altKey: true }, { altName: 'option' }), 'option+space')
    assert.equal(hotkeyChord({ code: 'KeyM', key: 'ь', ctrlKey: true, shiftKey: true }), 'ctrl+shift+m')
    assert.equal(hotkeyChord({ code: 'F5', key: 'F5', metaKey: true }), 'meta+f5')
  })

  it('does not record a bare letter or a modifier', () => {
    assert.equal(hotkeyChord({ code: 'KeyK', key: 'л' }), '')
    assert.equal(hotkeyChord({ code: 'ControlLeft', key: 'Control', ctrlKey: true }), '')
    assert.equal(hotkeyChord({ code: 'Escape', key: 'Escape' }), '')
  })
})

describe('matchesKey', () => {
  it('matches palette and mode keys from code or key', () => {
    assert.equal(matchesKey({ code: 'Escape', key: 'Escape' }, 'escape'), true)
    assert.equal(matchesKey({ code: '', key: 'ArrowDown' }, 'arrowdown'), true)
    assert.equal(matchesKey({ code: 'Enter', key: 'Enter' }, 'enter'), true)
    assert.equal(matchesKey({ code: 'ArrowUp', key: 'ArrowUp' }, 'arrowup'), true)
    assert.equal(matchesKey({ code: 'KeyM', key: 'ь' }, 'm'), true)
    assert.equal(matchesKey({ code: '', key: 'ь' }, 'm'), true)
    assert.equal(matchesKey({ code: 'ShiftLeft', key: 'Shift' }, 'shift'), true)
  })
})
