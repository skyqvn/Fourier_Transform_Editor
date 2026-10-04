package main

import (
	"flag"
	"fmt"
	"image/color"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/madelynnblue/go-dsp/fft"
)

// =============================================================================
// edit 子命令：编辑频谱并输出
// =============================================================================

func cmdEdit(args []string) {
	fs := flag.NewFlagSet("edit", flag.ExitOnError)
	in := fs.String("i", "", "input 输入图片路径")
	out := fs.String("o", "fte_out.png", "output 输出 PNG 文件路径")
	saveInter := fs.Bool("save-inter", false, "保存中间频谱图（编辑前/后），用于调试")
	fs.Parse(args)

	if *in == "" {
		fmt.Fprintln(os.Stderr, "错误：-i 输入路径不能为空")
		fs.Usage()
		os.Exit(1)
	}

	// 收集编辑操作（从 -e 参数或位置参数）
	edits := collectEdits(fs.Args())

	if len(edits) == 0 {
		fmt.Fprintln(os.Stderr, "错误：至少需要一个编辑操作（-e 参数）")
		os.Exit(1)
	}

	runFTE(*in, *out, edits, *saveInter)
}

// =============================================================================
// 编辑操作解析（pipe 分隔格式）
// =============================================================================

// collectEdits 从参数中收集编辑操作。
// 格式规范（用 | 分隔各字段）：
//
//	文字: t|<text>|<cx>|<cy>|<size>|<fgcolor>|<bgcolor>|<alpha>|<angle>|<font>
//	图片: i|<path>|<cx>|<cy>|<scale>|<alpha>|<angle>
//
// 支持通过 -e 前缀传入：-e t|hello|100|200|24|#FFFFFF|none|1.0|0|simhei
// 或作为位置参数。
func collectEdits(args []string) []editOp {
	var edits []editOp

	for _, arg := range args {
		arg = strings.TrimPrefix(arg, "-e=")
		arg = strings.TrimPrefix(arg, "-e")

		if arg == "" {
			continue
		}

		parts := strings.SplitN(arg, "|", 2)
		if len(parts) < 2 {
			fmt.Fprintf(os.Stderr, "警告：无法解析编辑参数 %q（需要 | 分隔），跳过\n", arg)
			continue
		}

		cmd := strings.TrimSpace(parts[0])
		rest := parts[1]

		switch cmd {
		case "t":
			if e := parseTextEdit(rest); e != nil {
				edits = append(edits, e)
			}
		case "i":
			if e := parseImageEdit(rest); e != nil {
				edits = append(edits, e)
			}
		default:
			fmt.Fprintf(os.Stderr, "警告：未知命令 %q（仅支持 t/i），跳过\n", cmd)
		}
	}

	return edits
}

// parseTextEdit 解析 t 命令。
// 格式: <text>|<cx>|<cy>|<size>|<fgcolor>|<bgcolor>|<alpha>|<angle>|<font>
//
//	fgcolor: 字体颜色，hex RGB(A) 如 #FFFFFF, FFFFFF80，默认 #FFFFFF
//	bgcolor: 背景颜色，hex RGBA 或 "none"，默认 none（完全透明）
//	alpha:   整体不透明度 [0,1]，默认 1.0
//	angle:   旋转角度（度），默认 0
//	font:    字体路径/名称，默认空（自动匹配系统字体）
func parseTextEdit(params string) *textEdit {
	parts := strings.SplitN(params, "|", 9)
	if len(parts) < 4 {
		fmt.Fprintf(os.Stderr, "警告：t 命令参数不足（需要 text|cx|cy|size[...]）：%q\n", params)
		return nil
	}

	text := parts[0]
	if text == "" {
		fmt.Fprintln(os.Stderr, "警告：t 命令文字内容为空，跳过")
		return nil
	}

	cx, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		fmt.Fprintf(os.Stderr, "警告：t cx 坐标无效：%q\n", parts[1])
		return nil
	}
	cy, err := strconv.Atoi(strings.TrimSpace(parts[2]))
	if err != nil {
		fmt.Fprintf(os.Stderr, "警告：t cy 坐标无效：%q\n", parts[2])
		return nil
	}
	size, err := strconv.ParseFloat(strings.TrimSpace(parts[3]), 64)
	if err != nil || size <= 0 {
		fmt.Fprintf(os.Stderr, "警告：t size 无效：%q\n", parts[3])
		return nil
	}

	// 默认值
	fgColor := color.RGBA{255, 255, 255, 255} // 白色
	bgColor := color.RGBA{0, 0, 0, 0}         // 完全透明
	alpha := 1.0
	angle := 0.0
	font := ""

	if len(parts) >= 5 {
		fgColor = parseColor(parts[4], color.RGBA{255, 255, 255, 255})
	}
	if len(parts) >= 6 {
		if strings.TrimSpace(strings.ToLower(parts[5])) == "none" {
			bgColor = color.RGBA{0, 0, 0, 0}
		} else {
			bgColor = parseColor(parts[5], color.RGBA{0, 0, 0, 0})
		}
	}
	if len(parts) >= 7 {
		a, err := strconv.ParseFloat(strings.TrimSpace(parts[6]), 64)
		if err == nil && a >= 0 && a <= 1 {
			alpha = a
		}
	}
	if len(parts) >= 8 {
		ang, err := strconv.ParseFloat(strings.TrimSpace(parts[7]), 64)
		if err == nil {
			angle = ang
		}
	}
	if len(parts) >= 9 {
		font = strings.TrimSpace(parts[8])
	}

	return &textEdit{
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
}

