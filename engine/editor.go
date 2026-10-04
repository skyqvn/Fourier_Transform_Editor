package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/madelynnblue/go-dsp/fft"
)

// RunFTE 执行完整的 FTE 流程：
// 1. 加载图像并逐通道 FFT
// 2. 对频谱幅度应用所有编辑操作
// 3. 重建频谱并 IFFT 输出
func RunFTE(inputPath, outputPath string, edits []EditOp, saveInter bool) {
	start := time.Now()

	// 1. 加载图像
	img, err := LoadImage(inputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误：加载图片失败：%v\n", err)
		os.Exit(1)
	}
	b := img.Bounds()
	ow, oh := b.Dx(), b.Dy()
	fmt.Printf("输入 %q (%dx%d)\n", inputPath, ow, oh)

	// 2. 三通道并行 FFT + 提取幅度相位
	var spectra [3]*SpectrumData
	var wg sync.WaitGroup
	for ch := 0; ch < 3; ch++ {
		wg.Add(1)
		go func(ch int) {
			defer wg.Done()
			mat := ExtractChannel(img, ch)
			spec := fft.FFT2Real(mat)
			FFTShiftComplex(spec)
			spectra[ch] = NewSpectrumData(spec)
		}(ch)
	}
	wg.Wait()

	// 3. 收集三通道，编辑一次写入全部通道
	fmt.Printf("应用 %d 个编辑操作...\n", len(edits))
	for _, e := range edits {
		fmt.Printf("  - %s\n", e.Name())
	}

	if saveInter {
		prefix := strings.TrimSuffix(outputPath, filepath.Ext(outputPath))
		for ch := 0; ch < 3; ch++ {
			SaveMagNormAsPNG(spectra[ch], fmt.Sprintf("%s_ch%d_before.png", prefix, ch))
		}
	}

	allMags := [3][][]float64{
		spectra[0].MagNorm,
		spectra[1].MagNorm,
		spectra[2].MagNorm,
	}
	for _, e := range edits {
		e.Apply(allMags, ow, oh)
	}

	if saveInter {
		prefix := strings.TrimSuffix(outputPath, filepath.Ext(outputPath))
		for ch := 0; ch < 3; ch++ {
			SaveMagNormAsPNG(spectra[ch], fmt.Sprintf("%s_ch%d_after.png", prefix, ch))
		}
	}

	// 4. 各通道并行：反向归一化 + 重建 + IFFT
	var restored [3][][]float64
	var wg2 sync.WaitGroup
	for ch := 0; ch < 3; ch++ {
		wg2.Add(1)
		go func(ch int) {
			defer wg2.Done()
			s := spectra[ch]
			ReverseLogCompress(s.MagNorm, s.MagMax)
			s.Complex = ReconstructSpectrum(s.MagNorm, s.Ph)
			FFTShiftComplex(s.Complex)
			res := fft.IFFT2(s.Complex)
			matData := make([]float64, oh*ow)
			mat := make([][]float64, oh)
			for y := 0; y < oh; y++ {
				mat[y] = matData[y*ow : (y+1)*ow]
				for x := 0; x < ow; x++ {
					mat[y][x] = real(res[y][x])
				}
			}
			restored[ch] = mat
		}(ch)
	}
	wg2.Wait()

	// 5. 合并通道并输出
	outImg := MergeChannels(restored[0], restored[1], restored[2], ow, oh)
	if err := SavePNG(outImg, outputPath); err != nil {
		fmt.Fprintf(os.Stderr, "错误：保存输出失败：%v\n", err)
		os.Exit(1)
	}
	fmt.Printf("输出 -> %q (%dx%d, 耗时 %v)\n", outputPath, ow, oh, time.Since(start).Round(time.Millisecond))
}
