//go:build darwin

package bridge

import (
	"strings"
	"testing"
)

func TestDescribeType2WriteResponseForFirstDataPageReject(t *testing.T) {
	t.Parallel()

	message := describeType2WriteResponse(type2UserDataPage, []byte{0x63, 0x00})

	if !strings.Contains(message, "first data page 4") {
		t.Fatalf("expected page detail in message, got %q", message)
	}
	if !strings.Contains(message, "tag may be locked") {
		t.Fatalf("expected lock guidance in message, got %q", message)
	}
}

func TestDescribeType2WriteResponseFallback(t *testing.T) {
	t.Parallel()

	message := describeType2WriteResponse(7, []byte{0x6A, 0x81})

	if message != "unexpected write response at page 7: 6A 81" {
		t.Fatalf("unexpected message: %q", message)
	}
}