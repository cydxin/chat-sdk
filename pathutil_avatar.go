package chat_sdk

import (
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// defaultAvatarSourcePath returns the on-disk path to chat-sdk/default/default.png.
// We can't rely on the host app's working directory, so we resolve it relative to this file.
func defaultAvatarSourcePath() (string, bool) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", false
	}
	// thisFile == <...>/chat-sdk/pathutil_avatar.go
	root := filepath.Dir(thisFile)
	src := filepath.Join(root, "default", "default.png")
	if _, err := os.Stat(src); err != nil {
		return "", false
	}
	return src, true
}

// copyDefaultAvatarBestEffort copies default.png into destDir/default.png.
func copyDefaultAvatarBestEffort(destDir string) {
	src, ok := defaultAvatarSourcePath()
	if !ok {
		log.Printf("default avatar not found, skip copy")
		return
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		log.Printf("create avatar dir failed: %v (dir=%s)", err, destDir)
		return
	}

	data, err := os.ReadFile(src)
	if err != nil {
		log.Printf("read default avatar failed: %v (src=%s)", err, src)
		return
	}

	destPath := filepath.Join(destDir, "default.png")
	if err := os.WriteFile(destPath, data, 0o644); err != nil {
		log.Printf("write default avatar failed: %v (dest=%s)", err, destPath)
		return
	}
	log.Printf("default avatar copied to %s", destPath)
}

// defaultGroupAvatarMergeOutputDir derives a stable default OutputDir.
// Priority:
//  1. explicit configured OutputDir
//  2. <exeDir>/uploads/auto_avatar
//  3. os.TempDir()/chat-sdk-avatars (最后兜底)
//
// Note: "main.go 所在目录"在编译后的二进制里不可得，所以这里用可执行文件所在目录
// 来作为应用根目录，更适合线上部署。
func defaultGroupAvatarMergeOutputDir(configured string) string {
	if strings.TrimSpace(configured) != "" {
		// 用户指定 OutputDir 时，也尽力把默认头像放进去
		copyDefaultAvatarBestEffort(configured)
		return configured
	}

	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		destDir := filepath.Join(exeDir, "uploads", "auto_avatar")
		copyDefaultAvatarBestEffort(destDir)
		return destDir
	}

	destDir := filepath.Join(os.TempDir(), "chat-sdk-avatars")
	copyDefaultAvatarBestEffort(destDir)
	return destDir
}

func DefaultGroupAvatarMergeURLPrefix(configured string) string {
	if strings.TrimSpace(configured) != "" {
		return configured
	}
	// 默认写库为相对路径，交给宿主应用的静态资源路由处理。
	return "uploads/auto_avatar"
}
