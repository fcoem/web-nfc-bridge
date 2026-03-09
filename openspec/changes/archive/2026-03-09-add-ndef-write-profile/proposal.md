## Why

The project now has a validated cross-platform web bridge and a working macOS read path, but it still lacks a product-defined write format. Without a formal write profile, the Connector can only reject writes safely and the team cannot implement interoperable, low-risk card writing behavior.

## What Changes

- Define a first-version NFC write profile based on NDEF records rather than raw block writes.
- Standardize a compact `application/json` payload shape with explicit `version` and `type` fields.
- Restrict v1 writes to non-sensitive demo payloads and server-resolved reference tokens.
- Define safe write boundaries, including payload size limits, rejected content classes, and unsupported card behavior.
- Document the migration path from demo JSON payloads to production token/reference payloads.

## Capabilities

### New Capabilities

- `ndef-write-profile`: Defines the first supported NFC write format, allowed payload types, and safe write policy for the product.

### Modified Capabilities

(none)

## Impact

- Affected specs: `ndef-write-profile`
- Affected code: Connector write policy, write payload validation, web write flow, product documentation
