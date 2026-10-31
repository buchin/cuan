import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const inboxViewPath = path.resolve(__dirname, '..', 'InboxView.tsx')
const idLocalePath = path.resolve(__dirname, '..', '..', '..', 'locales', 'id.json')
const enLocalePath = path.resolve(__dirname, '..', '..', '..', 'locales', 'en.json')
const languageContextPath = path.resolve(__dirname, '..', '..', '..', 'context', 'LanguageContext.tsx')

test('1. Customer Email Header: renders customer email beside name when activeContact is customer with email', () => {
  const inboxViewSource = fs.readFileSync(inboxViewPath, 'utf8')

  // Find the header section
  const headerStart = inboxViewSource.indexOf('wa-chat-header')
  assert.ok(headerStart !== -1, 'wa-chat-header container must exist in InboxView')
  const headerEnd = inboxViewSource.indexOf('Sub-tabs: Obrolan (Chat) & Pekerjaan (Tasks)', headerStart)
  assert.ok(headerEnd !== -1, 'Sub-tabs container separator must exist')
  const headerSection = inboxViewSource.slice(headerStart, headerEnd)

  // Verify conditional rendering: activeContact.type === 'customer' && activeContact.email
  assert.ok(
    headerSection.includes("activeContact.type === 'customer' && activeContact.email"),
    'Customer email section must be conditioned on activeContact.type === customer and activeContact.email'
  )

  // Verify email container & testid attributes
  assert.ok(
    headerSection.includes('data-testid="customer-email-header"'),
    'Header must contain data-testid="customer-email-header"'
  )
  assert.ok(
    headerSection.includes('data-testid="customer-email-text"'),
    'Email text must contain data-testid="customer-email-text"'
  )
  assert.ok(
    headerSection.includes('{activeContact.email}'),
    'Header must render activeContact.email'
  )
})

test('2. One-Click Copy Button: touch-friendly standards (min-h-11 min-w-11) and accessible attributes', () => {
  const inboxViewSource = fs.readFileSync(inboxViewPath, 'utf8')

  // Verify copy button exists with data-testid
  assert.ok(
    inboxViewSource.includes('data-testid="copy-customer-email-button"'),
    'Copy email button must exist with data-testid="copy-customer-email-button"'
  )

  // Verify touch-friendly target min-h-11 (44px) and min-w-11 (44px) for WCAG 2.1 AA / mobile standard
  assert.ok(
    inboxViewSource.includes('min-h-11') && inboxViewSource.includes('min-w-11'),
    'Copy email button must specify min-h-11 and min-w-11 for touch accessibility (>= 44x44px)'
  )

  // Verify accessible aria-label and title
  assert.ok(
    inboxViewSource.includes("aria-label={copiedEmail ? t('Disalin!') : t('Copy email')}"),
    'Copy email button must provide dynamic localized aria-label'
  )
  assert.ok(
    inboxViewSource.includes('title={copiedEmail ?'),
    'Copy email button must provide title for desktop hover'
  )
})

test('3. Copy Interaction & Visual Feedback: Clipboard write, Check icon, and "Disalin!" feedback', () => {
  const inboxViewSource = fs.readFileSync(inboxViewPath, 'utf8')

  // Verify copy handler implementation
  assert.ok(
    inboxViewSource.includes('navigator.clipboard.writeText(activeContact.email)'),
    'Copy handler must copy activeContact.email via navigator.clipboard.writeText'
  )
  assert.ok(
    inboxViewSource.includes('e.preventDefault()') && inboxViewSource.includes('e.stopPropagation()'),
    'Copy handler must isolate event bubbling with preventDefault and stopPropagation'
  )

  // Verify visual feedback states
  assert.ok(
    inboxViewSource.includes('<Check className="h-3.5 w-3.5 shrink-0" />') ||
    inboxViewSource.includes('<Check className="h-3.5 w-3.5'),
    'Visual feedback must render Check icon when copied'
  )
  assert.ok(
    inboxViewSource.includes("<Copy className=\"h-3.5 w-3.5 shrink-0\" />"),
    'Default state must render Lucide Copy icon'
  )
  assert.ok(
    inboxViewSource.includes("t('Disalin!')"),
    'Visual feedback must render localized "Disalin!" label'
  )

  // Verify 2-second timeout auto-reset
  assert.ok(
    inboxViewSource.includes('2000'),
    'Visual feedback must reset back to default after 2000ms (2s)'
  )
  assert.ok(
    inboxViewSource.includes('setCopiedEmail(false)'),
    'Timer must reset copiedEmail state to false'
  )
})

test('4. Contact Switch & Lifecycle: resets copy state and clears timer on activeContact change', () => {
  const inboxViewSource = fs.readFileSync(inboxViewPath, 'utf8')

  // Verify reset effect on activeContact?.id change
  assert.ok(
    inboxViewSource.includes('setCopiedEmail(false)'),
    'Must reset copiedEmail state when activeContact changes'
  )
  assert.ok(
    inboxViewSource.includes('[activeContact?.id]'),
    'Must observe activeContact?.id in useEffect dependency'
  )
  assert.ok(
    inboxViewSource.includes('clearTimeout(copyEmailTimeoutRef.current)'),
    'Must clear active timeout on contact change or unmount'
  )
})

test('5. Localization: id.json and en.json include all customer email header & copy strings', () => {
  assert.ok(fs.existsSync(idLocalePath), 'web/src/locales/id.json must exist')
  assert.ok(fs.existsSync(enLocalePath), 'web/src/locales/en.json must exist')

  const idLocales = JSON.parse(fs.readFileSync(idLocalePath, 'utf8'))
  const enLocales = JSON.parse(fs.readFileSync(enLocalePath, 'utf8'))

  // Indonesian strings
  assert.equal(idLocales['Copy email'], 'Salin email')
  assert.equal(idLocales['Disalin!'], 'Disalin!')
  assert.equal(idLocales['Email copied!'], 'Email disalin!')

  // English strings
  assert.equal(enLocales['Copy email'], 'Copy email')
  assert.equal(enLocales['Disalin!'], 'Copied!')
  assert.equal(enLocales['Email copied!'], 'Email copied!')

  // LanguageContext supports en.json dictionary
  const langContextSource = fs.readFileSync(languageContextPath, 'utf8')
  assert.ok(
    langContextSource.includes("import en from '../locales/en.json'"),
    'LanguageContext must import en.json'
  )
  assert.ok(
    langContextSource.includes('dictionaries'),
    'LanguageContext must manage localized dictionaries'
  )
})

test('6. Mobile Responsiveness & Layout Safeguards (360px - 390px)', () => {
  const inboxViewSource = fs.readFileSync(inboxViewPath, 'utf8')

  // Email text truncation
  assert.ok(
    inboxViewSource.includes('truncate text-xs text-muted-foreground font-normal'),
    'Customer email text must use truncate to prevent horizontal overflowing'
  )
  assert.ok(
    inboxViewSource.includes('max-w-['),
    'Customer email must set responsive max-w constraint'
  )
  assert.ok(
    inboxViewSource.includes('min-w-0'),
    'Customer email container must use min-w-0 for flex child truncation'
  )
})
