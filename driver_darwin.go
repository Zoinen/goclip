//go:build darwin

package goclip

import (
	"runtime"
	"sync"
)

// DarwinDriver talks to NSPasteboard directly (see pasteboard_darwin.go) and
// falls back to pbcopy/pbpaste only when AppKit cannot be loaded.
type DarwinDriver struct {
	mu  sync.Mutex
	cli *CLIDriver
}

func NewDarwinDriver() *DarwinDriver {
	return &DarwinDriver{
		cli: NewCLIDriver(),
	}
}
func init() {
	RegisterDriverPriority(NewDarwinDriver(), true)
}

func (d *DarwinDriver) Name() string {
	return "darwin"
}

func (d *DarwinDriver) Available() bool {
	return runtime.GOOS == "darwin"
}

func (d *DarwinDriver) ReadText() (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if p, err := loadNativePasteboard(); err == nil {
		return p.readText()
	}
	return d.cli.ReadText()
}

func (d *DarwinDriver) WriteText(text string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if p, err := loadNativePasteboard(); err == nil {
		return p.writeText(text)
	}
	return d.cli.WriteText(text)
}

func (d *DarwinDriver) Clear() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if p, err := loadNativePasteboard(); err == nil {
		return p.clear()
	}
	return d.cli.Clear()
}
