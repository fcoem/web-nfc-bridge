//go:build windows

package bridge

import (
	"encoding/binary"
	"errors"
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

	procCreateFileW    = kernel32.NewProc("CreateFileW")
	procDeviceIoCtl    = kernel32.NewProc("DeviceIoControl")
	procCloseHandle    = kernel32.NewProc("CloseHandle")
)

// GUID_DEVINTERFACE_SMARTCARD_READER {50DD5230-BA8A-11D1-BF5D-0000F805F530}
var guidSmartCardReader = syscall.GUID{
	Data1: 0x50DD5230,
	Data2: 0xBA8A,
	Data3: 0x11D1,
	Data4: [8]byte{0xBF, 0x5D, 0x00, 0x00, 0xF8, 0x05, 0xF5, 0x30},
}

const (
	digcfPresent         = 0x00000002
	digcfDeviceInterface = 0x00000010

	genericRead  = 0x80000000
	genericWrite = 0x40000000
	openExisting = 3

	fileFlagOverlapped = 0x40000000

	invalidHandleValue = ^syscall.Handle(0)

	// Smart card IOCTL codes — CTL_CODE(FILE_DEVICE_SMARTCARD, Function, METHOD_BUFFERED, FILE_ANY_ACCESS)
	// = (0x31 << 16) | (Function << 2)
	ioctlSmartCardPower        = 0x00310004 // Function 1
	ioctlSmartCardGetAttribute = 0x00310008 // Function 2
	ioctlSmartCardTransmit     = 0x00310014 // Function 5
	ioctlSmartCardIsPresent    = 0x00310028 // Function 10
	ioctlSmartCardIsAbsent     = 0x0031002C // Function 11
	ioctlSmartCardSetProtocol  = 0x00310030 // Function 12
	ioctlSmartCardGetState     = 0x00310038 // Function 14

	// Power dispositions
	scardColdReset = 1

	// Protocols
	scardProtocolT0  = 1
	scardProtocolT1  = 2
	scardProtocolRaw = 0x10000

	// Attributes
	scardAttrATRString = 0x00090303

	// SP_DEVICE_INTERFACE_DATA size: cbSize(4) + GUID(16) + Flags(4) + Reserved(uintptr=8) = 32 on 64-bit
	spDeviceInterfaceDataSize = 32

	// SP_DEVICE_INTERFACE_DETAIL_DATA_W.cbSize on 64-bit = 8
	// (4-byte DWORD cbSize + alignment padding to WCHAR boundary)
	spDeviceInterfaceDetailDataWCbSize = 8
)

// SP_DEVICE_INTERFACE_DATA
type spDeviceInterfaceData struct {
	cbSize             uint32
	interfaceClassGuid syscall.GUID
	flags              uint32
	reserved           uintptr
}

// directCard wraps a handle to a smart card reader opened via CreateFileW.
// It implements CardTransmitter for use with the shared NDEF logic.
type directCard struct {
	handle   syscall.Handle
	protocol uint32
}

func (c *directCard) Transmit(cmd []byte) ([]byte, error) {
	return smartCardTransmit(c.handle, c.protocol, cmd)
}

func (c *directCard) Close() error {
	if c.handle != invalidHandleValue {
		procCloseHandle.Call(uintptr(c.handle))
		c.handle = invalidHandleValue
	}
	return nil
}

