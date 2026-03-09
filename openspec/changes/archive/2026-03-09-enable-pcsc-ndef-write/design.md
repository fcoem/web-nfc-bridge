## Context

The Connector already validates `ndef-v1` payloads and exposes a controlled write API, but prior behavior stopped before any physical page write. Real hardware validation on macOS with ACR1252U-M1 showed that the reader can access NDEF formatted Type 2 tags through PC/SC page read and page write commands, with some device-specific constraints around read lengths.

## Goals / Non-Goals

**Goals:**

- Enable physical `ndef-v1` writes on macOS for NDEF formatted Type 2 tags.
- Keep the write path constrained to the existing safe write profile rather than exposing raw block access.
- Verify write success by reading back the written TLV bytes before reporting success.
- Return explicit operator-visible rejection reasons when the tag is unsupported, read-only, or too small.

**Non-Goals:**

- Adding arbitrary raw APDU passthrough.
- Supporting every NFC card family on macOS in this change.
- Extending the same write path to Windows or Linux before those platforms are validated.

## Decisions

### Decision 1: Use Type 2 page I/O behind the existing `ndef-v1` API

The macOS `pcsc` driver will keep the public API at the `ndef-v1` profile level and translate it internally into Type 2 page reads and page writes. This preserves the existing Connector security boundary while making the driver-specific details an internal concern.

### Decision 2: Require NDEF formatting and reject unsupported cards early

Before writing, the driver will read the Capability Container and confirm that the card is NDEF formatted, writable, and large enough for the generated TLV. This avoids partial writes onto cards that do not match the v1 support boundary.

### Decision 3: Verify the physical write by read-back before reporting success

The driver will read back the same byte range after writing and compare it with the expected TLV payload. The Connector will only return `accepted: true` after this verification succeeds. This avoids reporting success based only on intermediate write acknowledgements.

### Decision 4: Adapt reads to the observed ACR1252U-M1 behavior on macOS

Hardware probing showed that some short or oversized PC/SC reads can return `63 00` even though nearby ranges succeed. The implementation therefore uses a compatible Capability Container read length and chunked read-back verification rather than assuming a single fixed read length works for all ranges.

## Risks / Trade-offs

- [Risk] The implementation is intentionally limited to NDEF formatted Type 2 tags. -> Mitigation: return explicit rejection reasons and keep broader card-family support as future work.
- [Risk] Reader-specific I/O behavior may differ across platforms. -> Mitigation: scope this change to macOS validation only and keep Windows/Linux support pending separate verification.
- [Risk] Read-back verification adds extra I/O latency. -> Mitigation: keep payloads bounded and read in small compatible chunks.
