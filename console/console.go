package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
	"unsafe"
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

// IOCTL codes
const (
	IOCTL_WRITE = 0x831020C0
	IOCTL_READ  = 0x831020C4
)

var (
	kernel32            = syscall.MustLoadDLL("kernel32.dll")
	procCreateFileW     = kernel32.MustFindProc("CreateFileW")
	procDeviceIoControl = kernel32.MustFindProc("DeviceIoControl")
	procCloseHandle     = kernel32.MustFindProc("CloseHandle")
	procGetLastError    = kernel32.MustFindProc("GetLastError")
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

// 持续保持高速风扇模式（可选超时）
func keepFast(duration int) {
	interval := 1000 // ms
	start := time.Now().UnixNano() / int64(time.Millisecond)

	for {
		for readState() != FanModeFast {
			fanControl(FanModeFast)
			time.Sleep(100 * time.Millisecond)
		}

		if duration > 0 {
			elapsed := (time.Now().UnixNano()/int64(time.Millisecond) - start)
			leftTime := int64(duration) - elapsed
			if leftTime < int64(interval) {
				time.Sleep(time.Duration(leftTime) * time.Millisecond)
				for readState() != FanModeNormal {
					fanControl(FanModeNormal)
					time.Sleep(100 * time.Millisecond)
				}
				break
			}
		}

		time.Sleep(time.Duration(interval) * time.Millisecond)
	}
}

// 处理 Ctrl+C 中断信号
func setupSignalHandler() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		fmt.Println("\nInterrupt received, restoring normal fan mode...")
		fanControl(FanModeNormal)
		os.Exit(0)
	}()
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [options]\n", os.Args[0])
	flag.PrintDefaults()
}

func main() {
	mode := flag.String("mode", "toggle", "Fan mode: 'normal', 'fast', or 'toggle'")
	duration := flag.Int("duration", -1, "Duration in seconds to keep fast mode (-1 for infinite)")
	help := flag.Bool("help", false, "Show help")

	flag.Parse()

	if *help {
		printUsage()
		return
	}

	// 注册中断信号处理
	setupSignalHandler()

	currentMode := readState()
	if currentMode == -1 {
		fmt.Println("Failed to open \\\\.\\EnergyDrv or read state.")
		return
	}

	switch *mode {
	case "normal":
		fmt.Println("Setting fan to NORMAL mode.")
		if fanControl(FanModeNormal) == -1 {
			fmt.Println("Failed to set fan to normal mode.")
		}
	case "fast":
		fmt.Println("Setting fan to FAST mode.")
		if fanControl(FanModeFast) == -1 {
			fmt.Println("Failed to set fan to fast mode.")
		} else if *duration >= 0 {
			fmt.Printf("Keeping fast mode for %d seconds...\n", *duration)
			go keepFast(*duration * 1000)
		}
	case "toggle":
		if currentMode == FanModeNormal {
			fmt.Println("FAST mode on")
			go keepFast(*duration * 1000)
		} else {
			fmt.Println("NORMAL mode on")
			if fanControl(FanModeNormal) == -1 {
				fmt.Println("Failed to restore normal mode.")
			}
		}
	default:
		fmt.Printf("Invalid mode: %s\n", *mode)
		printUsage()
		return
	}

	fmt.Println("Press Enter to exit...")
	fmt.Scanln()
}
