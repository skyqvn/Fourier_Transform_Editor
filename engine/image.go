package engine

import (
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"os"
	"strings"

	_ "golang.org/x/image/bmp"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"
)

// LoadImage 加载任意图片格式。
func LoadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

// SavePNG 将图像保存为 PNG 文件。若路径不含 .png 后缀则自动补齐。
func SavePNG(img image.Image, path string) error {
	if !strings.HasSuffix(strings.ToLower(path), ".png") {
		path = path + ".png"
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// ExtractChannel 从图像中提取单个颜色通道，返回 [][]float64。
func ExtractChannel(img image.Image, ch int) [][]float64 {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()

	data := make([]float64, w*h)
	mat := make([][]float64, h)

	switch src := img.(type) {
	case *image.RGBA:
		extractRGBAPix(data, mat, src.Pix, src.Stride, src.PixOffset(b.Min.X, b.Min.Y), ch, w, h)
		return mat

	case *image.NRGBA:
		extractRGBAPix(data, mat, src.Pix, src.Stride, src.PixOffset(b.Min.X, b.Min.Y), ch, w, h)
		return mat

	case *image.YCbCr:
		extractYCbCrPix(data, mat, src, ch, w, h)
		return mat

	case *image.Gray:
		pix := src.Pix
		stride := src.Stride
		offset := src.PixOffset(b.Min.X, b.Min.Y)
		for y := 0; y < h; y++ {
			mat[y] = data[y*w : (y+1)*w]
			for x := 0; x < w; x++ {
				mat[y][x] = float64(pix[offset+y*stride+x])
			}
		}
		return mat
	}

	for y := 0; y < h; y++ {
		mat[y] = data[y*w : (y+1)*w]
		for x := 0; x < w; x++ {
			r, g, bb, _ := img.At(x+b.Min.X, y+b.Min.Y).RGBA()
			var v uint32
			switch ch {
			case 0:
				v = r
			case 1:
				v = g
			case 2:
				v = bb
			}
			mat[y][x] = float64(v >> 8)
		}
	}
	return mat
}

func extractRGBAPix(data []float64, mat [][]float64, pix []byte, stride, pixOff, ch, w, h int) {
	for y := 0; y < h; y++ {
		mat[y] = data[y*w : (y+1)*w]
		row := pix[pixOff+y*stride:]
		for x := 0; x < w; x++ {
			mat[y][x] = float64(row[x*4+ch])
		}
	}
}

func extractYCbCrPix(data []float64, mat [][]float64, src *image.YCbCr, ch, w, h int) {
	cw, chDiv := ycbcrSubsampleRatio(src.SubsampleRatio)
	for y := 0; y < h; y++ {
		mat[y] = data[y*w : (y+1)*w]
		for x := 0; x < w; x++ {
			yi := y*src.YStride + x
			ci := (y/chDiv)*src.CStride + x/cw
			yy, cb, cr := src.Y[yi], src.Cb[ci], src.Cr[ci]
			rr, gg, bb := color.YCbCrToRGB(yy, cb, cr)
			mat[y][x] = float64(pickChannel(rr, gg, bb, ch))
		}
	}
}

func ycbcrSubsampleRatio(ratio image.YCbCrSubsampleRatio) (cw, chDiv int) {
	switch ratio {
	case image.YCbCrSubsampleRatio444:
		return 1, 1
	case image.YCbCrSubsampleRatio422:
		return 2, 1
	case image.YCbCrSubsampleRatio420:
		return 2, 2
	case image.YCbCrSubsampleRatio440:
		return 1, 2
	case image.YCbCrSubsampleRatio411:
		return 4, 1
	default:
		return 2, 2
	}
}

func pickChannel(r, g, b uint8, ch int) uint8 {
	switch ch {
	case 0:
		return r
	case 1:
		return g
	default:
		return b
	}
}

// MergeChannels 将三通道 [][]float64 合并为 *image.RGBA。
func MergeChannels(r, g, b [][]float64, w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	pix := img.Pix
	stride := img.Stride
	for y := 0; y < h; y++ {
		row := pix[y*stride : y*stride+w*4]
		ry, gy, by := r[y], g[y], b[y]
		for x := 0; x < w; x++ {
			off := x * 4
			row[off+0] = Clamp255(ry[x])
			row[off+1] = Clamp255(gy[x])
			row[off+2] = Clamp255(by[x])
			row[off+3] = 255
		}
	}
	return img
}

// Clamp255 将 float64 值钳制到 [0, 255] 并转为 uint8。
func Clamp255(v float64) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}
