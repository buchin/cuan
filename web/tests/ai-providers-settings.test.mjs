import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const webDir = path.resolve(__dirname, '..')

const aiProvidersApiPath = path.join(webDir, 'src/api/aiProviders.ts')
const aiProvidersViewPath = path.join(webDir, 'src/features/settings/AIProvidersView.tsx')
const settingsMasterNavPath = path.join(webDir, 'src/features/settings/SettingsMasterNav.tsx')
const settingsViewPath = path.join(webDir, 'src/features/settings/SettingsView.tsx')
const appPath = path.join(webDir, 'src/App.tsx')
const editAgentModalPath = path.join(webDir, 'src/features/team/EditAgentModal.tsx')
const idLocalePath = path.join(webDir, 'src/locales/id.json')

test('1. API Layer: AI Providers, Binary Manager, and Store Default in aiProviders.ts', () => {
  assert.ok(fs.existsSync(aiProvidersApiPath), 'aiProviders.ts must exist in web/src/api/')

  const apiSrc = fs.readFileSync(aiProvidersApiPath, 'utf8')

  // Check exported interfaces
  assert.ok(apiSrc.includes('export interface AIProviderDTO'), 'Must export AIProviderDTO interface')
  assert.ok(apiSrc.includes('export interface HarnessBinaryStatus'), 'Must export HarnessBinaryStatus interface')
  assert.ok(apiSrc.includes('export interface SaveAIProviderRequest'), 'Must export SaveAIProviderRequest interface')
  assert.ok(apiSrc.includes('export interface TestAIProviderRequest'), 'Must export TestAIProviderRequest interface')
  assert.ok(apiSrc.includes('export interface TestAIProviderResponse'), 'Must export TestAIProviderResponse interface')
  assert.ok(apiSrc.includes('export interface InstallBinaryResponse'), 'Must export InstallBinaryResponse interface')
  assert.ok(apiSrc.includes('export interface AgentProviderBinding'), 'Must export AgentProviderBinding interface')
  assert.ok(apiSrc.includes('export interface UpdateAgentProviderRequest'), 'Must export UpdateAgentProviderRequest interface')

  // Check API methods
  assert.ok(apiSrc.includes('list: async (): Promise<AIProviderDTO[]>'), 'Must provide list() method')
  assert.ok(apiSrc.includes("'/api/v1/ai/providers'"), 'Must call /api/v1/ai/providers endpoint')
  assert.ok(apiSrc.includes('save: async'), 'Must provide save() method')
  assert.ok(apiSrc.includes('test: async'), 'Must provide test() method')
  assert.ok(apiSrc.includes("'/api/v1/ai/providers/test'"), 'Must call /api/v1/ai/providers/test')
  assert.ok(apiSrc.includes('installBinary: async'), 'Must provide installBinary() method')
  assert.ok(apiSrc.includes("'/api/v1/ai/providers/install-binary'"), 'Must call install-binary')
  assert.ok(apiSrc.includes('getAgentProvider: async'), 'Must provide getAgentProvider() method')
  assert.ok(apiSrc.includes('updateAgentProvider: async'), 'Must provide updateAgentProvider() method')

  // Check Global Default Store helpers
  assert.ok(apiSrc.includes('export function getDefaultStoreProvider'), 'Must export getDefaultStoreProvider')
  assert.ok(apiSrc.includes('export function setDefaultStoreProvider'), 'Must export setDefaultStoreProvider')
  assert.ok(apiSrc.includes('cuan_default_store_provider'), 'Must persist default store provider key')
  assert.ok(apiSrc.includes('cuan:default-provider-changed'), 'Must dispatch default-provider-changed custom event')
})

