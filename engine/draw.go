package engine

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"os"

	"github.com/golang/freetype"
	"github.com/golang/freetype/truetype"
	"golang.org/x/image/math/fixed"
)

// =============================================================================
// 在 float64 矩阵上绘制文字（freetype + 旋转 + 背景矩形）
// =============================================================================

// drawTextOnMatrix 在 float64 矩阵上绘制文字（RGB 分通道）。
// mats[0]=R通道, mats[1]=G通道, mats[2]=B通道（值域 [0,255]）
func drawTextOnMatrix(mats [3][][]float64, matW, matH int, text string, cx, cy int, size float64,
	fontPath string, fgColor, bgColor color.RGBA, angle float64, alpha float64) {

	if text == "" || alpha <= 0 {
		return
	}
	if alpha > 1 {
		alpha = 1
	}

	// 加载字体
	font, err := loadFont(fontPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "警告：加载字体 %q 失败：%v，跳过文字\n", fontPath, err)
		return
	}

	// 1. 渲染文字到离屏 RGBA 图像
	textImg, textW, textH, baselineY := renderText(text, font, size, fgColor, bgColor)
	if textImg == nil || textW == 0 || textH == 0 {
		return
	}

	// 2. 如果需要旋转，对文字图像做旋转
	var finalImg *image.RGBA
	var finalW, finalH int
	var offsetX, offsetY int

	if angle != 0 {
		rotated := rotateImage(textImg, angle)
		finalImg = rotated
		finalW = rotated.Bounds().Dx()
		finalH = rotated.Bounds().Dy()
		offsetX = finalW / 2
		offsetY = finalH / 2
	} else {
		finalImg = textImg
		finalW = textW
		finalH = textH
		offsetX = textW / 2
		offsetY = baselineY // 以基线为垂直中心（更自然）
	}

	// 3. 将图像以 (cx, cy) 为中心放置，RGB 分通道混合，超出画框的部分裁剪
	pasteRGBWithCenterAndClip(mats, matW, matH, finalImg, cx-offsetX, cy-offsetY, alpha)
}

// renderText 使用 freetype 将文字渲染到 RGBA 图像。
func renderText(text string, font *truetype.Font, size float64, fgColor, bgColor color.RGBA) (*image.RGBA, int, int, int) {
	ctx := freetype.NewContext()
	ctx.SetFont(font)
	ctx.SetFontSize(size)
	ctx.SetDPI(72)

	// 计算文字总宽度
	var totalWidth int
	for _, r := range text {
		idx := font.Index(r)
		hmet := font.HMetric(fixed.Int26_6(size*64), idx)
		totalWidth += int(hmet.AdvanceWidth >> 6)
	}
	if totalWidth == 0 {
		totalWidth = 1
	}

	face := truetype.NewFace(font, &truetype.Options{
		Size: size,
		DPI:  72,
	})
	defer face.Close()

	metrics := face.Metrics()
	ascent := metrics.Ascent.Ceil()
	descent := metrics.Descent.Ceil()
	textH := ascent + descent
	if textH < 1 {
		textH = 1
	}

	pad := int(size / 4)
	imgW := totalWidth + pad*2
	imgH := textH + pad*2

	img := image.NewRGBA(image.Rect(0, 0, imgW, imgH))

	// 绘制背景矩形
	if bgColor.A > 0 {
		bg := color.RGBA{bgColor.R, bgColor.G, bgColor.B, bgColor.A}
		for y := 0; y < imgH; y++ {
			for x := 0; x < imgW; x++ {
				img.Set(x, y, bg)
			}
		}
	}

	// 绘制文字
	ctx = freetype.NewContext()
	ctx.SetFont(font)
	ctx.SetFontSize(size)
	ctx.SetDPI(72)
	ctx.SetClip(img.Bounds())
	ctx.SetDst(img)
	ctx.SetSrc(image.NewUniform(color.RGBA{fgColor.R, fgColor.G, fgColor.B, fgColor.A}))

	baselineX := pad
	baselineY := pad + ascent
	pt := freetype.Pt(baselineX, baselineY)
	_, err := ctx.DrawString(text, pt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "警告：渲染文字失败：%v\n", err)
		return nil, 0, 0, 0
	}

	return img, imgW, imgH, baselineY
}

