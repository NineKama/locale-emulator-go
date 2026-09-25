# Third-party notices

Locale Studio's own code is licensed under [MIT](LICENSE). Dependencies retain
their original licenses and copyright notices; the project's MIT license does
not replace them.

Every portable ZIP includes `THIRD-PARTY-NOTICES.txt`, generated during packaging
by `scripts/third-party-notices.mjs`. A standalone copy accompanies the release.
The collector includes:

- Go modules recorded in the four release binaries, including nested notice files.
- The matching Go runtime's license, patents file, and notices under its source tree.
- Installed frontend packages from the npm lockfile, including build dependencies
  conservatively because packages such as Svelte contribute runtime bundle code.
- LLVM-MinGW or supported GCC/MinGW distribution notices, including runtime exceptions.

Original upstream texts are retained. Package names, versions and relative notice
paths identify their sources without including local workstation paths. The list
can include build-only components and notices for other platforms; inclusion does
not mean every listed component is shipped or executed by Locale Studio.

## Known upstream packaging exceptions

`is-reference@3.0.3` and `locate-character@3.0.0` declare MIT in their npm metadata
and README but contain no standalone license text. The collector preserves those
declarations and author/repository metadata and includes the standard MIT terms.
It does not invent a missing copyright year. These exceptions are limited to the
reviewed versions and need review when updating dependencies.

Native esbuild/Rollup npm packages use the license text from their matching-version
parent distribution, since the native packages omit a separate license file.

## Release maintenance

Packaging regenerates notices and stops on missing required dependency sources,
unrecognized missing license texts, local Go module replacements, a mismatched Go
toolchain, or unsupported compiler notice layouts. Run it with the same frontend
installation and compiler distributions used for the build. CI supplies its pinned
LLVM-MinGW distribution automatically.

Microsoft Windows and the separately installed WebView2 Runtime are not bundled
in the portable ZIP. Their respective terms still apply to their installation/use.

This collection is an attribution aid, not a complete legal compatibility audit.
Review license changes, additional source-file notices, assets and redistribution
conditions when introducing or upgrading dependencies. Preserve upstream notices
even when they include the original authors' contact information.
