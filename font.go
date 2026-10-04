package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// resolveFont 解析字体路径。支持：
//   - 绝对/相对文件路径（直接使用）
//   - 简短字体名（在系统字体目录中搜索，如 "simhei", "arial", "msyh"）
//
// 返回字体文件路径和可能的错误。
func resolveFont(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("字体名不能为空")
	}

	// 如果已经是有效的文件路径，直接返回
	if _, err := os.Stat(name); err == nil {
		return name, nil
	}

	// 补齐常见扩展名
	candidates := []string{name, name + ".ttf", name + ".otf", name + ".ttc"}

	// 在系统字体目录中搜索
	for _, dir := range systemFontDirs() {
		for _, cand := range candidates {
			base := filepath.Base(cand)
			path := filepath.Join(dir, base)
			if _, err := os.Stat(path); err == nil {
				return path, nil
			}
		}
		// 部分匹配（如 "simhei" 匹配 "simhei.ttf" 或 "SimHei.ttf"）
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		lowerName := strings.ToLower(name)
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			entryLower := strings.ToLower(entry.Name())
			entryNoExt := strings.TrimSuffix(entryLower, filepath.Ext(entryLower))
			if entryNoExt == lowerName {
				return filepath.Join(dir, entry.Name()), nil
			}
		}
	}

	return "", fmt.Errorf("找不到字体 %q，已搜索系统字体目录", name)
}

// systemFontDirs 返回当前平台的系统字体目录列表。
func systemFontDirs() []string {
	switch runtime.GOOS {
	case "windows":
		windir := os.Getenv("WINDIR")
		if windir == "" {
			windir = `C:\Windows`
		}
		return []string{
			filepath.Join(windir, "Fonts"),
			filepath.Join(os.Getenv("LOCALAPPDATA"), "Microsoft", "Windows", "Fonts"),
		}
	case "darwin":
		home, _ := os.UserHomeDir()
		return []string{
			"/System/Library/Fonts",
			"/System/Library/Fonts/Supplemental",
			"/Library/Fonts",
			filepath.Join(home, "Library", "Fonts"),
		}
	default: // linux
		home, _ := os.UserHomeDir()
		return []string{
			"/usr/share/fonts",
			"/usr/local/share/fonts",
			filepath.Join(home, ".fonts"),
			filepath.Join(home, ".local", "share", "fonts"),
		}
	}
}