test('2. UI Component: AIProvidersView.tsx Dual-Mode & Zero-Friction Architecture', () => {
  assert.ok(fs.existsSync(aiProvidersViewPath), 'AIProvidersView.tsx must exist in web/src/features/settings/')

  const viewSrc = fs.readFileSync(aiProvidersViewPath, 'utf8')

  // Component definition & imports
  assert.ok(viewSrc.includes('export function AIProvidersView'), 'Must export AIProvidersView component')
  assert.ok(viewSrc.includes('aiProvidersApi'), 'Must import aiProvidersApi')

  // Hero Banner & Store Default
  assert.ok(viewSrc.includes('Default Ruang Kerja Saat Ini') || viewSrc.includes('Default Toko Saat Ini'), 'Must display Current Store Default badge')
  assert.ok(viewSrc.includes('handleSetDefaultStore'), 'Must provide handleSetDefaultStore action')
  assert.ok(viewSrc.includes('Jadikan Default Ruang Kerja') || viewSrc.includes('Jadikan Default Toko'), 'Must provide button to set provider as store default')

  // Dual-Mode Selector (Subscription vs BYOK)
  assert.ok(viewSrc.includes('Mode Langganan Flat'), 'Must render Mode Langganan Flat selector')
  assert.ok(viewSrc.includes('Mode API Key (BYOK)'), 'Must render Mode API Key (BYOK) selector')
  assert.ok(viewSrc.includes('handleModeChange'), 'Must provide handleModeChange handler')

  // Binary Manager (Status & 1-Click Install/Update)
  assert.ok(viewSrc.includes('Status Binary Harness'), 'Must show binary harness status')
  assert.ok(viewSrc.includes('handleInstallBinary'), 'Must provide handleInstallBinary action')
  assert.ok(viewSrc.includes('~/.local/bin'), 'Must indicate user-space ~/.local/bin path')
  assert.ok(viewSrc.includes('Pasang Binary 1-Klik'), 'Must include 1-click binary install button')

  // BYOK Form & Security
  assert.ok(viewSrc.includes('apiKeys'), 'Must manage API keys in component state')
  assert.ok(viewSrc.includes('api_key_masked'), 'Must support masked API key display')
  assert.ok(viewSrc.includes('Kredensial dienkripsi AES-256-GCM'), 'Must inform user of AES-256-GCM encryption')
  assert.ok(viewSrc.includes('handleTestConnection'), 'Must provide handleTestConnection action')
  assert.ok(viewSrc.includes('handleSaveConfig'), 'Must provide handleSaveConfig action')

  // Provider Highlights & Badges
  assert.ok(viewSrc.includes('Paling Hemat'), 'Must render Paling Hemat badge for DeepSeek')
  assert.ok(viewSrc.includes('Bawaan Cuan'), 'Must render Bawaan Cuan badge for Google Gemini')

  // Visual Latency Feedback Badge
  assert.ok(viewSrc.includes('testRes.latency_ms'), 'Must display measured latency in milliseconds')
  assert.ok(viewSrc.includes('testRes.latency_ms < 500'), 'Must conditionally style fast response (<500ms) with green')

  // Touch Accessibility Standards (min 44x44px touch targets)
  assert.ok(viewSrc.includes('min-h-11'), 'Must adhere to WCAG min-h-11 touch target standard')
})

test('3. Settings Navigation & Routing Integration', () => {
  const masterNavSrc = fs.readFileSync(settingsMasterNavPath, 'utf8')
  const settingsViewSrc = fs.readFileSync(settingsViewPath, 'utf8')
  const appSrc = fs.readFileSync(appPath, 'utf8')

  // Master Sidebar navigation category
  assert.ok(masterNavSrc.includes("id: 'ai-providers'"), 'SettingsMasterNav must register ai-providers item')
  assert.ok(masterNavSrc.includes("to: isAccount ? '/account/ai-providers' : '/settings/ai-providers'"), 'SettingsMasterNav must link to /settings/ai-providers')

  // SettingsView detail pane route
  assert.ok(settingsViewSrc.includes("['ai-providers', t('AI Providers')]"), 'SettingsView must include ai-providers in accessible navigation list')
  assert.ok(settingsViewSrc.includes("<AIProvidersView />"), 'SettingsView must render AIProvidersView for section === ai-providers')

  // App.tsx header titles
  assert.ok(appSrc.includes("'ai-providers': 'AI Providers'"), 'App.tsx settingTitles must include ai-providers')
})

test('4. EditAgentModal: Per-Agent Provider & Model Selection with Store Default', () => {
  assert.ok(fs.existsSync(editAgentModalPath), 'EditAgentModal.tsx must exist in web/src/features/team/')

  const modalSrc = fs.readFileSync(editAgentModalPath, 'utf8')

  // Provider state & API integration
  assert.ok(modalSrc.includes('selectedProvider'), 'Must track selectedProvider state')
  assert.ok(modalSrc.includes('availableProviders'), 'Must track availableProviders catalog')
  assert.ok(modalSrc.includes('aiProvidersApi.getAgentProvider'), 'Must fetch agent provider binding on open')
  assert.ok(modalSrc.includes('aiProvidersApi.updateAgentProvider'), 'Must call updateAgentProvider on save')

  // UI Dropdown
  assert.ok(modalSrc.includes('id="agent-ai-provider"'), 'Must render agent-ai-provider select element')
  assert.ok(modalSrc.includes('Ikuti Default Ruang Kerja') || modalSrc.includes('Ikuti Default Toko'), 'Must provide Ikuti Default Ruang Kerja default option')
  assert.ok(modalSrc.includes('handleProviderChange'), 'Must handle provider changes dynamically')

  // Dynamic model catalog update per provider
  assert.ok(modalSrc.includes('effectiveProvider'), 'Must compute effectiveProvider for vision capability and catalog')
  assert.ok(modalSrc.includes('Paling Hemat'), 'Must highlight DeepSeek as Paling Hemat in agent modal')
})

