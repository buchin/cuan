import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import ts from 'typescript'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const initialsPath = path.resolve(__dirname, '../src/features/inbox/contact-initials.ts')
const avatarPath = path.resolve(__dirname, '../src/components/ui/Avatar.tsx')
const subPanelPath = path.resolve(__dirname, '../src/features/inbox/InboxSubPanel.tsx')
const contactsViewPath = path.resolve(__dirname, '../src/features/contacts/ContactsView.tsx')
const inboxViewPath = path.resolve(__dirname, '../src/features/inbox/InboxView.tsx')
const newMessageDialogPath = path.resolve(__dirname, '../src/features/inbox/NewMessageDialog.tsx')
const chatHandlersPath = path.resolve(__dirname, '../../internal/server/chat_handlers.go')
const chatContactsHandlersPath = path.resolve(__dirname, '../../internal/server/chat_contacts_handlers.go')

const compile = async (fileUrl) => {
  const source = fs.readFileSync(fileUrl, 'utf8')
  return import('data:text/javascript;base64,' + Buffer.from(ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.ESNext }
  }).outputText).toString('base64'))
}

const { getContactInitials, isAnonymousContact } = await compile(new URL('../src/features/inbox/contact-initials.ts', import.meta.url))

test('DoD 1: getContactInitials accurately extracts 1-2 uppercase monogram letters for named contacts', () => {
  // Multi-word names: First letter of first word + first letter of second word
  assert.equal(getContactInitials('fahmi putra'), 'FP', 'fahmi putra must yield FP')
  assert.equal(getContactInitials('kisworo izar'), 'KI', 'kisworo izar must yield KI')
  assert.equal(getContactInitials('Website pelanggan'), 'WP', 'Website pelanggan must yield WP')
  assert.equal(getContactInitials('Fahmi Putra Pratama'), 'FP', 'Fahmi Putra Pratama must yield FP')

  // Names with trailing CRM timestamps or identifiers
  assert.equal(getContactInitials('azizoke 2022-05-30 09:30:43'), 'AZ', 'azizoke with timestamp must yield AZ')

  // Single word names: First two letters
  assert.equal(getContactInitials('azizoke'), 'AZ', 'azizoke must yield AZ')
  assert.equal(getContactInitials('Alice'), 'AL', 'Alice must yield AL')

  // Single letter name
  assert.equal(getContactInitials('A'), 'A', 'Single letter name must yield A')
})

test('DoD 2: isAnonymousContact and fallback handling for anonymous or placeholder contacts', () => {
  assert.equal(isAnonymousContact('Visitor #123456'), true)
  assert.equal(isAnonymousContact('Visitor'), true)
  assert.equal(isAnonymousContact('Pengunjung #987'), true)
  assert.equal(isAnonymousContact('Pengunjung'), true)
  assert.equal(isAnonymousContact('Guest #42'), true)
  assert.equal(isAnonymousContact('Guest'), true)
  assert.equal(isAnonymousContact('Customer Room'), true)
  assert.equal(isAnonymousContact('Agent Room'), true)
  assert.equal(isAnonymousContact('Room #12345'), true)
  assert.equal(isAnonymousContact(''), true)
  assert.equal(isAnonymousContact(undefined), true)

  // Named contacts must not be considered anonymous
  assert.equal(isAnonymousContact('fahmi putra'), false)
  assert.equal(isAnonymousContact('kisworo izar'), false)
  assert.equal(isAnonymousContact('azizoke'), false)

  // getContactInitials returns empty string for anonymous contacts so UI falls back to silhouette icon
  assert.equal(getContactInitials('Visitor #123456'), '')
  assert.equal(getContactInitials('Pengunjung'), '')
  assert.equal(getContactInitials('Guest #42'), '')
  assert.equal(getContactInitials(''), '')
  assert.equal(getContactInitials(undefined), '')
  assert.equal(getContactInitials('12345'), '')
})

