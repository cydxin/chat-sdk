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
// 说明：本项目没有对象存储/静态资源服务的统一约束，因此这里默认落盘到 outputDir，返回一个 file:// URL，然后获取这个文件流
// 可把 outputDir 替换成上传逻辑，然后返回远程 URL,实现oss
type MergeAvatarsConfig struct {
	CanvasSize int           // 画布大小（正方形，像素）
	Padding    int           // 外边距
	Gap        int           // 小图间距
	Timeout    time.Duration // 下载头像超时
	OutputDir  string        // 输出目录（为空则使用 os.TempDir()/chat-sdk-avatars）

	// LocalPathRoot 用于解析非 http(s) 的相对路径头像（例如 DB 里存 uploads/2026/...）。
	// - 为空：保持 best-effort（先按当前进程工作目录打开；失败后再尝试用 OutputDir 的父目录兜底）
	// - 非空：会优先用 filepath.Join(LocalPathRoot, url) 读取。
	LocalPathRoot string

	// URLPrefix 写库/对外访问前缀：
	// - 为空：默认使用 OutputDir 作为前缀（会移除 file://，并去掉前导 /，生成相对路径）
	// - 非空：直接用该前缀拼 filename（会自动处理斜杠）
	URLPrefix string

	// Headers 访问头像 URL 时附带的请求头（可用于私有 CDN、鉴权网关等）。
	Headers map[string]string
	// UserAgent 访问头像 URL 时使用的 UA（部分图床会拒绝空 UA）。
	UserAgent string

	// FailIfAllFetchFailed 当所有头像都拉取失败时，直接返回 error（避免“全灰但成功”）。
	FailIfAllFetchFailed bool
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

	fetchFailed := 0
	var lastFetchErr error
	for _, u := range urls {
		img, err := fetchAvatarImage(client, u, cfg)
		if err != nil {
			// 保留错误，用于“全失败”场景抛出，避免静默全灰。
			lastFetchErr = err
			fetchFailed++
		}
		if img == nil {
			img = placeholderImage(128, 128)
		}
		imgs = append(imgs, img)
	}

	if cfg.FailIfAllFetchFailed && len(urls) > 0 && fetchFailed == len(urls) {
		if lastFetchErr != nil {
			return nil, fmt.Errorf("all avatar fetch failed: %w", lastFetchErr)
		}
		return nil, fmt.Errorf("all avatar fetch failed")
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

func fetchAvatarImage(client *http.Client, url string, cfg MergeAvatarsConfig) (image.Image, error) {
	url = strings.TrimSpace(url)
	if url == "" {
		return nil, nil
	}

	// 支持 file:// 以及本地路径（常见：DB 里存的是 uploads/xxx.png 或磁盘绝对路径）。
	if strings.HasPrefix(url, "file://") {
		p := strings.TrimPrefix(url, "file://")
		p = strings.TrimPrefix(p, "/") // 兼容 file:///C:/...
		p = strings.ReplaceAll(p, "/", "\\")
		return decodeImageFromFile(p)
	}
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		// 非 http(s)：优先按 LocalPathRoot 解析相对路径
		if root := strings.TrimSpace(cfg.LocalPathRoot); root != "" && !filepath.IsAbs(url) {
			cand := filepath.Join(root, filepath.FromSlash(url))
			if img, err := decodeImageFromFile(cand); err == nil && img != nil {
				return img, nil
			}
		}

		// best-effort 当作本地路径处理：
		// 1) 先按原样打开（支持绝对路径 / 当前工作目录相对路径）
		if img, err := decodeImageFromFile(url); err == nil && img != nil {
			return img, nil
		}
		// 2) 再尝试把相对路径（如 uploads/2026/...）解析到 OutputDir 的父目录下
		//    典型部署：OutputDir = <project>/uploads/auto_avatar，那么父目录就是 <project>，拼上 uploads/... 即可。
		if !filepath.IsAbs(url) {
			base := strings.TrimSpace(cfg.OutputDir)
			if base == "" {
				base = os.TempDir()
			}
			cand := filepath.Join(filepath.Dir(base), filepath.FromSlash(url))
			return decodeImageFromFile(cand)
		}
		return decodeImageFromFile(url)
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	ua := strings.TrimSpace(cfg.UserAgent)
	if ua != "" {
		req.Header.Set("User-Agent", ua)
	}
	for k, v := range cfg.Headers {
		kk := strings.TrimSpace(k)
		if kk == "" {
			continue
		}
		req.Header.Set(kk, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch avatar failed: %s", resp.Status)
	}

	// 先读一小段判断 Content-Type，避免 HTML/JSON 错误页导致 decode 行为不明确。
	data, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	if err != nil {
		return nil, err
	}
	ct := strings.ToLower(strings.TrimSpace(resp.Header.Get("Content-Type")))
	if ct != "" && !strings.HasPrefix(ct, "image/") {
		// 有些 CDN 不带 content-type，这里只有在“明确不是 image”时才报错。
		return nil, fmt.Errorf("fetch avatar content-type not image: %s", ct)
	}

	img, _, err := image.Decode(bytes.NewReader(data))
	return img, err
}

func decodeImageFromFile(path string) (image.Image, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	data, err := io.ReadAll(io.LimitReader(f, 10<<20))
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
