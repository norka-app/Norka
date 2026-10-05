import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { describe, it } from 'node:test'
import { fileURLToPath } from 'node:url'
import {
  foreignLocalBindHint,
  isForeignLocalBindError,
  parseExternalBindError,
  rawTunnelError,
  suggestAlternateLocalPort,
} from './local-bind-error.js'

const here = dirname(fileURLToPath(import.meta.url))
const ru = JSON.parse(readFileSync(join(here, '../locales/ru.json'), 'utf8'))
const en = JSON.parse(readFileSync(join(here, '../locales/en.json'), 'utf8'))

const windowsBind = 'listen 127.0.0.1:3000 failed: listen tcp 127.0.0.1:3000: bind: Only one usage of each socket address (protocol/network address/port) is normally permitted.'
const linuxBind = 'listen 127.0.0.1:3000 failed: listen tcp 127.0.0.1:3000: bind: address already in use'
const marked = `norka:local-port-external port=3000 suggested=13000 remote=8080\n${windowsBind}`

function tunnel(extra) {
  return {
    id: 1,
    mode: 'local',
    status: 'error',
    localHost: '127.0.0.1',
    localPort: 3000,
    remotePort: 8080,
    lastError: windowsBind,
    ...extra,
  }
}

describe('suggestAlternateLocalPort', () => {
  it('adds 10000 when the result still fits', () => {
    assert.equal(suggestAlternateLocalPort(3000), 13000)
    assert.equal(suggestAlternateLocalPort(13000), 23000)
  })

  it('uses 13000 as an example when port+10000 does not fit', () => {
    assert.equal(suggestAlternateLocalPort(60000), 13000)
  })
})

describe('isForeignLocalBindError', () => {
  it('matches Windows, Linux, and a Russian Windows message', () => {
    assert.equal(isForeignLocalBindError(windowsBind), true)
    assert.equal(isForeignLocalBindError(linuxBind), true)
    assert.equal(isForeignLocalBindError('listen tcp 127.0.0.1:3000: bind: Обычно разрешается только одно использование адреса сокета (протокол/сетевой адрес/порт).'), true)
  })

  it('ignores remote listen and unrelated failures', () => {
    assert.equal(isForeignLocalBindError('remote listen 0.0.0.0:80 failed: bind: address already in use'), false)
    assert.equal(isForeignLocalBindError('ssh: handshake failed'), false)
    assert.equal(isForeignLocalBindError(''), false)
  })
})

describe('foreignLocalBindHint', () => {
  it('explains a Docker-style bind and keeps the OS text', () => {
    const hint = foreignLocalBindHint(tunnel(), [])
    assert.deepEqual(hint, {
      port: 3000,
      suggested: 13000,
      remote: 8080,
      raw: windowsBind,
    })
  })

  it('uses the engine marker, including a probed suggestion', () => {
    const hint = foreignLocalBindHint(tunnel({ lastError: marked }), [])
    assert.equal(hint.suggested, 13000)
    assert.equal(hint.raw, windowsBind)
    assert.equal(rawTunnelError(marked), windowsBind)
  })

  it('does not replace a Norka-vs-Norka conflict', () => {
    const holder = {
      id: 2,
      name: 'api',
      status: 'running',
      mode: 'local',
      localHost: '127.0.0.1',
      localPort: 3000,
    }
    assert.equal(foreignLocalBindHint(tunnel(), [holder]), null)
    assert.equal(foreignLocalBindHint(tunnel({ lastError: marked }), [holder]), null)
  })

  it('leaves remote forwards and other errors alone', () => {
    assert.equal(foreignLocalBindHint(tunnel({ mode: 'remote' }), []), null)
    assert.equal(foreignLocalBindHint(tunnel({ mode: 'dynamic' }), []), null)
    assert.equal(foreignLocalBindHint(tunnel({ lastError: 'ssh: handshake failed' }), []), null)
    assert.equal(foreignLocalBindHint(tunnel({ status: 'stopped' }), []), null)
  })
})

describe('locale copy', () => {
  it('matches the RU and EN hint and shares placeholders', () => {
    const ruText = ru.app.tunnels.externalPortBusy
    const enText = en.app.tunnels.externalPortBusy
    assert.equal(
      ruText,
      'Локальный порт {port} уже занят другой программой (часто Docker или другой сервис), а не туннелем Norka. Смени локальный порт в туннеле, например на {suggested}, и открывай http://localhost:{suggested} — удалённый порт можно оставить {remote}.',
    )
    assert.match(enText, /Local port \{port\} is already used by another program/)
    assert.match(enText, /for example to \{suggested\}/)
    assert.match(enText, /http:\/\/localhost:\{suggested\}/)
    assert.match(enText, /remote port can stay \{remote\}/)
    assert.equal(ru.app.tunnels.externalPortBusyDetail.length > 0, true)
    assert.equal(en.app.tunnels.externalPortBusyDetail.length > 0, true)
    for (const token of ['{port}', '{suggested}', '{remote}']) {
      assert.equal(ruText.includes(token), true)
      assert.equal(enText.includes(token), true)
    }
  })
})

describe('parseExternalBindError', () => {
  it('rejects a broken marker', () => {
    assert.equal(parseExternalBindError('norka:local-port-external port=0 suggested=1 remote=1\nx'), null)
  })
})