// enumerateSmartCardReaders returns device paths for all present smart card readers.
// For ACR1252 dual-interface readers: MI_00 = PICC (contactless/NFC), MI_01 = SAM (contact).
// MI_00 is sorted first since NFC operations target the contactless reader.
func enumerateSmartCardReaders() ([]string, error) {
	hDevInfo, _, err := procSetupDiGetClassDevsW.Call(
		uintptr(unsafe.Pointer(&guidSmartCardReader)),
		0,
		0,
		digcfPresent|digcfDeviceInterface,
	)
	if hDevInfo == uintptr(invalidHandleValue) {
		return nil, fmt.Errorf("SetupDiGetClassDevsW: %w", err)
	}
	defer procSetupDiDestroyDeviceInfoList.Call(hDevInfo)

	var picc, other []string
	for i := uint32(0); ; i++ {
		var did spDeviceInterfaceData
		did.cbSize = spDeviceInterfaceDataSize

		ret, _, _ := procSetupDiEnumDeviceInterfaces.Call(
			hDevInfo,
			0,
			uintptr(unsafe.Pointer(&guidSmartCardReader)),
			uintptr(i),
			uintptr(unsafe.Pointer(&did)),
		)
		if ret == 0 {
			break // ERROR_NO_MORE_ITEMS
		}

		path, pathErr := getDeviceInterfaceDetailPath(hDevInfo, &did)
		if pathErr != nil {
			continue
		}

		// ACR1252: MI_00 = PICC (contactless/NFC), MI_01 = SAM (contact)
		upper := strings.ToUpper(path)
		if strings.Contains(upper, "MI_00") {
			picc = append(picc, path)
		} else {
			other = append(other, path)
		}
	}

	// PICC (MI_00) first — NFC operations need the contactless interface
	return append(picc, other...), nil
}

func getDeviceInterfaceDetailPath(hDevInfo uintptr, did *spDeviceInterfaceData) (string, error) {
	// First call to get required size
	var requiredSize uint32
	procSetupDiGetDeviceInterfaceDetailW.Call(
		hDevInfo,
		uintptr(unsafe.Pointer(did)),
		0,
		0,
		uintptr(unsafe.Pointer(&requiredSize)),
		0,
	)
	if requiredSize == 0 {
		return "", errors.New("SetupDiGetDeviceInterfaceDetailW: zero size")
	}

	// Allocate buffer for SP_DEVICE_INTERFACE_DETAIL_DATA_W
	buf := make([]byte, requiredSize)
	// Set cbSize field (first 4 bytes) = 8 for 64-bit
	binary.LittleEndian.PutUint32(buf[0:4], spDeviceInterfaceDetailDataWCbSize)

	ret, _, err := procSetupDiGetDeviceInterfaceDetailW.Call(
		hDevInfo,
		uintptr(unsafe.Pointer(did)),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(requiredSize),
		0,
		0,
	)
	if ret == 0 {
		return "", fmt.Errorf("SetupDiGetDeviceInterfaceDetailW: %w", err)
	}

	// Device path starts at offset 4 (after cbSize), is a null-terminated UTF-16 string
	pathBytes := buf[4:]
	path := syscall.UTF16ToString((*[4096]uint16)(unsafe.Pointer(&pathBytes[0]))[:len(pathBytes)/2])
	return path, nil
}

// openSmartCardReader opens a handle to the smart card reader at the given device path.
func openSmartCardReader(devicePath string) (syscall.Handle, error) {
	pathPtr, err := syscall.UTF16PtrFromString(devicePath)
	if err != nil {
		return invalidHandleValue, err
	}

	handle, _, callErr := procCreateFileW.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		genericRead|genericWrite,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE,
		0,
		openExisting,
		0,
		0,
	)
	if syscall.Handle(handle) == invalidHandleValue {
		return invalidHandleValue, fmt.Errorf("CreateFileW: %w", callErr)
	}
	return syscall.Handle(handle), nil
}

// smartCardPower sends IOCTL_SMARTCARD_POWER to the reader.
// Returns the ATR bytes on success.
func smartCardPower(handle syscall.Handle, disposition uint32) ([]byte, error) {
	inBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(inBuf, disposition)

	outBuf := make([]byte, 64)
	var bytesReturned uint32

	ret, _, err := procDeviceIoCtl.Call(
		uintptr(handle),
		ioctlSmartCardPower,
		uintptr(unsafe.Pointer(&inBuf[0])),
		uintptr(len(inBuf)),
		uintptr(unsafe.Pointer(&outBuf[0])),
		uintptr(len(outBuf)),
		uintptr(unsafe.Pointer(&bytesReturned)),
		0,
	)
	if ret == 0 {
		return nil, fmt.Errorf("IOCTL_SMARTCARD_POWER: %w", err)
	}
	return outBuf[:bytesReturned], nil
}

