package media

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"

	_ "golang.org/x/image/bmp"
	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp"
	_ "image/gif"
	_ "image/png"
)

// MaxEdge 是入库图片的最长边，超过则等比缩小，控制体积。
const MaxEdge = 1600

// Import 把任意本地图片转成规整的 JPEG 存进 imagesDir，返回文件名。
func Import(src, imagesDir string) (string, error) {
	f, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return "", fmt.Errorf("无法解析图片 %s: %w", filepath.Base(src), err)
	}

	img = resize(img, MaxEdge)

	if err := os.MkdirAll(imagesDir, 0o755); err != nil {
		return "", err
	}
	name := randomName() + ".jpg"
	dst := filepath.Join(imagesDir, name)
	out, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	defer out.Close()

	if err := jpeg.Encode(out, img, &jpeg.Options{Quality: 88}); err != nil {
		os.Remove(dst)
		return "", err
	}
	return name, nil
}

// DataURL 把已入库的图片读成 data URL，供前端 <img> 直接使用。
func DataURL(imagesDir, name string) (string, error) {
	name = filepath.Base(name)
	if name == "" || name == "." || name == ".." {
		return "", errors.New("非法文件名")
	}
	path := filepath.Join(imagesDir, name)
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	if info.Size() > 24<<20 {
		return "", errors.New("图片过大")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	mime := "image/jpeg"
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png":
		mime = "image/png"
	case ".webp":
		mime = "image/webp"
	case ".gif":
		mime = "image/gif"
	case ".bmp":
		mime = "image/bmp"
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(raw), nil
}

func resize(src image.Image, maxEdge int) image.Image {
	b := src.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= maxEdge && h <= maxEdge {
		return src
	}
	var nw, nh int
	if w >= h {
		nw = maxEdge
		nh = int(float64(h) * float64(maxEdge) / float64(w))
	} else {
		nh = maxEdge
		nw = int(float64(w) * float64(maxEdge) / float64(h))
	}
	if nh < 1 {
		nh = 1
	}
	if nw < 1 {
		nw = 1
	}
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	draw.CatmullRom.Scale(dst, dst.Bounds(), src, b, draw.Over, nil)
	return dst
}

func randomName() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", os.Getpid())
	}
	return fmt.Sprintf("%d_%s", os.Getpid(), hex.EncodeToString(buf))
}
