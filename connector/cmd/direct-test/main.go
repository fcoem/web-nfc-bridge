//go:build windows

package main

import (
	"encoding/binary"
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

var (
	setupapi = syscall.NewLazyDLL("setupapi.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	procSetupDiGetClassDevsW             = setupapi.NewProc("SetupDiGetClassDevsW")
	procSetupDiEnumDeviceInterfaces      = setupapi.NewProc("SetupDiEnumDeviceInterfaces")
	procSetupDiGetDeviceInterfaceDetailW = setupapi.NewProc("SetupDiGetDeviceInterfaceDetailW")
	procSetupDiDestroyDeviceInfoList     = setupapi.NewProc("SetupDiDestroyDeviceInfoList")

	procCreateFileW = kernel32.NewProc("CreateFileW")
	procDeviceIoCtl = kernel32.NewProc("DeviceIoControl")
	procCloseHandle = kernel32.NewProc("CloseHandle")
)

var guidSmartCardReader = syscall.GUID{
	Data1: 0x50DD5230,
	Data2: 0xBA8A,
	Data3: 0x11D1,
	Data4: [8]byte{0xBF, 0x5D, 0x00, 0x00, 0xF8, 0x05, 0xF5, 0x30},
}

const (
	digcfPresent         = 0x00000002
	digcfDeviceInterface = 0x00000010
	invalidHandleValue   = ^uintptr(0)

	ioctlPower       = 0x00310004
	ioctlGetAttr     = 0x00310008
	ioctlSetProtocol = 0x00310030 // CTL_CODE(0x31, 12, 0, 0)
	ioctlTransmit    = 0x00310014
	ioctlGetState    = 0x00310038
)

type spDeviceInterfaceData struct {
	cbSize             uint32
	interfaceClassGuid syscall.GUID
	flags              uint32
	reserved           uintptr
}

func main() {
	fmt.Println("=== TRANSMIT 深度診斷 ===")
	fmt.Println("請先: Stop-Service SCardSvr -Force")
	fmt.Println("NFC 卡放在讀卡機上")
	fmt.Println()

	paths := enumReaders()
	if len(paths) == 0 {
		fmt.Println("找不到智慧卡讀卡機")
		return
	}

	// 只測試 MI_00 (已確認是 PICC 介面)
	for _, path := range paths {
		upper := strings.ToUpper(path)
		if !strings.Contains(upper, "MI_00") {
			continue
		}
		fmt.Println("=== 測試 MI_00 (PICC) ===")
		testPICC(path)
		return
	}
	fmt.Println("找不到 MI_00 介面")
}

func testPICC(path string) {
	pathPtr, _ := syscall.UTF16PtrFromString(path)
	handle, _, err := procCreateFileW.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		0x80000000|0x40000000,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE,
		0, 3, 0, 0,
	)
	if syscall.Handle(handle) == syscall.InvalidHandle {
		fmt.Printf("CreateFileW FAILED: %v\n", err)
		return
	}
	defer procCloseHandle.Call(handle)

	// POWER
	atr := power(handle, 1)
	if atr == nil {
		fmt.Println("POWER 失敗")
		return
	}
	fmt.Printf("ATR: %X\n", atr)
	fmt.Printf("GET_STATE: %d (%s)\n\n", getState(handle), stateName(getState(handle)))

	getUID := []byte{0xFF, 0xCA, 0x00, 0x00, 0x00}

	// === 測試 1: POWER 後直接 TRANSMIT（不先 SET_PROTOCOL）===
	fmt.Println("========================================")
	fmt.Println("測試 1: POWER 後直接 TRANSMIT (不先 SET_PROTOCOL)")
	fmt.Println("========================================")
	for _, proto := range []uint32{2, 1, 0, 0x10000} {
		fmt.Printf("\n  protocol=%d (0x%X):\n", proto, proto)
		transmitShowAll(handle, proto, getUID)
	}

	// === 測試 2: 不用 SCARD_IO_REQUEST header，只送 raw APDU ===
	fmt.Println("\n========================================")
	fmt.Println("測試 2: 不用 header，只送 raw APDU bytes")
	fmt.Println("========================================")
	transmitRaw(handle, getUID)

	// === 測試 3: 重新 POWER，SET_PROTOCOL 一次，再 TRANSMIT ===
	fmt.Println("\n========================================")
	fmt.Println("測試 3: 重新 POWER -> SET_PROTOCOL(T0|T1) 一次 -> TRANSMIT")
	fmt.Println("========================================")
	atr = power(handle, 1)
	if atr == nil {
		fmt.Println("重新 POWER 失敗")
		return
	}
	fmt.Printf("ATR: %X\n", atr)

	negotiated := setProtocol(handle, 3) // T0|T1
	fmt.Printf("SET_PROTOCOL(T0|T1) negotiated: %d\n", negotiated)
	fmt.Printf("GET_STATE: %d (%s)\n", getState(handle), stateName(getState(handle)))

	if negotiated > 0 {
		fmt.Printf("\n  用協商後的 protocol=%d:\n", negotiated)
		transmitShowAll(handle, negotiated, getUID)
	}
	// 也試 protocol=0
	fmt.Printf("\n  用 protocol=0:\n")
	transmitShowAll(handle, 0, getUID)

	// === 測試 4: GET_ATTRIBUTE — 讀取驅動資訊 ===
	fmt.Println("\n========================================")
	fmt.Println("測試 4: GET_ATTRIBUTE 讀取驅動資訊")
	fmt.Println("========================================")

	attrs := []struct {
		id   uint32
		name string
	}{
		{0x00090303, "ATR_STRING"},
		{0x7A020, "CURRENT_PROTOCOL_TYPE"},
		{0x80100, "VENDOR_NAME"},
		{0x80102, "VENDOR_IFD_TYPE"},
		{0x80110, "VENDOR_IFD_VERSION"},
		{0x80120, "VENDOR_IFD_SERIAL_NO"},
		{0x7A021, "ICC_TYPE_PER_ATR"},
		{0x7A010, "CHANNEL_ID"},
		{0x7FFF0003, "DEVICE_UNIT"},
	}

	for _, a := range attrs {
		data := getAttribute(handle, a.id)
		if data != nil {
			s := ""
			for _, b := range data {
				if b >= 0x20 && b < 0x7F {
					s += string(b)
				}
			}
			if len(s) > 2 {
				fmt.Printf("  %s (0x%X): %X = \"%s\"\n", a.name, a.id, data, s)
			} else {
				fmt.Printf("  %s (0x%X): %X\n", a.name, a.id, data)
			}
		} else {
			fmt.Printf("  %s (0x%X): 失敗\n", a.name, a.id)
		}
	}

	// === 測試 5: Escape IOCTL ===
	fmt.Println("\n========================================")
	fmt.Println("測試 5: Escape IOCTLs")
	fmt.Println("========================================")

	// SCARD_CTL_CODE(3500) = (0x31 << 16) | (3500 << 2) = 0x3136B0
	fmt.Printf("  SCARD_CTL_CODE(3500) = 0x3136B0:\n")
	fmt.Printf("    ACS 韌體 (FF 00 48 00 00):\n")
	escapeCall(handle, 0x3136B0, []byte{0xFF, 0x00, 0x48, 0x00, 0x00})

	fmt.Printf("    GetUID (FF CA 00 00 00):\n")
	escapeCall(handle, 0x3136B0, []byte{0xFF, 0xCA, 0x00, 0x00, 0x00})

	// SCARD_CTL_CODE(3400) — CM_IOCTL_GET_FEATURE_REQUEST
	fmt.Printf("\n  SCARD_CTL_CODE(3400) = 0x313520:\n")
	escapeCall(handle, 0x313520, nil)

	// 也試 SCARD_CTL_CODE(2048) = standard escape
	// = (0x31 << 16) | (2048 << 2) = 0x312000
	fmt.Printf("\n  SCARD_CTL_CODE(2048) = 0x312000:\n")
	escapeCall(handle, 0x312000, []byte{0xFF, 0xCA, 0x00, 0x00, 0x00})
}

func transmitShowAll(handle uintptr, protocol uint32, cmd []byte) {
	txIn := make([]byte, 8+len(cmd))
	binary.LittleEndian.PutUint32(txIn[0:4], protocol)
	binary.LittleEndian.PutUint32(txIn[4:8], 8)
	copy(txIn[8:], cmd)

	txOut := make([]byte, 300)
	var bytesReturned uint32

	ret, _, err := procDeviceIoCtl.Call(
		handle, ioctlTransmit,
		uintptr(unsafe.Pointer(&txIn[0])), uintptr(len(txIn)),
		uintptr(unsafe.Pointer(&txOut[0])), uintptr(len(txOut)),
		uintptr(unsafe.Pointer(&bytesReturned)), 0,
	)

	errno, _ := err.(syscall.Errno)
	fmt.Printf("    ret=%d errno=%d (%v) bytesReturned=%d\n", ret, errno, err, bytesReturned)

	if bytesReturned > 0 {
		raw := txOut[:bytesReturned]
		fmt.Printf("    原始輸出 (%d bytes): %X\n", bytesReturned, raw)
		if bytesReturned > 8 {
			data := raw[8:]
			fmt.Printf("    回應資料 (%d bytes): %X\n", len(data), data)
			if len(data) >= 2 {
				sw1, sw2 := data[len(data)-2], data[len(data)-1]
				fmt.Printf("    SW1=%02X SW2=%02X", sw1, sw2)
				if sw1 == 0x90 && sw2 == 0x00 {
					fmt.Printf(" (成功!) 有效資料: %X", data[:len(data)-2])
				}
				fmt.Println()
			}
		}
	}
}

func transmitRaw(handle uintptr, cmd []byte) {
	txOut := make([]byte, 300)
	var bytesReturned uint32

	ret, _, err := procDeviceIoCtl.Call(
		handle, ioctlTransmit,
		uintptr(unsafe.Pointer(&cmd[0])), uintptr(len(cmd)),
		uintptr(unsafe.Pointer(&txOut[0])), uintptr(len(txOut)),
		uintptr(unsafe.Pointer(&bytesReturned)), 0,
	)

	errno, _ := err.(syscall.Errno)
	fmt.Printf("    ret=%d errno=%d (%v) bytesReturned=%d\n", ret, errno, err, bytesReturned)

	if bytesReturned > 0 {
		fmt.Printf("    原始輸出 (%d bytes): %X\n", bytesReturned, txOut[:bytesReturned])
	}
}

func escapeCall(handle uintptr, ioctl uintptr, cmd []byte) {
	outBuf := make([]byte, 300)
	var bytesReturned uint32
	var inPtr uintptr
	var inLen uintptr
	if len(cmd) > 0 {
		inPtr = uintptr(unsafe.Pointer(&cmd[0]))
		inLen = uintptr(len(cmd))
	}

	ret, _, err := procDeviceIoCtl.Call(
		handle, ioctl,
		inPtr, inLen,
		uintptr(unsafe.Pointer(&outBuf[0])), uintptr(len(outBuf)),
		uintptr(unsafe.Pointer(&bytesReturned)), 0,
	)

	errno, _ := err.(syscall.Errno)
	fmt.Printf("    ret=%d errno=%d (%v) bytesReturned=%d\n", ret, errno, err, bytesReturned)

	if bytesReturned > 0 {
		raw := outBuf[:bytesReturned]
		fmt.Printf("    輸出 (%d bytes): %X\n", bytesReturned, raw)
		s := ""
		for _, b := range raw {
			if b >= 0x20 && b < 0x7F {
				s += string(b)
			}
		}
		if len(s) > 2 {
			fmt.Printf("    字串: \"%s\"\n", s)
		}
	}
}

func power(handle uintptr, disposition uint32) []byte {
	inBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(inBuf, disposition)
	outBuf := make([]byte, 64)
	var bytesReturned uint32

	ret, _, err := procDeviceIoCtl.Call(
		handle, ioctlPower,
		uintptr(unsafe.Pointer(&inBuf[0])), 4,
		uintptr(unsafe.Pointer(&outBuf[0])), 64,
		uintptr(unsafe.Pointer(&bytesReturned)), 0,
	)
	if ret != 0 {
		return outBuf[:bytesReturned]
	}
	fmt.Printf("  POWER: FAILED (%v)\n", err)
	return nil
}

func setProtocol(handle uintptr, protocol uint32) uint32 {
	inBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(inBuf, protocol)
	outBuf := make([]byte, 4)
	var bytesReturned uint32

	ret, _, err := procDeviceIoCtl.Call(
		handle, ioctlSetProtocol,
		uintptr(unsafe.Pointer(&inBuf[0])), 4,
		uintptr(unsafe.Pointer(&outBuf[0])), 4,
		uintptr(unsafe.Pointer(&bytesReturned)), 0,
	)
	if ret != 0 && bytesReturned >= 4 {
		return binary.LittleEndian.Uint32(outBuf[:4])
	}
	if ret != 0 {
		fmt.Printf("  SET_PROTOCOL: OK but bytesReturned=%d\n", bytesReturned)
		return 0
	}
	errno, _ := err.(syscall.Errno)
	fmt.Printf("  SET_PROTOCOL: FAILED errno=%d (%v)\n", errno, err)
	return 0
}

func getAttribute(handle uintptr, attrID uint32) []byte {
	inBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(inBuf, attrID)
	outBuf := make([]byte, 256)
	var bytesReturned uint32

	ret, _, _ := procDeviceIoCtl.Call(
		handle, ioctlGetAttr,
		uintptr(unsafe.Pointer(&inBuf[0])), 4,
		uintptr(unsafe.Pointer(&outBuf[0])), 256,
		uintptr(unsafe.Pointer(&bytesReturned)), 0,
	)
	if ret != 0 && bytesReturned > 0 {
		return outBuf[:bytesReturned]
	}
	return nil
}

func getState(handle uintptr) uint32 {
	outBuf := make([]byte, 64)
	var bytesReturned uint32
	ret, _, _ := procDeviceIoCtl.Call(
		handle, ioctlGetState,
		0, 0,
		uintptr(unsafe.Pointer(&outBuf[0])), 64,
		uintptr(unsafe.Pointer(&bytesReturned)), 0,
	)
	if ret != 0 {
		return binary.LittleEndian.Uint32(outBuf[:4])
	}
	return 0
}

func stateName(s uint32) string {
	names := map[uint32]string{
		0: "?", 1: "absent", 2: "present", 3: "swallowed",
		4: "powered", 5: "negotiable", 6: "specific",
	}
	if n, ok := names[s]; ok {
		return n
	}
	return fmt.Sprintf("unknown(%d)", s)
}

func enumReaders() []string {
	hDevInfo, _, err := procSetupDiGetClassDevsW.Call(
		uintptr(unsafe.Pointer(&guidSmartCardReader)),
		0, 0, digcfPresent|digcfDeviceInterface,
	)
	if hDevInfo == invalidHandleValue {
		fmt.Printf("SetupDiGetClassDevsW FAILED: %v\n", err)
		return nil
	}
	defer procSetupDiDestroyDeviceInfoList.Call(hDevInfo)

	var paths []string
	for i := uint32(0); ; i++ {
		var did spDeviceInterfaceData
		did.cbSize = uint32(unsafe.Sizeof(did))
		ret, _, _ := procSetupDiEnumDeviceInterfaces.Call(
			hDevInfo, 0, uintptr(unsafe.Pointer(&guidSmartCardReader)),
			uintptr(i), uintptr(unsafe.Pointer(&did)),
		)
		if ret == 0 {
			break
		}
		var reqSize uint32
		procSetupDiGetDeviceInterfaceDetailW.Call(
			hDevInfo, uintptr(unsafe.Pointer(&did)), 0, 0,
			uintptr(unsafe.Pointer(&reqSize)), 0,
		)
		if reqSize == 0 {
			continue
		}
		buf := make([]byte, reqSize)
		binary.LittleEndian.PutUint32(buf[0:4], 8)
		ret2, _, _ := procSetupDiGetDeviceInterfaceDetailW.Call(
			hDevInfo, uintptr(unsafe.Pointer(&did)),
			uintptr(unsafe.Pointer(&buf[0])), uintptr(reqSize), 0, 0,
		)
		if ret2 == 0 {
			continue
		}
		pathBytes := buf[4:]
		path := syscall.UTF16ToString((*[4096]uint16)(unsafe.Pointer(&pathBytes[0]))[:len(pathBytes)/2])
		paths = append(paths, path)
	}
	return paths
}
