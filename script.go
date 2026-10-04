package main

import (
	"fmt"
	"image/color"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/madelynnblue/go-dsp/fft"
)

// Engine 管理 FTE 脚本的执行状态。
// 流程：load → (t|i)* → save
type Engine struct {
	inputPath  string
	outputPath string
	edits      []editOp
	spectra    [3]*spectrumData
	imgW       int
	imgH       int
	loaded     bool

	// 命令行的 line 号映射（用于错误提示）
	cmdLine map[*editOp]int
}

func newEngine() *Engine {
	return &Engine{
		cmdLine: make(map[*editOp]int),
	}
}

// =============================================================================
// 命令执行入口
// =============================================================================

// Execute 执行解析后的命令列表。
func (eng *Engine) Execute(cmds []Command) error {
	for _, cmd := range cmds {
		if err := eng.exec(&cmd); err != nil {
			return fmt.Errorf("第 %d 行 (%s): %w", cmd.Line, cmd.Name, err)
		}
	}
	return nil
}

func (eng *Engine) exec(cmd *Command) error {
	switch cmd.Name {
	case "load":
		return eng.doLoad(cmd)
	case "t":
		return eng.doText(cmd)
	case "i":
		return eng.doImage(cmd)
	case "save":
		return eng.doSave(cmd)
	case "debug":
		return eng.doDebug(cmd)
	default:
		return fmt.Errorf("未知命令 %q", cmd.Name)
	}
}

// =============================================================================
// load -i <path>
// =============================================================================

func (eng *Engine) doLoad(cmd *Command) error {
	if eng.loaded {
		return fmt.Errorf("load 只能执行一次")
	}
	path, ok := cmd.Flags["-i"]
	if !ok {
		return fmt.Errorf("load 需要 -i <路径>")
	}
	fmt.Printf("[load] 加载 %q\n", path)

	img, err := loadImage(path)
	if err != nil {
		return fmt.Errorf("加载图片失败: %w", err)
	}
	b := img.Bounds()
	eng.imgW, eng.imgH = b.Dx(), b.Dy()
	fmt.Printf("       尺寸 %dx%d\n", eng.imgW, eng.imgH)

	// 三通道并行 FFT
	var wg sync.WaitGroup
	for ch := 0; ch < 3; ch++ {
		wg.Add(1)
		go func(ch int) {
			defer wg.Done()
			mat := extractChannel(img, ch)
			spec := fft.FFT2Real(mat)
			fftShiftComplex(spec)
			eng.spectra[ch] = newSpectrumData(spec)
		}(ch)
	}
	wg.Wait()

	eng.inputPath = path
	eng.loaded = true
	return nil
}

// =============================================================================
// t -x <int> -y <int> -s <float> [-c <color>] [-bg <color>] [-a <float>]
//
//	[-r <angle>] [-f <font>] <text>
//
// =============================================================================

func (eng *Engine) doText(cmd *Command) error {
	if !eng.loaded {
		return fmt.Errorf("请先执行 load")
	}
	if len(cmd.Args) < 1 {
		return fmt.Errorf("t 需要文字内容作为位置参数")
	}
	text := cmd.Args[0]

	cx, err := flagInt(cmd, "-x", 0)
	if err != nil {
		return fmt.Errorf("t 需要 -x <中心x>")
	}
	cy, err := flagInt(cmd, "-y", 0)
	if err != nil {
		return fmt.Errorf("t 需要 -y <中心y>")
	}
	size, _ := flagFloat(cmd, "-s", 24)
	fgColor := parseColorFlag(cmd, "-c", color.RGBA{255, 255, 255, 255})
	bgColor := parseColorFlag(cmd, "-bg", color.RGBA{0, 0, 0, 0})
	if cmd.Flags["-bg"] == "none" || cmd.Flags["-bg"] == "None" {
		bgColor = color.RGBA{0, 0, 0, 0}
	}
	alpha, _ := flagFloat(cmd, "-a", 1.0)
	angle, _ := flagFloat(cmd, "-r", 0)
	font := cmd.Flags["-f"]

	// 检查坐标是否在画框内
	if cx < 0 || cx > eng.imgW || cy < 0 || cy > eng.imgH {
		fmt.Printf("[警告] 文字中心 (%d,%d) 超出画框 (%dx%d)，该文字可能完全被裁剪\n",
			cx, cy, eng.imgW, eng.imgH)
	}

	edit := &textEdit{
		text:    text,
		cx:      cx,
		cy:      cy,
		size:    size,
		font:    font,
		fgColor: fgColor,
		bgColor: bgColor,
		alpha:   alpha,
		angle:   angle,
	}
	eng.edits = append(eng.edits, edit)
	eng.cmdLine[&eng.edits[len(eng.edits)-1]] = cmd.Line

	fmt.Printf("[t]    文字 %q 中心(%d,%d) 字号%.0f\n", text, cx, cy, size)
	return nil
}

// =============================================================================
// i -x <int> -y <int> [-s <float>] [-a <float>] [-r <angle>] <path>
// =============================================================================

