// 用代码画出应用图标：圆角方形深空底 + 粉色粒子心 + 青色星尘地面，并打包成多尺寸 ICO。
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"math/rand"
	"os"
	"path/filepath"

	"golang.org/x/image/draw"
)

const S = 2048 // 逻辑画布边长

func main() {
	root, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}
	// 允许从仓库根目录或 tools/genicon 下运行
	for i := 0; i < 3; i++ {
		if _, err := os.Stat(filepath.Join(root, "wails.json")); err == nil {
			break
		}
		root = filepath.Dir(root)
	}

	canvas := render(S)

	pngPath := filepath.Join(root, "build", "appicon.png")
	must(os.MkdirAll(filepath.Dir(pngPath), 0o755))
	must(writePNG(pngPath, scale(canvas, 1024)))

	icoPath := filepath.Join(root, "build", "windows", "icon.ico")
	must(writeICO(icoPath, canvas, []int{16, 24, 32, 48, 64, 128, 256}))

	fmt.Println("icon ok:", pngPath)
	fmt.Println("icon ok:", icoPath)
}

func render(size int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	f := float64(size)
	rnd := rand.New(rand.NewSource(20261112))

	// 1. 圆角方形底色：从左上暖紫到右下深蓝的斜向渐变 + 中心微光
	cx, cy := f*0.5, f*0.44
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			tx := float64(x) / f
			ty := float64(y) / f
			// 斜向渐变
			tt := (tx*0.55 + ty*0.85)
			r := 16 - 9*tt
			g := 13 - 7*tt
			b := 34 - 16*tt
			// 中心柔光
			d := math.Hypot(float64(x)-cx, float64(y)-cy) / (f * 0.62)
			if d > 1 {
				d = 1
			}
			glow := math.Pow(1-d, 2.4)
			r += 26 * glow
			g += 20 * glow
			b += 46 * glow
			img.Set(x, y, color.RGBA{uint8(clamp(r)), uint8(clamp(g)), uint8(clamp(b)), 255})
		}
	}

	// 2. 远处星点
	for i := 0; i < 220; i++ {
		x := rnd.Float64() * f
		y := rnd.Float64() * f * 0.75
		a := 0.10 + rnd.Float64()*0.35
		rad := 1.2 + rnd.Float64()*2.2
		additive(img, x, y, rad, 0.75, 0.82, 1.0, a)
	}

	// 3. 心形粒子
	scaleUnit := f * 0.198
	heartCX, heartCY := f*0.5, f*0.408

	// 3a. 主体填充：拒绝采样，越靠近边缘越亮
	filled := 0
	for filled < 42000 {
		u := (rnd.Float64()*2 - 1) * 1.5
		v := (rnd.Float64()*2 - 1) * 1.55
		if !insideHeart(u, v) {
			continue
		}
		filled++

		// 边缘因子：靠近轮廓 -> 更亮更粉
		edge := 1 - math.Min(1, math.Abs(heartField(u, v))*2.6)

		px := heartCX + u*scaleUnit
		py := heartCY - v*scaleUnit*1.12

		rad := (0.9 + rnd.Float64()*2.0) * (f / S * 2.6)
		// 粉 -> 亮粉 -> 白
		t := 0.55 + 0.45*rnd.Float64()
		cr := 1.0
		cg := 0.44 + 0.32*t
		cb := 0.64 + 0.22*t
		a := (0.62 + 0.30*edge) * (0.75 + 0.25*rnd.Float64())
		additive(img, px, py, rad, cr, cg, cb, a)
	}

	// 3b. 边缘飞散的粒子
	for i := 0; i < 3200; i++ {
		ang := rnd.Float64() * math.Pi * 2
		u := math.Cos(ang) * (0.92 + rnd.Float64()*0.5)
		v := math.Sin(ang) * (0.92 + rnd.Float64()*0.5)
		if !insideHeart(u, v) {
			u *= 1.0 + rnd.Float64()*0.12
			v *= 1.0 + rnd.Float64()*0.12
		}
		px := heartCX + u*scaleUnit*1.02
		py := heartCY - v*scaleUnit*1.14
		rad := (0.6 + rnd.Float64()*1.5) * (f / S * 2.6)
		additive(img, px, py, rad, 1.0, 0.55, 0.74, 0.06+rnd.Float64()*0.26)
	}

	// 4. 青色星尘：贴着底边的一层薄雾，中间浓两边淡，没有硬边
	baseY := f * 1.02
	for i := 0; i < 36000; i++ {
		xr := rnd.NormFloat64() * 0.23
		if math.Abs(xr) > 0.58 {
			continue
		}
		px := f*0.5 + xr*f

		// 高度：越靠近底边越密，往上迅速稀薄
		hh := rnd.Float64()
		py := baseY - math.Pow(hh, 2.0)*f*0.20

		// 水平衰减：中间浓、两侧淡，形成自然散开的边缘
		wf := math.Exp(-math.Pow(xr/0.30, 2))
		if rnd.Float64() > 0.10+0.90*wf*(1-0.65*hh) {
			continue
		}

		rad := (0.6 + rnd.Float64()*1.6) * (f / S * 2.6)
		a := (0.05 + 0.24*wf) * (1 - 0.55*hh)
		additive(img, px, py, rad, 0.55, 0.92, 1.0, a)
	}

	// 4b. 底部几颗亮一点的星点，给这层雾一点颗粒感
	for i := 0; i < 900; i++ {
		xr := rnd.NormFloat64() * 0.17
		if math.Abs(xr) > 0.55 {
			continue
		}
		hh := rnd.Float64()
		px := f*0.5 + xr*f
		py := baseY - math.Pow(hh, 2.0)*f*0.16
		rad := (1.0 + rnd.Float64()*1.8) * (f / S * 2.6)
		additive(img, px, py, rad, 0.72, 0.96, 1.0, 0.18+rnd.Float64()*0.30)
	}

	// 5. 顶部内高光 + 圆角裁剪：外圈透明，像微信桌面图标那种圆角方形
	innerRim(img, size)
	roundedMask(img, size, 0.225)

	return img
}