test('5. Localization Completeness: Indonesian dictionary contains 100% of new strings', () => {
  assert.ok(fs.existsSync(idLocalePath), 'id.json must exist in web/src/locales/')

  const idLocale = JSON.parse(fs.readFileSync(idLocalePath, 'utf8'))

  const requiredKeys = [
    'AI Providers',
    'AI engines, subscriptions, and BYOK credentials',
    'Pilih dan atur penyedia AI toko Anda, langganan resmi, dan kunci API hemat.',
    'AI Provider & Engine',
    'Penyedia AI & Mesin (Provider)',
    'Ikuti Default Toko ({provider})',
    'Paling Hemat',
    'Agen akan otomatis mengikuti penyedia AI utama yang Anda jadikan Default Toko di Pengaturan.',
    'Penyedia AI khusus untuk agen ini (override dari default toko).',
    'Failed to load AI providers',
    'Berhasil dijadikan default toko. Seluruh agen otomatis menggunakan {name}.',
    'Koneksi berhasil! Latensi respon: {ms}ms',
    'Koneksi gagal. Periksa kembali API Key atau URL.',
    'Gagal melakukan uji koneksi',
    'Pengaturan {name} berhasil disimpan dengan aman.',
    'Gagal menyimpan konfigurasi',
    'Binary CLI {name} berhasil dipasang ke user-space (~/.local/bin).',
    'Gagal memasang binary CLI',
    'Arsitektur Dual-Mode & Zero-Friction',
    'Penyedia AI & Mesin Cerdas',
    'Set Sekali, Jalan Semua. Pilih penyedia AI default untuk toko Anda, atau gunakan akun langganan resmi & API Key hemat langsung tanpa perlu membuka terminal.',
    'Default Toko Saat Ini',
    'Semua agen otomatis memakai engine ini',
    'Coba Lagi',
    'Bawaan Cuan',
    'Model cerdas open-weights dengan efisiensi biaya tertinggi untuk UMKM.',
    'Mesin bawaan siap pakai terintegrasi harness Google Antigravity.',
    'Kecerdasan penalaran mendalam Claude Pro/Team via CLI resmi.',
    'Eksekusi GPT-4o via ChatGPT Plus/Team atau API Key langsung.',
    'Penyedia model AI global dengan fleksibilitas multi-engine.',
    'Default Toko',
    'Pilihan Mode Eksekusi',
    'Mode Langganan Flat',
    'Mode API Key (BYOK)',
    'Mode Eksklusif: API Key Hemat (BYOK)',
    'Status Binary Harness',
    'Terpasang',
    'Belum Terpasang',
    'Binary resmi terpasang di user-space: {path}. Siap digunakan tanpa biaya token API.',
    'Binary CLI resmi belum terpasang. Klik tombol di bawah untuk memasang otomatis ke ruang pengguna tanpa hak akses root.',
    'Memasang binary resmi ke user-space... : Memasang binary resmi ke user-space...',
    'Perbarui Binary 1-Klik (~/.local/bin)',
    'Pasang Binary 1-Klik (~/.local/bin)',
    'Model Default Langganan',
    'Model Bawaan',
    'API Key {name}',
    'Kunci Tersimpan Aman',
    'Masukkan API Key...',
    'Sembunyikan Kunci',
    'Lihat Kunci',
    'Kredensial dienkripsi AES-256-GCM. Tidak pernah ditampilkan utuh kembali.',
    'Model Utama Pilihan',
    'URL Endpoint API Kustom (Opsional)',
    'Koneksi Terverifikasi ({status})',
    'Sangat Cepat',
    'Stabil',
    'Koneksi Gagal',
    'Default Toko Aktif',
    'Jadikan Default Toko',
    'Menguji...',
    'Uji Koneksi',
    'Menyimpan...',
    'Simpan',
  ]

  for (const key of requiredKeys) {
    if (key.includes(' : ')) {
      const [k, v] = key.split(' : ')
      assert.ok(idLocale[k] !== undefined, `Missing key in id.json: "${k}"`)
      continue
    }
    const altKey = key.replace(/toko/g, 'ruang kerja').replace(/Toko/g, 'Ruang Kerja')
    const hasKey = idLocale[key] !== undefined || idLocale[altKey] !== undefined
    assert.ok(hasKey, `Missing key in id.json: "${key}" or "${altKey}"`)
    const val = idLocale[key] !== undefined ? idLocale[key] : idLocale[altKey]
    assert.ok(
      typeof val === 'string' && val.length > 0,
      `Value for key "${key}" must not be empty`,
    )
  }
})
