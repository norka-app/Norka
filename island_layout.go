package main

// Размеры Dynamic Island утверждены для 1920×1080 при масштабе 100%.
// Это логические пиксели (DIP), не физические.
const (
	islandTopGapDIP = 12
	islandPanelGap  = 28
	islandPanelMinY = 8
)

// IslandStageLayout — капсула и полное окно внутри одного окна-хозяина.
// Координаты в DIP относительно левого верхнего угла хозяина.
type IslandStageLayout struct {
	IslandX int `json:"islandX"`
	IslandY int `json:"islandY"`
	IslandW int `json:"islandW"`
	IslandH int `json:"islandH"`
	IslandR int `json:"islandR"`
	PanelX  int `json:"panelX"`
	PanelY  int `json:"panelY"`
	PanelW  int `json:"panelW"`
	PanelH  int `json:"panelH"`
	PanelR  int `json:"panelR"`
}

// IslandShape — скруглённый прямоугольник в DIP относительно окна.
type IslandShape struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	W      int `json:"w"`
	H      int `json:"h"`
	Radius int `json:"radius"`
}

// capsuleOrigin ставит капсулу по центру рабочей области, в 12 DIP от верхнего края.
func capsuleOrigin(work dipRect, width, height int) dipRect {
	if width < 1 {
		width = 1
	}
	if height < 1 {
		height = 1
	}
	x := work.X
	if work.W > width {
		x = work.X + (work.W-width)/2
	}
	return dipRect{X: x, Y: work.Y + islandTopGapDIP, W: width, H: height}
}

// layoutIslandStage раскладывает капсулу и панель полного окна внутри хозяина.
// panelX < 0 или panelY < 0 — панель ставится сама: по центру, под капсулой.
// Нулевые panelW/panelH заменяются на 1024×768 и ужимаются в хозяина.
func layoutIslandStage(hostW, hostH, islandW, islandH, islandR, panelX, panelY, panelW, panelH int) IslandStageLayout {
	if hostW < 1 {
		hostW = 1
	}
	if hostH < 1 {
		hostH = 1
	}
	if islandW < 1 {
		islandW = 1
	}
	if islandH < 1 {
		islandH = 1
	}
	if islandW > hostW {
		islandW = hostW
	}
	if islandH > hostH {
		islandH = hostH
	}
	if panelW <= 0 {
		panelW = 1024
	}
	if panelH <= 0 {
		panelH = 768
	}
	if panelW > hostW {
		panelW = hostW
	}
	if panelH > hostH {
		panelH = hostH
	}
	layout := IslandStageLayout{
		IslandX: 0,
		IslandY: islandTopGapDIP,
		IslandW: islandW,
		IslandH: islandH,
		IslandR: islandR,
		PanelW:  panelW,
		PanelH:  panelH,
		PanelR:  14,
	}
	if hostW > islandW {
		layout.IslandX = (hostW - islandW) / 2
	}
	if layout.IslandY+layout.IslandH > hostH {
		layout.IslandY = 0
	}
	floor := layout.IslandY + layout.IslandH + islandPanelMinY
	if panelX < 0 || panelY < 0 {
		panelX = 0
		if hostW > panelW {
			panelX = (hostW - panelW) / 2
		}
		panelY = layout.IslandY + layout.IslandH + islandPanelGap
	}
	if panelX < 0 {
		panelX = 0
	}
	if panelY < floor {
		panelY = floor
	}
	if panelX+panelW > hostW {
		panelX = hostW - panelW
	}
	if panelY+panelH > hostH {
		panelY = hostH - panelH
	}
	if panelX < 0 {
		panelX = 0
	}
	if panelY < 0 {
		panelY = 0
	}
	layout.PanelX = panelX
	layout.PanelY = panelY
	return layout
}