// parseImageEdit 解析 i 命令。
// 格式: <path>|<cx>|<cy>|<scale>|<alpha>|<angle>
func parseImageEdit(params string) *imageEdit {
	parts := strings.SplitN(params, "|", 6)
	if len(parts) < 4 {
		fmt.Fprintf(os.Stderr, "警告：i 命令参数不足（需要 path|cx|cy|scale[...]）：%q\n", params)
		return nil
	}

	path := strings.TrimSpace(parts[0])
	if path == "" {
		fmt.Fprintln(os.Stderr, "警告：i 命令图片路径为空，跳过")
		return nil
	}

	cx, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		fmt.Fprintf(os.Stderr, "警告：i cx 坐标无效：%q\n", parts[1])
		return nil
	}
	cy, err := strconv.Atoi(strings.TrimSpace(parts[2]))
	if err != nil {
		fmt.Fprintf(os.Stderr, "警告：i cy 坐标无效：%q\n", parts[2])
		return nil
	}
	scale, err := strconv.ParseFloat(strings.TrimSpace(parts[3]), 64)
	if err != nil || scale <= 0 {
		fmt.Fprintf(os.Stderr, "警告：i scale 无效：%q，使用默认 1.0\n", parts[3])
		scale = 1.0
	}

	alpha := 1.0
	angle := 0.0

	if len(parts) >= 5 {
		a, err := strconv.ParseFloat(strings.TrimSpace(parts[4]), 64)
		if err == nil && a >= 0 && a <= 1 {
			alpha = a
		}
	}
	if len(parts) >= 6 {
		ang, err := strconv.ParseFloat(strings.TrimSpace(parts[5]), 64)
		if err == nil {
			angle = ang
		}
	}

	return &imageEdit{
		path:  path,
		cx:    cx,
		cy:    cy,
		scale: scale,
		alpha: alpha,
		angle: angle,
	}
}

// =============================================================================
// 颜色解析
// =============================================================================

// parseColor 解析 hex 颜色字符串，支持以下格式：
//
//	#RRGGBB    (如 #FF0000)
//	#RRGGBBAA  (如 #FF000080)
//	RRGGBB     (如 FF0000)
//	RRGGBBAA   (如 FF000080)
//	none / -   (返回默认值)
func parseColor(s string, defaultVal color.RGBA) color.RGBA {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "#")

	lower := strings.ToLower(s)
	if lower == "none" || lower == "-" || s == "" {
		return defaultVal
	}

	if len(s) == 6 {
		// RRGGBB -> RGBA (alpha=255)
		r := hexByte(s[0:2])
		g := hexByte(s[2:4])
		b := hexByte(s[4:6])
		return color.RGBA{r, g, b, 255}
	}
	if len(s) == 8 {
		// RRGGBBAA
		r := hexByte(s[0:2])
		g := hexByte(s[2:4])
		b := hexByte(s[4:6])
		a := hexByte(s[6:8])
		return color.RGBA{r, g, b, a}
	}

	fmt.Fprintf(os.Stderr, "警告：颜色格式无效 %q，使用默认值\n", s)
	return defaultVal
}

func hexByte(s string) uint8 {
	v, err := strconv.ParseUint(s, 16, 8)
	if err != nil {
		return 0
	}
	return uint8(v)
}

// =============================================================================
// view 子命令：查看频谱图
// =============================================================================

func cmdViewSpec(args []string) {
	fs := flag.NewFlagSet("view", flag.ExitOnError)
	in := fs.String("i", "", "input 输入图片路径")
	out := fs.String("o", "", "output 输出前缀（默认=输入文件名）")
	fs.Parse(args)

	if *in == "" {
		fmt.Fprintln(os.Stderr, "错误：-i 输入路径不能为空")
		os.Exit(1)
	}

	img, err := loadImage(*in)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误：加载图片失败：%v\n", err)
		os.Exit(1)
	}

	b := img.Bounds()
	ow, oh := b.Dx(), b.Dy()
	fmt.Printf("输入 %dx%d\n", ow, oh)

	prefix := *out
	if prefix == "" {
		prefix = strings.TrimSuffix(*in, "."+strings.ToLower(fileExt(*in)))
	}

	var spectra [3]*spectrumData
	var wg sync.WaitGroup
	for ch := 0; ch < 3; ch++ {
		wg.Add(1)
		go func(ch int) {
			defer wg.Done()
			mat := extractChannel(img, ch)
			spec := fft.FFT2Real(mat)
			fftShiftComplex(spec)
			spectra[ch] = newSpectrumData(spec)
		}(ch)
	}
	wg.Wait()

	for ch := 0; ch < 3; ch++ {
		saveMagNormAsPNG(spectra[ch], fmt.Sprintf("%s_ch%d_mag.png", prefix, ch))
	}
	fmt.Printf("频谱图已保存为 %s_ch0_mag.png, %s_ch1_mag.png, %s_ch2_mag.png\n", prefix, prefix, prefix)
}

func fileExt_old(path string) string {
	parts := strings.Split(path, ".")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return ""
}
