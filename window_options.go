package main

import (
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

// windowBackground matches the historical colour when the frame is unchanged.
// Mica and the macOS hidden title bar need a fully transparent webview or the
// backdrop never shows through.
func windowBackground(plan ChromePlan) *options.RGBA {
	// Windows всегда с прозрачным WebView: углы капсулы сглаживает CSS, а не маска окна.
	// Полный и минималистичный вид закрашивают себя сами.
	if plan.WindowsTranslucent || plan.MacInset || plan.Platform == "windows" {
		return &options.RGBA{R: 0, G: 0, B: 0, A: 0}
	}
	return &options.RGBA{R: 27, G: 38, B: 54, A: 1}
}

func windowsWindowOptions(plan ChromePlan) *windows.Options {
	opts := &windows.Options{
		WebviewUserDataPath:  webviewUserDataPath(),
		WebviewGpuIsDisabled: plan.GPUDisabled,
	}
	opts.WebviewIsTransparent = true
	opts.WindowIsTranslucent = true
	if plan.WindowsTranslucent {
		opts.BackdropType = windows.Mica
		opts.WebviewGpuIsDisabled = false
		return opts
	}
	// Без Mica фон не системный: прозрачны только пиксели, которые не закрасил интерфейс.
	// Программный кадр WebView2 непрозрачный и закрыл бы сглаженные углы капсулы.
	opts.BackdropType = windows.None
	opts.WebviewGpuIsDisabled = false
	return opts
}

func macWindowOptions(plan ChromePlan) *mac.Options {
	if !plan.MacInset {
		return nil
	}
	return &mac.Options{
		TitleBar:             mac.TitleBarHiddenInset(),
		WebviewIsTransparent: true,
		WindowIsTranslucent:  true,
	}
}
