//go:build windows && (amd64 || arm64)

// Package winapi 提供系统 DLL 加载与 UTF-16 转换辅助函数。仅支持 64 位 Windows ABI。
package winapi

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// DLL 强制从 System32 目录加载，防止工作目录同名 DLL 劫持。
func DLL(name string) *syscall.LazyDLL {
	return syscall.NewLazyDLL(filepath.Join(os.Getenv("SystemRoot"), "System32", name))
}

func UTF16(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		panic(err)
	}
	return p
}

func Error(api string, err error) error {
	if err == syscall.Errno(0) {
		return fmt.Errorf("%s 失败", api)
	}
	return fmt.Errorf("%s: %w", api, err)
}
