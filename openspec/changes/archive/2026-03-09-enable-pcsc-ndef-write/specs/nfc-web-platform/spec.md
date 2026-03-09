# Delta for nfc-web-platform

## ADDED Requirements

### Requirement: Validated macOS PC/SC NDEF write path

The system SHALL support controlled physical `ndef-v1` writes on macOS when the Connector is using a validated PC/SC path with an NDEF formatted Type 2 tag.

#### Scenario: Writing a supported tag on macOS

- **GIVEN** the Connector is running on macOS with the `pcsc` driver
- **AND** the presented card is NDEF formatted, writable, and within the supported Type 2 capacity
- **WHEN** the website submits a valid `ndef-v1` write request
- **THEN** the Connector MUST write the encoded TLV payload onto the card through controlled page writes
- **AND** MUST return write metadata that identifies the driver, profile, and written byte counts

#### Scenario: Verifying a physical write before reporting success

- **GIVEN** the Connector has written the requested TLV payload to a supported Type 2 tag on macOS
- **WHEN** the Connector is about to return the write result to the website
- **THEN** it MUST read back the written byte range from the card
- **AND** MUST compare the read-back bytes with the expected TLV payload
- **AND** MUST NOT report `accepted: true` unless the read-back verification succeeds

#### Scenario: Rejecting unsupported tags on macOS

- **WHEN** the macOS `pcsc` driver detects that the presented card is not NDEF formatted, is read-only, or does not have enough data area for the requested TLV payload
- **THEN** the Connector MUST reject the write
- **AND** MUST return an explicit reason without exposing raw page or APDU access to the website
