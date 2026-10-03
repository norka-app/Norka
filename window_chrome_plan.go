package main

// ChromePlan is the window frame chosen before wails.Run. Wails reads
// Frameless, translucency and the macOS title bar only at startup, so a
// later change of the frameless_window flag waits for the next process.
type ChromePlan struct {
	// Frameless is the design flag as applied to this process.
	// Linux never sets it: the native frame stays.
	Frameless bool
	// Platform is runtime.GOOS: windows, darwin or linux.
	Platform string
	// Backdrop is what the webview should paint over.
	// mica: Win11 22H2+ DWM backdrop (header and sidebar stay transparent).
	// solid: older Windows, opaque header and sidebar.
	// vibrancy: macOS translucent sidebar, solid content.
	// native: the flag is off, or the platform keeps the system frame.
	Backdrop string
	// CustomTitleBar is the existing Windows caption drawn by the frontend.
	CustomTitleBar bool
	// WindowsTranslucent asks DWM for a Mica backdrop and a transparent webview.
	WindowsTranslucent bool
	// MacInset hides the macOS title bar and insets the traffic lights.
	MacInset bool
	// GPUDisabled is the Windows WebView2 software-rasterizer switch.
	// It stays on unless Mica is requested: --disable-gpu paints an opaque
	// frame that covers the DWM backdrop.
	GPUDisabled bool
}

// planWindowChrome decides the frame from the saved flag and the OS.
// micaOK is true only on Windows 11 22H2 (build 22621) or newer.
func planWindowChrome(goos string, flagOn bool, micaOK bool) ChromePlan {
	plan := ChromePlan{
		Platform:       goos,
		Backdrop:       "native",
		CustomTitleBar: goos == "windows",
		// Historical default. Only the Windows webview reads it. Mica is the
		// one case that turns it off, because a software frame hides the backdrop.
		GPUDisabled: true,
	}
	if !flagOn || goos == "linux" {
		return plan
	}
	switch goos {
	case "windows":
		plan.Frameless = true
		if micaOK {
			plan.Backdrop = "mica"
			plan.WindowsTranslucent = true
			plan.GPUDisabled = false
			return plan
		}
		plan.Backdrop = "solid"
	case "darwin":
		plan.Frameless = true
		plan.Backdrop = "vibrancy"
		plan.MacInset = true
	}
	return plan
}
