package service

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestMergeMembersAvatar_HTTPPNG(t *testing.T) {
	// serve a tiny red png
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.Set(x, y, color.RGBA{R: 0xFF, A: 0xFF})
		}
	}
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/png")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(pngBuf.Bytes())
	}))
	defer srv.Close()

	outDir := t.TempDir()
	res, err := MergeMembersAvatar([]string{srv.URL}, MergeAvatarsConfig{OutputDir: outDir, CanvasSize: 64})
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if res == nil || res.FilePath == "" {
		t.Fatalf("unexpected result: %+v", res)
	}
	if _, err := os.Stat(res.FilePath); err != nil {
		t.Fatalf("output not exists: %v", err)
	}

	// Quick sanity: merged image should not be all placeholder gray.
	f, err := os.Open(res.FilePath)
	if err != nil {
		t.Fatalf("open merged: %v", err)
	}
	defer func() { _ = f.Close() }()

	merged, _, err := image.Decode(f)
	if err != nil {
		t.Fatalf("decode merged: %v", err)
	}
	mid := merged.At(merged.Bounds().Dx()/2, merged.Bounds().Dy()/2)
	r, g, b, _ := mid.RGBA()
	// placeholder is 0xCC; background is 0xF2. red should make R noticeably higher
	if r <= g || r <= b {
		t.Fatalf("expected center pixel to be reddish, got r=%d g=%d b=%d", r, g, b)
	}
}

func TestMergeMembersAvatar_FileURL(t *testing.T) {
	// Write a green png to disk and load via file://
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			img.Set(x, y, color.RGBA{G: 0xFF, A: 0xFF})
		}
	}

	tmp := t.TempDir()
	inPath := filepath.Join(tmp, "a.png")
	{
		f, err := os.Create(inPath)
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		if err := png.Encode(f, img); err != nil {
			_ = f.Close()
			t.Fatalf("encode: %v", err)
		}
		_ = f.Close()
	}

	res, err := MergeMembersAvatar([]string{"file://" + inPath}, MergeAvatarsConfig{OutputDir: tmp, CanvasSize: 64})
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if res == nil {
		t.Fatalf("nil res")
	}
}

func TestMergeMembersAvatar_FailIfAllFetchFailed(t *testing.T) {
	outDir := t.TempDir()
	_, err := MergeMembersAvatar([]string{"http://127.0.0.1:1/not-exist"}, MergeAvatarsConfig{OutputDir: outDir, FailIfAllFetchFailed: true})
	if err == nil {
		t.Fatalf("expected error")
	}
}
