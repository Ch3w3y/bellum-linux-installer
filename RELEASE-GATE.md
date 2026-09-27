# Release gate evidence

**No tag is authorized.** Keep each item unchecked until its underlying review
is complete for the exact candidate commit: the commit of the release
candidate (`vX.Y.Z-rc.N`) that was tested. Record that commit's full SHA on the
`Candidate:` line; each completed HTTPS evidence URL must contain it. Evidence
records live under `docs/release-evidence/<candidate SHA>/`. The gate script
rejects unchecked, placeholder and stale URLs, and any release commit that
differs from the candidate in anything but this file and those records. A URL
alone is not an approval: the required reviewer of GitHub's `release` environment must inspect the linked
record and verify its contents before allowing publication. Do not tag while
required-reviewer protection is absent.

Candidate: 59ae4837bfde5a72e010741cc8792e6b08993f4a

QA: [ ] https://example.invalid/qa-evidence
EAC: [ ] https://example.invalid/eac-evidence
Security: [x] https://github.com/Ch3w3y/bellum-linux-installer/blob/main/docs/release-evidence/59ae4837bfde5a72e010741cc8792e6b08993f4a/security.md
Provenance: [x] https://github.com/Ch3w3y/bellum-linux-installer/blob/main/docs/release-evidence/59ae4837bfde5a72e010741cc8792e6b08993f4a/provenance.md

Release candidates (`vX.Y.Z-rc.N`) are exempt: they publish as GitHub
pre-releases so the QA and EAC evidence above can be gathered on real
hardware (`install.sh --version vX.Y.Z-rc.N`). Releases can be started by
pushing the tag or from the Actions tab (**Tagged release** → Run workflow →
tag), which creates the tag at the chosen branch's head. The one-line installer's
default path only follows the latest full release.

After an authorized release, download both archives and `SHA256SUMS`, run
`sha256sum --check --strict SHA256SUMS`, and verify each downloaded archive's
GitHub attestation with `gh attestation verify <archive> --repo
Ch3w3y/bellum-linux-installer`. Retain the command output, archive digests,
expected workflow, tag and source commit in the release review record.
