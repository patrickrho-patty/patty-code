//go:build darwin

package cli

import (
	"sync"

	"github.com/ebitengine/purego"
)

const (
	carbonFramework = "/System/Library/Frameworks/Carbon.framework/Carbon"
	coreFoundation  = "/System/Library/Frameworks/CoreFoundation.framework/CoreFoundation"
	coreGraphics    = "/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics"
	cfStringUTF8    = 0x08000100
	cgHIDSystem     = 1
	macBackspaceKey = 0x33
)

var macKeyboardInputSourceAPI struct {
	sync.Once
	available                    bool
	inputSourceIDProperty        uintptr
	copyCurrentInputSource       func() uintptr
	getInputSourceProperty       func(uintptr, uintptr) uintptr
	stringLength                 func(uintptr) int64
	stringMaximumSizeForEncoding func(int64, uint32) int64
	stringGetCString             func(uintptr, *byte, int64, uint32) bool
	stringCreateWithCString      func(uintptr, *byte, uint32) uintptr
	release                      func(uintptr)
}

var macPhysicalKeyStateAPI struct {
	sync.Once
	available bool
	keyState  func(int32, uint16) bool
}

func loadMacPhysicalKeyStateAPI() {
	macPhysicalKeyStateAPI.Do(func() {
		defer func() { _ = recover() }()
		framework, err := purego.Dlopen(coreGraphics, purego.RTLD_LAZY|purego.RTLD_LOCAL)
		if err != nil {
			return
		}
		purego.RegisterLibFunc(&macPhysicalKeyStateAPI.keyState, framework, "CGEventSourceKeyState")
		macPhysicalKeyStateAPI.available = true
	})
}

func currentPhysicalBackspaceDown() bool {
	loadMacPhysicalKeyStateAPI()
	return macPhysicalKeyStateAPI.available && macPhysicalKeyStateAPI.keyState(cgHIDSystem, macBackspaceKey)
}

func physicalBackspaceStateAvailable() bool {
	loadMacPhysicalKeyStateAPI()
	return macPhysicalKeyStateAPI.available
}

func loadMacKeyboardInputSourceAPI() {
	macKeyboardInputSourceAPI.Do(func() {
		// The TIS and CoreFoundation symbols are stable macOS system APIs. Loading
		// them dynamically keeps the CLI CGO-free for the existing release build.
		defer func() { _ = recover() }()
		carbon, err := purego.Dlopen(carbonFramework, purego.RTLD_LAZY|purego.RTLD_LOCAL)
		if err != nil {
			return
		}
		cf, err := purego.Dlopen(coreFoundation, purego.RTLD_LAZY|purego.RTLD_LOCAL)
		if err != nil {
			return
		}
		purego.RegisterLibFunc(&macKeyboardInputSourceAPI.copyCurrentInputSource, carbon, "TISCopyCurrentKeyboardInputSource")
		purego.RegisterLibFunc(&macKeyboardInputSourceAPI.getInputSourceProperty, carbon, "TISGetInputSourceProperty")
		purego.RegisterLibFunc(&macKeyboardInputSourceAPI.stringLength, cf, "CFStringGetLength")
		purego.RegisterLibFunc(&macKeyboardInputSourceAPI.stringMaximumSizeForEncoding, cf, "CFStringGetMaximumSizeForEncoding")
		purego.RegisterLibFunc(&macKeyboardInputSourceAPI.stringGetCString, cf, "CFStringGetCString")
		purego.RegisterLibFunc(&macKeyboardInputSourceAPI.stringCreateWithCString, cf, "CFStringCreateWithCString")
		purego.RegisterLibFunc(&macKeyboardInputSourceAPI.release, cf, "CFRelease")

		// The exported Carbon constant contains this CFString value. Constructing
		// the key through CoreFoundation avoids unsafe access to a Dlsym data
		// symbol and stays compatible with the repository's CGO-free build.
		propertyName := append([]byte("TISPropertyInputSourceID"), 0)
		macKeyboardInputSourceAPI.inputSourceIDProperty =
			macKeyboardInputSourceAPI.stringCreateWithCString(0, &propertyName[0], cfStringUTF8)
		macKeyboardInputSourceAPI.available = macKeyboardInputSourceAPI.inputSourceIDProperty != 0
	})
}

func prewarmKeyboardCompatibilityAPIs() {
	loadMacKeyboardInputSourceAPI()
	loadMacPhysicalKeyStateAPI()
}

func currentKeyboardInputSourceID() string {
	loadMacKeyboardInputSourceAPI()
	api := &macKeyboardInputSourceAPI
	if !api.available {
		return ""
	}
	inputSource := api.copyCurrentInputSource()
	if inputSource == 0 {
		return ""
	}
	defer api.release(inputSource)

	value := api.getInputSourceProperty(inputSource, api.inputSourceIDProperty)
	if value == 0 {
		return ""
	}
	length := api.stringLength(value)
	capacity := api.stringMaximumSizeForEncoding(length, cfStringUTF8) + 1
	if capacity <= 1 {
		return ""
	}
	buffer := make([]byte, capacity)
	if !api.stringGetCString(value, &buffer[0], capacity, cfStringUTF8) {
		return ""
	}
	for i, b := range buffer {
		if b == 0 {
			return string(buffer[:i])
		}
	}
	return string(buffer)
}
