## Context

The existing bridge change established how the website, localhost Connector, and PC/SC reader communicate, and the current Connector intentionally rejects real writes unless a safe profile is defined. A dedicated write profile is now needed so that the product can move from read-only validation to controlled, interoperable writes without exposing raw card memory operations.

## Goals / Non-Goals

**Goals:**

- Define a first-version write format that is small, portable, and easy to validate.
- Keep v1 writes compatible with NDEF-capable tags and simple enough for web and Connector implementations.
- Enforce a safe write policy that rejects sensitive or unbounded payloads.
- Preserve a clear migration path from demo payloads to production reference tokens.

**Non-Goals:**

- Supporting arbitrary raw block writes or unrestricted APDU passthrough.
- Supporting every NFC card family in v1.
- Storing large business payloads or sensitive data directly on the card.
- Solving card authentication key management for non-NDEF secure cards in this change.

## Decisions

### Decision 1: Support NDEF-compatible tags first

Version 1 will target NDEF-compatible tags as the only supported write target. This keeps the first write profile interoperable across common tools and avoids card-family-specific memory layout rules. Alternatives considered: raw page or sector writes were rejected because they couple the product to individual tag families and increase the risk of destructive writes.

### Decision 2: Carry a compact application/json payload inside NDEF

Version 1 payloads will be encoded as NDEF records with an `application/json` media type. JSON was selected because it is easy to inspect during development and easy to validate in both the web app and Connector. Alternatives considered: CBOR and custom binary layouts would save space, but they make initial debugging and operator verification harder.

### Decision 3: Separate demo payloads from production reference payloads

Version 1 will support two payload classes: demo payloads for local verification and reference payloads for production use. Demo payloads allow fast end-to-end validation, while production payloads store only a short reference or signed token so that authoritative business data remains on the server. Alternative considered: storing full business data directly on the card was rejected because it creates update, revocation, and privacy problems.

### Decision 4: Enforce safe write policy in the Connector

The Connector will accept only explicitly defined write profiles and will reject unsupported payload types, oversized payloads, or sensitive fields. This preserves the existing security model in which the Connector exposes only high-level operations. Alternative considered: letting the web app build arbitrary payloads and write them directly was rejected because it weakens the localhost trust boundary.

### Decision 5: Require versioned payload metadata

Every accepted payload will include `version` and `type` fields, and reference payloads will additionally require a `token` field. This ensures forward compatibility and lets the Connector validate the schema before writing. Alternative considered: implicit schema detection was rejected because it makes migrations ambiguous.

## Risks / Trade-offs

- [Risk] JSON consumes more space than binary encodings. -> Mitigation: keep payloads intentionally small and define a strict size limit in the spec.
- [Risk] Some cards presented to the product will not be NDEF-compatible. -> Mitigation: treat unsupported cards as out of scope for v1 and return explicit rejection reasons.
- [Risk] Demo payloads could be mistaken for production-ready storage. -> Mitigation: separate demo and reference types in the schema and document that production data belongs on the server.
- [Risk] Future secure-card support may require a different write path. -> Mitigation: keep v1 scoped to NDEF so later secure-card work can be introduced as a separate profile rather than overloading this one.