// roundedMask 把画布裁成圆角方形（外圈 alpha=0，边缘 1px 抗锯齿）。
func roundedMask(img *image.RGBA, size int, radiusRatio float64) {
	f := float64(size)
	rad := f * radiusRatio
	// 圆角矩形 SDF：四角按圆处理，其余为直边
	sd := func(x, y float64) float64 {
		half := f * 0.5
		qx := math.Abs(x-half) - (half - rad)
		qy := math.Abs(y-half) - (half - rad)
		ax := math.Max(qx, 0)
		ay := math.Max(qy, 0)
		return math.Hypot(ax, ay) + math.Min(math.Max(qx, qy), 0) - rad
	}
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			d := sd(float64(x)+0.5, float64(y)+0.5)
			// d<0 在内部；1px 过渡
			cov := 0.0
			if d <= -1 {
				cov = 1
			} else if d < 1 {
				cov = (1 - d) * 0.5
			}
			i := img.PixOffset(x, y)
			img.Pix[i+3] = uint8(255 * cov)
			if cov < 0.999 {
				// 预乘一下，避免缩放时边缘泛白
				for k := 0; k < 3; k++ {
					img.Pix[i+k] = uint8(float64(img.Pix[i+k]) * cov)
				}
			}
		}
	}
}

// innerRim 沿圆角边缘画一圈很淡的内描边，让图标在深色背景上也有轮廓。
func innerRim(img *image.RGBA, size int) {
	f := float64(size)
	rad := f * 0.225
	half := f * 0.5
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			px := float64(x) + 0.5
			py := float64(y) + 0.5
			qx := math.Abs(px-half) - (half - rad)
			qy := math.Abs(py-half) - (half - rad)
			ax := math.Max(qx, 0)
			ay := math.Max(qy, 0)
			d := math.Hypot(ax, ay) + math.Min(math.Max(qx, qy), 0) - rad
			if d < -f*0.004 || d > 0 {
				continue
			}
			k := 1 - math.Abs(d+f*0.002)/(f*0.002)
			if k <= 0 {
				continue
			}
			additive(img, px, py, f*0.0016, 1.0, 0.86, 0.94, 0.10*k)
		}
	}
}

// insideHeart 判断 (u,v) 是否落在标准心形内部（v 向上为正）。
func insideHeart(u, v float64) bool {
	x := u / 1.15
	y := v / 1.15
	return heartField(x, y) <= 0
}

func heartField(x, y float64) float64 {
	a := x*x + y*y - 1
	return a*a*a - x*x*y*y*y
}

