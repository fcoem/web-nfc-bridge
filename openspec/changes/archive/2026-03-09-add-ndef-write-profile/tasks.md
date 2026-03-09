## 1. Write Profile Definition

- [x] 1.1 Document Decision 1: Support NDEF-compatible tags first in product docs and Connector write policy notes
- [x] 1.2 Define the NDEF v1 write target capability in implementation-facing docs and validation logic
- [x] 1.3 Define Decision 2: Carry a compact application/json payload inside NDEF and implement the NDEF application/json payload schema for `nfc-tool/demo` and `nfc-tool/ref`

## 2. Safe Write Policy

- [x] 2.1 Implement Decision 4: Enforce safe write policy in the Connector request validation path
- [x] 2.2 Implement Decision 5: Require versioned payload metadata and validate the Versioned payload types requirement including required `version`, `type`, and `token` fields where applicable
- [x] 2.3 Add bounded payload size checks and disallowed content checks for the Safe write policy

## 3. Web Flow And Documentation

- [x] 3.1 Update the web write flow to submit only profile-based NDEF payloads
- [x] 3.2 Document Decision 3: Separate demo payloads from production reference payloads in user and operator docs
- [x] 3.3 Document write rejection behavior for unsupported tags and invalid payloads
