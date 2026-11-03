package render

import "math"

var (
	skyTop     = RGB{4, 2, 16}
	skyLow     = RGB{60, 8, 72}
	horizonHot = RGB{255, 60, 150}
	sunTop     = RGB{255, 230, 110}
	sunLow     = RGB{255, 40, 150}
	floorFar   = RGB{20, 2, 34}
	floorNear  = RGB{6, 2, 18}
	lineFar    = RGB{255, 40, 200}
	lineNear   = RGB{0, 200, 255}
	ridgeCol   = RGB{18, 4, 34}
	ridgeEdge  = RGB{140, 40, 160}
)

// Backdrop paints the menu scene: a striped synthwave sun over a mountain
// ridge, drifting stars, and a perspective grid floor scrolling toward the
// viewer.
func Backdrop(f *Frame, t float64) {
	hy := f.H * 11 / 20
	cx := float64(f.W) / 2
	sunR := math.Min(float64(f.W)*0.17, float64(hy)*0.5)
	sunY := float64(hy) - sunR*0.2
	for y := 0; y < f.H; y++ {
		row := f.Pix[y*f.W : (y+1)*f.W]
		if y < hy {
			sky(row, y, hy, cx, sunY, sunR, t)
		} else {
			floor(row, y, hy, f.H, cx, t)
		}
		hg := math.Exp(-math.Abs(float64(y-hy)) / 1.6)
		if hg > 0.02 {
			glow := scale(horizonHot, int(hg*150))
			for x := range row {
				row[x] = add(row[x], glow)
			}
		}
	}
	mountains(f, hy, cx, sunR*1.3, t)
	stars(f, hy, t)
}

func sky(row []RGB, y, hy int, cx, sunY, sunR, t float64) {
	v := float64(y) / float64(hy)
	base := lerp(skyTop, skyLow, v*v)
	dy := float64(y) - sunY
	gap := 0.0
	if dy > -sunR*0.6 {
		period := 4.0
		ph := math.Mod(dy+sunR*0.6+t*2.5, period)
		gap = (dy + sunR*0.6) / (sunR * 0.8) * 2.6
		if ph >= gap {
			gap = 0
		}
	}
	sunC := lerp(sunTop, sunLow, (dy+sunR)/(2*sunR))
	for x := range row {
		dx := float64(x) - cx
		d := math.Sqrt(dx*dx + dy*dy)
		c := base
		if d < sunR && gap == 0 {
			c = sunC
		} else if d < sunR*1.9 {
			k := 1 - (d-sunR)/(sunR*0.9)
			if d < sunR {
				k = 0.5
			}
			c = add(c, scale(sunLow, int(k*k*90)))
		}
		row[x] = c
	}
}

func floor(row []RGB, y, hy, h int, cx, t float64) {
	v := float64(y-hy) / float64(h-hy)
	base := lerp(floorFar, floorNear, v)
	depth := func(yy int) float64 { return 60 / (float64(yy-hy) + 0.6) }
	z0, z1 := depth(y), depth(y+1)
	scroll := t * 3
	hline := math.Floor(z0+scroll) != math.Floor(z1+scroll)
	// Rows near the horizon cross several grid lines each; dim them by
	// density so the far floor reads as haze instead of a solid band.
	density := math.Min(1, 0.7/(z0-z1))
	fade := math.Min(1, v*1.8) * density
	pulse := 0.8 + 0.2*math.Sin(t*2.4)
	lc := lerp(lineFar, lineNear, v*v)
	line := scale(lc, int(fade*pulse*256))
	vfade := math.Min(1, 40/z0/3)
	for x := range row {
		c := base
		if hline {
			c = add(c, line)
		} else {
			wx0 := (float64(x) - cx) * z0 / 40
			wx1 := (float64(x) + 1 - cx) * z0 / 40
			if math.Floor(wx0) != math.Floor(wx1) {
				c = add(c, scale(lc, int(vfade*math.Min(1, v*2.5+0.15)*pulse*220)))
			}
		}
		row[x] = c
	}
}