// smartCardSetProtocol negotiates the card protocol via IOCTL_SMARTCARD_SET_PROTOCOL.
// protocolMask is a bitmask of acceptable protocols (e.g. scardProtocolT0|scardProtocolT1).
// Returns the negotiated protocol value on success.
// This must be called after POWER to transition the card from "negotiable" (state 5)
// to "specific" (state 6) before TRANSMIT will work.
func smartCardSetProtocol(handle syscall.Handle, protocolMask uint32) (uint32, error) {
	inBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(inBuf, protocolMask)

	outBuf := make([]byte, 4)
	var bytesReturned uint32

	ret, _, err := procDeviceIoCtl.Call(
		uintptr(handle),
		ioctlSmartCardSetProtocol,
		uintptr(unsafe.Pointer(&inBuf[0])),
		uintptr(len(inBuf)),
		uintptr(unsafe.Pointer(&outBuf[0])),
		uintptr(len(outBuf)),
		uintptr(unsafe.Pointer(&bytesReturned)),
		0,
	)
	if ret == 0 {
		return 0, fmt.Errorf("IOCTL_SMARTCARD_SET_PROTOCOL: %w", err)
	}
	if bytesReturned >= 4 {
		return binary.LittleEndian.Uint32(outBuf[:4]), nil
	}
	return 0, nil
}

// smartCardTransmit sends an APDU command via IOCTL_SMARTCARD_TRANSMIT.
// The input buffer format: SCARD_IO_REQUEST header (8 bytes) + APDU command.
// The output buffer format: SCARD_IO_REQUEST header (8 bytes) + response.
func smartCardTransmit(handle syscall.Handle, protocol uint32, command []byte) ([]byte, error) {
	// Build input: SCARD_IO_REQUEST{dwProtocol, cbPciLength=8} + command
	inBuf := make([]byte, 8+len(command))
	binary.LittleEndian.PutUint32(inBuf[0:4], protocol)
	binary.LittleEndian.PutUint32(inBuf[4:8], 8) // cbPciLength
	copy(inBuf[8:], command)

	outBuf := make([]byte, 8+258) // header + max response
	var bytesReturned uint32

	ret, _, err := procDeviceIoCtl.Call(
		uintptr(handle),
		ioctlSmartCardTransmit,
		uintptr(unsafe.Pointer(&inBuf[0])),
		uintptr(len(inBuf)),
		uintptr(unsafe.Pointer(&outBuf[0])),
		uintptr(len(outBuf)),
		uintptr(unsafe.Pointer(&bytesReturned)),
		0,
	)
	if ret == 0 {
		return nil, fmt.Errorf("IOCTL_SMARTCARD_TRANSMIT: %w", err)
	}

	if bytesReturned <= 8 {
		return nil, errors.New("IOCTL_SMARTCARD_TRANSMIT: no response data")
	}

	// Skip the 8-byte SCARD_IO_REQUEST header in output
	return outBuf[8:bytesReturned], nil
}

// smartCardIsPresent checks if a card is present in the reader.
func smartCardIsPresent(handle syscall.Handle) bool {
	var bytesReturned uint32
	ret, _, _ := procDeviceIoCtl.Call(
		uintptr(handle),
		ioctlSmartCardIsPresent,
		0, 0,
		0, 0,
		uintptr(unsafe.Pointer(&bytesReturned)),
		0,
	)
	return ret != 0
}

// smartCardGetAttribute reads a smart card attribute.
func smartCardGetAttribute(handle syscall.Handle, attrID uint32) ([]byte, error) {
	inBuf := make([]byte, 4)
	binary.LittleEndian.PutUint32(inBuf, attrID)

	outBuf := make([]byte, 256)
	var bytesReturned uint32

	ret, _, err := procDeviceIoCtl.Call(
		uintptr(handle),
		ioctlSmartCardGetAttribute,
		uintptr(unsafe.Pointer(&inBuf[0])),
		uintptr(len(inBuf)),
		uintptr(unsafe.Pointer(&outBuf[0])),
		uintptr(len(outBuf)),
		uintptr(unsafe.Pointer(&bytesReturned)),
		0,
	)
	if ret == 0 {
		return nil, fmt.Errorf("IOCTL_SMARTCARD_GET_ATTRIBUTE: %w", err)
	}
	return outBuf[:bytesReturned], nil
}
