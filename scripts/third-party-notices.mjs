// Collect upstream texts without rewriting copyright holders or license terms.
// This is a notice collector, not a legal compatibility or provenance audit.
import fs from 'node:fs'
import path from 'node:path'
import { execFileSync } from 'node:child_process'
import { fileURLToPath } from 'node:url'

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..')
const output = path.join(root, 'build', 'release', 'THIRD-PARTY-NOTICES.txt')
const toolchains = process.argv.slice(2)
if (!toolchains.length) throw new Error('Supply the compiler distribution directories used to build the binaries.')
const run = (...args) => execFileSync('go', args, { cwd: root, encoding: 'utf8' }).trim()
const sections = []
const noticeName = /^(licen[cs]e|copying|notice|copyright|patents|authors|disclaimer)([._-].*)?$/i

function collect(directory) {
  const found = []
  for (const entry of fs.readdirSync(directory, { withFileTypes: true }).sort((a, b) => a.name.localeCompare(b.name))) {
    const location = path.join(directory, entry.name)
    if (entry.isSymbolicLink()) throw new Error(`Linked notice input: ${entry.name}`)
    if (entry.isDirectory() && !['node_modules', '.git'].includes(entry.name)) found.push(...collect(location))
    else if (entry.isFile() && noticeName.test(entry.name)) found.push(location)
  }
  return found
}

function add(label, directory, files = collect(directory)) {
  if (!files.some(file => /licen[cs]e|copying|copyright/i.test(path.basename(file)))) {
    throw new Error(`No license text found for ${label}; review upstream terms before packaging.`)
  }
  const text = files.map(file => `--- ${path.relative(directory, file).replaceAll('\\', '/')} ---\n${fs.readFileSync(file, 'utf8')}`).join('\n\n')
  sections.push(`=== ${label} ===\n\n${text}`)
}

// Read dependencies from the actual four release binaries, not all CLI/test tools.
const modules = new Set()
const goVersions = new Set()
for (const binary of ['locale-emulator-go.exe', 'locale-run-x86.exe', 'locale-engine.dll', 'locale-engine-x86.dll']) {
  const metadata = run('version', '-m', path.join('build', 'bin', binary))
  goVersions.add(metadata.split('\n')[0].split(': ').at(-1).trim())
  for (const line of metadata.split('\n')) {
    const fields = line.trim().split(/\s+/)
    if (fields[0] === '=>') throw new Error('Replaced Go module requires manual notice review.')
    if (fields[0] === 'dep') modules.add(`${fields[1]}@${fields[2]}`)
  }
}
if (goVersions.size !== 1 || !goVersions.has(run('env', 'GOVERSION'))) {
  throw new Error('Generate notices using the same Go version used for all release binaries.')
}
for (const module of [...modules].sort()) {
  const info = JSON.parse(run('mod', 'download', '-json', module))
  if (info.Error || !info.Dir) throw new Error(`Cannot locate source for ${module}`)
  add(`Go module ${module}`, info.Dir)
}
const goroot = run('env', 'GOROOT')
add(`Go runtime ${[...goVersions][0]}`, goroot, [
  path.join(goroot, 'LICENSE'), path.join(goroot, 'PATENTS'), ...collect(path.join(goroot, 'src')),
])