// mountains frame the sun from both sides and leave its striped base clear.
func mountains(f *Frame, hy int, cx, clear, t float64) {
	for x := 0; x < f.W; x++ {
		fx := float64(x)
		side := clamp01((math.Abs(fx-cx) - clear) / 12)
		h := 1.5 + 2.5*math.Abs(math.Sin(fx*0.07+1.3))*math.Abs(math.Sin(fx*0.021)) + 1.2*math.Abs(math.Sin(fx*0.23+t*0.05)) + 0.6*math.Sin(fx*0.61)
		top := hy - int(h*side*side*(3-2*side)+0.5)
		for y := top; y < hy && side > 0; y++ {
			c := ridgeCol
			if y == top {
				c = ridgeEdge
			}
			f.Set(x, y, c)
		}
	}
}

func stars(f *Frame, hy int, t float64) {
	var r rng = 0x57A125
	for i := 0; i < 70; i++ {
		n := r.next()
		depth := 0.3 + float64(n>>56)/255
		x := math.Mod(float64(n%uint64(f.W*8))/8-t*depth*1.5, float64(f.W))
		if x < 0 {
			x += float64(f.W)
		}
		y := int((n >> 20) % uint64(hy-4))
		tw := 0.5 + 0.5*math.Sin(t*(1.5+depth*3)+float64(i))
		c := scale(RGB{200, 210, 255}, int((0.3+0.7*tw)*depth*200))
		if xi := int(x); xi < f.W {
			if p := f.Pix[y*f.W+xi]; int(p.R)+int(p.G)+int(p.B) < 240 {
				f.Add(xi, y, c)
			}
		}
	}
}

var titleScratch *Frame

// Title draws the ESTELA logo centered on cx with its top at y: chrome
// gradient letters with a 3D extrusion, a pulsing neon halo and a
// highlight sweeping across.
func Title(f *Frame, cx, y int, t float64) {
	const word = "ESTELA"
	k := 2
	if TextWidthScaled(word, 3) <= f.W-8 {
		k = 3
	}
	tw, th := TextWidthScaled(word, k), 7*k
	const m = 4
	sw, sh := tw+2*m, th+2*m
	if titleScratch == nil || titleScratch.W < sw || titleScratch.H < sh {
		titleScratch = NewFrame(sw, sh)
	}
	sc := titleScratch
	sc.Clear(black)
	DrawTextScaled(sc, m, m, word, white, k)
	x0, y0 := cx-tw/2-m, y-m
	pulse := 0.75 + 0.25*math.Sin(t*2.5)
	haloC := lerp(RGB{255, 0, 200}, RGB{120, 40, 255}, 0.5+0.5*math.Sin(t*0.7))
	for py := 0; py < sh; py++ {
		for px := 0; px < sw; px++ {
			if lit(sc, px, py) {
				continue
			}
			n := 0
			for dy := -2; dy <= 2; dy++ {
				for dx := -2; dx <= 2; dx++ {
					if lit(sc, px+dx, py+dy) {
						n += 3 - max(abs(dx), abs(dy))
					}
				}
			}
			if n > 0 {
				f.Add(x0+px, y0+py, scale(haloC, int(math.Min(1, float64(n)/9)*pulse*170)))
			}
		}
	}
	for py := sh - 1; py >= 0; py-- {
		for px := sw - 1; px >= 0; px-- {
			if lit(sc, px, py) {
				for e := k; e >= 1; e-- {
					f.Set(x0+px+e, y0+py+e, scale(RGB{110, 10, 90}, 256-e*40))
				}
			}
		}
	}
	sweep := math.Mod(t*0.5, 1.6)*float64(sw+th) - float64(th)
	for py := 0; py < sh; py++ {
		v := float64(py-m) / float64(th)
		var c RGB
		if v < 0.5 {
			c = lerp(RGB{90, 200, 255}, RGB{235, 250, 255}, v*2)
		} else {
			c = lerp(RGB{150, 20, 120}, RGB{255, 170, 60}, (v-0.5)*2)
		}
		for px := 0; px < sw; px++ {
			if !lit(sc, px, py) {
				continue
			}
			pc := c
			if d := math.Abs(float64(px) + float64(py)*0.6 - sweep); d < 3 {
				pc = lerp(pc, white, 1-d/3)
			}
			f.Set(x0+px, y0+py, pc)
		}
	}
}

func lit(f *Frame, x, y int) bool {
	return uint(x) < uint(f.W) && uint(y) < uint(f.H) && f.Pix[y*f.W+x].R != 0
}
