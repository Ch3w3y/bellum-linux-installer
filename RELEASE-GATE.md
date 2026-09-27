# Release gate evidence

**No tag is authorized.** Keep each item unchecked until its underlying review
is complete for the exact candidate commit. Each completed HTTPS evidence URL
must contain the full 40-character candidate commit SHA. The gate script rejects
unchecked, placeholder and stale URLs. A URL alone is not an approval: the
required reviewer of GitHub's `release` environment must inspect the linked
record and verify its contents before allowing publication. Do not tag while
required-reviewer protection is absent.

QA: [ ] https://example.invalid/qa-evidence
EAC: [ ] https://example.invalid/eac-evidence
Security: [ ] https://example.invalid/security-evidence
Provenance: [ ] https://example.invalid/pin-evidence

Release candidates (`vX.Y.Z-rc.N`) are exempt: they publish as GitHub
pre-releases so the QA and EAC evidence above can be gathered on real
hardware (`install.sh --version vX.Y.Z-rc.N`). The one-line installer's
default path only follows the latest full release.

After an authorized release, download both archives and `SHA256SUMS`, run
`sha256sum --check --strict SHA256SUMS`, and verify each downloaded archive's
GitHub attestation with `gh attestation verify <archive> --repo
Ch3w3y/bellum-linux-installer`. Retain the command output, archive digests,
expected workflow, tag and source commit in the release review record.