func (eng *Engine) doImage(cmd *Command) error {
	if !eng.loaded {
		return fmt.Errorf("请先执行 load")
	}
	if len(cmd.Args) < 1 {
		return fmt.Errorf("i 需要图片路径作为位置参数")
	}
	imgPath := cmd.Args[0]

	cx, err := flagInt(cmd, "-x", 0)
	if err != nil {
		return fmt.Errorf("i 需要 -x <中心x>")
	}
	cy, err := flagInt(cmd, "-y", 0)
	if err != nil {
		return fmt.Errorf("i 需要 -y <中心y>")
	}
	scale, _ := flagFloat(cmd, "-s", 1.0)
	alpha, _ := flagFloat(cmd, "-a", 1.0)
	angle, _ := flagFloat(cmd, "-r", 0)

	// 检查文件存在
	if _, err := os.Stat(imgPath); err != nil {
		return fmt.Errorf("贴图文件不存在: %q", imgPath)
	}

	edit := &imageEdit{
		path:  imgPath,
		cx:    cx,
		cy:    cy,
		scale: scale,
		alpha: alpha,
		angle: angle,
	}
	eng.edits = append(eng.edits, edit)
	eng.cmdLine[&eng.edits[len(eng.edits)-1]] = cmd.Line

	fmt.Printf("[i]    贴图 %q 中心(%d,%d) 缩放%.2f\n", imgPath, cx, cy, scale)
	return nil
}

// =============================================================================
// save -o <path>
// =============================================================================

func (eng *Engine) doSave(cmd *Command) error {
	if !eng.loaded {
		return fmt.Errorf("请先执行 load")
	}
	output := cmd.Flags["-o"]
	if output == "" {
		output = "fte_out.png"
	}
	eng.outputPath = output

	fmt.Printf("\n执行 %d 个编辑操作...\n", len(eng.edits))
	start := time.Now()

	for _, e := range eng.edits {
		fmt.Printf("  - %s\n", e.name())
	}

	// 1. 收集三通道矩阵，编辑一次写入全部通道
	allMags := [3][][]float64{
		eng.spectra[0].magNorm,
		eng.spectra[1].magNorm,
		eng.spectra[2].magNorm,
	}
	for _, e := range eng.edits {
		e.apply(allMags, eng.imgW, eng.imgH)
	}

	// 2. 各通道并行：反向归一化 + 重建 + IFFT
	var restored [3][][]float64
	var wg sync.WaitGroup
	for ch := 0; ch < 3; ch++ {
		wg.Add(1)
		go func(ch int) {
			defer wg.Done()
			s := eng.spectra[ch]
			reverseLogCompress(s.magNorm, s.magMax)
			s.complex = reconstructSpectrum(s.magNorm, s.ph)
			fftShiftComplex(s.complex)
			res := fft.IFFT2(s.complex)
			matData := make([]float64, eng.imgH*eng.imgW)
			mat := make([][]float64, eng.imgH)
			for y := 0; y < eng.imgH; y++ {
				mat[y] = matData[y*eng.imgW : (y+1)*eng.imgW]
				for x := 0; x < eng.imgW; x++ {
					mat[y][x] = real(res[y][x])
				}
			}
			restored[ch] = mat
		}(ch)
	}
	wg.Wait()

	// 3. 合并输出
	outImg := mergeChannels(restored[0], restored[1], restored[2], eng.imgW, eng.imgH)
	if err := savePNG(outImg, output); err != nil {
		return fmt.Errorf("保存输出失败: %w", err)
	}
	fmt.Printf("[save] 输出 -> %q (%dx%d, 耗时 %v)\n", output, eng.imgW, eng.imgH,
		time.Since(start).Round(time.Millisecond))
	return nil
}

// =============================================================================
// debug -inter
// =============================================================================

func (eng *Engine) doDebug(cmd *Command) error {
	if !eng.loaded {
		return fmt.Errorf("请先执行 load")
	}
	if cmd.Flags["-inter"] == "true" {
		prefix := strings.TrimSuffix(eng.outputPath, filepath.Ext(eng.outputPath))
		if prefix == "" {
			prefix = "debug"
		}
		for ch := 0; ch < 3; ch++ {
			saveMagNormAsPNG(eng.spectra[ch], fmt.Sprintf("%s_ch%d.png", prefix, ch))
		}
		fmt.Printf("[debug] 频谱图已保存\n")
	}
	return nil
}

// =============================================================================
// 标志解析辅助
// =============================================================================

func flagInt(cmd *Command, name string, fallback int) (int, error) {
	raw := cmd.Flags[name]
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("标志 %s 的值 %q 不是有效整数", name, raw)
	}
	return v, nil
}

func flagFloat(cmd *Command, name string, fallback float64) (float64, error) {
	raw := cmd.Flags[name]
	if raw == "" {
		return fallback, nil
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("标志 %s 的值 %q 不是有效数字", name, raw)
	}
	return v, nil
}

// parseColorFlag 解析颜色标志，支持 hex 字符串如 "FF0000"、"#FF0000"、"FF000080"。
// 默认值 fallback 在标志未提供时使用。
func parseColorFlag(cmd *Command, name string, fallback color.RGBA) color.RGBA {
	raw := cmd.Flags[name]
	if raw == "" {
		return fallback
	}
	return parseColor(raw, fallback)
}

// truncateText 截断文本用于日志显示。
func truncateText(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "..."
}

// clampFloat 将浮点数限制在 [lo, hi] 区间。
func clampFloat(v, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, v))
}
