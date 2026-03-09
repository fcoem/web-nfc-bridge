package bridge

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	NDEFWriteOperation   = "ndef-v1"
	NDEFWriteProfile     = "ndef-write-profile/v1"
	NDEFApplicationJSON  = "application/json"
	NDEFDemoPayloadType  = "nfc-tool/demo"
	NDEFRefPayloadType   = "nfc-tool/ref"
	NDEFMaxPayloadBytes  = 256
	NDEFDemoMaxFields    = 8
	NDEFDemoMaxKeyBytes  = 32
	NDEFDemoMaxTextBytes = 96
)

var sensitiveFieldNames = []string{
	"password",
	"secret",
	"fullName",
	"name",
	"email",
	"phone",
	"address",
	"personalId",
	"nationalId",
	"accessToken",
	"refreshToken",
	"credential",
}

func ValidateWriteRequest(operation string, payload map[string]any) (*WriteRequest, error) {
	if operation == "" {
		operation = NDEFWriteOperation
	}
	if operation != NDEFWriteOperation {
		return nil, fmt.Errorf("unsupported write operation: %s", operation)
	}
	if len(payload) == 0 {
		return nil, errors.New("write payload is required")
	}

	normalized, err := normalizePayload(payload)
	if err != nil {
		return nil, err
	}

	version, err := readPayloadVersion(normalized["version"])
	if err != nil {
		return nil, err
	}
	if version != 1 {
		return nil, fmt.Errorf("unsupported payload version: %d", version)
	}

	payloadType, ok := normalized["type"].(string)
	if !ok || strings.TrimSpace(payloadType) == "" {
		return nil, errors.New("payload type is required")
	}

	allowedKeys, err := validatePayloadType(normalized, payloadType)
	if err != nil {
		return nil, err
	}

	if err := validateAllowedKeys(normalized, allowedKeys); err != nil {
		return nil, err
	}
	if err := validateDisallowedContent(normalized, payloadType); err != nil {
		return nil, err
	}

	encoded, err := json.Marshal(normalized)
	if err != nil {
		return nil, fmt.Errorf("unable to encode payload: %w", err)
	}
	if len(encoded) > NDEFMaxPayloadBytes {
		return nil, fmt.Errorf("payload exceeds safe limit of %d bytes", NDEFMaxPayloadBytes)
	}

	return &WriteRequest{
		Operation:      operation,
		Profile:        NDEFWriteProfile,
		MediaType:      NDEFApplicationJSON,
		PayloadType:    payloadType,
		Payload:        normalized,
		EncodedPayload: encoded,
	}, nil
}

func normalizePayload(payload map[string]any) (map[string]any, error) {
	normalized := make(map[string]any, len(payload))
	for key, value := range payload {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			return nil, errors.New("payload field names must not be empty")
		}
		normalized[trimmedKey] = value
	}
	return normalized, nil
}

func readPayloadVersion(value any) (int, error) {
	switch typed := value.(type) {
	case int:
		return typed, nil
	case int32:
		return int(typed), nil
	case int64:
		return int(typed), nil
	case float64:
		return int(typed), nil
	case json.Number:
		parsed, err := typed.Int64()
		return int(parsed), err
	default:
		return 0, errors.New("payload version is required")
	}
}

func validatePayloadType(payload map[string]any, payloadType string) (map[string]struct{}, error) {
	switch payloadType {
	case NDEFDemoPayloadType:
		return map[string]struct{}{
			"version":   {},
			"type":      {},
			"label":     {},
			"content":   {},
			"updatedAt": {},
		}, validateDemoPayload(payload)
	case NDEFRefPayloadType:
		return map[string]struct{}{
			"version":   {},
			"type":      {},
			"token":     {},
			"updatedAt": {},
		}, validateRefPayload(payload)
	default:
		return nil, fmt.Errorf("unsupported payload type: %s", payloadType)
	}
}

