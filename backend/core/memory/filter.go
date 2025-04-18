// Package memory 记忆服务
package memory

import (
	"path/filepath"
	"runtime"
	"strings"
)

var windowsSystemRoots = []string{
	`c:\windows`,
	`c:\program files`,
	`c:\program files (x86)`,
	`c:\programdata`,
}

var windowsSystemNames = map[string]struct{}{
	"pagefile.sys":              {},
	"hiberfil.sys":              {},
	"swapfile.sys":              {},
	"system volume information": {},
	"$recycle.bin":              {},
}

var binaryExts = map[string]struct{}{
	".exe": {}, ".dll": {}, ".so": {}, ".dylib": {}, ".bin": {}, ".dat": {},
	".class": {}, ".o": {}, ".obj": {}, ".a": {}, ".lib": {}, ".pyc": {},
	".jpg": {}, ".jpeg": {}, ".png": {}, ".gif": {}, ".webp": {}, ".bmp": {},
	".mp3": {}, ".wav": {}, ".flac": {}, ".mp4": {}, ".mkv": {}, ".avi": {},
	".zip": {}, ".7z": {}, ".rar": {}, ".gz": {}, ".tar": {},
}

// shouldSkipPath 检查是否应跳过该路径
func shouldSkipPath(path string) bool {
	clean := strings.ToLower(filepath.Clean(path))
	base := strings.ToLower(filepath.Base(clean))

	if _, ok := windowsSystemNames[base]; ok {
		return true
	}

	if runtime.GOOS == "windows" {
		for _, root := range windowsSystemRoots {
			if clean == root || strings.HasPrefix(clean, root+`\`) {
				return true
			}
		}
	}

	if _, ok := binaryExts[strings.ToLower(filepath.Ext(clean))]; ok {
		return true
	}

	return false
}
