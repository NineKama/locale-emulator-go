# Publishing the project

Author attribution is retained in `AUTHORS.md`, both READMEs, and `wails.json`.
Personal contact information, workstation paths, private game fixture names,
unused template assets, and unsupported installer templates have been removed.

## Source review

Run these checks from the repository root before publishing:

```powershell
# Install Gitleaks separately and place it on PATH, or pass -Gitleaks <executable>.
./scripts/scan-source.ps1
git status --short --untracked-files=all
git diff --check
```

The scanner copies tracked and unignored files into a temporary local snapshot,
runs Gitleaks with redacted output, and checks for email addresses and Windows
home directory paths. It does not upload source. Reports go to ignored
`build/reports/`; the temporary snapshot is removed afterward. Review findings
individually, including any legitimate third-party attribution.

This checks the current source tree, not Git history, ignored files, or external
application data. If commits exist before a future publication, also run
`gitleaks git . --redact` to audit history. Automated scans cannot prove the
absence of every secret or personal detail; review the files being committed.
Never force-add private data or generated files just to bypass `.gitignore`.

## What belongs in Git

Commit source, tests, documentation, build metadata, and dependency lockfiles.
The ignore rules exclude executables, DLLs, generated Wails bindings, frontend
bundles, dependencies, logs, reports, caches, local environment files, common
credential files, and library history. Vietnamese UI text remains in locale
JSON resources; implementation code, comments, and default diagnostics use English.

Run the build and tests described in the main README. For a binary release,
zip the four runtime files in `build/bin` together and upload that archive to
GitHub Releases. Do not include test output, logs, the source audit snapshot,
or a user's application library. Build scripts use Go's `-trimpath` option
to avoid embedding local Go source paths.

## Repository and license

Choose a GitHub repository and review the commit contents before pushing.
No remote or automatic upload is configured by these scripts.
No project license has been selected. Choose a license before inviting reuse;
author attribution alone does not grant permission to reuse the code.
Preserve applicable dependency licenses when distributing binaries.