// additive 画一个发光粒子（加色混合）。
func additive(img *image.RGBA, cx, cy, rad, cr, cg, cb, alpha float64) {
	if rad < 0.6 {
		rad = 0.6
	}
	x0 := int(cx - rad - 1)
	x1 := int(cx + rad + 1)
	y0 := int(cy - rad - 1)
	y1 := int(cy + rad + 1)
	b := img.Bounds()
	for y := y0; y <= y1; y++ {
		if y < b.Min.Y || y >= b.Max.Y {
			continue
		}
		for x := x0; x <= x1; x++ {
			if x < b.Min.X || x >= b.Max.X {
				continue
			}
			dx := float64(x) + 0.5 - cx
			dy := float64(y) + 0.5 - cy
			d := math.Hypot(dx, dy) / rad
			if d >= 1 {
				continue
			}
			k := math.Pow(1-d, 1.7) * alpha
			i := img.PixOffset(x, y)
			img.Pix[i+0] = clamp(float64(img.Pix[i+0]) + cr*k*255)
			img.Pix[i+1] = clamp(float64(img.Pix[i+1]) + cg*k*255)
			img.Pix[i+2] = clamp(float64(img.Pix[i+2]) + cb*k*255)
		}
	}
}

func clamp(v float64) uint8 {
	if v <= 0 {
		return 0
	}
	if v >= 255 {
		return 255
	}
	return uint8(v)
}

func scale(src *image.RGBA, size int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, src.Bounds(), draw.Src, nil)
	return dst
}

func writePNG(path string, img *image.RGBA) error {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return err
	}
	return os.WriteFile(path, buf.Bytes(), 0o644)
}

// writeICO 手写 ICO：<=64 用 32bpp DIB，>=128 用 PNG 压缩，兼容 Win Vista+。
func writeICO(path string, src *image.RGBA, sizes []int) error {
	type entry struct {
		size int
		data []byte
	}
	entries := make([]entry, 0, len(sizes))
	for _, s := range sizes {
		im := scale(src, s)
		var data []byte
		if s >= 128 {
			var buf bytes.Buffer
			if err := png.Encode(&buf, im); err != nil {
				return err
			}
			data = buf.Bytes()
		} else {
			data = dibBytes(im)
		}
		entries = append(entries, entry{size: s, data: data})
	}

	var out bytes.Buffer
	// ICONDIR
	binary.Write(&out, binary.LittleEndian, uint16(0)) // reserved
	binary.Write(&out, binary.LittleEndian, uint16(1)) // type: icon
	binary.Write(&out, binary.LittleEndian, uint16(len(entries)))

	offset := 6 + 16*len(entries)
	for _, e := range entries {
		w := byte(e.size)
		if e.size >= 256 {
			w = 0
		}
		out.WriteByte(w)                                    // width
		out.WriteByte(w)                                    // height
		out.WriteByte(0)                                    // palette
		out.WriteByte(0)                                    // reserved
		binary.Write(&out, binary.LittleEndian, uint16(1))  // planes
		binary.Write(&out, binary.LittleEndian, uint16(32)) // bpp
		binary.Write(&out, binary.LittleEndian, uint32(len(e.data)))
		binary.Write(&out, binary.LittleEndian, uint32(offset))
		offset += len(e.data)
	}
	for _, e := range entries {
		out.Write(e.data)
	}
	return os.WriteFile(path, out.Bytes(), 0o644)
}

// dibBytes 生成 bottom-up 的 32bpp BGRA 位图 + AND 掩码。
func dibBytes(img *image.RGBA) []byte {
	w := img.Bounds().Dx()
	h := img.Bounds().Dy()

	rowSize := w * 4
	maskRow := ((w + 31) / 32) * 4
	imgSize := rowSize * h
	var buf bytes.Buffer

	// BITMAPINFOHEADER
	binary.Write(&buf, binary.LittleEndian, uint32(40))
	binary.Write(&buf, binary.LittleEndian, int32(w))
	binary.Write(&buf, binary.LittleEndian, int32(h*2))
	binary.Write(&buf, binary.LittleEndian, uint16(1))
	binary.Write(&buf, binary.LittleEndian, uint16(32))
	binary.Write(&buf, binary.LittleEndian, uint32(0))
	binary.Write(&buf, binary.LittleEndian, uint32(imgSize+maskRow*h))
	binary.Write(&buf, binary.LittleEndian, int32(2835))
	binary.Write(&buf, binary.LittleEndian, int32(2835))
	binary.Write(&buf, binary.LittleEndian, uint32(0))
	binary.Write(&buf, binary.LittleEndian, uint32(0))

	// 像素：自下而上，BGRA
	for y := h - 1; y >= 0; y-- {
		for x := 0; x < w; x++ {
			i := img.PixOffset(x, y)
			buf.WriteByte(img.Pix[i+2])
			buf.WriteByte(img.Pix[i+1])
			buf.WriteByte(img.Pix[i+0])
			buf.WriteByte(img.Pix[i+3])
		}
	}
	// AND 掩码全 0
	buf.Write(make([]byte, maskRow*h))
	return buf.Bytes()
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
