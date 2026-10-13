import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { describe, it } from 'node:test'
import { fileURLToPath } from 'node:url'
import { notesForLocale, RELEASE_NOTES_EN_MARKER } from './release-notes.js'

const here = dirname(fileURLToPath(import.meta.url))
const notes161 = readFileSync(join(here, '../../../docs/releases/v1.6.1.md'), 'utf8')

describe('notesForLocale', () => {
  it('keeps a single-language body', () => {
    assert.equal(notesForLocale('## Исправлено\n\n- тише', 'en'), '## Исправлено\n\n- тише')
    assert.equal(notesForLocale('  ', 'ru'), '')
  })

  it('picks the interface language and drops the language heading', () => {
    const body = `## Русский\n\nПорт занят.\n\n${RELEASE_NOTES_EN_MARKER}\n\n## English\n\nPort is taken.`
    assert.equal(notesForLocale(body, 'ru'), 'Порт занят.')
    assert.equal(notesForLocale(body, 'en'), 'Port is taken.')
    assert.equal(notesForLocale(body, 'auto'), 'Порт занят.')
  })

  it('reads both languages from the 1.6.1 notes', () => {
    const ru = notesForLocale(notes161, 'ru')
    const en = notesForLocale(notes161, 'en')
    assert.equal(ru.includes('13000'), true)
    assert.equal(ru.includes('Docker'), true)
    assert.equal(ru.includes('## English'), false)
    assert.equal(ru.includes(RELEASE_NOTES_EN_MARKER), false)
    assert.equal(en.includes('13000'), true)
    assert.equal(en.includes('Technical detail'), true)
    assert.equal(en.includes('## Русский'), false)
    assert.equal(en.startsWith('## English'), false)
    assert.equal(ru.startsWith('## Русский'), false)
    assert.equal(en.includes('Подключить вместо'), false)
  })
})
