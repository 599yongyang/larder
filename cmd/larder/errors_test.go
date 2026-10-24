package larder

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestErrorEnvelopeRoundTrip(t *testing.T) {
	err := messageError("common.executionFailed", "操作失败", map[string]string{"name": "demo"})
	raw := err.Error()
	if !strings.HasPrefix(raw, errorMarker) {
		t.Fatalf("missing marker: %q", raw)
	}
	var envelope ErrorEnvelope
	if err := json.Unmarshal([]byte(strings.TrimPrefix(raw, errorMarker)), &envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Code != "common.executionFailed" || envelope.Params["name"] != "demo" || envelope.Fallback != "操作失败" {
		t.Fatalf("unexpected envelope: %#v", envelope)
	}
}
