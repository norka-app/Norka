// Нечёткий поиск для палитры: точное вхождение выше подпоследовательности.
export function fuzzyScore(query, text) {
  const needle = String(query || '').trim().toLocaleLowerCase()
  const haystack = String(text || '').toLocaleLowerCase()
  if (!needle) return 1
  const at = haystack.indexOf(needle)
  if (at === 0) return 200
  if (at > 0) return 140 - Math.min(at, 40)

  let matched = 0
  let score = 0
  let streak = 0
  for (let i = 0; i < haystack.length && matched < needle.length; i += 1) {
    if (haystack[i] !== needle[matched]) {
      streak = 0
      continue
    }
    matched += 1
    streak += 1
    score += 4 + streak * 2
    const prev = haystack[i - 1]
    if (i === 0 || prev === ' ' || prev === ':' || prev === '.' || prev === '-') score += 6
  }
  return matched === needle.length ? score : 0
}

// Лучший балл по отдельным полям, чтобы «sta» не склеивалось из кусков разных слов.
export function bestFuzzyScore(query, parts) {
  const scores = (Array.isArray(parts) ? parts : [parts]).map((part) => fuzzyScore(query, part))
  return scores.length ? Math.max(...scores) : 0
}
