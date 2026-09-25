<script lang="ts">
  import { onMount } from 'svelte'
  import { library } from '../wailsjs/go/models'
  import {
    SelectExecutable,
    Launch,
    GetLibrary,
    RemoveFromLibrary,
  } from '../wailsjs/go/main/App'
  import {
    copy,
    diagnostic,
    launchMessage,
    readLanguage,
    saveLanguage,
    type Language,
  } from './i18n'
  let language = $state<Language>(readLanguage())
  const t = $derived(copy[language])
  let target = $state('')
  let pending = $state<'choose' | 'run' | 'remove' | null>(null)
  let entries = $state<library.Entry[]>([])
  let query = $state('')
  let libraryLoading = $state(true)
  let libraryError = $state('')
  let saveWarning = $state(false)
  const filtered = $derived(
    entries.filter((entry) =>
      `${entry.name} ${entry.path}`
        .toLocaleLowerCase()
        .includes(query.trim().toLocaleLowerCase()),
    ),
  )
  onMount(() => {
    void refreshLibrary()
  })
  async function refreshLibrary() {
    libraryLoading = true
    try {
      entries = await GetLibrary()
      libraryError = ''
      saveWarning = false
    } catch (e) {
      libraryError = String(e)
      saveWarning = false
    } finally {
      libraryLoading = false
    }
  }
  async function removeEntry(path: string) {
    pending = 'remove'
    try {
      await RemoveFromLibrary(path)
      await refreshLibrary()
    } catch (e) {
      libraryError = String(e)
      saveWarning = false
    } finally {
      pending = null
    }
  }
  function selectEntry(path: string) {
    target = path
    status = 'ready'
    error = ''
  }
  function playedAt(date: unknown) {
    const value = new Date(String(date))
    return Number.isNaN(value.getTime())
      ? '—'
      : new Intl.DateTimeFormat(language === 'vi' ? 'vi-VN' : 'en-US', {
          dateStyle: 'medium',
          timeStyle: 'short',
        }).format(value)
  }
  let status = $state<
    'idle' | 'ready' | 'running' | 'success' | 'launchError' | 'dialogError'
  >('idle')
  let error = $state('')
  let pid = $state(0)
  let architecture = $state('x64')
  const failed = $derived(status === 'launchError' || status === 'dialogError')
  const message = $derived(
    status === 'success'
      ? launchMessage(language, pid, architecture)
      : t[status],
  )
  $effect(() => {
    document.documentElement.lang = language
    saveLanguage(language)
  })
  async function choose() {
    pending = 'choose'
    try {
      const path = await SelectExecutable(t.fileDialogTitle, t.executableFilter)
      if (path) {
        target = path
        error = ''
        status = 'ready'
      }
    } catch (e) {
      error = String(e)
      status = 'dialogError'
    } finally {
      pending = null
    }
  }
  async function run(path = target) {
    target = path
    pending = 'run'
    error = ''
    status = 'running'
    try {
      const result = await Launch(target)
      pid = result.pid
      architecture = result.architecture === '386' ? 'x86' : 'x64'
      status = 'success'
      await refreshLibrary()
      if (result.libraryWarning) {
        libraryError = result.libraryWarning
        saveWarning = true
      }
    } catch (e) {
      error = String(e)
      status = 'launchError'
    } finally {
      pending = null
    }
  }
</script>

