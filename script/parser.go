package script

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// Command 表示解析后的一条脚本命令。
type Command struct {
	Name  string
	Flags map[string]string
	Args  []string
	Line  int
}

// ParseScript 解析 .fte 脚本文件，返回命令列表。
// 规则：
//   - 空行忽略
//   - 以 # 开头的行是注释
//   - 每行一条命令，格式：cmd -flag value ... positional
//   - 双引号或单引号内的内容视为一个 token
func ParseScript(filePath string) ([]Command, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var cmds []Command
	scanner := bufio.NewScanner(f)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		cmd, err := ParseLine(line, lineNo)
		if err != nil {
			return nil, fmt.Errorf("第 %d 行: %w", lineNo, err)
		}
		cmds = append(cmds, cmd)
	}
	return cmds, scanner.Err()
}

// ParseLine 解析单行命令（供 REPL 使用）。
func ParseLine(line string, lineNo int) (Command, error) {
	tokens := tokenize(line)
	if len(tokens) == 0 {
		return Command{}, fmt.Errorf("空命令")
	}

	cmd := Command{
		Name:  tokens[0],
		Flags: make(map[string]string),
		Line:  lineNo,
	}

	i := 1
	for i < len(tokens) {
		tok := tokens[i]
		if strings.HasPrefix(tok, "-") && len(tok) > 1 && tok != "-" && tok != "--" && !isNegNum(tok) {
			flag := tok
			i++
			if flag == "--" {
				break
			}
			if i < len(tokens) && (!strings.HasPrefix(tokens[i], "-") || isNegNum(tokens[i])) {
				cmd.Flags[flag] = tokens[i]
				i++
			} else {
				cmd.Flags[flag] = "true"
			}
		} else {
			cmd.Args = append(cmd.Args, tok)
			i++
		}
	}
	return cmd, nil
}

// isNegNum 判断 token 是否为负数（如 -30、-1.5），而不是 flag。
func isNegNum(s string) bool {
	if len(s) < 2 || s[0] != '-' {
		return false
	}
	return s[1] >= '0' && s[1] <= '9'
}

// tokenize 将一行文本切分为 token，支持双引号和单引号。
func tokenize(line string) []string {
	var tokens []string
	i := 0
	runes := []rune(line)
	n := len(runes)

	for i < n {
		for i < n && (runes[i] == ' ' || runes[i] == '\t') {
			i++
		}
		if i >= n {
			break
		}

		if runes[i] == '"' || runes[i] == '\'' {
			quote := runes[i]
			i++
			var sb strings.Builder
			for i < n && runes[i] != quote {
				sb.WriteRune(runes[i])
				i++
			}
			if i < n {
				i++
			}
			tokens = append(tokens, sb.String())
		} else {
			start := i
			for i < n && runes[i] != ' ' && runes[i] != '\t' {
				i++
			}
			tokens = append(tokens, string(runes[start:i]))
		}
	}
	return tokens
}
