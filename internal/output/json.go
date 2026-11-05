package output

import (
    "encoding/json"
    "fmt"
    "io"
)

func WriteJSON(w io.Writer, value any) error {
    enc := json.NewEncoder(w)
    // Output is read by terminals and scripts, not embedded in HTML, so keep
    // paths such as /search?a=1&b=<x> literal instead of \u0026 and \u003c.
    enc.SetEscapeHTML(false)
    enc.SetIndent("", "  ")
    if err := enc.Encode(value); err != nil {
        return fmt.Errorf("encode JSON: %w", err)
    }
    return nil
}
