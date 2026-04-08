package clipboard

import (
	"encoding/base64"
	"fmt"
	"io"
)

// WriteOSC52 writes the OSC 52 escape sequence for clipboard copy to the given writer.
func WriteOSC52(w io.Writer, text string) error {
	encoded := base64.StdEncoding.EncodeToString([]byte(text))
	_, err := fmt.Fprintf(w, "\x1b]52;c;%s\x07", encoded)
	return err
}
