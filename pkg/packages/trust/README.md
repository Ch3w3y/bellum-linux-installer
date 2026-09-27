# Extra code-signing trust anchors

Linux CA bundles follow Mozilla's TLS root store, which leaves out roots that
only issue code-signing certificates. The Astarte Launcher is signed through
one of those, so its root ships here and is added to the system roots for
launcher verification only.

Each `.pem` file must be listed with its SHA-256 fingerprint in
`pinnedRoots` in `pkg/packages/trust.go`; the tests fail otherwise.

| File | Certificate | Source |
| --- | --- | --- |
| `globalsign-code-signing-root-r45.pem` | GlobalSign Code Signing Root R45 | <https://secure.globalsign.com/cacert/codesigningrootr45.crt> |
