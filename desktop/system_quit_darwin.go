//go:build darwin

package main

/*
#cgo darwin LDFLAGS: -framework Cocoa
void installPattyCodeSystemQuitHook(void);
*/
import "C"

import "sync"

var installSystemQuitHookOnce sync.Once

func installSystemQuitHook() {
	installSystemQuitHookOnce.Do(func() {
		C.installPattyCodeSystemQuitHook()
	})
}

//export PattyCodeMarkSystemQuit
func PattyCodeMarkSystemQuit() {
	markSystemQuitRequested()
}
