package main

import "testing"

func TestPlanWindowChromeFlagOffMatchesToday(t *testing.T) {
	for _, goos := range []string{"windows", "darwin", "linux"} {
		plan := planWindowChrome(goos, false, true)
		if plan.Frameless || plan.WindowsTranslucent || plan.MacInset || plan.Backdrop != "native" {
			t.Fatalf("%s flag off changed the frame: %+v", goos, plan)
		}
		if !plan.GPUDisabled {
			t.Fatalf("%s flag off must keep the GPU disabled", goos)
		}
		if plan.CustomTitleBar != (goos == "windows") {
			t.Fatalf("%s custom title bar: %+v", goos, plan)
		}
	}
}

func TestPlanWindowChromeMicaNeedsGPU(t *testing.T) {
	plan := planWindowChrome("windows", true, true)
	if !plan.Frameless || !plan.WindowsTranslucent || plan.Backdrop != "mica" || plan.GPUDisabled {
		t.Fatalf("mica plan: %+v", plan)
	}
	if !plan.CustomTitleBar || plan.MacInset {
		t.Fatalf("windows controls: %+v", plan)
	}
}

func TestPlanWindowChromeOlderWindowsIsSolid(t *testing.T) {
	plan := planWindowChrome("windows", true, false)
	if !plan.Frameless || plan.WindowsTranslucent || plan.Backdrop != "solid" || !plan.GPUDisabled {
		t.Fatalf("solid fallback: %+v", plan)
	}
}

func TestPlanWindowChromeMacInset(t *testing.T) {
	plan := planWindowChrome("darwin", true, false)
	if !plan.Frameless || !plan.MacInset || plan.Backdrop != "vibrancy" || plan.WindowsTranslucent {
		t.Fatalf("mac plan: %+v", plan)
	}
	if plan.CustomTitleBar {
		t.Fatal("macOS keeps the system traffic lights")
	}
}

func TestPlanWindowChromeLinuxIgnoresFlag(t *testing.T) {
	plan := planWindowChrome("linux", true, true)
	if plan.Frameless || plan.WindowsTranslucent || plan.MacInset || plan.Backdrop != "native" {
		t.Fatalf("linux must keep the native frame: %+v", plan)
	}
}
