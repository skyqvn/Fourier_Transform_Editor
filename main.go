package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"

	"fte/engine"
	"fte/script"

	"github.com/madelynnblue/go-dsp/fft"
)

func main() {
	if len(os.Args) < 2 {
		runREPL()
		return
	}

	first := os.Args[1]

	switch {
	case first == "repl":
		runREPL()
	case first == "view":
		cmdView(os.Args[2:])
	case first == "help" || first == "-h" || first == "--help" || first == "h":
		printHelp()
	case first == "run":
		if len(os.Args) < 3 {
			fmt.Fprintln(os.Stderr, "错误: run 需要一个脚本文件路径")
			fmt.Fprintln(os.Stderr, "用法: fte run <script.fte>")
			os.Exit(1)
		}
		runScriptFile(os.Args[2])
	default:
		if strings.HasSuffix(first, ".fte") || fileExists(first) {
			runScriptFile(first)
		} else {
			fmt.Fprintf(os.Stderr, "错误：未知命令 %q\n\n", first)
			printHelp()
			os.Exit(1)
		}
	}
}

func runScriptFile(path string) {
	cmds, err := script.ParseScript(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "脚本解析失败: %v\n", err)
		os.Exit(1)
	}
	if len(cmds) == 0 {
		fmt.Fprintln(os.Stderr, "警告: 脚本中没有有效命令")
		os.Exit(0)
	}

	eng := script.NewEngine()
	if err := eng.Execute(cmds); err != nil {
		fmt.Fprintf(os.Stderr, "执行失败: %v\n", err)
		os.Exit(1)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// =============================================================================
// view 子命令：查看频谱图
// =============================================================================

func cmdView(args []string) {
	fs := flag.NewFlagSet("view", flag.ExitOnError)
	in := fs.String("i", "", "input 输入图片路径")
	out := fs.String("o", "", "output 输出前缀（默认=输入文件名）")
	fs.Parse(args)

	if *in == "" {
		fmt.Fprintln(os.Stderr, "错误：-i 输入路径不能为空")
		os.Exit(1)
	}

	img, err := engine.LoadImage(*in)
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

	var spectra [3]*engine.SpectrumData
	var wg sync.WaitGroup
	for ch := 0; ch < 3; ch++ {
		wg.Add(1)
		go func(ch int) {
			defer wg.Done()
			mat := engine.ExtractChannel(img, ch)
			spec := fft.FFT2Real(mat)
			engine.FFTShiftComplex(spec)
			spectra[ch] = engine.NewSpectrumData(spec)
		}(ch)
	}
	wg.Wait()

	for ch := 0; ch < 3; ch++ {
		engine.SaveMagNormAsPNG(spectra[ch], fmt.Sprintf("%s_ch%d_mag.png", prefix, ch))
	}
	fmt.Printf("频谱图已保存为 %s_ch0_mag.png, %s_ch1_mag.png, %s_ch2_mag.png\n", prefix, prefix, prefix)
}

func fileExt(path string) string {
	parts := strings.Split(path, ".")
	if len(parts) > 1 {
		return parts[len(parts)-1]
	}
	return ""
}

// =============================================================================
// 帮助信息
// =============================================================================

func printHelp() {
	fmt.Print(`fte — 傅里叶变换编辑器 (Fourier Transform Editor)

语法
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  fte                       启动交互终端
  fte repl                  同上
  fte <script.fte>          执行 .fte 脚本文件
  fte run <script.fte>      同上（显式方式）
  fte view -i <img>         查看图片的频谱幅度图
  fte help                  显示此帮助

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
项目结构
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  engine/   核心库（FFT、频谱编辑、文字/图片绘制、字体）
  script/   脚本解析与执行引擎
  main.go   CLI 入口、路由、帮助

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
.fte 脚本命令集
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  格式: 命令 -标志 值 "位置参数"

  命令            描述
  ──────          ──────────────────────────────────
  reset           清空当前状态，放弃编辑并重新开始
  load -i <path>  加载输入图片，执行 FFT
  t               在频谱图上写文字
  i               在频谱图上贴图片
  save -o <path>  应用所有编辑，IFFT 输出 PNG
  debug -inter    保存中间频谱图（调试用）

  流程: load → (t|i)* → save → (reset → load → ...)*

  # 开头的行和空行被忽略

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
写文字命令: t
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  语法: t -x <cx> -y <cy> -s <size> [-c <color>] [-bg <color>] [-a <alpha>] [-r <angle>] [-f <font>] "文字"

  标志        默认值      说明
  ────        ──────      ──────────────────────────
  -x          (必填)      文字包围盒中心 x 坐标
  -y          (必填)      文字包围盒中心 y 坐标
  -s          (必填)      字号（磅，如 24）
  -c          #FFFFFF     字体颜色 hex RGBA
  -bg         none        背景矩形颜色 hex RGBA
                          none = 完全透明
  -a          1.0         整体不透明度 [0-1]
  -r          0           旋转角度（度，逆时针）
  -f          auto        字体名或路径
                          (simhei/msyh/arial/...)

  示例:
    t -x 256 -y 256 -s 32 "Hello"
    t -x 400 -y 300 -s 48 -c #FF0000 -bg #00000080 -a 0.8 -r 45 "水印"
    t -x 100 -y 50 -s 24 -c "00FF00" -f simhei "中文文字"

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
贴图片命令: i
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  语法: i -x <cx> -y <cy> [-s <scale>] [-a <alpha>] [-r <angle>] "图片路径"

  标志        默认值      说明
  ────        ──────      ──────────────────────────
  -x          (必填)      图像中心 x 坐标
  -y          (必填)      图像中心 y 坐标
  -s          1.0         缩放比例
  -a          1.0         整体不透明度 [0-1]
  -r          0           旋转角度（度，逆时针）

  示例:
    i -x 512 -y 384 logo.png
    i -x 512 -y 384 -s 0.5 -a 0.6 -r 30 "wm/logo.png"

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
颜色格式
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  支持 hex RGB / RGBA，带或不带 # 前缀：
    #FFFFFF  FFFFFF    白色，完全不透明
    #FF000080          红色，50% 透明
    FF0000             红色，完全不透明
    none / None        透明（-bg 专用）

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
完整脚本示例 (demo.fte)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  # 加载输入图片
  load -i photo.png

  # 三个文字编辑
  t -x 100 -y 50 -s 24 -c #FFFFFF -bg #00000040 -f msyh "左上角"
  t -x 512 -y 384 -s 48 -c #FF0000 -bg none -a 0.8 -r 45 "中间旋转"
  t -x 900 -y 700 -s 16 -c "00FF00" -r -15 "右下角"

  # 贴图
  i -x 512 -y 200 -s 0.5 -a 0.6 -r 30 logo.png

  # 输出
  save -o output.png

  运行: fte demo.fte

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
查看频谱
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  fte view -i photo.png             输出到 photo_ch0_mag.png 等
  fte view -i photo.png -o spec     输出到 spec_ch0_mag.png 等

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
字体
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  支持系统 TTF/OTF 字体，自动搜索：
    Windows:  %WINDIR%\Fonts\
    macOS:    /System/Library/Fonts/, /Library/Fonts/
    Linux:    /usr/share/fonts/, ~/.fonts/

  -f 可指定字体名（simhei, msyh, arial）或完整路径。
  留空自动尝试 simhei → msyh → arial → helvetica。

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
输入格式
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  PNG / JPEG / GIF / BMP / TIFF / WebP

输出格式: PNG

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
交互终端模式
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
  直接运行 fte（无参数）进入交互终端：

    fte:~/photos> load -i cat.png
    fte:~/photos *> t -x 256 -y 256 -s 32 "hello"
    fte:~/photos *> i -x 512 -y 384 -s 0.5 logo.png
    fte:~/photos *> save -o out.png
    fte:~/photos *> reset
    fte:~/photos> exit

  提示符中 * 表示已加载图片（可进行 t/i/save 操作）。
  内置命令：help / exit / quit / q / cd <path>
`)
}