// loadFont 加载 TTF/OTF 字体文件。
func loadFont(path string) (*truetype.Font, error) {
	if path == "" {
		for _, def := range []string{"simhei", "msyh", "arial", "helvetica"} {
			if p, err := ResolveFont(def); err == nil {
				path = p
				break
			}
		}
		if path == "" {
			return nil, fmt.Errorf("未指定字体且无可用系统字体")
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取字体文件失败: %w", err)
	}
	font, err := freetype.ParseFont(data)
	if err != nil {
		return nil, fmt.Errorf("解析字体失败: %w", err)
	}
	return font, nil
}

// =============================================================================
// 图像旋转
// =============================================================================

// rotateImage 将图像绕其中心旋转指定角度（度），逆时针。
func rotateImage(src image.Image, angleDeg float64) *image.RGBA {
	if angleDeg == 0 {
		b := src.Bounds()
		dst := image.NewRGBA(b)
		draw.Draw(dst, b, src, b.Min, draw.Src)
		return dst
	}

	rad := angleDeg * math.Pi / 180.0
	cos := math.Cos(rad)
	sin := math.Sin(rad)

	srcB := src.Bounds()
	srcW, srcH := srcB.Dx(), srcB.Dy()

	corners := [][2]float64{
		{0, 0},
		{float64(srcW), 0},
		{float64(srcW), float64(srcH)},
		{0, float64(srcH)},
	}
	var minX, minY, maxX, maxY float64
	for i, c := range corners {
		rx := c[0]*cos - c[1]*sin
		ry := c[0]*sin + c[1]*cos
		if i == 0 {
			minX, minY, maxX, maxY = rx, ry, rx, ry
		} else {
			if rx < minX {
				minX = rx
			}
			if rx > maxX {
				maxX = rx
			}
			if ry < minY {
				minY = ry
			}
			if ry > maxY {
				maxY = ry
			}
		}
	}

	dstW := int(math.Ceil(maxX - minX))
	dstH := int(math.Ceil(maxY - minY))
	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))

	srcCX := float64(srcW) / 2.0
	srcCY := float64(srcH) / 2.0
	dstCX := float64(dstW) / 2.0
	dstCY := float64(dstH) / 2.0

	for dy := 0; dy < dstH; dy++ {
		for dx := 0; dx < dstW; dx++ {
			tdx := float64(dx) - dstCX
			tdy := float64(dy) - dstCY

			sx := tdx*cos + tdy*sin + srcCX
			sy := -tdx*sin + tdy*cos + srcCY

			sx += float64(srcB.Min.X)
			sy += float64(srcB.Min.Y)
			c := bilinearSample(src, sx, sy)
			dst.SetRGBA(dx, dy, c)
		}
	}

	return dst
}

// bilinearSample 对图像做双线性插值采样。
func bilinearSample(img image.Image, x, y float64) color.RGBA {
	x0 := int(math.Floor(x))
	y0 := int(math.Floor(y))
	x1 := x0 + 1
	y1 := y0 + 1

	b := img.Bounds()
	if x0 < b.Min.X-1 || x1 > b.Max.X || y0 < b.Min.Y-1 || y1 > b.Max.Y {
		return color.RGBA{0, 0, 0, 0}
	}

	if x0 < b.Min.X {
		x0 = b.Min.X
	}
	if y0 < b.Min.Y {
		y0 = b.Min.Y
	}
	if x1 >= b.Max.X {
		x1 = b.Max.X - 1
	}
	if y1 >= b.Max.Y {
		y1 = b.Max.Y - 1
	}

	fx := x - float64(x0)
	fy := y - float64(y0)

	c00 := rgbaFrom(img, x0, y0)
	c10 := rgbaFrom(img, x1, y0)
	c01 := rgbaFrom(img, x0, y1)
	c11 := rgbaFrom(img, x1, y1)

	interp := func(a, b uint8) uint8 {
		return uint8(float64(a)*(1-fx) + float64(b)*fx + 0.5)
	}
	topR := interp(c00.R, c10.R)
	topG := interp(c00.G, c10.G)
	topB := interp(c00.B, c10.B)
	topA := interp(c00.A, c10.A)

	botR := interp(c01.R, c11.R)
	botG := interp(c01.G, c11.G)
	botB := interp(c01.B, c11.B)
	botA := interp(c01.A, c11.A)

	return color.RGBA{
		R: uint8(float64(topR)*(1-fy) + float64(botR)*fy + 0.5),
		G: uint8(float64(topG)*(1-fy) + float64(botG)*fy + 0.5),
		B: uint8(float64(topB)*(1-fy) + float64(botB)*fy + 0.5),
		A: uint8(float64(topA)*(1-fy) + float64(botA)*fy + 0.5),
	}
}

func rgbaFrom(img image.Image, x, y int) color.RGBA {
	r, g, b, a := img.At(x, y).RGBA()
	return color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), uint8(a >> 8)}
}

