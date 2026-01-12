package service

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// MergeAvatarsConfig 合成群头像配置。
// 说明：本项目没有对象存储/静态资源服务的统一约束，因此这里默认落盘到 outputDir，返回一个 file:// URL。
// 如果你有 CDN/OSS，可把 outputDir 替换成上传逻辑，然后返回远程 URL。
type MergeAvatarsConfig struct {
	CanvasSize int           // 画布大小（正方形，像素）
	Padding    int           // 外边距
	Gap        int           // 小图间距
	Timeout    time.Duration // 下载头像超时
	OutputDir  string        // 输出目录（为空则使用 os.TempDir()/chat-sdk-avatars）

	// URLPrefix 写库/对外访问前缀：
	// - 为空：默认使用 OutputDir 作为前缀（会移除 file://，并去掉前导 /，生成相对路径）
	// - 非空：直接用该前缀拼 filename（会自动处理斜杠）
	URLPrefix string
}

func (c MergeAvatarsConfig) withDefaults() MergeAvatarsConfig {
	out := c
	if out.CanvasSize <= 0 {
		out.CanvasSize = 256
	}
	if out.Padding < 0 {
		out.Padding = 8
	}
	if out.Gap < 0 {
		out.Gap = 4
	}
	if out.Timeout <= 0 {
		out.Timeout = 5 * time.Second
	}
	if strings.TrimSpace(out.OutputDir) == "" {
		out.OutputDir = filepath.Join(os.TempDir(), "chat-sdk-avatars")
	}
	return out
}

// MergeAvatarResult 合成结果。
type MergeAvatarResult struct {
	URL      string
	FilePath string
}

// MergeMembersAvatar 以微信风格将多张头像拼成一张。
// - 取自己+前若干（调用方控制顺序/截断），建议最多 9 张。
// - 输入 avatarURLs 允许为空字符串，会用灰色占位。
func MergeMembersAvatar(avatarURLs []string, cfg MergeAvatarsConfig) (*MergeAvatarResult, error) {
	cfg = cfg.withDefaults()

	// 规范化：最多 9 张
	urls := make([]string, 0, len(avatarURLs))
	for _, u := range avatarURLs {
		u = strings.TrimSpace(u)
		if u == "" {
			urls = append(urls, "")
			continue
		}
		urls = append(urls, u)
		if len(urls) >= 9 {
			break
		}
	}
	if len(urls) == 0 {
		// 至少生成一个默认头像
		urls = []string{""}
	}

	imgs := make([]image.Image, 0, len(urls))
	client := &http.Client{Timeout: cfg.Timeout}
	for _, u := range urls {
		img, _ := fetchAvatarImage(client, u)
		if img == nil {
			img = placeholderImage(128, 128)
		}
		imgs = append(imgs, img)
	}

	canvas := image.NewRGBA(image.Rect(0, 0, cfg.CanvasSize, cfg.CanvasSize))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: color.RGBA{R: 0xF2, G: 0xF2, B: 0xF2, A: 0xFF}}, image.Point{}, draw.Src)

	layout := calcWeChatLikeGrid(len(imgs))

	cellSize := (cfg.CanvasSize - 2*cfg.Padding - (layout.cols-1)*cfg.Gap) / layout.cols
	if cellSize <= 0 {
		cellSize = 1
	}

	gridW := layout.cols*cellSize + (layout.cols-1)*cfg.Gap
	gridH := layout.rows*cellSize + (layout.rows-1)*cfg.Gap
	startX := (cfg.CanvasSize - gridW) / 2
	startY := (cfg.CanvasSize - gridH) / 2

	for i, img := range imgs {
		r := i / layout.cols
		c := i % layout.cols
		x := startX + c*(cellSize+cfg.Gap)
		y := startY + r*(cellSize+cfg.Gap)

		thumb := resizeNearest(img, cellSize, cellSize)
		draw.Draw(canvas, image.Rect(x, y, x+cellSize, y+cellSize), thumb, image.Point{}, draw.Over)
	}

	// 输出文件
	if err := os.MkdirAll(cfg.OutputDir, 0o755); err != nil {
		return nil, err
	}

	// 生成稳定文件名：对 url 列表 hash
	h := sha1.New()
	// 为了稳定性，按原顺序合成，但 hash 用排序后的保证同一组用户拿到同一头像
	sorted := append([]string(nil), urls...)
	sort.Strings(sorted)
	for _, u := range sorted {
		_, _ = io.WriteString(h, u)
		_, _ = io.WriteString(h, "|")
	}
	name := hex.EncodeToString(h.Sum(nil)) + ".png"
	outPath := filepath.Join(cfg.OutputDir, name)

	var buf bytes.Buffer
	if err := png.Encode(&buf, canvas); err != nil {
		return nil, err
	}
	if err := os.WriteFile(outPath, buf.Bytes(), 0o644); err != nil {
		return nil, err
	}

	// 生成写库/访问 URL
	prefix := strings.TrimSpace(cfg.URLPrefix)
	if prefix == "" {
		prefix = strings.TrimSpace(cfg.OutputDir)
		prefix = strings.TrimPrefix(prefix, "file://")
		prefix = strings.ReplaceAll(prefix, "\\", "/")
		prefix = strings.TrimPrefix(prefix, "/")
		prefix = strings.TrimSuffix(prefix, "/")
	} else {
		prefix = strings.TrimSuffix(prefix, "/")
	}

	url := name
	if prefix != "" {
		url = prefix + "/" + name
	}

	return &MergeAvatarResult{URL: url, FilePath: outPath}, nil
}

type gridLayout struct{ rows, cols int }

// calcWeChatLikeGrid 简化版“微信群头像”布局（最多 9 宫格）。
func calcWeChatLikeGrid(n int) gridLayout {
	if n <= 1 {
		return gridLayout{rows: 1, cols: 1}
	}
	if n <= 4 {
		return gridLayout{rows: 2, cols: 2}
	}
	return gridLayout{rows: 3, cols: 3}
}

func fetchAvatarImage(client *http.Client, url string) (image.Image, error) {
	if strings.TrimSpace(url) == "" {
		return nil, nil
	}

	// 目前仅支持 http(s)；如要支持 file:// 可在此扩展
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		return nil, fmt.Errorf("unsupported avatar url: %s", url)
	}

	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch avatar failed: %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	return img, err
}

func placeholderImage(w, h int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: color.RGBA{R: 0xCC, G: 0xCC, B: 0xCC, A: 0xFF}}, image.Point{}, draw.Src)
	return img
}

// resizeNearest 最近邻缩放（无额外依赖，足够用作群头像拼图）。
func resizeNearest(src image.Image, w, h int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, w, h))
	sb := src.Bounds()
	sw := sb.Dx()
	sh := sb.Dy()
	if sw <= 0 || sh <= 0 {
		return dst
	}
	for y := 0; y < h; y++ {
		sy := sb.Min.Y + y*sh/h
		for x := 0; x < w; x++ {
			sx := sb.Min.X + x*sw/w
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}
