// Package clipboard provides clipboard copy via OSC 52 escape sequences
// with a system-clipboard fallback.
package clipboard

import (
	"encoding/base64"
	"fmt"
	"io"

	system "github.com/atotto/clipboard"
)

// WriteOSC52 writes the OSC 52 escape sequence for clipboard copy to the given writer.
func WriteOSC52(w io.Writer, text string) error {
	encoded := base64.StdEncoding.EncodeToString([]byte(text))
	_, err := fmt.Fprintf(w, "\x1b]52;c;%s\x07", encoded)
	return err
}

// WriteSystem writes text to the host clipboard via the platform-native
// clipboard tool (pbcopy on macOS, xclip/xsel on Linux, etc.). This is
// preferred over OSC 52 because it bypasses tmux/terminal forwarding
// quirks that can silently corrupt or drop the escape payload.
func WriteSystem(text string) error {
	return system.WriteAll(text)
}