func validateDemoPayload(payload map[string]any) error {
	hasLabel := false
	if label, ok := payload["label"]; ok {
		labelValue, ok := label.(string)
		if !ok || strings.TrimSpace(labelValue) == "" {
			return errors.New("demo payload label must be a non-empty string when provided")
		}
		if len(labelValue) > 96 {
			return errors.New("demo payload label exceeds safe limit")
		}
		hasLabel = true
	}
	if content, ok := payload["content"]; ok {
		if err := validateDemoContent(content); err != nil {
			return err
		}
	} else if !hasLabel {
		return errors.New("demo payload requires label or content")
	}
	if updatedAt, ok := payload["updatedAt"]; ok {
		if updatedAtValue, ok := updatedAt.(string); !ok || strings.TrimSpace(updatedAtValue) == "" {
			return errors.New("demo payload updatedAt must be a non-empty string when provided")
		}
	}
	return nil
}

func validateDemoContent(value any) error {
	content, ok := value.(map[string]any)
	if !ok {
		return errors.New("demo payload content must be a JSON object")
	}
	if len(content) == 0 {
		return errors.New("demo payload content must not be empty")
	}
	if len(content) > NDEFDemoMaxFields {
		return fmt.Errorf("demo payload content exceeds safe field limit of %d", NDEFDemoMaxFields)
	}

	for key, value := range content {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey == "" {
			return errors.New("demo payload content field names must not be empty")
		}
		if len(trimmedKey) > NDEFDemoMaxKeyBytes {
			return fmt.Errorf("demo payload content field exceeds safe key limit: %s", trimmedKey)
		}
		if err := validateDemoContentValue(trimmedKey, value); err != nil {
			return err
		}
	}

	return nil
}

func validateDemoContentValue(key string, value any) error {
	switch typed := value.(type) {
	case string:
		if strings.TrimSpace(typed) == "" {
			return fmt.Errorf("demo payload content field must be a non-empty string: %s", key)
		}
		if len(typed) > NDEFDemoMaxTextBytes {
			return fmt.Errorf("demo payload content field exceeds safe text limit: %s", key)
		}
		return nil
	case bool, int, int8, int16, int32, int64, float32, float64, json.Number:
		return nil
	case nil:
		return nil
	default:
		return fmt.Errorf("demo payload content field must use primitive JSON values: %s", key)
	}
}

func validateRefPayload(payload map[string]any) error {
	token, ok := payload["token"].(string)
	if !ok || strings.TrimSpace(token) == "" {
		return errors.New("reference payload token is required")
	}
	if len(token) > 128 {
		return errors.New("reference payload token exceeds safe limit")
	}
	if updatedAt, ok := payload["updatedAt"]; ok {
		if updatedAtValue, ok := updatedAt.(string); !ok || strings.TrimSpace(updatedAtValue) == "" {
			return errors.New("reference payload updatedAt must be a non-empty string when provided")
		}
	}
	return nil
}

func validateAllowedKeys(payload map[string]any, allowedKeys map[string]struct{}) error {
	for key := range payload {
		if _, ok := allowedKeys[key]; !ok {
			return fmt.Errorf("payload field is not allowed in v1 profile: %s", key)
		}
	}
	return nil
}

func validateDisallowedContent(payload map[string]any, payloadType string) error {
	for key, value := range payload {
		if err := validateFieldName(key, payloadType, key); err != nil {
			return err
		}
		if err := validateNestedContent(value, payloadType, key); err != nil {
			return err
		}
	}
	return nil
}

func validateNestedContent(value any, payloadType string, path string) error {
	switch typed := value.(type) {
	case map[string]any:
		for key, nestedValue := range typed {
			nestedPath := path + "." + key
			if err := validateFieldName(key, payloadType, nestedPath); err != nil {
				return err
			}
			if err := validateNestedContent(nestedValue, payloadType, nestedPath); err != nil {
				return err
			}
		}
	case []any:
		for index, item := range typed {
			if err := validateNestedContent(item, payloadType, fmt.Sprintf("%s[%d]", path, index)); err != nil {
				return err
			}
		}
	}

	return nil
}

func validateFieldName(key string, payloadType string, path string) error {
	for _, sensitive := range sensitiveFieldNames {
		if strings.EqualFold(key, sensitive) {
			if payloadType == NDEFRefPayloadType && key == "token" {
				return nil
			}
			return fmt.Errorf("payload field is disallowed for safe write policy: %s", path)
		}
	}

	return nil
}