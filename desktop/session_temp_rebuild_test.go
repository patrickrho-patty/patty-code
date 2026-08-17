package main

import (
	"testing"

	"patty/internal/control"
)

func TestSessionTempFromControllerHelper(t *testing.T) {
	if sessionTempFromController(nil) != nil {
		t.Fatal("nil controller should yield nil SessionTemp")
	}
	ctrl := control.New(control.Options{})
	defer ctrl.Close()
	if sessionTempFromController(ctrl) == nil {
		t.Fatal("want non-nil SessionTemp from live controller")
	}
	if sessionTempFromController(ctrl) != ctrl.SessionTemp() {
		t.Fatal("helper must return the controller's Manager")
	}
}
