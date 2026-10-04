package engine

import (
	"fmt"
	"image/color"
	"math"
)

// SpectrumData 持有频谱的完整无损数据，包括原始复数和导出的幅度/相位。
// 编辑操作发生在归一化后的幅度矩阵上，编辑完成后通过反向归一化
// 还原为真实幅度值，再与原始相位合并重建复数频谱。
type SpectrumData struct {
	// 原始复数频谱（shifted，零频在中心）
	Complex [][]complex128

	// 原始幅度和相位（from complex spectrum）
	Mag [][]float64
	Ph  [][]float64

	// 归一化后的幅度矩阵（[0, 255]），编辑操作在此矩阵上进行
	MagNorm [][]float64

	// 原始幅度最大值（用于反向归一化）
	MagMax float64

	Width  int
	Height int
}

// NewSpectrumData 从 FFT 后的复数频谱创建 SpectrumData。
// 频谱应当已经过 FFTShiftComplex 处理（零频在中心）。
func NewSpectrumData(spec [][]complex128) *SpectrumData {
	h := len(spec)
	w := len(spec[0])

	_, _, mag, ph := extractComponents(spec, false, false, true, true)

	// 记录原始幅度最大值
	var mx float64
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if mag[y][x] > mx {
				mx = mag[y][x]
			}
		}
	}
	if mx == 0 {
		mx = 1
	}

	// 创建归一化幅度副本（编辑操作在此上进行）
	magNorm := copyFloat64Matrix(mag, h, w)
	logCompress(magNorm, false)

	return &SpectrumData{
		Complex: spec,
		Mag:     mag,
		Ph:      ph,
		MagNorm: magNorm,
		MagMax:  mx,
		Width:   w,
		Height:  h,
	}
}

// ReverseLogCompress 将对数压缩归一化后的值 [0,255] 反向还原为原始幅度值。
func ReverseLogCompress(norm [][]float64, mx float64) {
	invLog := math.Log(1 + mx)
	for y := range norm {
		for x := range norm[y] {
			v := norm[y][x] / 255.0
			if v < 0 {
				v = 0
			}
			if v > 1 {
				v = 1
			}
			norm[y][x] = math.Exp(v*invLog) - 1
		}
	}
}

// copyFloat64Matrix 创建一个矩阵的深拷贝。
func copyFloat64Matrix(src [][]float64, h, w int) [][]float64 {
	dst := allocFloat64Matrix(h, w, h*w)
	for y := 0; y < h; y++ {
		copy(dst[y], src[y])
	}
	return dst
}

// =============================================================================
// EditOp 编辑操作接口
// =============================================================================

// EditOp 表示一个频谱编辑操作。
// 编辑操作在归一化后的幅度矩阵（[0,255]）上进行。
// mats[0]=R通道, mats[1]=G通道, mats[2]=B通道
type EditOp interface {
	Apply(mats [3][][]float64, w, h int)
	Name() string
}

// =============================================================================
// TextEdit 文字编辑操作 (命令: t)
// =============================================================================

// TextEdit 在频谱图上写文字。
type TextEdit struct {
	Text    string     // 文字内容
	Cx, Cy  int        // 包围盒中心坐标（像素）
	Size    float64    // 字号（磅）
	Font    string     // 字体路径/名称（空=自动匹配系统字体）
	FgColor color.RGBA // 字体颜色
	BgColor color.RGBA // 背景矩形颜色（A=0 表示无背景）
	Alpha   float64    // 整体不透明度 [0,1]
	Angle   float64    // 旋转角度（度）
}

func (te *TextEdit) Apply(mats [3][][]float64, w, h int) {
	drawTextOnMatrix(mats, w, h, te.Text, te.Cx, te.Cy, te.Size, te.Font, te.FgColor, te.BgColor, te.Angle, te.Alpha)
}

func (te *TextEdit) Name() string {
	return fmt.Sprintf("t %q @(%d,%d) sz=%.0f", te.Text, te.Cx, te.Cy, te.Size)
}

// =============================================================================
// ImageEdit 贴图编辑操作 (命令: i)
// =============================================================================

// ImageEdit 在频谱图上叠加外部图像。
type ImageEdit struct {
	Path   string  // 贴图文件路径
	Cx, Cy int     // 图像中心坐标（像素）
	Scale  float64 // 缩放比例，1.0=原始大小
	Alpha  float64 // 混合透明度 [0,1]
	Angle  float64 // 旋转角度（度）
}

func (ie *ImageEdit) Apply(mats [3][][]float64, w, h int) {
	overlayImageOnMatrix(mats, w, h, ie.Path, ie.Cx, ie.Cy, ie.Scale, ie.Angle, ie.Alpha)
}

func (ie *ImageEdit) Name() string {
	return fmt.Sprintf("i %q @(%d,%d)", ie.Path, ie.Cx, ie.Cy)
}
