import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import { formatBytes, formatDuration, liveConnectedSeconds, sessionUptimeSeconds } from './tunnel-stats.js'

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
  it('stays compact', () => {
    assert.equal(formatBytes(0), '0 B')
    assert.equal(formatBytes(512), '512 B')
    assert.equal(formatBytes(1536), '1.5 KB')
    assert.equal(formatBytes(1024 * 1024), '1 MB')
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
