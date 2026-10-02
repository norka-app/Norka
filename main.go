package main

import (
	"embed"
	"os"
	"path/filepath"
	"runtime"

	"github.com/energye/systray"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailslinux "github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
	"norka/internal/notify"
	"norka/internal/traytext"
	"norka/internal/uilocale"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/windows/icon.ico
var trayIconWindows []byte

//go:embed build/macos-systray.png
var trayIconMacOS []byte

//go:embed build/appicon.png
var trayIconFallback []byte

func main() {
	// Create an instance of the app structure
	app := NewApp()
	configDir := "."
	if app.storage != nil {
		configDir = filepath.Dir(app.storage.Path())
	}
	localeTag := uilocale.Resolve(configDir)
	trayLabels := traytext.ForLocale(localeTag)

	showMainWindow := func() {
		if app != nil {
			app.showMainWindow()
		}
	}

	startTray, endTray := systray.RunWithExternalLoop(func() {
		iconBytes := trayIconFallback
		switch runtime.GOOS {
		case "windows":
			if len(trayIconWindows) > 0 {
				iconBytes = trayIconWindows
			}
			if len(iconBytes) > 0 {
				systray.SetIcon(iconBytes)
			}
		case "darwin":
			if len(trayIconMacOS) > 0 {
				iconBytes = trayIconMacOS
			}
			// macOS menu bar icon prefers template icons.
			if len(iconBytes) > 0 {
				systray.SetTemplateIcon(iconBytes, iconBytes)
			}
		default:
			if len(iconBytes) > 0 {
				systray.SetIcon(iconBytes)
			}
		}
		if runtime.GOOS != "darwin" {
			systray.SetTitle(trayLabels.AppTitle)
		}
		systray.SetTooltip(trayLabels.IconTooltip)

		// статус, туннели, «Отключить все», режимы окна (tray_menu.go)
		app.buildTrayMenu(showMainWindow)
		showWinItem := systray.AddMenuItem(trayLabels.ShowMainTitle, trayLabels.ShowMainTooltip)
		showWinItem.Click(showMainWindow)
		systray.AddSeparator()
		quitMenu := systray.AddMenuItem(trayLabels.QuitTitle, trayLabels.QuitTooltip)
		quitMenu.Click(func() {
			if app != nil && app.ctx != nil {
				app.PrepareForQuit()
				wailsruntime.Quit(app.ctx)
				return
			}
			// Fallback for edge cases where Wails context isn't ready yet.
			os.Exit(0)
		})

		// По клику на иконку открывается меню (как в примере energye/systray).
		// macOS: CreateMenu вешает меню на NSStatusItem, система сама показывает его по левому клику (в Wails это надёжнее, чем SetOnClick).
		// Windows: ShowMenu показывает тот же набор пунктов.
		// Linux: меню — это DBusMenu StatusNotifier. Библиотека вызывает SetOnClick без объекта меню,
		// поэтому левый клик открывает окно, а меню рисует оболочка (AppIndicator / KDE).
		popupTrayMenu := func(menu systray.IMenu) {
			if menu != nil {
				_ = menu.ShowMenu()
			}
		}
		switch runtime.GOOS {
		case "darwin":
			systray.CreateMenu()
		case "linux":
			systray.SetOnClick(func(systray.IMenu) { showMainWindow() })
		default:
			systray.SetOnClick(popupTrayMenu)
			systray.SetOnRClick(popupTrayMenu)
		}

		app.SetTrayMenuItems(showWinItem, quitMenu)
	}, func() {})
	startTray()
	defer endTray()

	// Create application with options
	err := wails.Run(&options.App{
		Title:         "Norka",
		Width:         1024,
		Height:        768,
		DisableResize: true,
		// Windows: без системной рамки, заголовок рисует фронтенд (AppTitleBar.vue, window_chrome.go).
		// Размер не меняется пользователем, поэтому рамка для ресайза не нужна, а без кнопки
		// «Развернуть» двойной клик по заголовку окно не разворачивает.
		Frameless: customTitleBar,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour:  &options.RGBA{R: 27, G: 38, B: 54, A: 1},
		OnStartup:         app.startup,
		OnBeforeClose:     app.beforeClose,
		OnShutdown:        app.shutdown,
		HideWindowOnClose: runtime.GOOS == "darwin",
		// Keep WebView2 state outside the installation directory. This is essential
		// for packaged Windows builds, whose install location is not writable.
		// GPU acceleration is disabled because this application is a form/list UI;
		// it avoids WebView2 renderer blank-screen failures on some Windows builds.
		Windows: &windows.Options{
			WebviewUserDataPath:  webviewUserDataPath(),
			WebviewGpuIsDisabled: true,
		},
		// Linux: нативные рамка и иконка окна. GPU выключен так же, как если Linux == nil:
		// интерфейс — формы и списки, программный рендеринг устойчивее на Xvfb и части дистрибутивов.
		Linux: &wailslinux.Options{
			Icon:             trayIconFallback,
			ProgramName:      "norka",
			WebviewGpuPolicy: wailslinux.WebviewGpuPolicyNever,
		},
		SingleInstanceLock: &options.SingleInstanceLock{
			UniqueId: "norka-single-instance",
			OnSecondInstanceLaunch: func(secondInstanceData options.SecondInstanceData) {
				if id := notify.ParseFocusArg(secondInstanceData.Args); id > 0 {
					app.FocusTunnel(id)
					return
				}
				showMainWindow()
			},
		},
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
	// Ensure tray resources are released when the window exits directly.
	systray.Quit()
}

func webviewUserDataPath() string {
	if runtime.GOOS != "windows" {
		return ""
	}

	cacheDir, err := os.UserCacheDir()
	if err != nil || cacheDir == "" {
		return ""
	}

	path := filepath.Join(cacheDir, "Norka", "WebView2")
	if err := os.MkdirAll(path, 0o700); err != nil {
		return ""
	}
	return path
}
