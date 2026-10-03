// Шаги тура первого запуска. page — куда перейти перед подсветкой,
// target — элемент интерфейса. Пустой target оставляет карточку по центру.
export const FIRST_LAUNCH_TOUR_FLAG = 'first_launch_tour'

export const TOUR_STEPS = [
  { id: 'welcome', page: '', target: '' },
  { id: 'add', page: 'tunnels', target: '[data-tour="add-tunnel"]' },
  { id: 'mode', page: 'config', target: '[data-tour="window-mode"]' },
  { id: 'features', page: 'config', target: '[data-tour="features"]' },
]

export function tourStep(index) {
  const step = TOUR_STEPS[index]
  return step || TOUR_STEPS[0]
}

export function isLastTourStep(index) {
  return index >= TOUR_STEPS.length - 1
}
