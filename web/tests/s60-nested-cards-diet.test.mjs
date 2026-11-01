import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const rootDir = path.resolve(__dirname, '..')
const srcDir = path.join(rootDir, 'src')

const appsSkillsViewPath = path.join(srcDir, 'features/settings/AppsSkillsView.tsx')
const workerHealthCardPath = path.join(srcDir, 'features/settings/WorkerHealthCard.tsx')
const generalSettingsPath = path.join(srcDir, 'features/settings/GeneralSettings.tsx')
const settingsViewPath = path.join(srcDir, 'features/settings/SettingsView.tsx')
const sprint60Path = path.resolve(rootDir, '../docs/sprints/S60.md')

test('1. S60-T2 AppsSkillsView: Zero SaaS Education Banners & Apple-Style Inset Grouped Rows', () => {
  assert.ok(fs.existsSync(appsSkillsViewPath), 'AppsSkillsView.tsx must exist')
  const content = fs.readFileSync(appsSkillsViewPath, 'utf8')

  // Banner edukasi SaaS dibersihkan total
  assert.ok(
    !content.includes('Zero-Developer Friction & Safe Execution'),
    'AppsSkillsView must NOT contain "Zero-Developer Friction & Safe Execution" banner'
  )
  assert.ok(
    !content.includes('No API keys, complex JSON setups, or ambient credential leaks'),
    'AppsSkillsView must NOT contain SaaS educational marketing paragraph'
  )
  assert.ok(
    !content.includes('bg-primary/[0.03]') && !content.includes('border-primary/20'),
    'AppsSkillsView must NOT render bulky blue educational banner box'
  )

  // Menggunakan GroupedSection untuk Connected Apps dan Skills Catalog
  assert.match(
    content,
    /import\s+\{\s*GroupedSection.*\}\s+from\s+['"]\.\.\/\.\.\/components\/layout\/GroupedSection['"]/,
    'AppsSkillsView must import GroupedSection primitive'
  )
  assert.match(
    content,
    /<GroupedSection[\s\S]*?title=\{t\('Connected Apps'\)\}/,
    'AppsSkillsView must render Connected Apps within GroupedSection'
  )
  assert.match(
    content,
    /<GroupedSection[\s\S]*?title=\{t\('Skills Catalog'\)\}/,
    'AppsSkillsView must render Skills Catalog within GroupedSection'
  )

  // Menggunakan StatusDot halus untuk status koneksi & skill aktif
  assert.match(
    content,
    /import\s+\{\s*StatusDot.*\}\s+from\s+['"]\.\.\/\.\.\/components\/layout\/StatusDot['"]/,
    'AppsSkillsView must import StatusDot primitive'
  )
  assert.match(
    content,
    /<StatusDot[\s\S]*?status=\{isConnected\s*\?\s*'healthy'/,
    'Connected Apps must use StatusDot for calm status indication'
  )
  assert.match(
    content,
    /<StatusDot\s+status="healthy"/,
    'Skills Catalog must use StatusDot for active skills'
  )

  // 0 nested cards
  assert.ok(!content.includes('<Card>'), 'AppsSkillsView must not use <Card>')
  assert.ok(!content.includes('<CardContent>'), 'AppsSkillsView must not use <CardContent>')
  assert.ok(!content.includes('<CardHeader>'), 'AppsSkillsView must not use <CardHeader>')
})

test('2. S60-T2 WorkerHealthCard: Single Calm Status Row with StatusDot & Zero Nested Boxes', () => {
  assert.ok(fs.existsSync(workerHealthCardPath), 'WorkerHealthCard.tsx must exist')
  const content = fs.readFileSync(workerHealthCardPath, 'utf8')

  // Menggunakan GroupedSection & StatusDot
  assert.match(
    content,
    /import\s+\{\s*GroupedSection.*\}\s+from\s+['"]\.\.\/\.\.\/components\/layout\/GroupedSection['"]/,
    'WorkerHealthCard must import GroupedSection'
  )
  assert.match(
    content,
    /import\s+\{\s*StatusDot.*\}\s+from\s+['"]\.\.\/\.\.\/components\/layout\/StatusDot['"]/,
    'WorkerHealthCard must import StatusDot'
  )
  assert.match(
    content,
    /<GroupedSection[\s\S]*?data-testid="worker-health-card"/,
    'WorkerHealthCard must be framed by GroupedSection'
  )

  // Menghapus kotak bertingkat (grid 3 kotak & Activity icon wrapper dihapus)
  assert.ok(
    !content.includes('grid-cols-3'),
    'WorkerHealthCard must eliminate 3-column nested card grid'
  )
  assert.ok(
    !content.includes('<Activity'),
    'WorkerHealthCard must eliminate redundant Activity icon card header'
  )
  assert.ok(!content.includes('<Card>'), 'WorkerHealthCard must not use <Card>')
  assert.ok(!content.includes('<CardContent>'), 'WorkerHealthCard must not use <CardContent>')

  // Menyatukan status ke dalam satu baris tenang dengan StatusDot halus
  assert.match(
    content,
    /<StatusDot[\s\S]*?status=\{isReady\s*\?\s*'healthy'\s*:\s*isBusy\s*\?\s*'warning'\s*:\s*'exception'\}[\s\S]*?size="md"/,
    'WorkerHealthCard must render subtle StatusDot micro-indicator'
  )

  // Mempertahankan semua testids yang esensial untuk observabilitas
  assert.ok(content.includes('data-testid="worker-health-card"'), 'Must retain worker-health-card testid')
  assert.ok(content.includes('data-testid="worker-health-status"'), 'Must retain worker-health-status testid')
  assert.ok(content.includes('data-testid="worker-queue-depth"'), 'Must retain worker-queue-depth testid')
  assert.ok(content.includes('data-testid="worker-heartbeat"'), 'Must retain worker-heartbeat testid')
  assert.ok(content.includes('data-testid="worker-refresh-btn"'), 'Must retain worker-refresh-btn testid')
})

test('3. S60-T2 GeneralSettings & SettingsView: Inset Grouped Rows & Zero Nested Cards', () => {
  assert.ok(fs.existsSync(generalSettingsPath), 'GeneralSettings.tsx must exist')
  assert.ok(fs.existsSync(settingsViewPath), 'SettingsView.tsx must exist')

  const generalContent = fs.readFileSync(generalSettingsPath, 'utf8')
  const settingsContent = fs.readFileSync(settingsViewPath, 'utf8')

  // Zero nested cards in GeneralSettings & SettingsView
  for (const [name, code] of [['GeneralSettings', generalContent], ['SettingsView', settingsContent]]) {
    assert.ok(!code.includes('<Card>'), `${name} must not use <Card>`)
    assert.ok(!code.includes('<CardContent>'), `${name} must not use <CardContent>`)
    assert.ok(!code.includes('<CardHeader>'), `${name} must not use <CardHeader>`)
    assert.ok(!code.includes('<CardFooter>'), `${name} must not use <CardFooter>`)
    assert.match(code, /GroupedSection/, `${name} must use GroupedSection primitive`)
  }

  // Preserves link to /settings/memory
  assert.ok(
    generalContent.includes('to="/settings/memory"') && settingsContent.includes('to="/settings/memory"'),
    'Both GeneralSettings and SettingsView must link to /settings/memory'
  )

  // Preserves keyboard shortcuts trigger
  assert.ok(
    generalContent.includes("cuan:keyboard-shortcuts") && settingsContent.includes("cuan:keyboard-shortcuts"),
    'Both GeneralSettings and SettingsView must provide trigger for keyboard shortcuts'
  )
  assert.ok(
    generalContent.includes("t('View shortcuts')") && settingsContent.includes("t('View shortcuts')"),
    'Both GeneralSettings and SettingsView must include View shortcuts action'
  )

  // Encapsulates WorkerHealthCard without nested borders
  assert.ok(
    generalContent.includes('<WorkerHealthCard />'),
    'GeneralSettings must include WorkerHealthCard'
  )
  assert.ok(
    settingsContent.includes('<WorkerHealthCard />'),
    'SettingsView must include WorkerHealthCard'
  )
})

test('4. S60-T2 Quality Gate: Zero Nested Cards & Sprint Documentation', () => {
  assert.ok(fs.existsSync(sprint60Path), 'docs/sprints/S60.md must exist')
  const sprintContent = fs.readFileSync(sprint60Path, 'utf8')

  // Sprint 60 Backlog mencakup S60-T2
  assert.match(sprintContent, /S60-T2.*Eliminate Nested Cards Hell & SaaS Banners/i)
  assert.match(sprintContent, /0 nested cards dan 0 banner edukasi di Apps & Skills dan General Settings/)

  // Audit zero educational banners
  const targetFiles = [appsSkillsViewPath, workerHealthCardPath, generalSettingsPath]
  for (const f of targetFiles) {
    const code = fs.readFileSync(f, 'utf8')
    assert.ok(!code.includes('Zero-Developer Friction'), `${path.basename(f)} must not contain SaaS educational banner`)
  }
})