// Include installed frontend build dependencies conservatively: devDependencies
// can contribute runtime code (notably Svelte) to the embedded frontend bundle.
const lock = JSON.parse(fs.readFileSync(path.join(root, 'frontend/package-lock.json'), 'utf8'))
for (const [relative, pkg] of Object.entries(lock.packages).sort(([a], [b]) => a.localeCompare(b))) {
  if (!relative) continue
  const directory = path.join(root, 'frontend', relative)
  if (!fs.existsSync(directory)) {
    if (pkg.optional) continue // Platform-specific optional package, not installed.
    throw new Error(`Missing frontend dependency: ${relative}; run npm ci --include=dev.`)
  }
  const manifest = JSON.parse(fs.readFileSync(path.join(directory, 'package.json'), 'utf8'))
  if (manifest.version !== pkg.version) throw new Error(`Frontend lockfile mismatch: ${relative}`)
  // These exact upstream packages declare MIT in package.json and README but
  // omit a LICENSE file. Preserve their supplied attribution; do not invent a
  // copyright year. Include the standard MIT terms alongside that declaration.
  const declarationOnly = { 'is-reference': '3.0.3', 'locate-character': '3.0.0' }
  if (declarationOnly[manifest.name] === manifest.version && !collect(directory).length) {
    const readme = fs.readFileSync(path.join(directory, 'README.md'), 'utf8')
    if (manifest.license !== 'MIT' || !/## License\s+MIT\s*$/m.test(readme)) throw new Error('Upstream license declaration changed')
    const mit = fs.readFileSync(path.join(root, 'LICENSE'), 'utf8')
    const terms = mit.slice(mit.indexOf('Permission is hereby granted'))
    if (!terms.startsWith('Permission is hereby granted')) throw new Error('MIT license template missing')
    sections.push(`=== Frontend ${manifest.name}@${manifest.version} ===\n\nUpstream package.json / README declare: MIT\nAuthor: ${JSON.stringify(manifest.author)}\nRepository: ${JSON.stringify(manifest.repository)}\nNo separate copyright notice was included in this npm package.\n\n${terms}`)
    continue
  }
  let noticeDirectory = directory
  // Native esbuild/Rollup packages ship build executables separately; their
  // matching JS distribution carries the upstream license text.
  const parentName = manifest.name.startsWith('@esbuild/') ? 'esbuild' : manifest.name.startsWith('@rollup/rollup-') ? 'rollup' : null
  if (!collect(directory).length && parentName) {
    noticeDirectory = path.join(root, 'frontend/node_modules', parentName)
    const parent = JSON.parse(fs.readFileSync(path.join(noticeDirectory, 'package.json'), 'utf8'))
    if (parent.version !== manifest.version) throw new Error(`Native build tool version mismatch: ${manifest.name}`)
  }
  add(`Frontend ${manifest.name}@${manifest.version} (${manifest.license || 'see license text'})`, noticeDirectory)
}

for (const directory of toolchains) {
  const absolute = path.resolve(directory)
  if (fs.existsSync(path.join(absolute, 'LICENSE.TXT')) && fs.existsSync(path.join(absolute, 'x86_64-w64-mingw32/share/mingw32'))) {
    add('LLVM-MinGW: LLVM and runtime notices', absolute, [path.join(absolute, 'LICENSE.TXT'),
      ...collect(path.join(absolute, 'x86_64-w64-mingw32/share/mingw32')),
      ...collect(path.join(absolute, 'i686-w64-mingw32/share/mingw32'))])
  } else if (fs.existsSync(path.join(absolute, 'licenses/gcc/COPYING.RUNTIME'))) {
    add('GCC/MinGW: compiler runtime notices and exceptions', absolute,
      ['gcc', 'mingw-w64', 'winpthreads'].flatMap(name => collect(path.join(absolute, 'licenses', name))))
  } else throw new Error('Unsupported compiler notice layout; add a reviewed collector for this distribution.')
}
const heading = `Locale Studio — Third-party notices

Locale Studio's own source is licensed under the accompanying MIT LICENSE.
The following upstream notices retain their original terms and attribution.
Inclusion here does not relicense third-party components under Locale Studio's license.
This inventory includes linked Go modules, Go runtime notices, installed frontend
dependencies (including build tools), and compiler/runtime notices. Some listed
components are build-only or concern platforms not present in the Windows binary.
Microsoft Windows and the separately installed WebView2 Runtime are not included
in the ZIP and remain subject to their respective terms.

`
fs.mkdirSync(path.dirname(output), { recursive: true })
fs.writeFileSync(output, heading + sections.join('\n\n') + '\n')
console.log(`Collected ${sections.length} notice sections into build/release/THIRD-PARTY-NOTICES.txt`)
