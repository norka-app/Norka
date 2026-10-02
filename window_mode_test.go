package main

import "testing"

func TestClampToWorkArea(t *testing.T) {
	work := windowRect{X: 0, Y: 0, W: 1920, H: 1040}
	cases := []struct {
		name string
		in   windowRect
		work windowRect
		want windowRect
	}{
		{"inside", windowRect{100, 100, 1040, 807}, work, windowRect{100, 100, 1040, 807}},
		{"above top", windowRect{100, -400, 1040, 807}, work, windowRect{100, 0, 1040, 807}},
		{"past right and bottom", windowRect{1500, 900, 1040, 807}, work, windowRect{880, 233, 1040, 807}},
		{"left of screen", windowRect{-300, 50, 429, 188}, work, windowRect{0, 50, 429, 188}},
		{"taller than work area", windowRect{10, 300, 1040, 1200}, work, windowRect{10, 0, 1040, 1200}},
		{"secondary monitor on the left", windowRect{-2000, -50, 1040, 807}, windowRect{X: -1920, Y: 0, W: 1920, H: 1040}, windowRect{-1920, 0, 1040, 807}},
		{"top taskbar", windowRect{200, 10, 429, 188}, windowRect{X: 0, Y: 48, W: 1920, H: 1032}, windowRect{200, 48, 429, 188}},
		// Невидимая рамка безрамочного окна после перетаскивания к верхнему краю.
		// GetSystemMetricsForDpi(SM_CXFRAME)+GetSystemMetricsForDpi(SM_CXPADDEDBORDER):
		// 8 px при 96 DPI (100%), 9 px при 120 DPI (125%), 11 px при 144 DPI (150%).
		{"frameless border 100%", windowRect{40, -8, 1024, 768}, windowRect{X: 0, Y: 0, W: 1920, H: 1152}, windowRect{40, 0, 1024, 768}},
		{"frameless border 125%", windowRect{40, -9, 1280, 960}, windowRect{X: 0, Y: 0, W: 1920, H: 1152}, windowRect{40, 0, 1280, 960}},
		{"frameless border 150%", windowRect{40, -11, 1536, 800}, windowRect{X: 0, Y: 0, W: 1920, H: 1128}, windowRect{40, 0, 1536, 800}},
	}
	for _, tc := range cases {
		if got := clampToWorkArea(tc.in, tc.work); got != tc.want {
			t.Errorf("%s: clampToWorkArea(%+v) = %+v, want %+v", tc.name, tc.in, got, tc.want)
		}
	}
}

// Windows после SetWindowPos иногда повторяет сдвиг на ширину рамки.
// Компенсация имеет смысл только если второй вызов получил тот же сдвиг:
// тогда окно оказывается на краю рабочей области, а не на ширину рамки ниже.
func TestFrameShiftCompensation(t *testing.T) {
	requested := windowRect{X: 40, Y: 0, W: 1024, H: 768}
	for _, border := range []int{8, 9, 11} {
		afterFirst := requested
		afterFirst.Y -= border
		compensated := compensateSystemShift(requested, afterFirst)
		if compensated.Y != border {
			t.Fatalf("border %d: compensated Y = %d, want %d", border, compensated.Y, border)
		}
		// Система снова вычитает рамку — видимый верх совпадает с запросом.
		afterSecond := compensated
		afterSecond.Y -= border
		if afterSecond.Y != requested.Y {
			t.Fatalf("border %d: landed Y = %d, want %d", border, afterSecond.Y, requested.Y)
		}
		if !frameShiftStuck(requested, afterFirst, compensated, afterSecond) {
			t.Fatalf("border %d: constant shift was not recognised", border)
		}
		// Сдвига не было: компенсация сдвинула бы окно вниз, её нужно отменить.
		if frameShiftStuck(requested, afterFirst, compensated, compensated) {
			t.Fatalf("border %d: one-off shift must not be kept", border)
		}
	}
}
