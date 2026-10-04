package main

import (
	"math"
	"math/cmplx"
)

// fftShiftComplex 对二维复数频谱进行原地四象限交换，
// 将零频分量移至图像中心。
func fftShiftComplex(spec [][]complex128) {
	h, w := len(spec), len(spec[0])
	hh, hw := h/2, w/2
	for y := 0; y < hh; y++ {
		for x := 0; x < hw; x++ {
			spec[y][x], spec[y+hh][x+hw] = spec[y+hh][x+hw], spec[y][x]
		}
	}
	for y := 0; y < hh; y++ {
		for x := hw; x < w; x++ {
			spec[y][x], spec[y+hh][x-hw] = spec[y+hh][x-hw], spec[y][x]
		}
	}
}

// extractComponents 从复数频谱中按需提取实部、虚部、幅度、相位。
func extractComponents(spec [][]complex128, needRe, needIm, needMag, needPh bool) (re, im, mag, ph [][]float64) {
	h := len(spec)
	w := len(spec[0])
	total := w * h

	if needRe {
		re = allocFloat64Matrix(h, w, total)
	}
	if needIm {
		im = allocFloat64Matrix(h, w, total)
	}
	if needMag {
		mag = allocFloat64Matrix(h, w, total)
	}
	if needPh {
		ph = allocFloat64Matrix(h, w, total)
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := spec[y][x]
			if needRe {
				re[y][x] = real(c)
			}
			if needIm {
				im[y][x] = imag(c)
			}
			if needMag {
				mag[y][x] = cmplx.Abs(c)
			}
			if needPh {
				ph[y][x] = cmplx.Phase(c)
			}
		}
	}
	return
}

// allocFloat64Matrix 分配一个 hXw 的 [][]float64 矩阵，底层使用连续内存。
func allocFloat64Matrix(h, w, total int) [][]float64 {
	data := make([]float64, total)
	m := make([][]float64, h)
	for y := 0; y < h; y++ {
		m[y] = data[y*w : (y+1)*w]
	}
	return m
}

// allocComplex128Matrix 分配一个 hXw 的 [][]complex128 矩阵，底层使用连续内存。
func allocComplex128Matrix(h, w, total int) [][]complex128 {
	data := make([]complex128, total)
	m := make([][]complex128, h)
	for y := 0; y < h; y++ {
		m[y] = data[y*w : (y+1)*w]
	}
	return m
}

// logCompress 对矩阵做对数压缩后归一化到 [0,255]。
// symmetric=true 时保留符号，适用于实部/虚部。
func logCompress(mat [][]float64, symmetric bool) {
	var mx float64
	for y := range mat {
		for x := range mat[y] {
			if a := math.Abs(mat[y][x]); a > mx {
				mx = a
			}
		}
	}
	if mx == 0 {
		mx = 1
	}
	invLog := 1.0 / math.Log(1+mx)
	for y := range mat {
		for x := range mat[y] {
			v := mat[y][x]
			lv := math.Log(1+math.Abs(v)) * invLog
			if symmetric {
				if v < 0 {
					lv = -lv
				}
				mat[y][x] = (lv + 1) / 2 * 255
			} else {
				mat[y][x] = lv * 255
			}
		}
	}
}

func normalizeMag(mag [][]float64) { logCompress(mag, false) }

func normalizePh(ph [][]float64) {
	for y := range ph {
		for x := range ph[y] {
			ph[y][x] = (ph[y][x] + math.Pi) / (2 * math.Pi) * 255
		}
	}
}

// reconstructSpectrum 根据幅度和相位重建复数频谱。
func reconstructSpectrum(mag, ph [][]float64) [][]complex128 {
	h := len(mag)
	w := len(mag[0])
	total := w * h
	spec := allocComplex128Matrix(h, w, total)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			spec[y][x] = cmplx.Rect(mag[y][x], ph[y][x])
		}
	}
	return spec
}
