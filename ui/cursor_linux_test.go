//go:build linux

package ui

import (
	"errors"
	"os"
	"testing"
)

var errNoPortal = errors.New("no portal")

// variant stands in for dbus.Variant, which the portal's replies arrive in.
type variant struct{ value any }

func (v variant) Value() any { return v.value }

func fakePortal(theme, size any) readSetting {
	return func(namespace, key string) (any, error) {
		if namespace != "org.gnome.desktop.interface" {
			return nil, errNoPortal
		}
		switch key {
		case "cursor-theme":
			return theme, nil
		case "cursor-size":
			return size, nil
		}

		return nil, errNoPortal
	}
}

func TestConfigureCursorTheme(t *testing.T) {
	tests := []struct {
		name        string
		theme, size any
	}{
		{"ReadOne reply", variant{"breeze_cursors"}, variant{int32(24)}},
		{"older Read reply, wrapped twice", variant{variant{"breeze_cursors"}}, variant{variant{int32(24)}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("XCURSOR_THEME", "")
			t.Setenv("XCURSOR_SIZE", "")

			configureCursorThemeFrom(fakePortal(tt.theme, tt.size))
			if got := os.Getenv("XCURSOR_THEME"); got != "breeze_cursors" {
				t.Errorf("XCURSOR_THEME = %q, want %q", got, "breeze_cursors")
			}
			if got := os.Getenv("XCURSOR_SIZE"); got != "24" {
				t.Errorf("XCURSOR_SIZE = %q, want %q", got, "24")
			}
		})
	}
}

func TestConfigureCursorThemeKeepsSessionValues(t *testing.T) {
	t.Setenv("XCURSOR_THEME", "Adwaita")
	t.Setenv("XCURSOR_SIZE", "32")

	configureCursorThemeFrom(fakePortal(variant{"breeze_cursors"}, variant{int32(24)}))
	if got := os.Getenv("XCURSOR_THEME"); got != "Adwaita" {
		t.Errorf("XCURSOR_THEME = %q, want the session's %q", got, "Adwaita")
	}
	if got := os.Getenv("XCURSOR_SIZE"); got != "32" {
		t.Errorf("XCURSOR_SIZE = %q, want the session's %q", got, "32")
	}
}

func TestConfigureCursorThemeWithoutPortal(t *testing.T) {
	t.Setenv("XCURSOR_THEME", "")
	t.Setenv("XCURSOR_SIZE", "")

	configureCursorThemeFrom(func(string, string) (any, error) { return nil, errNoPortal })
	if os.Getenv("XCURSOR_THEME") != "" || os.Getenv("XCURSOR_SIZE") != "" {
		t.Fatal("set cursor variables with no portal to read from")
	}
}