<main>
  <header>
    <div class="brand"><span class="mark">文</span> Locale Studio</div>
    <div class="header-actions">
      <div class="language-switch" role="group" aria-label={t.language}>
        <button
          class:active={language === 'vi'}
          aria-pressed={language === 'vi'}
          aria-label={copy.vi.languageName}
          lang="vi"
          onclick={() => (language = 'vi')}>VI</button
        >
        <button
          class:active={language === 'en'}
          aria-pressed={language === 'en'}
          aria-label="English"
          lang="en"
          onclick={() => (language = 'en')}>EN</button
        >
      </div>
    </div>
  </header>
  <section class="intro">
    <h1>{t.title1}<br />{t.title2}</h1>
    <p>{t.intro}</p>
  </section>
  <div class="workspace">
    <section class="card library" aria-labelledby="library-heading">
      <div class="library-heading">
        <div>
          <h2 id="library-heading">
            {t.libraryTitle} <span class="library-count">{entries.length}</span>
          </h2>
          <p>{t.librarySubtitle}</p>
        </div>
        <button
          class="refresh"
          onclick={refreshLibrary}
          disabled={libraryLoading || pending !== null}>{t.refresh}</button
        >
      </div>
      <input
        class="library-search"
        type="search"
        aria-label={t.search}
        placeholder={t.search}
        bind:value={query}
      />
      {#if libraryError}<div class="library-notice" role="status">
          <p>{saveWarning ? t.librarySaveError : t.libraryError}</p>
          <details class="error-details">
            <summary>{t.details}</summary>
            <p>{diagnostic(libraryError, language)}</p>
          </details>
        </div>{/if}
      {#if libraryLoading}<p class="library-empty" role="status">
          {t.loadingLibrary}
        </p>
      {:else if entries.length === 0}<div class="library-empty">
          <span class="empty-icon" aria-hidden="true">▦</span>
          <h3>{t.emptyLibrary}</h3>
          <p>{t.emptyLibraryHint}</p>
        </div>
      {:else if filtered.length === 0}<p class="library-empty">{t.noMatches}</p>
      {:else}
        <ul class="library-list">
          {#each filtered as entry (entry.path)}
            <li
              class:current={target.toLocaleLowerCase() ===
                entry.path.toLocaleLowerCase()}
            >
              <button
                class="entry-select"
                title={entry.path}
                onclick={() => selectEntry(entry.path)}
                disabled={pending !== null}
              >
                <span class="app-tile" aria-hidden="true"
                  >{Array.from(entry.name)
                    .slice(0, 2)
                    .join('')
                    .toLocaleUpperCase()}</span
                >
                <span class="entry-info"
                  ><strong>{entry.name}</strong><span class="entry-path"
                    >{entry.path}</span
                  ><span class="entry-meta"
                    >{entry.architecture === '386' ? 'x86' : 'x64'} ·
                    <time title={t.lastPlayed}
                      >{playedAt(entry.lastPlayed)}</time
                    ></span
                  >{#if !entry.available}<span class="missing-file"
                      >{t.missingFile}</span
                    >{/if}</span
                >
              </button>
              <div class="entry-actions">
                <button
                  class="entry-play"
                  onclick={() => run(entry.path)}
                  disabled={pending !== null || !entry.available}
                  aria-label={`${t.play}: ${entry.name}`}>{t.play}</button
                ><button
                  class="entry-remove"
                  onclick={() => removeEntry(entry.path)}
                  disabled={pending !== null}
                  title={t.removeHint}
                  aria-label={`${t.remove}: ${entry.name}`}>×</button
                >
              </div>
            </li>
          {/each}
        </ul>
      {/if}
    </section>
    <section class="card launcher-card">
      <div class="section-title">
        <span class="step">01</span>
        <h2>{t.application}</h2>
      </div>
      <label for="target">{t.path}</label>
      <div class="file-row">
        <input
          id="target"
          bind:value={target}
          disabled={pending !== null}
          placeholder="C:\Apps\example.exe"
        /><button class="secondary" onclick={choose} disabled={pending !== null}
          >{t.browse}</button
        >
      </div>
      <p class="hint">{t.hint}</p>
      <div class="divider"></div>
      <div class="section-title">
        <span class="step">02</span>
        <h2>{t.locale}</h2>
      </div>
      <div class="profile">
        <span class="flag">JP</span>
        <div>
          <strong>{t.japanese}</strong><span
            >ja-JP · Code page 932 · LCID 0x0411</span
          >
        </div>
        <span class="selected">{t.selected}</span>
      </div>
      <button
        class="primary"
        onclick={() => run()}
        disabled={!target.trim() || pending !== null}
        >{pending === 'run' ? t.starting : t.start}</button
      >
      <div class:failed class="status" role="status" aria-live="polite">
        <span class="dot"></span>{message}
      </div>
      {#if failed && error}<details class="error-details">
          <summary>{t.details}</summary>
          <p>{diagnostic(error, language)}</p>
        </details>{/if}
    </section>
  </div>
  <aside>
    <strong>{t.scope}</strong>
    <p>{t.limits}</p>
  </aside>
  <footer><span>Windows · x86 / x64</span><span>v0.2</span></footer>
</main>
