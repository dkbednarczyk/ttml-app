//go:build linux

package ui

import (
	"fmt"
	"os"

	"fyne.io/fyne/v2"
	"github.com/rymdport/portal/settings"
)

// readSetting looks up one desktop setting, like settings.ReadOne.
type readSetting func(namespace, key string) (any, error)

// configureCursorTheme passes the desktop's cursor theme to GLFW. On Wayland,
// GLFW 3.4 only reads XCURSOR_THEME and XCURSOR_SIZE, which desktops such as
// Plasma don't set, so without this it falls back to its default cursor. The
// theme comes from the XDG desktop portal, which GNOME, Plasma and others
// serve, Flatpak included.
func configureCursorTheme() {
	configureCursorThemeFrom(settings.ReadOne)
}

func configureCursorThemeFrom(read readSetting) {
	setFromPortal(read, "XCURSOR_THEME", "cursor-theme")
	setFromPortal(read, "XCURSOR_SIZE", "cursor-size")
}

func setFromPortal(read readSetting, env, key string) {
	if os.Getenv(env) != "" {
		return // the session already set it
	}
	value, err := read("org.gnome.desktop.interface", key)
	if err != nil {
		return // no portal: GLFW keeps its default cursor
	}
	// The reply is a D-Bus variant (twice over from the portal's older Read call).
	for {
		variant, ok := value.(interface{ Value() any })
		if !ok {
			break
		}
		value = variant.Value()
	}
	if err := os.Setenv(env, fmt.Sprint(value)); err != nil { // a string theme, an int32 size
		fyne.LogError("set "+env, err)
	}
}