test('DoD 3: Shared Avatar component, InboxSubPanel, and ContactsView unify avatar rendering and preserve status badges', () => {
  const avatarSource = fs.readFileSync(avatarPath, 'utf8')
  const subPanelSource = fs.readFileSync(subPanelPath, 'utf8')
  const contactsViewSource = fs.readFileSync(contactsViewPath, 'utf8')

  // Shared Avatar component implements monogram initials, emoji/photo, Bot, and User fallback
  assert.ok(
    avatarSource.includes('getContactInitials'),
    'Avatar must use getContactInitials helper'
  )
  assert.ok(
    avatarSource.includes("!== '👤'"),
    'Avatar must ignore generic silhouette emoji 👤'
  )
  assert.ok(
    avatarSource.includes('getContactInitials(name)'),
    'Avatar must extract and display monogram initials for contact name'
  )
  assert.ok(
    avatarSource.includes('User className={cn(iconSize, \'text-muted-foreground\')}') ||
      avatarSource.includes('<User className='),
    'Avatar must render minimal User icon for anonymous contacts'
  )
  assert.ok(
    avatarSource.includes('Bot className={cn(iconSize, \'text-primary\')}') ||
      avatarSource.includes('<Bot className='),
    'Avatar must render Bot icon for agent contacts'
  )
  assert.ok(
    avatarSource.includes('bg-secondary/80') &&
      avatarSource.includes('text-foreground') &&
      avatarSource.includes('border-border/50'),
    'Avatar circle must use calm neutral secondary background, border, and high-contrast foreground text'
  )

  // Both InboxSubPanel and ContactsView use the shared Avatar component
  assert.ok(
    subPanelSource.includes("from '../../components/ui/Avatar'") &&
      subPanelSource.includes('<Avatar'),
    'InboxSubPanel must render the shared Avatar component'
  )
  assert.ok(
    contactsViewSource.includes("from '../../components/ui/Avatar'") &&
      contactsViewSource.includes('<Avatar'),
    'ContactsView must render the shared Avatar component'
  )
  assert.ok(
    !contactsViewSource.includes("Array.from(label(c)).slice(0,2).join('').toUpperCase()"),
    'ContactsView must not slice raw 2 characters for avatar initials (prevents "VI" for Visitor)'
  )

  // Preserves workflow status indicator position at bottom-right corner of avatar
  assert.ok(
    subPanelSource.includes('workflow-status-toggle absolute -bottom-0.5 -right-0.5 rounded-full bg-transparent'),
    'Workflow status indicator badge must remain positioned at bottom-right corner of avatar'
  )
})

test('DoD 4: InboxView header and empty state render monogram initials for named contacts', () => {
  const inboxViewSource = fs.readFileSync(inboxViewPath, 'utf8')

  assert.ok(
    inboxViewSource.includes("getContactInitials(activeContact.name)"),
    'InboxView header and empty state must render monogram initials for active contact'
  )

  assert.ok(
    inboxViewSource.includes("activeContact.avatar"),
    'InboxView must retain activeContact.avatar reference'
  )
})

test('DoD 5: NewMessageDialog recipient picker renders monogram initials', () => {
  const dialogSource = fs.readFileSync(newMessageDialogPath, 'utf8')

  assert.ok(
    dialogSource.includes("getContactInitials(c.name)"),
    'NewMessageDialog must render monogram initials for contacts'
  )
})

test('DoD 6: Backend handlers eliminate hardcoded blue silhouette emoji', () => {
  const chatHandlersSource = fs.readFileSync(chatHandlersPath, 'utf8')
  const chatContactsHandlersSource = fs.readFileSync(chatContactsHandlersPath, 'utf8')

  assert.ok(
    !chatHandlersSource.includes('Avatar:         "👤"'),
    'chat_handlers.go must not hardcode Avatar: "👤"'
  )
  assert.ok(
    !chatContactsHandlersSource.includes('Avatar:         "👤"'),
    'chat_contacts_handlers.go must not hardcode Avatar: "👤"'
  )
})
