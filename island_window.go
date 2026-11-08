package main

import (
	"math"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// IslandHost — окно, растянутое на рабочую область, внутри которого живут
// капсула и полное окно. Width и Height в DIP.
type IslandHost struct {
	OK     bool `json:"ok"`
	Shape  bool `json:"shape"`
	Width  int  `json:"width"`
	Height int  `json:"height"`
}

// LayoutIslandStage — раскладка капсулы и полного окна для фронтенда. Координаты в DIP.
func (a *App) LayoutIslandStage(hostW, hostH, islandW, islandH, islandR, panelX, panelY, panelW, panelH int) IslandStageLayout {
	return layoutIslandStage(hostW, hostH, islandW, islandH, islandR, panelX, panelY, panelW, panelH)
}

// IslandShapeSupported оставлен для совместимости вызовов. Маска окна больше не используется:
// лесенка от SetWindowRgn хуже сглаженного CSS.
func (a *App) IslandShapeSupported() bool {
	return false
}

// PlaceIslandCapsule ставит одно окно-капсулу по центру сверху, поверх остальных.
// width, height и radius — DIP.
func (a *App) PlaceIslandCapsule(width, height, radius int) {
	if a == nil || a.ctx == nil || width < 1 || height < 1 {
		return
	}
	a.window.mu.Lock()
	defer a.window.mu.Unlock()
	wailsruntime.WindowSetAlwaysOnTop(a.ctx, true)
	wailsruntime.WindowSetSize(a.ctx, width, height)
	if a.placeCapsuleNative(width, height, radius) {
		return
	}
	a.placeCapsuleFallback(width, height)
}

// SetAdvancedChrome включает изменение размера и кнопку «Развернуть» только у полного окна.
// Капсула и минималистичный вид остаются фиксированными.
func (a *App) SetAdvancedChrome(enabled bool) {
	if a == nil || a.ctx == nil {
		return
	}
	a.window.mu.Lock()
	defer a.window.mu.Unlock()
	setAdvancedChrome(enabled)
	flag := "false"
	if enabled {
		flag = "true"
	}
	wailsruntime.WindowExecJS(a.ctx, "if (window.wails && window.wails.flags) window.wails.flags.enableResize = "+flag+";")
}

// SetIslandBackdrop красит фон окна в цвет капсулы, чтобы углы без вырезания
// не вспыхивали чужим цветом. clear=true возвращает фон, выбранный при старте.
func (a *App) SetIslandBackdrop(dark bool, clear bool) {
	if a == nil || a.ctx == nil {
		return
	}
	if clear {
		bg := windowBackground(startupChrome)
		if bg == nil {
			return
		}
		wailsruntime.WindowSetBackgroundColour(a.ctx, bg.R, bg.G, bg.B, bg.A)
		return
	}
	if dark {
		wailsruntime.WindowSetBackgroundColour(a.ctx, 22, 24, 31, 255)
		return
	}
	wailsruntime.WindowSetBackgroundColour(a.ctx, 255, 255, 255, 255)
}

// ClearIslandChrome снимает вырезание и режим «поверх всех».
func (a *App) ClearIslandChrome() {
	if a == nil || a.ctx == nil {
		return
	}
	a.window.mu.Lock()
	defer a.window.mu.Unlock()
	clearIslandRegion()
	a.islandScale = 0
	wailsruntime.WindowSetAlwaysOnTop(a.ctx, false)
	bg := windowBackground(startupChrome)
	if bg != nil {
		wailsruntime.WindowSetBackgroundColour(a.ctx, bg.R, bg.G, bg.B, bg.A)
	}
}

// SetWindowPresentation запоминает вид для галочки в трее.
// compactRestore: закрытие полного окна возвращает компактный вид, а не прячет в трей.
func (a *App) SetWindowPresentation(mode string, compactRestore bool) {
	switch mode {
	case "simple", "island", "advanced":
	default:
		mode = "advanced"
	}
	if a == nil {
		return
	}
	a.trayView.Store(mode)
	a.compactRestore.Store(compactRestore)
	a.invalidateTrayMenu()
}

func (a *App) currentTrayView() string {
	if a == nil {
		return "advanced"
	}
	mode, _ := a.trayView.Load().(string)
	if mode == "" {
		return "advanced"
	}
	return mode
}

func (a *App) placeCapsuleFallback(width, height int) {
	wailsruntime.WindowCenter(a.ctx)
	x, y := wailsruntime.WindowGetPosition(a.ctx)
	screenH := 0
	if screens, err := wailsruntime.ScreenGetAll(a.ctx); err == nil {
		for _, screen := range screens {
			if screen.IsCurrent {
				screenH = screen.Size.Height
				break
			}
			if screenH == 0 && screen.IsPrimary {
				screenH = screen.Size.Height
			}
		}
	}
	if screenH <= 0 {
		wailsruntime.WindowSetPosition(a.ctx, x, islandTopGapDIP)
		return
	}
	top := y - (screenH-height)/2
	if top < 0 {
		top = 0
	}
	wailsruntime.WindowSetPosition(a.ctx, x, top+islandTopGapDIP)
}

// dipRect живёт здесь, чтобы остров и прежняя геометрия окна делили один тип.
// window_mode.go объявляет windowRect в физических пикселях; dipRect — логические.
type dipRect struct {
	X, Y, W, H int
}

func physicalToDIP(px int, scale float64) int {
	if scale <= 0 {
		scale = 1
	}
	return int(math.Round(float64(px) / scale))
}
