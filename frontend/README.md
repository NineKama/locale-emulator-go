# Frontend

Svelte 5 + TypeScript + Vite 7, embedded in the Wails desktop application.

From the repository root, run a full `scripts/build.ps1` build to generate the Wails bindings. Then:

```powershell
npm --prefix frontend run check
npm --prefix frontend run build
```

`src/App.svelte` owns the library and launch UI. `src/i18n.ts` implements language selection; `src/locales/*.json` contains translation data. Generated `wailsjs/`, installed `node_modules/` and compiled `dist/` assets are not source files to commit (except the empty dist placeholder).

Use the root [README](../README.md) for prerequisites and complete build instructions.
