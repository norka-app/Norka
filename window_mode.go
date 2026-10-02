package main

import (
	"sync"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// Размер окна по умолчанию (как options.App Width/Height в main.go), DIP.
const (
	defaultWindowWidth  = 1024
	defaultWindowHeight = 768
)

// windowRect — внешний прямоугольник окна в физических пикселях экрана.
type windowRect struct {
	X, Y, W, H int
}

// windowModeState хранит геометрию окна при переключении «Расширенный / Простой».
//
// Позицию и размер расширенного окна запоминаем и восстанавливаем целиком на стороне Go:
// вызовы Wails runtime из JS обрабатываются каждый в своей горутине (без порядка),
// а WindowGetPosition и WindowSetPosition на Windows работают в разных системах координат,
// поэтому «чтение-поправка» из JS могла увести окно за верхний край экрана.
type windowModeState struct {
	mu          sync.Mutex
	advanced    windowRect
	hasAdvanced bool
	advancedW   int // размер в DIP — запасной вариант, если нативная геометрия недоступна
	advancedH   int
	simple      windowRect // где оказалось компактное окно после перехода
	hasSimple   bool
}

// SaveAdvancedWindow запоминает геометрию окна перед переходом в простой режим.
func (a *App) SaveAdvancedWindow() {
	if a.ctx == nil {
		return
	}
	a.window.mu.Lock()
	defer a.window.mu.Unlock()
	a.window.advancedW, a.window.advancedH = wailsruntime.WindowGetSize(a.ctx)
	a.window.advanced, a.window.hasAdvanced = nativeWindowBounds()
	a.window.hasSimple = false
}

// ResizeWindowBy меняет внешний размер окна на dw×dh (DIP), сохраняя левый верхний угол,
// и следит, чтобы окно осталось в пределах рабочей области монитора.
func (a *App) ResizeWindowBy(dw, dh int) {
	if a.ctx == nil || (dw == 0 && dh == 0) {
		return
	}
	a.window.mu.Lock()
	defer a.window.mu.Unlock()
	w, h := wailsruntime.WindowGetSize(a.ctx)
	wailsruntime.WindowSetSize(a.ctx, w+dw, h+dh)
	a.keepWindowOnScreenLocked()
	a.window.simple, a.window.hasSimple = nativeWindowBounds()
}

// RestoreAdvancedWindow возвращает окну размер и место расширенного режима.
// Если компактное окно перетаскивали, расширенное открывается от его левого верхнего угла.
// Окно всегда остаётся целиком (с заголовком) в рабочей области монитора; если сохранённое
// место вне экранов — окно центрируется.
func (a *App) RestoreAdvancedWindow() {
	if a.ctx == nil {
		return
	}
	a.window.mu.Lock()
	defer a.window.mu.Unlock()
	defer func() { a.window.hasAdvanced, a.window.hasSimple = false, false }()

	wailsruntime.WindowSetAlwaysOnTop(a.ctx, false)
	if a.window.hasAdvanced {
		target := a.window.advanced
		if cur, ok := nativeWindowBounds(); ok && a.window.hasSimple &&
			(cur.X != a.window.simple.X || cur.Y != a.window.simple.Y) {
			target.X, target.Y = cur.X, cur.Y
		}
		if a.placeWindowLocked(target) {
			return
		}
	}
	w, h := a.window.advancedW, a.window.advancedH
	if w <= 0 || h <= 0 {
		w, h = defaultWindowWidth, defaultWindowHeight
	}
	wailsruntime.WindowSetSize(a.ctx, w, h)
	wailsruntime.WindowCenter(a.ctx)
}

// EnsureWindowOnScreen возвращает окно в видимую область (вызывается при старте фронтенда).
func (a *App) EnsureWindowOnScreen() {
	if a.ctx == nil {
		return
	}
	a.window.mu.Lock()
	defer a.window.mu.Unlock()
	a.keepWindowOnScreenLocked()
}

func (a *App) keepWindowOnScreenLocked() {
	r, ok := nativeWindowBounds()
	if !ok {
		return
	}
	work, onScreen := nativeWorkArea(r)
	if !onScreen {
		wailsruntime.WindowCenter(a.ctx)
		return
	}
	if clampToWorkArea(r, work) == r {
		return
	}
	if !a.placeWindowLocked(r) {
		wailsruntime.WindowCenter(a.ctx)
	}
}

// placeWindowLocked ставит окно в рабочую область. Если Windows тут же сдвигает
// его на ширину невидимой рамки, повторяем постановку с обратным сдвигом.
func (a *App) placeWindowLocked(target windowRect) bool {
	work, onScreen := nativeWorkArea(target)
	if !onScreen {
		return false
	}
	requested := clampToWorkArea(target, work)
	if !nativeSetWindowBounds(requested) {
		return false
	}
	afterFirst, ok := nativeWindowBounds()
	if !ok || clampToWorkArea(afterFirst, work) == afterFirst {
		return true
	}
	compensated := compensateSystemShift(requested, afterFirst)
	if !nativeSetWindowBounds(compensated) {
		return true
	}
	afterSecond, ok := nativeWindowBounds()
	if !ok || !frameShiftStuck(requested, afterFirst, compensated, afterSecond) {
		nativeSetWindowBounds(requested)
	}
	return true
}

// clampToWorkArea сдвигает нарисованный прямоугольник окна внутрь рабочей области.
// Если окно больше области, прижимаем его к левому/верхнему краю, чтобы заголовок
// оставался доступным.
//
// У безрамочного окна Wails клиентская область совпадает с GetWindowRect
// (WM_NCCALCSIZE). Невидимая рамка DWM — это не пустое поле, а пиксели интерфейса.
// На 96 DPI она 8 px (SM_CXFRAME+SM_CXPADDEDBORDER), на 120 DPI — 9, на 144 DPI — 11.
// После перетаскивания к краю экрана Windows оставляет эту рамку за пределами
// рабочей области, и верх окна уезжает вверх на ширину рамки.
func clampToWorkArea(r, work windowRect) windowRect {
	r.X = clampAxis(r.X, r.W, work.X, work.W)
	r.Y = clampAxis(r.Y, r.H, work.Y, work.H)
	return r
}

// compensateSystemShift — вторая попытка, если после SetWindowPos Windows
// сдвинул окно на постоянную величину (ширину невидимой рамки).
// Просили requested, получили actual: запрашиваем точку, из которой тот же
// сдвиг попадёт в requested.
func compensateSystemShift(requested, actual windowRect) windowRect {
	return windowRect{
		X: requested.X - (actual.X - requested.X),
		Y: requested.Y - (actual.Y - requested.Y),
		W: requested.W,
		H: requested.H,
	}
}

// frameShiftStuck — true, если второй SetWindowPos получил тот же сдвиг, что и первый.
// Тогда компенсация попала в рабочую область. Если второй вызов сдвиг не повторил,
// компенсацию нужно отменить: иначе окно окажется ниже края на ширину рамки.
func frameShiftStuck(requested, afterFirst, compensated, afterSecond windowRect) bool {
	dx1 := afterFirst.X - requested.X
	dy1 := afterFirst.Y - requested.Y
	if dx1 == 0 && dy1 == 0 {
		return false
	}
	return afterSecond.X-compensated.X == dx1 && afterSecond.Y-compensated.Y == dy1
}

func clampAxis(pos, size, start, length int) int {
	if size >= length || pos < start {
		return start
	}
	if end := start + length; pos+size > end {
		return end - size
	}
	return pos
}
