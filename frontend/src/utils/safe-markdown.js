const ESCAPE = {
  '&': '&amp;',
  '<': '&lt;',
  '>': '&gt;',
  '"': '&quot;',
  "'": '&#39;',
}

function escapeHtml(value) {
  return String(value ?? '').replace(/[&<>"']/g, (ch) => ESCAPE[ch])
}

function safeHref(raw) {
  const url = String(raw ?? '').trim()
  if (!/^https?:\/\//i.test(url)) return ''
  if (/[\u0000-\u0020<>"']/.test(url)) return ''
  return url
}

function link(href, label) {
  const safe = safeHref(href)
  const text = escapeHtml(label)
  if (!safe) return text
  return `<a href="${escapeHtml(safe)}" target="_blank" rel="noopener noreferrer">${text}</a>`
}

function renderInline(raw) {
  const source = String(raw ?? '')
  const pattern = /(`+)([^`]+)\1|\*\*([^*]+)\*\*|\[([^\]]+)\]\(([^)\s]+)\)|(https?:\/\/[^\s<]+)/g
  let html = ''
  let last = 0
  for (const match of source.matchAll(pattern)) {
    html += escapeHtml(source.slice(last, match.index))
    if (match[2] != null) {
      html += `<code>${escapeHtml(match[2])}</code>`
    } else if (match[3] != null) {
      html += `<strong>${escapeHtml(match[3])}</strong>`
    } else if (match[4] != null) {
      html += link(match[5], match[4])
    } else if (match[6] != null) {
      const trimmed = match[6].replace(/[),.;:!?]+$/, '')
      const trailing = match[6].slice(trimmed.length)
      html += `${link(trimmed, trimmed)}${escapeHtml(trailing)}`
    }
    last = match.index + match[0].length
  }
  html += escapeHtml(source.slice(last))
  return html
}

function renderBlocks(chunk) {
  const lines = String(chunk ?? '').split('\n')
  const blocks = []
  let paragraph = []
  let list = null

  function flushParagraph() {
    if (paragraph.length === 0) return
    const body = paragraph.join('\n').trim()
    paragraph = []
    if (!body) return
    blocks.push(`<p>${renderInline(body).replace(/\n/g, '<br>')}</p>`)
  }

  function flushList() {
    if (!list) return
    blocks.push(`<${list.kind}>${list.items.join('')}</${list.kind}>`)
    list = null
  }

  for (const line of lines) {
    const heading = /^(#{1,3})\s+(.*)$/.exec(line)
    const unordered = /^[-*]\s+(.*)$/.exec(line)
    const ordered = /^\d+\.\s+(.*)$/.exec(line)
    if (heading) {
      flushParagraph()
      flushList()
      const level = heading[1].length
      blocks.push(`<h${level}>${renderInline(heading[2])}</h${level}>`)
      continue
    }
    if (unordered || ordered) {
      flushParagraph()
      const kind = unordered ? 'ul' : 'ol'
      if (!list || list.kind !== kind) {
        flushList()
        list = { kind, items: [] }
      }
      list.items.push(`<li>${renderInline((unordered || ordered)[1])}</li>`)
      continue
    }
    if (line.trim() === '') {
      flushParagraph()
      flushList()
      continue
    }
    flushList()
    paragraph.push(line)
  }
  flushParagraph()
  flushList()
  return blocks.join('')
}

export function renderSafeMarkdown(source) {
  const text = String(source ?? '').replace(/\r\n/g, '\n')
  const chunks = text.split('```')
  return chunks
    .map((chunk, index) => {
      if (index % 2 === 1) {
        const breakAt = chunk.indexOf('\n')
        const body = breakAt === -1 ? chunk : chunk.slice(breakAt + 1)
        return `<pre><code>${escapeHtml(body.replace(/\n$/, ''))}</code></pre>`
      }
      return renderBlocks(chunk)
    })
    .join('')
}
