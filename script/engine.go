package script

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

	"fte/engine"

	"github.com/madelynnblue/go-dsp/fft"
)

// Engine 管理 FTE 脚本的执行状态。
// 流程：load → (t|i)* → save
type Engine struct {
	inputPath  string
	outputPath string
	edits      []engine.EditOp
	spectra    [3]*engine.SpectrumData
	imgW       int
	imgH       int
	loaded     bool
	cmdLine    map[engine.EditOp]int
}

// NewEngine 创建新的脚本执行引擎。
func NewEngine() *Engine {
	return &Engine{
		cmdLine: make(map[engine.EditOp]int),
	}
}

// Loaded 返回当前是否已加载图片。
func (eng *Engine) Loaded() bool { return eng.loaded }

// Execute 执行解析后的命令列表。
func (eng *Engine) Execute(cmds []Command) error {
	for _, cmd := range cmds {
		if err := eng.ExecOne(&cmd); err != nil {
			return fmt.Errorf("第 %d 行 (%s): %w", cmd.Line, cmd.Name, err)
		}
	}
	return nil
}

// ExecOne 执行单条命令（供 REPL 使用）。
// 返回 error 表示执行失败；对于 cd/exit，调用方需自行处理。
func (eng *Engine) ExecOne(cmd *Command) error {
	switch cmd.Name {
	case "reset":
		return eng.doReset(cmd)
	case "cd":
		return eng.doChdir(cmd)
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
// cd <path> — 切换工作目录
// =============================================================================

func (eng *Engine) doChdir(cmd *Command) error {
	dir := "."
	if len(cmd.Args) > 0 {
		dir = cmd.Args[0]
	} else if v, ok := cmd.Flags["-d"]; ok {
		dir = v
	}
	if err := os.Chdir(dir); err != nil {
		return fmt.Errorf("cd: %w", err)
	}
	wd, _ := os.Getwd()
	fmt.Printf("[cd] %s\n", wd)
	return nil
}

// =============================================================================
// reset — 清空当前状态，允许重新 load
// =============================================================================

func (eng *Engine) doReset(_ *Command) error {
	eng.inputPath = ""
	eng.outputPath = ""
	eng.edits = nil
	eng.spectra = [3]*engine.SpectrumData{}
	eng.imgW = 0
	eng.imgH = 0
	eng.loaded = false
	fmt.Printf("[reset] 状态已清空，可以重新 load\n")
	return nil
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

	img, err := engine.LoadImage(path)
	if err != nil {
		return fmt.Errorf("加载图片失败: %w", err)
	}
	b := img.Bounds()
	eng.imgW, eng.imgH = b.Dx(), b.Dy()
	fmt.Printf("       尺寸 %dx%d\n", eng.imgW, eng.imgH)

	var wg sync.WaitGroup
	for ch := 0; ch < 3; ch++ {
		wg.Add(1)
		go func(ch int) {
			defer wg.Done()
			mat := engine.ExtractChannel(img, ch)
			spec := fft.FFT2Real(mat)
			engine.FFTShiftComplex(spec)
			eng.spectra[ch] = engine.NewSpectrumData(spec)
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

	if cx < 0 || cx > eng.imgW || cy < 0 || cy > eng.imgH {
		fmt.Printf("[警告] 文字中心 (%d,%d) 超出画框 (%dx%d)，该文字可能完全被裁剪\n",
			cx, cy, eng.imgW, eng.imgH)
	}

	edit := &engine.TextEdit{
		Text:    text,
		Cx:      cx,
		Cy:      cy,
		Size:    size,
		Font:    font,
		FgColor: fgColor,
		BgColor: bgColor,
		Alpha:   alpha,
		Angle:   angle,
	}
	eng.edits = append(eng.edits, edit)
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

	if _, err := os.Stat(imgPath); err != nil {
		return fmt.Errorf("贴图文件不存在: %q", imgPath)
	}

	edit := &engine.ImageEdit{
		Path:  imgPath,
		Cx:    cx,
		Cy:    cy,
		Scale: scale,
		Alpha: alpha,
		Angle: angle,
	}
	eng.edits = append(eng.edits, edit)
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
		fmt.Printf("  - %s\n", e.Name())
	}

	allMags := [3][][]float64{
		eng.spectra[0].MagNorm,
		eng.spectra[1].MagNorm,
		eng.spectra[2].MagNorm,
	}
	for _, e := range eng.edits {
		e.Apply(allMags, eng.imgW, eng.imgH)
	}

	// 若指定 -spectrum，保存编辑后的频谱幅度图
	if cmd.Flags["-spectrum"] == "true" {
		prefix := strings.TrimSuffix(output, filepath.Ext(output))
		for ch := 0; ch < 3; ch++ {
			engine.SaveMagNormAsPNG(eng.spectra[ch], fmt.Sprintf("%s_spectrum_ch%d.png", prefix, ch))
		}
		fmt.Printf("[save] 编辑后频谱已保存 (IFFT前)\n")
	}

	var restored [3][][]float64
	var wg sync.WaitGroup
	for ch := 0; ch < 3; ch++ {
		wg.Add(1)
		go func(ch int) {
			defer wg.Done()
			s := eng.spectra[ch]
			engine.ReverseLogCompress(s.MagNorm, s.MagMax)
			s.Complex = engine.ReconstructSpectrum(s.MagNorm, s.Ph)
			engine.FFTShiftComplex(s.Complex)
			res := fft.IFFT2(s.Complex)
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

	outImg := engine.MergeChannels(restored[0], restored[1], restored[2], eng.imgW, eng.imgH)
	if err := engine.SavePNG(outImg, output); err != nil {
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
			engine.SaveMagNormAsPNG(eng.spectra[ch], fmt.Sprintf("%s_ch%d.png", prefix, ch))
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

func parseColorFlag(cmd *Command, name string, fallback color.RGBA) color.RGBA {
	raw := cmd.Flags[name]
	if raw == "" {
		return fallback
	}
	return engine.ParseColor(raw, fallback)
}

func truncateText(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "..."
}

func clampFloat(v, lo, hi float64) float64 {
	return math.Max(lo, math.Min(hi, v))
}
