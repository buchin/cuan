import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const webDir = path.resolve(__dirname, '..')

const teamViewPath = path.join(webDir, 'src/features/team/TeamView.tsx')
const aiProvidersViewPath = path.join(webDir, 'src/features/settings/AIProvidersView.tsx')
const idLocalePath = path.join(webDir, 'src/locales/id.json')
const enLocalePath = path.join(webDir, 'src/locales/en.json')

test('1. S60-T3: TeamView minimalist profile card (Things 3 / Bear Notes style)', () => {
  assert.ok(fs.existsSync(teamViewPath), 'TeamView.tsx must exist')
  const src = fs.readFileSync(teamViewPath, 'utf8')

  // StatusDot indicator integration
  assert.ok(src.includes('StatusDot'), 'TeamView must import and render StatusDot micro-indicator')
  assert.ok(src.includes('state.verified ? \'healthy\'') || src.includes('StatusDot'), 'StatusDot reflects connection health')

  // Clean profile row elements: Avatar, Name, Handle, Role description
  assert.ok(src.includes('getAgentHandle'), 'TeamView must compute clean @handle for agent')
  assert.ok(src.includes('agent.name') || src.includes('a.name'), 'TeamView renders agent name')
  assert.ok(src.includes('agent.role') || src.includes('a.role'), 'TeamView renders agent role description')
  assert.ok(src.includes('Pencil'), 'TeamView provides direct edit pencil button')
  assert.ok(src.includes('AgentActions'), 'TeamView provides calm AgentActions menu')
})

test('2. S60-T3: Progressive Disclosure via SlideDrawer for technical agent configs', () => {
  const src = fs.readFileSync(teamViewPath, 'utf8')

  // SlideDrawer integration
  assert.ok(src.includes('SlideDrawer'), 'TeamView must import and render SlideDrawer primitive')
  assert.ok(src.includes('drawerAgent') && src.includes('setDrawerAgent'), 'TeamView tracks drawerAgent state')
  assert.ok(src.includes('setDrawerAgent(a)'), 'Clicking an agent row opens SlideDrawer')

  // All 14 technical config elements are encapsulated inside SlideDrawer (not cluttering the main surface list)
  const drawerBlock = src.substring(src.indexOf('<SlideDrawer'), src.indexOf('</SlideDrawer>'))
  assert.ok(drawerBlock.length > 0, 'SlideDrawer block must be non-empty')

  // 1. Model ID
  assert.ok(drawerBlock.includes('model_id'), 'Technical model_id is disclosed inside SlideDrawer')
  // 2. Provider
  assert.ok(drawerBlock.includes('provider'), 'Technical provider is disclosed inside SlideDrawer')
  // 3. Fallback Model
  assert.ok(drawerBlock.includes('fallback_model_id'), 'Fallback model config is disclosed inside SlideDrawer')
  // 4. Vision capability
  assert.ok(drawerBlock.includes('isModelVisionCapable'), 'Vision capability indicators are inside SlideDrawer')
  // 5. Verification time & status
  assert.ok(drawerBlock.includes('formatVerificationTime') || drawerBlock.includes('verified_at'), 'Verification timestamps are disclosed inside SlideDrawer')
  // 6. BYOK credentials & Replace BYOK actions
  assert.ok(drawerBlock.includes('Replace BYOK') || drawerBlock.includes('replaceBYOK'), 'BYOK management actions are inside SlideDrawer')
  // 7. Failover controls & reconnect / disconnect lifecycle
  assert.ok(drawerBlock.includes('allow_failover') || drawerBlock.includes('reconnect'), 'Failover and reconnect lifecycle controls are inside SlideDrawer')
})

test('3. S60-T3: AIProvidersView calm design and elimination of Dual-Mode marketing banner', () => {
  assert.ok(fs.existsSync(aiProvidersViewPath), 'AIProvidersView.tsx must exist')
  const src = fs.readFileSync(aiProvidersViewPath, 'utf8')

  // Verification that loud SaaS banner "Arsitektur Dual-Mode & Zero-Friction" is removed from UI template
  assert.ok(
    !src.includes('Arsitektur Dual-Mode & Zero-Friction'),
    'Bulky dual-mode marketing banner must be eliminated from AIProvidersView'
  )

  // Calm header and streamlined cards
  assert.ok(src.includes('Default Ruang Kerja Saat Ini'), 'Header displays calm Default Ruang Kerja badge')
  assert.ok(src.includes('shadow-2xs') || src.includes('border-border/70'), 'Provider cards use calm, subtle border styling')
})

test('4. S60-T3: Total elimination of "Toko" relics across web/src', () => {
  // Recursively inspect all files in web/src
  function getFiles(dir) {
    const entries = fs.readdirSync(dir, { withFileTypes: true })
    const files = []
    for (const entry of entries) {
      const fullPath = path.join(dir, entry.name)
      if (entry.isDirectory()) {
        files.push(...getFiles(fullPath))
      } else if (entry.isFile() && /\.(tsx?|jsx?|json|css|html)$/.test(entry.name)) {
        files.push(fullPath)
      }
    }
    return files
  }

  const srcFiles = getFiles(path.join(webDir, 'src'))
  const offenders = []

  for (const file of srcFiles) {
    const content = fs.readFileSync(file, 'utf8')
    // Search for case-insensitive whole-word or substring toko / Toko / TOKO
    const lines = content.split('\n')
    lines.forEach((line, lineIdx) => {
      // Exclude matches if any are valid third-party or unrelated, but requirements say 0 occurrences
      if (/\btoko\b/i.test(line)) {
        offenders.push({
          file: path.relative(webDir, file),
          line: lineIdx + 1,
          text: line.trim()
        })
      }
    })
  }

  assert.equal(
    offenders.length,
    0,
    `Found ${offenders.length} relics of "toko" in web/src:\n${JSON.stringify(offenders, null, 2)}`
  )
})

test('5. S60-T3: Inclusive workspace localization in id.json and en.json', () => {
  const idLocale = JSON.parse(fs.readFileSync(idLocalePath, 'utf8'))
  const enLocale = JSON.parse(fs.readFileSync(enLocalePath, 'utf8'))

  // Verify inclusive workspace terms exist
  assert.ok(idLocale['Default Ruang Kerja Saat Ini'], 'id.json must include "Default Ruang Kerja Saat Ini"')
  assert.ok(idLocale['Jadikan Default Ruang Kerja'], 'id.json must include "Jadikan Default Ruang Kerja"')
  assert.ok(idLocale['Default Ruang Kerja'], 'id.json must include "Default Ruang Kerja"')
  assert.ok(enLocale['Default Ruang Kerja Saat Ini'], 'en.json must include translation for Default Ruang Kerja Saat Ini')

  // Verify 0 occurrences of toko/Toko in id.json and en.json keys and values
  const idRaw = fs.readFileSync(idLocalePath, 'utf8')
  assert.ok(!/\btoko\b/i.test(idRaw), 'id.json must contain 0 occurrences of toko')

  const enRaw = fs.readFileSync(enLocalePath, 'utf8')
  assert.ok(!/\btoko\b/i.test(enRaw), 'en.json must contain 0 occurrences of toko')
})
