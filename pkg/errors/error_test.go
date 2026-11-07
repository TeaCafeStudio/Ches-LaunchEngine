package errors

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestErrorSerializationRedactsSensitiveDetailsAndCause(t *testing.T) {
	engineErr := Wrap(CodeAuthenticationFailed, "authentication failed", &testCause{"private cause"})
	engineErr.Details = map[string]string{
		"accessToken": "private-token",
		"account":     "player",
	}

	encoded, err := json.Marshal(engineErr)
	if err != nil {
		t.Fatal(err)
	}
	output := string(encoded)
	for _, privateValue := range []string{"private-token", "private cause"} {
		if strings.Contains(output, privateValue) {
			t.Fatalf("serialized error exposes %q: %s", privateValue, output)
		}
	}
	if !strings.Contains(output, "[REDACTED]") || !strings.Contains(output, "player") {
		t.Fatalf("serialized error is missing expected details: %s", output)
	}
}

type testCause struct {
	message string
}

func (cause *testCause) Error() string {
	return cause.message
}
