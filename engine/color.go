package engine

import (
	"fmt"
	"image/color"
	"os"
	"strconv"
	"strings"
)

// ParseColor 解析 hex 颜色字符串，支持以下格式：
//
//	#RRGGBB    (如 #FF0000)
//	#RRGGBBAA  (如 #FF000080)
//	RRGGBB     (如 FF0000)
//	RRGGBBAA   (如 FF000080)
//	none / -   (返回默认值)
func ParseColor(s string, defaultVal color.RGBA) color.RGBA {
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
