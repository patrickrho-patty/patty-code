//go:build darwin

package main

/*
#include <stdint.h>
#include <dispatch/dispatch.h>

extern void pattyDesktopMainHeartbeat(void);

static dispatch_source_t patty_main_heartbeat_timer;

static void patty_main_heartbeat_handler(void *ctx) {
	pattyDesktopMainHeartbeat();
}

static void patty_start_main_heartbeat(uint64_t interval_ms) {
	if (patty_main_heartbeat_timer != NULL) {
		return;
	}
	patty_main_heartbeat_timer = dispatch_source_create(DISPATCH_SOURCE_TYPE_TIMER, 0, 0, dispatch_get_main_queue());
	dispatch_set_context(patty_main_heartbeat_timer, NULL);
	dispatch_source_set_event_handler_f(patty_main_heartbeat_timer, patty_main_heartbeat_handler);
	dispatch_source_set_timer(patty_main_heartbeat_timer, dispatch_time(DISPATCH_TIME_NOW, 0), interval_ms * NSEC_PER_MSEC, 100 * NSEC_PER_MSEC);
	dispatch_resume(patty_main_heartbeat_timer);
}

static void patty_stop_main_heartbeat(void) {
	if (patty_main_heartbeat_timer == NULL) {
		return;
	}
	dispatch_source_cancel(patty_main_heartbeat_timer);
	patty_main_heartbeat_timer = NULL;
}
*/
import "C"

import "time"

func mainThreadWatchdogSupported() bool {
	return true
}

func startNativeMainThreadHeartbeat(intervalMS uint64) {
	C.patty_start_main_heartbeat(C.uint64_t(intervalMS))
}

func stopNativeMainThreadHeartbeat() {
	C.patty_stop_main_heartbeat()
}

//export pattyDesktopMainHeartbeat
func pattyDesktopMainHeartbeat() {
	recordMainThreadHeartbeat(time.Now())
}
