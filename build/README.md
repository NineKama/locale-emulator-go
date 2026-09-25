# Build assets

Committed files in this directory are Wails application icons and Windows metadata.

Generated directories are ignored:

- `bin/`: four-file runtime package.
- `generated/`: intermediate engine DLLs and C export headers.
- `tests/`: unit-test executables, native fixtures and test results.
- `logs/`, `tmp/`, `reports/`: local diagnostics, temporary work and audit reports.

Use `scripts/build.ps1` from the project root. Installer templates are not maintained; distribute the four runtime files together as a portable package.
