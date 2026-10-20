//go:build darwin

package goclip

// These run against the real pasteboard of the logged-in user. Whatever text
// was on it is put back afterwards.

import (
	"errors"
	"os/exec"
	"testing"
)

// The text of unxed/f4#2057, plus what a C string or a narrow encoding would
// lose: a NUL, a character outside the BMP, and right-to-left text.
const pasteboardSample = "Лунобот-2 (node 19d368003c6c048e43dbec70; LNX)\x00 🚀 日本語 العربية"

// localeOfFar2l is the environment of a process started from far2l's
// terminal: far2l.app comes from launchd with no LANG, and the shell it runs
// is not a login shell, so /etc/zprofile never sets one.
func localeOfFar2l(t *testing.T) {
	t.Helper()
	t.Setenv("LANG", "")
	t.Setenv("LC_ALL", "")
	t.Setenv("LC_CTYPE", "")
	t.Setenv("__CF_USER_TEXT_ENCODING", "0x1F5:0x0:0x0")
}

func needPasteboard(t *testing.T) *nativePasteboard {
	t.Helper()
	p, err := loadNativePasteboard()
	if err != nil {
		t.Fatalf("AppKit is part of every macOS, yet it did not load: %v", err)
	}
	saved, err := p.readText()
	switch {
	case errors.Is(err, ErrUnavailable):
		t.Skip("no pasteboard server in this session")
	case err == nil:
		t.Cleanup(func() { _ = p.writeText(saved) })
	default:
		t.Cleanup(func() { _ = p.clear() })
	}
	return p
}

func TestDarwinRoundTrip(t *testing.T) {
	needPasteboard(t)
	localeOfFar2l(t)
	d := NewDarwinDriver()

	if err := d.WriteText(pasteboardSample); err != nil {
		t.Fatalf("copy: %v", err)
	}
	got, err := d.ReadText()
	if err != nil {
		t.Fatalf("paste: %v", err)
	}
	if got != pasteboardSample {
		t.Errorf("clipboard: got %q, want %q", got, pasteboardSample)
	}
}

// What the driver puts on the pasteboard is what other applications see:
// pbpaste, in a UTF-8 locale, reads back the same text.
func TestDarwinWritesUnicodeForOthers(t *testing.T) {
	needPasteboard(t)
	localeOfFar2l(t)
	const text = "Лунобот-2 🚀"
	if err := NewDarwinDriver().WriteText(text); err != nil {
		t.Fatalf("copy: %v", err)
	}
	cmd := exec.Command("pbpaste")
	cmd.Env = []string{"LANG=en_US.UTF-8"}
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("pbpaste: %v", err)
	}
	if string(out) != text {
		t.Errorf("pbpaste: got %q, want %q", out, text)
	}
}

// And what other applications put there reaches the driver intact.
func TestDarwinReadsUnicodeOfOthers(t *testing.T) {
	needPasteboard(t)
	const text = "Лунобот-2 🚀"
	cmd := exec.Command("pbcopy")
	cmd.Env = []string{"LANG=en_US.UTF-8"}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatalf("pbcopy: %v", err)
	}
	_, _ = stdin.Write([]byte(text))
	_ = stdin.Close()
	if err := cmd.Wait(); err != nil {
		t.Fatalf("pbcopy: %v", err)
	}

	localeOfFar2l(t)
	got, err := NewDarwinDriver().ReadText()
	if err != nil {
		t.Fatalf("paste: %v", err)
	}
	if got != text {
		t.Errorf("clipboard: got %q, want %q", got, text)
	}
}

// The fallback has to survive the same environment: before utf8LocaleEnv,
// pbpaste here gave "???????-2".
func TestDarwinCLIFallbackIgnoresLocale(t *testing.T) {
	needPasteboard(t)
	localeOfFar2l(t)
	t.Setenv("LANG", "C")
	t.Setenv("LC_COLLATE", "C")
	cli := NewCLIDriver()

	const text = "Лунобот-2 🚀"
	if err := cli.WriteText(text); err != nil {
		t.Fatalf("pbcopy: %v", err)
	}
	got, err := cli.ReadText()
	if err != nil {
		t.Fatalf("pbpaste: %v", err)
	}
	if got != text {
		t.Errorf("pbcopy/pbpaste: got %q, want %q", got, text)
	}
}

// Bytes that are not UTF-8 still reach the pasteboard, with the bad ones
// replaced, instead of silently leaving the old contents in place.
func TestDarwinWritesInvalidUTF8(t *testing.T) {
	needPasteboard(t)
	d := NewDarwinDriver()
	if err := d.WriteText("ok\xffok"); err != nil {
		t.Fatalf("copy: %v", err)
	}
	got, err := d.ReadText()
	if err != nil {
		t.Fatalf("paste: %v", err)
	}
	if got != "ok�ok" {
		t.Errorf("clipboard: got %q, want %q", got, "ok�ok")
	}
}

func TestDarwinEmptyClipboard(t *testing.T) {
	needPasteboard(t)
	d := NewDarwinDriver()
	if err := d.Clear(); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if _, err := d.ReadText(); err != ErrEmpty {
		t.Errorf("an empty clipboard reads as ErrEmpty, got %v", err)
	}
	if err := d.WriteText(""); err != nil {
		t.Fatalf("copy of nothing: %v", err)
	}
	if _, err := d.ReadText(); err != ErrEmpty {
		t.Errorf("an empty string reads as ErrEmpty, got %v", err)
	}
}
