import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { describe, it } from 'node:test'
import { fileURLToPath } from 'node:url'
import { formatBytes, formatDuration, liveConnectedSeconds, sessionUptimeSeconds } from './tunnel-stats.js'

const here = dirname(fileURLToPath(import.meta.url))
const ru = JSON.parse(readFileSync(join(here, '../locales/ru.json'), 'utf8'))
const en = JSON.parse(readFileSync(join(here, '../locales/en.json'), 'utf8'))

function translator(messages) {
  return (key) => key.split('.').reduce((node, part) => node?.[part], messages)
}

function fakeT(key, params) {
  return `${key}:${JSON.stringify(params)}`
}

describe('formatDuration', () => {
  it('uses hours and minutes for a long session', () => {
    const twoHours = 2 * 3600 + 14 * 60
    assert.equal(formatDuration(twoHours, fakeT), 'app.tunnels.stats.hourMin:{"h":2,"m":14}')
  })

  it('keeps seconds under a minute and under ten minutes', () => {
    assert.equal(formatDuration(45, fakeT), 'app.tunnels.stats.sec:{"n":45}')
    assert.equal(formatDuration(9 * 60 + 5, fakeT), 'app.tunnels.stats.minSec:{"m":9,"s":5}')
    assert.equal(formatDuration(14 * 60, fakeT), 'app.tunnels.stats.min:{"m":14}')
  })

  it('rolls days without leftover minutes', () => {
    assert.equal(formatDuration(3 * 86400 + 2 * 3600, fakeT), 'app.tunnels.stats.dayHour:{"d":3,"h":2}')
    assert.equal(formatDuration(2 * 86400, fakeT), 'app.tunnels.stats.day:{"d":2}')
  })
})

describe('formatBytes', () => {
  const tEn = translator(en)
  const tRu = translator(ru)

  it('uses English units and a dot', () => {
    assert.equal(formatBytes(0, tEn, 'en'), '0 B')
    assert.equal(formatBytes(512, tEn, 'en'), '512 B')
    assert.equal(formatBytes(1536, tEn, 'en'), '1.5 KB')
    assert.equal(formatBytes(1024 * 1024, tEn, 'en'), '1 MB')
    assert.equal(formatBytes(Math.round(4.2 * 1024 ** 3), tEn, 'en'), '4.2 GB')
  })

  it('uses Russian units and a comma', () => {
    assert.equal(formatBytes(512, tRu, 'ru'), '512 Б')
    assert.equal(formatBytes(860 * 1024, tRu, 'ru'), '860 КБ')
    assert.equal(formatBytes(Math.round(1.2 * 1024 * 1024), tRu, 'ru'), '1,2 МБ')
    assert.equal(formatBytes(Math.round(4.2 * 1024 ** 3), tRu, 'ru'), '4,2 ГБ')
    assert.equal(formatBytes(1024 ** 4, tRu, 'ru'), '1 ТБ')
  })
})

describe('locale copy', () => {
  it('spells Russian days as дн and keeps English days', () => {
    assert.equal(ru.app.tunnels.stats.day, '{d} дн')
    assert.equal(ru.app.tunnels.stats.dayHour, '{d} дн {h} ч')
    assert.equal(en.app.tunnels.stats.day, '{d} d')
    assert.equal(en.app.tunnels.stats.dayHour, '{d} d {h} h')
  })
})

describe('live clocks', () => {
  it('ticks uptime from the session start and freezes when disconnected', () => {
    const stat = { connected: true, sessionStartedUnix: 1_000, asOfUnix: 5_000 }
    assert.equal(sessionUptimeSeconds(stat, 65_000), 64)
    assert.equal(liveConnectedSeconds(10, stat, 8_000), 13)
    assert.equal(sessionUptimeSeconds({ ...stat, connected: false }, 65_000), 0)
    assert.equal(liveConnectedSeconds(10, { ...stat, connected: false }, 80_000), 10)
  })
})
