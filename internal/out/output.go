package out

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

func WriteJSON(w io.Writer, v interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

type Error struct {
	Message string `json:"message"`
}

func WriteError(w io.Writer, asJSON bool, err error) error {
	if err == nil {
		return nil
	}

	if asJSON {
		return WriteJSON(w, Error{Message: err.Error()})
	}

	// Try to extract error message
	msg := err.Error()
	if idx := strings.Index(msg, ": "); idx != -1 {
		msg = msg[idx+2:]
	}
	_, writeErr := fmt.Fprintln(w, "Error:", msg)
	return writeErr
}
