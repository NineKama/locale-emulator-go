import en from './locales/en.json'
import vi from './locales/vi.json'
import vietnameseDiagnostics from './locales/diagnostics.vi.json'

export type Language = 'vi' | 'en'
const storageKey = 'locale-studio.language'

export function readLanguage(): Language {
  try {
    const saved = localStorage.getItem(storageKey)
    if (saved === 'vi' || saved === 'en') return saved
  } catch {
    /* Storage may be disabled by the host. */
  }
  return navigator.language.toLowerCase().startsWith('vi') ? 'vi' : 'en'
}

export function saveLanguage(language: Language) {
  try {
    localStorage.setItem(storageKey, language)
  } catch {
    /* Keep the session selection. */
  }
}

// English defines the message schema. Additional translations must supply the
// same keys; translation content lives in JSON rather than application logic.
export const copy = { en, vi } satisfies Record<Language, typeof en>

export function launchMessage(
  language: Language,
  pid: number,
  architecture: string,
): string {
  return copy[language].success
    .replace('{pid}', String(pid))
    .replace('{architecture}', architecture)
}

// Engine/CLI diagnostics are always English. Translate only application-owned
// fragments for the UI, retaining paths and native Windows error details.
export function diagnostic(error: string, language: Language): string {
  if (language === 'en') return error
  return Object.entries(vietnameseDiagnostics).reduce(
    (text, [en, vi]) => text.replaceAll(en, vi),
    error,
  )
}
