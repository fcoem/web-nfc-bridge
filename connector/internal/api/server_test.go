package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"nfc-tool/connector/internal/bridge"
)

func TestHealthEchoesOriginalOriginWithTrailingDot(t *testing.T) {
	t.Parallel()

	driver := bridge.NewMockDriver("Mock Reader")
	t.Cleanup(func() {
		_ = driver.Close()
	})

	service := bridge.NewService(driver)
	server := NewServer(service, []string{"https://nfc.yudefine.com.tw"}, "test-secret", "test-version", "test-build-time")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.Header.Set("Origin", "https://nfc.yudefine.com.tw.")
	recorder := httptest.NewRecorder()

	server.Handler().ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, recorder.Code)
	}

	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "https://nfc.yudefine.com.tw." {
		t.Fatalf("expected CORS origin to echo request origin, got %q", got)
	}
}