// =============================================================================
// 在 float64 矩阵上叠加图像（支持旋转）
// =============================================================================

// overlayImageOnMatrix 在 float64 矩阵上叠加外部图像（RGB 分通道）。
// 图像的 R 分量→通道0, G 分量→通道1, B 分量→通道2
func overlayImageOnMatrix(mats [3][][]float64, matW, matH int, imgPath string, cx, cy int,
	scale float64, angle float64, alpha float64) {

	srcImg, err := LoadImage(imgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "警告：加载贴图 %q 失败：%v，跳过\n", imgPath, err)
		return
	}

	if alpha <= 0 {
		return
	}
	if alpha > 1 {
		alpha = 1
	}
	if scale <= 0 {
		scale = 1.0
	}

	srcBounds := srcImg.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()

	dstW := int(float64(srcW) * scale)
	dstH := int(float64(srcH) * scale)
	if dstW < 1 {
		dstW = 1
	}
	if dstH < 1 {
		dstH = 1
	}

	var scaledImg image.Image
	if scale == 1.0 && srcW == dstW && srcH == dstH {
		scaledImg = srcImg
	} else {
		scaledImg = resizeImage(srcImg, dstW, dstH)
	}

	var finalImg image.Image
	if angle != 0 {
		finalImg = rotateImage(scaledImg, angle)
	} else {
		finalImg = scaledImg
	}

	finalB := finalImg.Bounds()
	finalW := finalB.Dx()
	finalH := finalB.Dy()
	pasteX := cx - finalW/2
	pasteY := cy - finalH/2

	pasteRGBWithCenterAndClip(mats, matW, matH, finalImg, pasteX, pasteY, alpha)
}

// =============================================================================
// 通用：将图像粘贴到 float64 矩阵（alpha 混合 + 裁剪）
// =============================================================================

// pasteRGBWithCenterAndClip 将源图像以 (pasteX, pasteY) 为左上角贴到目标矩阵。
// 超出画框的部分自动裁剪。
func pasteRGBWithCenterAndClip(mats [3][][]float64, matW, matH int, src image.Image, pasteX, pasteY int, alpha float64) {
	b := src.Bounds()
	srcW := b.Dx()
	srcH := b.Dy()

	for sy := 0; sy < srcH; sy++ {
		matY := pasteY + sy
		if matY < 0 || matY >= matH {
			continue
		}
		for sx := 0; sx < srcW; sx++ {
			matX := pasteX + sx
			if matX < 0 || matX >= matW {
				continue
			}
			r, g, bv, a := src.At(sx+b.Min.X, sy+b.Min.Y).RGBA()
			srcAlpha := float64(a>>8) / 255.0 * alpha
			if srcAlpha <= 0 {
				continue
			}

			srcR := float64(r>>8) * srcAlpha
			srcG := float64(g>>8) * srcAlpha
			srcB := float64(bv>>8) * srcAlpha
			invA := 1 - srcAlpha

			mats[0][matY][matX] = mats[0][matY][matX]*invA + srcR
			mats[1][matY][matX] = mats[1][matY][matX]*invA + srcG
			mats[2][matY][matX] = mats[2][matY][matX]*invA + srcB
		}
	}
}

// =============================================================================
// 图像缩放（双线性插值）
// =============================================================================

func resizeImage(src image.Image, dstW, dstH int) image.Image {
	srcBounds := src.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()

	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	xRatio := float64(srcW) / float64(dstW)
	yRatio := float64(srcH) / float64(dstH)

	for dy := 0; dy < dstH; dy++ {
		for dx := 0; dx < dstW; dx++ {
			sx := (float64(dx) + 0.5) * xRatio
			sy := (float64(dy) + 0.5) * yRatio
			c := bilinearSample(src, sx, sy)
			dst.SetRGBA(dx, dy, c)
		}
	}
	return dst
}

// =============================================================================
// 调试/预览
// =============================================================================

// matrixToImage 将 float64 矩阵（值域 [0,255]）转换为灰度图像。
func matrixToImage(mat [][]float64, w, h int) image.Image {
	img := image.NewGray(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetGray(x, y, color.Gray{Clamp255(mat[y][x])})
		}
	}
	return img
}

// SaveMagNormAsPNG 保存归一化幅度图为 PNG（调试/预览用）。
func SaveMagNormAsPNG(spec *SpectrumData, path string) error {
	img := matrixToImage(spec.MagNorm, spec.Width, spec.Height)
	return SavePNG(img, path)
}
