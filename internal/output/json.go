package output

import (
    "encoding/json"
    "fmt"
    "io"
)

func WriteJSON(w io.Writer, value any) error {
    enc := json.NewEncoder(w)
    enc.SetIndent("", "  ")
    if err := enc.Encode(value); err != nil {
        return fmt.Errorf("encode JSON: %w", err)
    }
    return nil
}
