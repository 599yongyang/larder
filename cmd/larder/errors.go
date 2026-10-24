package larder

import (
	"encoding/json"
	"fmt"
)

const errorMarker = "larder-error:"

// ErrorEnvelope carries a stable message code plus optional params through the
// Wails error channel, which otherwise only transports an error string.
type ErrorEnvelope struct {
	Code     string            `json:"code"`
	Params   map[string]string `json:"params,omitempty"`
	Fallback string            `json:"fallback,omitempty"`
}

func (e *ErrorEnvelope) Error() string {
	data, err := json.Marshal(e)
	if err != nil {
		return e.Fallback
	}
	return errorMarker + string(data)
}

func messageError(code, fallback string, params map[string]string) error {
	return &ErrorEnvelope{Code: code, Params: params, Fallback: fallback}
}

func messageErrorf(code string, params map[string]string, fallback string, args ...any) error {
	return messageError(code, fmt.Sprintf(fallback, args...), params)
}
