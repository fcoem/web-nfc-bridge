## Why

The project already defines a safe `ndef-v1` write profile, and the web flow can submit validated write requests, but macOS still lacked a physical PC/SC write path that could actually persist the payload onto a tag. That leaves an important gap between profile validation and real hardware behavior.

To close that gap, the Connector needs a controlled implementation for NDEF formatted Type 2 tags on macOS, including write verification before success is reported back to the website.

## What Changes

- Enable the macOS `pcsc` driver to write `ndef-v1` payloads onto NDEF formatted Type 2 tags.
- Verify written TLV bytes by reading the tag back before returning `accepted: true`.
- Return explicit rejection reasons for unsupported, read-only, or too-small tags.
- Update platform and operator documentation to record that macOS now has validated controlled NDEF write support.

## Capabilities

### Modified Capabilities

- `nfc-web-platform`: Adds the validated macOS PC/SC NDEF write path and the requirement to verify physical writes before reporting success.

## Impact

- Affected specs: `nfc-web-platform`
- Affected code: macOS PC/SC bridge driver, Type 2 / NDEF write helpers, connector operation docs, platform support docs
