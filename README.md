# fte — 傅里叶变换编辑器 (Fourier Transform Editor)

`fte` 是一个命令行工具，通过在图像的**频域（频谱）**中嵌入文字和图片，再通过逆傅里叶变换（IFFT）还原到空间域，实现隐蔽的信息嵌入效果。

## 工作原理

```
原始图片 ──→ FFT（正变换）──→ 频谱图 ──→ 在频谱上嵌入文字/图片 ──→ IFFT（逆变换）──→ 输出图片
```

在频域中修改像素会影响整个空间域的图像，因此嵌入的内容会以不可见或半可见的方式"融入"原图，类似于频域水印。

## 项目结构

```
fte/
├── main.go          # CLI 入口、路由、帮助信息
├── repl.go          # 交互式 REPL 终端
├── help.go          # 帮助文案
├── engine/          # 核心引擎库
│   ├── color.go     # 颜色解析（hex RGBA）
│   ├── draw.go      # 频谱绘制（文字/图片混合）
│   ├── editor.go    # 编辑操作定义（文字编辑、图片编辑）
│   ├── fft.go       # FFT/IFFT 工具函数
│   ├── font.go      # 跨平台字体加载
│   ├── image.go     # 图片加载/保存
│   └── spectrum.go  # 频谱数据结构与归一化
├── script/          # 脚本引擎
│   ├── parser.go    # .fte 脚本解析器
│   └── engine.go    # 脚本执行引擎
└── demo/            # 示例文件
    ├── demo.fte     # 示例脚本
    ├── demo.png     # 输入示例图
    ├── partten.png  # 嵌入用图案
    ├── demo_out.png # 输出结果
    ├── demo_out_mag.png   # 频谱幅度图
    └── demo_out_phase.png # 频谱相位图
```

## 安装

### 前置条件

- Go 1.26+

### 编译

```bash
go build -o fte .
```

## 使用方式

### 1. 执行脚本文件

```bash
fte demo/demo.fte
# 或
fte run demo/demo.fte
```

### 2. 交互式 REPL 终端

```bash
fte
# 或
fte repl
```

进入交互模式后，可以逐条输入命令：

```
fte:~/project> load -i photo.png
[load] 加载 "photo.png"
       尺寸 1920x1080
fte:~/project *> t -x 960 -y 100 -s 50 -c #000000 -bg #888888 "demo"
fte:~/project *> i -x 860 -y 80 -s 0.3 -a 0.5 logo.png
fte:~/project *> save -o output.png
```

输入 `help` 查看帮助，`exit`/`quit`/`q` 退出。

### 3. 查看频谱图

```bash
fte view -i photo.png              # 输出 photo_ch0_mag.png 等
fte view -i photo.png -o spec      # 输出 spec_ch0_mag.png 等
```

## .fte 脚本语法

### 命令一览

| 命令 | 说明 |
|------|------|
| `load -i <path>` | 加载输入图片并执行 FFT |
| `t` | 在频谱图上写文字 |
| `i` | 在频谱图上贴图片 |
| `save -o <path>` | 应用所有编辑，IFFT 输出 PNG |
| `debug -inter` | 保存中间频谱图（调试用） |
| `reset` | 清空当前状态，可重新 load |

`#` 开头的行和空行会被忽略。

### 写文字命令 `t`

```
t -x <cx> -y <cy> -s <size> [-c <color>] [-bg <color>] [-a <alpha>] [-r <angle>] [-f <font>] "文字"
```

| 标志 | 默认值 | 说明 |
|------|--------|------|
| `-x` | (必填) | 文字包围盒中心 x 坐标 |
| `-y` | (必填) | 文字包围盒中心 y 坐标 |
| `-s` | (必填) | 字号（磅，如 24） |
| `-c` | `#FFFFFF` | 字体颜色 hex RGBA |
| `-bg` | `none` | 背景矩形颜色 hex RGBA，`none` = 完全透明 |
| `-a` | `1.0` | 整体不透明度 [0-1] |
| `-r` | `0` | 旋转角度（度，逆时针） |
| `-f` | `auto` | 字体名或路径 (simhei/msyh/arial/...) |

示例：

```bash
t -x 256 -y 256 -s 32 "Hello"
t -x 400 -y 300 -s 48 -c #FF0000 -bg #00000080 -a 0.8 -r 45 "水印"
t -x 100 -y 50 -s 24 -c "00FF00" -f simhei "中文文字"
```

### 贴图片命令 `i`

```
i -x <cx> -y <cy> [-s <scale>] [-a <alpha>] [-r <angle>] "图片路径"
```

| 标志 | 默认值 | 说明 |
|------|--------|------|
| `-x` | (必填) | 图像中心 x 坐标 |
| `-y` | (必填) | 图像中心 y 坐标 |
| `-s` | `1.0` | 缩放比例 |
| `-a` | `1.0` | 整体不透明度 [0-1] |
| `-r` | `0` | 旋转角度（度，逆时针） |

示例：

```bash
i -x 512 -y 384 logo.png
i -x 512 -y 384 -s 0.5 -a 0.6 -r 30 "wm/logo.png"
```

### 颜色格式

支持 hex RGB / RGBA，带或不带 `#` 前缀：

```
#FFFFFF  FFFFFF    白色，完全不透明
#FF000080          红色，50% 透明
FF0000             红色，完全不透明
none / None        透明（仅 -bg 可用）
```

### 完整脚本示例

```bash
# 加载输入图片
load -i photo.png

# 写文字
t -x 100 -y 50 -s 24 -c #FFFFFF -bg #00000040 -f msyh "左上角"
t -x 512 -y 384 -s 48 -c #FF0000 -bg none -a 0.8 -r 45 "中间旋转"

# 贴图
i -x 512 -y 200 -s 0.5 -a 0.6 -r 30 logo.png

# 输出
save -o output.png
```

## 字体支持

支持系统 TTF/OTF 字体，自动搜索以下路径：

| 平台 | 字体路径 |
|------|----------|
| Windows | `%WINDIR%\Fonts\` |
| macOS | `/System/Library/Fonts/`, `/Library/Fonts/` |
| Linux | `/usr/share/fonts/`, `~/.fonts/` |

`-f` 可指定字体名（如 `simhei`、`msyh`、`arial`）或完整路径。留空则自动尝试 simhei → msyh → arial → helvetica。

## 支持的图片格式

**输入**: PNG / JPEG / GIF / BMP / TIFF / WebP

**输出**: PNG

## 依赖

- [go-dsp](https://github.com/madelynnblue/go-dsp) — FFT/IFFT 实现
- [golang.org/x/image](https://pkg.go.dev/golang.org/x/image) — 扩展图像处理
- [go-freetype](https://github.com/golang/freetype) — TrueType 字体渲染

## 注意事项

> ⚠️ 在频谱图中，不要将任何像素的任何通道值设置为 `FF`（255）或接近该值的颜色，否则会导致 IFFT 还原后的图像出现严重失真。

## License

MIT
