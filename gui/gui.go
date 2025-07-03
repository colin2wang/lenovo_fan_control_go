package main

import (
	"fmt"
	"fyne.io/fyne/v2"
	"syscall"
	"time"
	"unsafe"

	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// 风扇模式常量
const (
	FanModeNormal = 0
	FanModeFast   = 1
)

// Win32 常量
const (
	GENERIC_READ          = 0x80000000
	GENERIC_WRITE         = 0x40000000
	FILE_SHARE_NONE       = 0
	OPEN_EXISTING         = 3
	FILE_ATTRIBUTE_NORMAL = 0x80
)

var (
	kernel32            = syscall.MustLoadDLL("kernel32.dll")
	procCreateFileW     = kernel32.MustFindProc("CreateFileW")
	procDeviceIoControl = kernel32.MustFindProc("DeviceIoControl")
	procCloseHandle     = kernel32.MustFindProc("CloseHandle")
	procGetLastError    = kernel32.MustFindProc("GetLastError")
)

const (
	IOCTL_WRITE = 0x831020C0
	IOCTL_READ  = 0x831020C4
)

func getLastError() uint32 {
	r, _, _ := procGetLastError.Call()
	return uint32(r)
}

func createFile(name string, access, share uint32, sa *syscall.SecurityAttributes, createMode, attrs uint32) syscall.Handle {
	ptr, err := syscall.UTF16PtrFromString(name)
	if err != nil {
		fmt.Printf("Error converting path to UTF16: %v\n", err)
		return syscall.InvalidHandle
	}
	ret, _, _ := procCreateFileW.Call(
		uintptr(unsafe.Pointer(ptr)),
		uintptr(access),
		uintptr(share),
		uintptr(unsafe.Pointer(sa)),
		uintptr(createMode),
		uintptr(attrs),
		0,
	)
	return syscall.Handle(ret)
}

func deviceIoControl(hdl syscall.Handle, ioctlCode uint32, inBuf unsafe.Pointer, inSize uintptr, outBuf unsafe.Pointer, outSize uintptr, bytesRet *uint32, overlapped *syscall.Overlapped) bool {
	ret, _, _ := syscall.Syscall9(
		procDeviceIoControl.Addr(),
		9,
		uintptr(hdl),
		uintptr(ioctlCode),
		uintptr(inBuf),
		inSize,
		uintptr(outBuf),
		outSize,
		uintptr(unsafe.Pointer(bytesRet)),
		uintptr(unsafe.Pointer(overlapped)),
		0,
	)
	return ret != 0
}

func closeHandle(hdl syscall.Handle) bool {
	r, _, _ := procCloseHandle.Call(uintptr(hdl))
	return r != 0
}

// 设置风扇模式
func fanControl(mode uint32) int {
	hndl := createFile(`\\.\EnergyDrv`, GENERIC_WRITE, 0, nil, OPEN_EXISTING, FILE_ATTRIBUTE_NORMAL)
	if hndl == syscall.InvalidHandle {
		fmt.Printf("Error: Failed to open EnergyDrv for writing. GetLastError=%d\n", getLastError())
		return -1
	}

	var inBuffer [3]uint32
	inBuffer[0] = 6
	inBuffer[1] = 1
	inBuffer[2] = mode

	var bytesReturned uint32
	success := deviceIoControl(hndl, IOCTL_WRITE,
		unsafe.Pointer(&inBuffer[0]), uintptr(unsafe.Sizeof(inBuffer)),
		nil, 0,
		&bytesReturned, nil)

	if !success {
		fmt.Printf("Error: DeviceIoControl (write) failed with error code %d\n", getLastError())
		closeHandle(hndl)
		return -1
	}

	closeHandle(hndl)
	return 1
}

// 获取当前风扇状态
func readState() int {
	hndl := createFile(`\\.\EnergyDrv`, GENERIC_READ, 0, nil, OPEN_EXISTING, FILE_ATTRIBUTE_NORMAL)
	if hndl == syscall.InvalidHandle {
		fmt.Printf("Error: Failed to open EnergyDrv for reading. GetLastError=%d\n", getLastError())
		return -1
	}

	var inBuffer [1]uint32
	inBuffer[0] = 14

	var outBuffer [1]uint32
	var bytesReturned uint32

	success := deviceIoControl(hndl, IOCTL_READ,
		unsafe.Pointer(&inBuffer[0]), uintptr(unsafe.Sizeof(inBuffer)),
		unsafe.Pointer(&outBuffer[0]), uintptr(unsafe.Sizeof(outBuffer)),
		&bytesReturned, nil)

	if !success {
		fmt.Printf("Error: DeviceIoControl (read) failed with error code %d\n", getLastError())
		closeHandle(hndl)
		return -1
	}

	closeHandle(hndl)

	if outBuffer[0] == 3 {
		return FanModeFast
	}
	return FanModeNormal
}

func main() {
	// 创建 Fyne 应用
	myApp := app.New()
	myWindow := myApp.NewWindow("Fan Controller")

	// 当前状态标签
	statusLabel := widget.NewLabel("Checking fan status...")

	// 切换按钮
	btn := widget.NewButton("Toggle Fan Mode", func() {
		current := readState()
		if current == FanModeNormal {
			if fanControl(FanModeFast) == -1 {
				statusLabel.SetText("Failed to set FAST mode.")
			} else {
				statusLabel.SetText("Fan mode: FAST")
			}
		} else if current == FanModeFast {
			if fanControl(FanModeNormal) == -1 {
				statusLabel.SetText("Failed to set NORMAL mode.")
			} else {
				statusLabel.SetText("Fan mode: NORMAL")
			}
		} else {
			statusLabel.SetText("Error reading fan state.")
		}
	})

	// 更新初始状态
	go func() {
		time.Sleep(500 * time.Millisecond)
		current := readState()
		if current == FanModeNormal {
			statusLabel.SetText("Fan mode: NORMAL")
		} else if current == FanModeFast {
			statusLabel.SetText("Fan mode: FAST")
		} else {
			statusLabel.SetText("Unknown fan state.")
		}
	}()

	// 布局
	content := container.NewVBox(statusLabel, btn)
	myWindow.SetContent(content)
	myWindow.Resize(fyne.NewSize(300, 150))
	myWindow.ShowAndRun()
}
