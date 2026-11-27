package httpapi

import "testing"

func TestAllowedMediaType(t *testing.T) {
	allowed := []string{"image/png", "video/mp4", "audio/mpeg", "application/pdf", "application/octet-stream", "text/plain"}
	for _, value := range allowed {
		if !allowedMediaType(value) {
			t.Errorf("expected %q to be allowed", value)
		}
	}
	if allowedMediaType("text/html") {
		t.Fatal("HTML must not be accepted as an upload type")
	}
}
