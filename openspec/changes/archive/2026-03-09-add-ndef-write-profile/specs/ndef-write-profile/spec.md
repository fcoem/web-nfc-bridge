## ADDED Requirements

### Requirement: NDEF v1 write target

The system SHALL support first-version write operations only for NDEF-compatible tags.

#### Scenario: Supported tag is presented for write

- **WHEN** the website requests a write using the v1 profile and the detected card is NDEF-compatible
- **THEN** the Connector MUST continue payload validation for the NDEF write flow

#### Scenario: Unsupported tag is presented for write

- **WHEN** the website requests a write using the v1 profile and the detected card is not NDEF-compatible
- **THEN** the Connector MUST reject the write
- **AND** MUST return a reason that the card is outside the supported write profile

### Requirement: NDEF application/json payload schema

The system SHALL encode version 1 write payloads as NDEF records whose media type is `application/json`.

#### Scenario: Writing a demo payload

- **WHEN** the website submits a demo payload for a v1 write
- **THEN** the payload MUST include `version` and `type`
- **AND** the Connector MUST encode the payload as an NDEF `application/json` record before writing

#### Scenario: Writing a reference payload

- **WHEN** the website submits a reference payload for a v1 write
- **THEN** the payload MUST include `version`, `type`, and `token`
- **AND** the Connector MUST reject the request if any required field is missing

### Requirement: Safe write policy

The system SHALL allow only bounded, non-sensitive payloads in the v1 write profile.

#### Scenario: Payload exceeds safe limits

- **WHEN** the submitted payload exceeds the configured size limit for the v1 write profile
- **THEN** the Connector MUST reject the write
- **AND** MUST return a validation error without attempting a card write

#### Scenario: Payload contains disallowed content

- **WHEN** the submitted payload contains fields or content classes marked as disallowed for the v1 write profile
- **THEN** the Connector MUST reject the write
- **AND** MUST not expose a lower-level raw write escape hatch

### Requirement: Versioned payload types

The system SHALL distinguish demo payloads from production reference payloads through explicit versioned metadata.

#### Scenario: Demo payload is accepted

- **WHEN** the payload uses `version: 1` and `type: nfc-tool/demo`
- **THEN** the Connector MUST treat the payload as a non-production demo payload
- **AND** MUST validate it against the demo schema

#### Scenario: Reference payload is accepted

- **WHEN** the payload uses `version: 1` and `type: nfc-tool/ref`
- **THEN** the Connector MUST treat the payload as a production reference payload
- **AND** MUST validate that the payload stores only the allowed reference fields for version 1
