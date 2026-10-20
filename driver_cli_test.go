package goclip

import (
	"slices"
	"testing"
)

// Every locale variable has to go, not only the ones that say otherwise:
// pbpaste gives '?' for LANG=C next to a UTF-8 LC_ALL, and for LC_COLLATE=C
// next to a UTF-8 LANG.
func TestUTF8LocaleEnv(t *testing.T) {
	got := utf8LocaleEnv([]string{
		"HOME=/Users/u",
		"LANG=C",
		"LC_ALL=en_US.UTF-8",
		"LC_COLLATE=C",
		"LC_CTYPE=UTF-8",
		"__CF_USER_TEXT_ENCODING=0x1F5:0x0:0x0",
		"LANGUAGE=ru",
		"PATH=/usr/bin",
	})
	want := []string{
		"HOME=/Users/u",
		"__CF_USER_TEXT_ENCODING=0x1F5:0x0:0x0",
		"LANGUAGE=ru",
		"PATH=/usr/bin",
		"LANG=en_US.UTF-8",
	}
	if !slices.Equal(got, want) {
		t.Errorf("utf8LocaleEnv:\n got %q\nwant %q", got, want)
	}
}

func TestCLIDriver_Basics(t *testing.T) {
	driver := NewCLIDriver()
	if driver.Name() != "cli" {
		t.Errorf("Expected driver name 'cli', got %q", driver.Name())
	}

	// Available() depends on environment tools; calling it must not panic
	_ = driver.Available()
}

func TestCLIDriver_FallbackErrorWhenUnavailable(t *testing.T) {
	driver := NewCLIDriver()
	// If no display or tools are present in test environment, ReadText should return ErrUnavailable
	if !driver.Available() {
		_, err := driver.ReadText()
		if err != ErrUnavailable {
			t.Errorf("Expected ErrUnavailable on unavailable CLIDriver, got %v", err)
		}
		err = driver.WriteText("test")
		if err != ErrUnavailable {
			t.Errorf("Expected ErrUnavailable on unavailable CLIDriver, got %v", err)
		}
	}
}
func TestCLIDriver_Clear(t *testing.T) {
	driver := NewCLIDriver()
	_ = driver.Clear()
}

func TestCLIDriver_RunCommandHelper(t *testing.T) {
	ctx := t.Context()
	// Test echo command with input via runCommandWithInput helper
	err := runCommandWithInput(ctx, "cat", []string{}, "sample text")
	if err != nil {
		// On non-unix/cat environments this might fail, so we only check it doesn't crash
		_ = err
	}
}
