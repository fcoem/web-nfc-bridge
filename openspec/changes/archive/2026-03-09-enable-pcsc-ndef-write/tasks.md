## 1. macOS PC/SC physical write path

- [x] 1.1 Implement Requirement: Validated macOS PC/SC NDEF write path by applying Decision 1: Use Type 2 page I/O behind the existing `ndef-v1` API
- [x] 1.2 Apply Decision 2: Require NDEF formatting and reject unsupported cards early for unsupported, read-only, or undersized tags
- [x] 1.3 Apply Decision 3: Verify the physical write by read-back before reporting success and return `accepted: true` only after verification passes

## 2. Documentation and validation

- [x] 2.1 Update operator and platform documentation to record Requirement: Validated macOS PC/SC NDEF write path
- [x] 2.2 Validate the Connector against real hardware, apply Decision 4: Adapt reads to the observed ACR1252U-M1 behavior on macOS, and capture the verification result in docs
