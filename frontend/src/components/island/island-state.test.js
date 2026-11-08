import test from 'node:test'
import assert from 'node:assert/strict'
import {
  islandAddress,
  islandDotCount,
  islandEyes,
  islandPhase,
  tunnelCountForm,
} from './island-state.js'
import { ISLAND_SIZE, islandDuration, lerpIsland } from './island-metrics.js'

const tunnel = (id, status, port = 5432) => ({
  id,
  name: `t${id}`,
  status,
  localHost: '127.0.0.1',
  localPort: port,
})

test('idle when nothing is running', () => {
  assert.equal(islandPhase({ tunnels: [tunnel(1, 'stopped')] }), 'idle')
  assert.equal(islandEyes('idle'), 'stopped')
})

test('one running tunnel uses the wide capsule', () => {
  assert.equal(islandPhase({ tunnels: [tunnel(1, 'running'), tunnel(2, 'stopped')] }), 'one')
  assert.deepEqual(ISLAND_SIZE.one, { w: 360, h: 36, r: 18 })
})

test('several running tunnels show only the count capsule', () => {
  const phase = islandPhase({
    tunnels: [tunnel(1, 'running'), tunnel(2, 'running', 6379), tunnel(3, 'stopped')],
  })
  assert.equal(phase, 'multi')
  assert.deepEqual(ISLAND_SIZE.multi, { w: 280, h: 36, r: 18 })
  assert.equal(islandDotCount(5), 5)
  assert.equal(islandDotCount(12), 8)
})

test('an error wins over live tunnels', () => {
  const phase = islandPhase({
    tunnels: [tunnel(1, 'running'), tunnel(2, 'error', 3000)],
  })
  assert.equal(phase, 'error')
  assert.equal(islandEyes('error'), 'error')
})

test('connecting and the open list take priority', () => {
  const tunnels = [tunnel(1, 'running'), tunnel(2, 'busy')]
  assert.equal(islandPhase({ tunnels }), 'connecting')
  assert.equal(islandPhase({ tunnels, expanded: true }), 'expanded')
  assert.equal(islandPhase({ tunnels, expanded: true, conflict: { id: 3 } }), 'conflict')
  assert.equal(islandEyes('conflict'), 'connecting')
  assert.equal(islandEyes('expanded', 'connected'), 'connected')
})

test('localhost is folded and russian plurals match', () => {
  assert.equal(islandAddress(tunnel(1, 'running')), 'localhost:5432')
  assert.equal(tunnelCountForm(1, 'ru'), 'one')
  assert.equal(tunnelCountForm(2, 'ru'), 'few')
  assert.equal(tunnelCountForm(5, 'ru'), 'many')
  assert.equal(tunnelCountForm(11, 'ru'), 'many')
  assert.equal(tunnelCountForm(1, 'en'), 'one')
  assert.equal(tunnelCountForm(2, 'en'), 'many')
})

test('approved morph timings', () => {
  assert.equal(islandDuration('idle', 'connecting'), 300)
  assert.equal(islandDuration('connecting', 'one'), 200)
  assert.equal(islandDuration('one', 'multi'), 250)
  assert.equal(islandDuration('one', 'expanded'), 350)
  assert.equal(islandDuration('expanded', 'one'), 280)
  assert.equal(islandDuration('one', 'error'), 200)
  assert.equal(islandDuration('one', 'conflict'), 250)
  assert.equal(ISLAND_SIZE.idle.w, 96)
  assert.equal(ISLAND_SIZE.expanded.h, 280)
  assert.equal(ISLAND_SIZE.expanded.r, 32)
  assert.equal(ISLAND_SIZE.conflict.w, 420)
  assert.equal(ISLAND_SIZE.conflict.h, 52)
  const mid = lerpIsland(ISLAND_SIZE.idle, ISLAND_SIZE.one, 1)
  assert.equal(mid.w, 360)
  assert.equal(mid.h, 36)
})
