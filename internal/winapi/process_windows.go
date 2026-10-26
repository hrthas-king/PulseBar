//go:build windows && (amd64 || arm64)

package winapi

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

const (
	WindowClass = "PulseBar.Window"
	WMDockLeft  = 0x8004
	WMDiag      = 0x8005
)

var (
	kernel            = DLL("kernel32.dll")
	createMutex       = kernel.NewProc("CreateMutexW")
	closeKernelHandle = kernel.NewProc("CloseHandle")
	processMemory     = kernel.NewProc("K32GetProcessMemoryInfo")
	guiResources      = DLL("user32.dll").NewProc("GetGuiResources")
)

func SingleInstance() (release func(), alreadyRunning bool, err error) {
	nameStr := `Local\PulseBar.Singleton`
	if os.Getenv("PULSEBAR_TEST_INSTANCE") != "" {
		nameStr = fmt.Sprintf(`Local\PulseBar.Singleton.%d`, os.Getpid())
	}
	name := UTF16(nameStr)
	h, _, callErr := createMutex.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if h == 0 {
		return nil, false, Error("CreateMutexW", callErr)
	}
	release = func() { closeKernelHandle.Call(h) }
	return release, callErr == syscall.Errno(183), nil
}

// SendWindowCommand 向当前会话已运行的 PulseBar 窗口投递控制消息（如退出、停靠最左侧、打开诊断）。
func SendWindowCommand(msg uint32) (bool, error) {
	user := DLL("user32.dll")
	name := UTF16(WindowClass)
	hwnd, _, _ := user.NewProc("FindWindowW").Call(uintptr(unsafe.Pointer(name)), 0)
	if hwnd == 0 {
		return false, nil
	}
	if ok, _, err := user.NewProc("PostMessageW").Call(hwnd, uintptr(msg), 0, 0); ok == 0 {
		return true, Error("PostMessageW", err)
	}
	return true, nil
}

func RequestExit() error {
	_, err := SendWindowCommand(0x10) // WM_CLOSE
	return err
}

type Memory struct {
	PrivateWorkingSetBytes *uint64 `json:"private_working_set_bytes,omitempty"`
	WorkingSetBytes        uint64  `json:"working_set_bytes"`
	PeakWorkingSetBytes    uint64  `json:"peak_working_set_bytes"`
	PrivateBytes           uint64  `json:"private_bytes"`
	GDIObjects             uint32  `json:"gdi_objects"`
	USERObjects            uint32  `json:"user_objects"`
}

func ProcessMemory() (Memory, error) {
	var counters struct {
		Size, PageFaultCount                                                                                  uint32
		PeakWorkingSet, WorkingSet, PeakPaged, Paged, PeakNonPaged, NonPaged, Pagefile, PeakPagefile, Private uintptr
		PrivateWorkingSet                                                                                     uintptr
		SharedCommit                                                                                          uint64
	}
	counters.Size = uint32(unsafe.Sizeof(counters))
	ok, _, err := processMemory.Call(^uintptr(0), uintptr(unsafe.Pointer(&counters)), uintptr(counters.Size))
	if ok == 0 {
		counters.Size = 80
		ok, _, err = processMemory.Call(^uintptr(0), uintptr(unsafe.Pointer(&counters)), 80)
	}
	if ok == 0 {
		return Memory{}, fmt.Errorf("读取内存: %w", err)
	}
	gdi, _, _ := guiResources.Call(^uintptr(0), 0)
	user, _, _ := guiResources.Call(^uintptr(0), 1)
	var privateWorkingSet *uint64
	if counters.PrivateWorkingSet > 0 {
		v := uint64(counters.PrivateWorkingSet)
		privateWorkingSet = &v
	}
	return Memory{
		PrivateWorkingSetBytes: privateWorkingSet,
		WorkingSetBytes:        uint64(counters.WorkingSet),
		PeakWorkingSetBytes:    uint64(counters.PeakWorkingSet),
		PrivateBytes:           uint64(counters.Private),
		GDIObjects:             uint32(gdi),
		USERObjects:            uint32(user),
	}, nil
}
