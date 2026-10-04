package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"fte/script"
)

const welcomeMsg = `fte — 傅里叶变换编辑器 (Fourier Transform Editor)
交互模式  输入 help 查看帮助  |  exit 退出  |  Ctrl+C 退出
`

func runREPL() {
	fmt.Print(welcomeMsg)
	eng := script.NewEngine()
	scanner := bufio.NewScanner(os.Stdin)
	lineNo := 0

	for {
		fmt.Print(prompt(eng))
		if !scanner.Scan() {
			fmt.Println()
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// REPL 内置命令：exit / quit / q
		if line == "exit" || line == "quit" || line == "q" {
			fmt.Println("再见")
			break
		}

		// help 显示帮助
		if line == "help" || line == "h" || line == "?" {
			printHelp()
			continue
		}

		lineNo++
		cmd, err := script.ParseLine(line, lineNo)
		if err != nil {
			fmt.Fprintf(os.Stderr, "解析错误: %v\n", err)
			continue
		}

		if err := eng.ExecOne(&cmd); err != nil {
			fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		}
	}
}

// prompt 根据当前引擎状态生成提示符。
func prompt(eng *script.Engine) string {
	wd, _ := os.Getwd()
	if eng.Loaded() {
		return fmt.Sprintf("\x1b[32mfte:%s *\x1b[0m> ", shorten(wd, 40))
	}
	return fmt.Sprintf("\x1b[36mfte:%s\x1b[0m> ", shorten(wd, 40))
}

func shorten(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "..." + s[len(s)-n+3:]
}

// =============================================================================
// 导出 Engine.Loaded 供 prompt 使用
// =============================================================================
// 需要在 script/engine.go 中添加 Loaded() 方法：
//   func (eng *Engine) Loaded() bool { return eng.loaded }
