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
The release workflow below uploads build assets only when run for a tag.
The project uses the MIT License in `LICENSE`;
author attribution alone does not grant permission to reuse the code.
Preserve applicable dependency licenses when distributing binaries.

## Automated ZIP releases

Push a version tag after committing and pushing the workflow:

```bash
git tag v0.2.0
git push origin v0.2.0
```

The `Windows release` workflow builds on Windows, runs x86/x64 unit and native
integration tests, checks the frontend, and publishes a GitHub Release only after
all checks pass. Tags containing a hyphen (for example `v0.3.0-beta.1`) produce
prereleases. Use a new version tag for each published release.

Users download `Locale-Studio-<version>-windows-x64.zip`, extract it, and open
`locale-emulator-go.exe`. The archive contains the four runtime files plus the
project license and author attribution. The x64 desktop app supports both x86
and x64 target applications. WebView2 Runtime is required. A companion `.sha256`
file is published for download verification.

You can also run **Actions > Windows release > Run workflow** on `main` to build
a downloadable artifact without publishing a release. A manual run on a version
tag publishes that tag. No personal access token is required: only the release
job receives `contents: write` through GitHub's built-in token.

To package an existing local build:

```powershell
./scripts/package.ps1 -Version v0.2.0
```

Output goes to ignored `build/release/`. The first GitHub run still needs to be
verified on the hosted runner; local packaging validation does not exercise
GitHub permissions or the hosted environment.
