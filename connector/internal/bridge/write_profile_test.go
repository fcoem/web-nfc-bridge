package bridge

import "testing"

func TestValidateWriteRequestAcceptsDemoContent(t *testing.T) {
	t.Parallel()

	request, err := ValidateWriteRequest("ndef-v1", map[string]any{
		"version": 1,
		"type":    NDEFDemoPayloadType,
		"content": map[string]any{
			"documentNo":  "MO-20260309-001",
			"itemCode":    "FG-1001",
			"workstation": "PACK-01",
			"quantity":    24,
			"status":      "ready",
		},
		"updatedAt": "2026-03-09T10:10:00Z",
	})
	if err != nil {
		t.Fatalf("expected request to be accepted, got error: %v", err)
	}
	if request.PayloadType != NDEFDemoPayloadType {
		t.Fatalf("expected payload type %q, got %q", NDEFDemoPayloadType, request.PayloadType)
	}
}

func TestValidateWriteRequestRejectsSensitiveDemoContent(t *testing.T) {
	t.Parallel()

	_, err := ValidateWriteRequest("ndef-v1", map[string]any{
		"version": 1,
		"type":    NDEFDemoPayloadType,
		"content": map[string]any{
			"documentNo": "MO-20260309-001",
			"email":      "ops@example.com",
		},
	})
	if err == nil {
		t.Fatal("expected sensitive field to be rejected")
	}
	if err.Error() != "payload field is disallowed for safe write policy: content.email" {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateWriteRequestRejectsNestedDemoContent(t *testing.T) {
	t.Parallel()

	_, err := ValidateWriteRequest("ndef-v1", map[string]any{
		"version": 1,
		"type":    NDEFDemoPayloadType,
		"content": map[string]any{
			"documentNo": "MO-20260309-001",
			"meta": map[string]any{
				"shift": "A",
			},
		},
	})
	if err == nil {
		t.Fatal("expected nested object to be rejected")
	}
	if err.Error() != "demo payload content field must use primitive JSON values: meta" {
		t.Fatalf("unexpected error: %v", err)
	}
}