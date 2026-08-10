package main

import (
	"sync"

	"fyne.io/systray"
)

type desktopTray struct {
	end       func()
	openItem  *systray.MenuItem
	quitItem  *systray.MenuItem
	once      sync.Once
	ready     chan struct{}
	readyOnce sync.Once
}

func newDesktopTray() *desktopTray {
	return &desktopTray{ready: make(chan struct{})}
}

func (t *desktopTray) markReady() {
	t.readyOnce.Do(func() {
		close(t.ready)
	})
}

func (a *App) startTray() bool {
	if !traySupported() {
		return false
	}
	a.mu.Lock()
	if a.tray != nil {
		a.mu.Unlock()
		return true
	}
	t := newDesktopTray()
	a.tray = t
	a.mu.Unlock()

	end := startDesktopTray(func() {
		systray.SetIcon(trayIconBytes)
		systray.SetTitle("Patty Code")
		systray.SetTooltip("Patty Code")
		systray.SetOnTapped(func() { a.goSafe("showFromTray", a.showFromTray) })
		systray.SetOnSecondaryTapped(nil)

		labels := trayMenuLabels(a.trayLocale())
		openItem := systray.AddMenuItem(labels.openTitle, labels.openTooltip)
		quitItem := systray.AddMenuItem(labels.quitTitle, labels.quitTooltip)

		a.mu.Lock()
		t.openItem = openItem
		t.quitItem = quitItem
		a.trayReady = true
		a.mu.Unlock()
		t.markReady()

		a.goSafe("trayOpenLoop", func() {
			for range openItem.ClickedCh {
				a.showFromTray()
			}
		})
		a.goSafe("trayQuitLoop", func() {
			for range quitItem.ClickedCh {
				a.quitFromTray()
			}
		})
	}, func() {
		a.mu.Lock()
		if a.tray == t {
			a.trayReady = false
			a.tray = nil
		}
		a.mu.Unlock()
	})
	a.mu.Lock()
	t.end = end
	a.mu.Unlock()
	return true
}

func (a *App) stopTray() {
	a.mu.RLock()
	t := a.tray
	var end func()
	if t != nil {
		end = t.end
	}
	a.mu.RUnlock()
	if t == nil || end == nil {
		return
	}
	t.once.Do(end)
}

func (a *App) updateTrayLocale(locale string) {
	a.mu.RLock()
	t := a.tray
	var openItem, quitItem *systray.MenuItem
	if t != nil {
		openItem = t.openItem
		quitItem = t.quitItem
	}
	a.mu.RUnlock()
	if openItem == nil || quitItem == nil {
		return
	}
	labels := trayMenuLabels(locale)
	openItem.SetTitle(labels.openTitle)
	openItem.SetTooltip(labels.openTooltip)
	quitItem.SetTitle(labels.quitTitle)
	quitItem.SetTooltip(labels.quitTooltip)
}

func (a *App) trayLocale() string {
	cfg, _, err := a.loadDesktopUserConfigForView()
	if err != nil {
		return ""
	}
	return cfg.DesktopLanguage()
}

func (a *App) showFromTray() {
	a.showMainWindowFrom("tray")
}

func (a *App) quitFromTray() {
	a.quitApp()
}

type trayLabels struct {
	openTitle   string
	openTooltip string
	quitTitle   string
	quitTooltip string
}

func trayMenuLabels(locale string) trayLabels {
	if locale == "ko-KR" {
		return trayLabels{
			openTitle:   "열기",
			openTooltip: "Patty Code 창 열기",
			quitTitle:   "종료",
			quitTooltip: "Patty Code 종료",
		}
	}
	return trayLabels{
		openTitle:   "Open",
		openTooltip: "Open the patty window",
		quitTitle:   "Quit",
		quitTooltip: "Quit Patty Code",
	}
}
