// Пасхалка «нокаут»: 7 кликов по крупной Норке за 2 с — случайная анимация (2–3 с), затем
// плавный возврат к обычным глазам и 5 с паузы, в которую клики не считаются.
// Сами анимации — SVG-слой и CSS в NorkaIcon.vue (классы norka--ko-<вариант>).

export const KNOCKOUT_CLICKS = 3
export const KNOCKOUT_WINDOW_MS = 2000
export const KNOCKOUT_COOLDOWN_MS = 1000
// последние столько мс слой нокаута гаснет, а обычные глаза возвращаются
export const KNOCKOUT_FADE_MS = 400

// вариант → длительность, мс
export const KNOCKOUT_VARIANTS = {
  stars: 2600, // звёздочки кружат над аркой, глаза зажмурены
  birds: 2800, // птички кружат над аркой, глаза настоящие: зрачки плывут и косят, веки опущены
  xEyes: 2200, // удар: тряска и глаза-крестики
  spiral: 2600, // глаза-спирали крутятся в разные стороны
  wobble: 2400 // Норку шатает, вокруг вспыхивают искорки, глаза «> <»
}

// prefers-reduced-motion: без движения — просто глаза-крестики на 1,5 с
export const KNOCKOUT_REDUCED = { variant: 'xEyes', duration: 1500 }

// Случайный вариант, не совпадающий с предыдущим.
export function pickKnockout(prev, random = Math.random) {
  const names = Object.keys(KNOCKOUT_VARIANTS).filter((name) => name !== prev)
  return names[Math.min(names.length - 1, Math.floor(random() * names.length))]
}
