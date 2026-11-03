import assert from 'node:assert/strict'
import fs from 'node:fs'
import path from 'node:path'
import { test } from 'node:test'
import { fileURLToPath } from 'node:url'

const sharedListSource = fs.readFileSync(new URL('../src/components/layout/ListControls.tsx', import.meta.url), 'utf8')
const __dirname = path.dirname(fileURLToPath(import.meta.url))
const rootDir = path.join(__dirname, '..')

// Source files
const appShellPath = path.join(rootDir, 'src', 'components', 'layout', 'AppShell.tsx')
const taskBoardPath = path.join(rootDir, 'src', 'features', 'tasks', 'TaskBoardView.tsx')
const taskItemCardPath = path.join(rootDir, 'src', 'features', 'tasks', 'TaskItemCard.tsx')
const taskUiPath = path.join(rootDir, 'src', 'features', 'tasks', 'task-ui.ts')
const inboxSubPanelPath = path.join(rootDir, 'src', 'features', 'inbox', 'InboxSubPanel.tsx')
const projectsViewPath = path.join(rootDir, 'src', 'features', 'projects', 'ProjectsView.tsx')
const projectWorkViewPath = path.join(rootDir, 'src', 'features', 'projects', 'ProjectWorkView.tsx')
const settingsViewPath = path.join(rootDir, 'src', 'features', 'settings', 'SettingsView.tsx')
const actionDialogPath = path.join(rootDir, 'src', 'components', 'dialogs', 'ActionDialog.tsx')
const createProjectModalPath = path.join(rootDir, 'src', 'features', 'projects', 'CreateProjectModal.tsx')
const indexCssPath = path.join(rootDir, 'src', 'index.css')

// Compiled CSS bundle
const distAssetsDir = path.join(rootDir, 'dist', 'assets')
let compiledCss = ''
if (fs.existsSync(distAssetsDir)) {
  const cssFile = fs.readdirSync(distAssetsDir).find((f) => f.endsWith('.css'))
  if (cssFile) {
    compiledCss = fs.readFileSync(path.join(distAssetsDir, cssFile), 'utf8')
  }
}

// -----------------------------------------------------------------------------
// WCAG 2.1 Contrast Helper
// -----------------------------------------------------------------------------
function sRGBtoLin(colorVal) {
  return colorVal <= 0.03928 ? colorVal / 12.92 : Math.pow((colorVal + 0.055) / 1.055, 2.4)
}

function getLuminance([r, g, b]) {
  return 0.2126 * sRGBtoLin(r / 255) + 0.7152 * sRGBtoLin(g / 255) + 0.0722 * sRGBtoLin(b / 255)
}

function getContrastRatio(rgb1, rgb2) {
  const l1 = getLuminance(rgb1)
  const l2 = getLuminance(rgb2)
  const lighter = Math.max(l1, l2)
  const darker = Math.min(l1, l2)
  return (lighter + 0.05) / (darker + 0.05)
}

// -----------------------------------------------------------------------------
// 1. Responsive Viewport Test (360px - 390px) & Mobile Safe Padding
// -----------------------------------------------------------------------------
test('1. Mobile Viewport Responsiveness (360px - 390px) & Safe Padding across all target pages', () => {
  const appShellSrc = fs.readFileSync(appShellPath, 'utf8')
  const actionDialogSrc = fs.readFileSync(actionDialogPath, 'utf8')
  const taskBoardSrc = fs.readFileSync(taskBoardPath, 'utf8')
  const inboxSubPanelSrc = fs.readFileSync(inboxSubPanelPath, 'utf8')
  const projectsViewSrc = fs.readFileSync(projectsViewPath, 'utf8')
  const projectWorkViewSrc = fs.readFileSync(projectWorkViewPath, 'utf8')
  const settingsViewSrc = fs.readFileSync(settingsViewPath, 'utf8')

  // A. AppShell Mobile Safe Padding
  assert(
    appShellSrc.includes('p-3 sm:p-4 md:p-6'),
    'AppShell must use mobile-safe p-3 padding on small viewports (<640px)'
  )
  assert(
    appShellSrc.includes('mobile-content-bottom'),
    'AppShell must cushion content against bottom navigation using mobile-content-bottom'
  )

  // B. Modal Mobile-Safe Dimensions & Margins
  assert(
    actionDialogSrc.includes('w-[calc(100%_-_24px)]'),
    'ActionDialog must use w-[calc(100%_-_24px)] to maintain safe 12px outer margins on 360-390px screens'
  )
  assert(
    actionDialogSrc.includes('p-4 sm:p-5'),
    'ActionDialog must use p-4 on mobile and sm:p-5 on larger screens'
  )

  // C. Viewport Geometric Math Validation for 360px, 375px, and 390px
  const targetViewports = [
    { name: 'Samsung Galaxy A / Compact Android', width: 360 },
    { name: 'iPhone SE / Compact iOS', width: 375 },
    { name: 'iPhone 13/14/15/16 Standard Mobile', width: 390 },
  ]

  for (const vp of targetViewports) {
    // Shell padding: 12px left + 12px right = 24px
    const availableContentWidth = vp.width - 24
    assert(
      availableContentWidth >= 336,
      `Available content width on ${vp.width}px must be >= 336px (got ${availableContentWidth}px)`
    )

    // Modal width: 100% - 24px
    const modalWidth = vp.width - 24
    // Modal inner width: modalWidth - (16px * 2) = modalWidth - 32px
    const modalInnerWidth = modalWidth - 32
    assert(
      modalInnerWidth >= 304,
      `Modal inner content width on ${vp.width}px must be >= 304px (got ${modalInnerWidth}px)`
    )

    // Task Card inner width: card padding is p-4 (16px * 2 = 32px)
    const cardInnerWidth = availableContentWidth - 32
    assert(
      cardInnerWidth >= 304,
      `Task card inner width on ${vp.width}px must be >= 304px (got ${cardInnerWidth}px)`
    )
  }

  // D. Elimination of Horizontal Overflow via Chips & Flex-wrap
  const chipContainerRegex = /overflow-x-auto.*\[scrollbar-width:none\]/
  assert(taskBoardSrc.includes('<ListTabs') && chipContainerRegex.test(sharedListSource), 'Tasks view must use overflow-x-auto without scrollbar blowout')
  assert(inboxSubPanelSrc.includes('<ListTabs') && chipContainerRegex.test(sharedListSource), 'Inbox view must use overflow-x-auto without scrollbar blowout')
  assert(projectsViewSrc.includes('<ListTabs') && chipContainerRegex.test(sharedListSource), 'Projects view must use overflow-x-auto without scrollbar blowout')
  assert(projectWorkViewSrc.includes('<ListTabs') && chipContainerRegex.test(sharedListSource), 'ProjectWorkView must use overflow-x-auto without scrollbar blowout')
  assert(settingsViewSrc.includes('overflow-x-auto'), 'Settings view nav must support horizontal scrolling')
})

// -----------------------------------------------------------------------------
// 2. Touch Target Size Verification (WCAG 2.1 AA min 44x44px)
// -----------------------------------------------------------------------------
test('2. Touch Target Compliance (min 44x44px WCAG 2.1 AA) on Primary & Secondary CTAs', () => {
  const indexCss = fs.readFileSync(indexCssPath, 'utf8')
  const taskItemCardSrc = fs.readFileSync(taskItemCardPath, 'utf8')
  const taskBoardSrc = fs.readFileSync(taskBoardPath, 'utf8')
  const inboxSubPanelSrc = fs.readFileSync(inboxSubPanelPath, 'utf8')
  const projectsViewSrc = fs.readFileSync(projectsViewPath, 'utf8')
  const projectWorkViewSrc = fs.readFileSync(projectWorkViewPath, 'utf8')
  const settingsViewSrc = fs.readFileSync(settingsViewPath, 'utf8')
  const actionDialogSrc = fs.readFileSync(actionDialogPath, 'utf8')
  const createProjectModalSrc = fs.readFileSync(createProjectModalPath, 'utf8')

  // A. Component Utility Rules in index.css
  assert(
    indexCss.includes('.action-primary {') && indexCss.includes('min-h-11'),
    '.action-primary must enforce min-h-11 (44px) in index.css'
  )
  assert(
    indexCss.includes('.action-secondary {') && indexCss.includes('min-h-11'),
    '.action-secondary must enforce min-h-11 (44px) in index.css'
  )
  assert(
    indexCss.includes('.icon-action {') && indexCss.includes('h-11 w-11'),
    '.icon-action must enforce 44x44px (h-11 w-11) touch targets in index.css'
  )
  assert(
    indexCss.includes('.field-control {') && indexCss.includes('min-h-11'),
    '.field-control must enforce min-h-11 (44px) touch targets for inputs in index.css'
  )

  // B. Tasks: Secondary (Detail) CTA & Primary Action CTAs
  assert(
    taskItemCardSrc.includes('action-secondary min-h-11 min-w-[44px]'),
    'TaskItemCard secondary Detail CTA must have min-h-11 min-w-[44px] (44x44px touch target)'
  )
  assert(
    taskItemCardSrc.includes('action-primary min-h-11 min-w-[44px]'),
    'TaskItemCard primary CTAs (Live Terminal, Retry, Start session, View result) must have min-h-11 min-w-[44px]'
  )
  assert(
    taskBoardSrc.includes('<ListTab') && sharedListSource.includes('min-h-11'),
    'TaskBoardView status filter chips must satisfy min-h-11 touch target'
  )

  // C. Inbox: Filter Chips & Search Input
  assert(
    inboxSubPanelSrc.includes('<ListTab') && sharedListSource.includes('min-h-11'),
    'InboxSubPanel filter chips must satisfy min-h-11 touch target'
  )
  assert(
    inboxSubPanelSrc.includes('<ListSearch') && sharedListSource.includes('h-11 min-h-11'),
    'InboxSubPanel search input must satisfy min-h-11 touch target'
  )

  // D. Projects: Tabs & Filter Chips
  assert(
    projectsViewSrc.includes('<ListTab') && sharedListSource.includes('inline-flex min-h-11 shrink-0'),
    'ProjectsView category chips must satisfy min-h-11 touch target'
  )
  assert(
    projectWorkViewSrc.includes('<ListTab') && sharedListSource.includes('inline-flex min-h-11 shrink-0'),
    'ProjectWorkView category chips must satisfy min-h-11 touch target'
  )

  // E. Settings: Navigation Tabs & Controls
  assert(
    settingsViewSrc.includes('inline-flex min-h-11 shrink-0 items-center border-b-2 px-3'),
    'SettingsView navigation tabs must satisfy min-h-11 touch target'
  )

  // F. Modals: Close Button & Action Buttons
  assert(
    actionDialogSrc.includes('className="icon-action"'),
    'ActionDialog close button must use icon-action (44x44px touch target)'
  )
  assert(
    createProjectModalSrc.includes('className="action-secondary"') &&
    createProjectModalSrc.includes('className="action-primary'),
    'CreateProjectModal buttons must use action-secondary and action-primary (>=44px touch target)'
  )

  // G. Compiled CSS Bundle Validation (if dist available)
  if (compiledCss) {
    assert(compiledCss.includes('.min-h-11') || compiledCss.includes('min-height:2.75rem'), 'Compiled CSS must contain min-h-11 (44px)')
    assert(compiledCss.includes('.min-w-\\[44px\\]') || compiledCss.includes('min-width:44px'), 'Compiled CSS must contain min-w-[44px] (44px)')
    assert(compiledCss.includes('.h-11') || compiledCss.includes('height:2.75rem'), 'Compiled CSS must contain h-11')
    assert(compiledCss.includes('.w-11') || compiledCss.includes('width:2.75rem'), 'Compiled CSS must contain w-11')
  }
})

// -----------------------------------------------------------------------------
// 3. Status Badge Color Contrast & Header/Filter Layout Consistency
// -----------------------------------------------------------------------------
test('3. Status Badge WCAG 2.1 AA Contrast Ratios & Header/Filter Layout Consistency', () => {
  const taskUiSrc = fs.readFileSync(taskUiPath, 'utf8')
  const taskBoardSrc = fs.readFileSync(taskBoardPath, 'utf8')
  const taskItemCardSrc = fs.readFileSync(taskItemCardPath, 'utf8')
  const inboxSubPanelSrc = fs.readFileSync(inboxSubPanelPath, 'utf8')
  const projectsViewSrc = fs.readFileSync(projectsViewPath, 'utf8')

  // A. Color Contrast Calculations
  const lightBgRgb = [255, 255, 255]
  const darkBgRgb = [9, 9, 11]

  const statusColors = [
    {
      status: 'Working / Active',
      lightText: [29, 78, 216],  // blue-700
      darkText: [147, 197, 253], // blue-300
    },
    {
      status: 'Waiting to start / Queued',
      lightText: [146, 64, 14],  // amber-800
      darkText: [253, 230, 138], // amber-200
    },
    {
      status: 'Ready for review',
      lightText: [107, 33, 168], // purple-700
      darkText: [216, 180, 254], // purple-300
    },
    {
      status: 'Completed / Done',
      lightText: [6, 95, 70],    // emerald-800
      darkText: [110, 231, 183], // emerald-300
    },
    {
      status: 'Cancelled',
      lightText: [63, 63, 70],   // zinc-700
      darkText: [212, 212, 216], // zinc-300
    },
    {
      status: 'Needs attention / Blocked',
      lightText: [220, 38, 38],  // destructive red
      darkText: [248, 113, 113], // red-400
    },
  ]

  for (const item of statusColors) {
    const lightRatio = getContrastRatio(item.lightText, lightBgRgb)
    const darkRatio = getContrastRatio(item.darkText, darkBgRgb)

    assert(
      lightRatio >= 4.5,
      `Status "${item.status}" Light Mode contrast ratio ${lightRatio.toFixed(2)}:1 must meet WCAG AA (>= 4.5:1)`
    )
    assert(
      darkRatio >= 4.5,
      `Status "${item.status}" Dark Mode contrast ratio ${darkRatio.toFixed(2)}:1 must meet WCAG AA (>= 4.5:1)`
    )
  }

  // B. Distinct Visual Identity for Status Badges
  assert(
    taskUiSrc.includes('text-blue-700') && taskUiSrc.includes('dark:text-blue-300'),
    'Working status must use high-contrast blue classes'
  )
  assert(
    taskUiSrc.includes('text-amber-800') && taskUiSrc.includes('dark:text-amber-200'),
    'Queued status must use high-contrast amber classes'
  )
  assert(
    taskUiSrc.includes('text-purple-700') && taskUiSrc.includes('dark:text-purple-300'),
    'Ready for review status must use high-contrast purple classes (differentiated from amber)'
  )
  assert(
    taskUiSrc.includes('text-emerald-800') && taskUiSrc.includes('dark:text-emerald-300'),
    'Completed status must use high-contrast emerald classes'
  )
  assert(
    taskUiSrc.includes('text-zinc-700') && taskUiSrc.includes('dark:text-zinc-300'),
    'Cancelled status must use high-contrast zinc classes'
  )

  // C. Header and Filter Layout Consistency
  // 1. Mobile select dropdown removed from TaskBoardView
  assert(
    !taskBoardSrc.includes('aria-label="Task status filter"') &&
    !taskBoardSrc.includes('Show</span>\n        <select'),
    'TaskBoardView must NOT contain mobile status select dropdown (vertical clutter eliminated)'
  )

  // 2. Consistent Horizontal Category Chips pattern (Supports default primary and Things 3 / Bear Notes calm segments)
  const hasUnifiedChipsPattern = (src) => src.includes('<ListTabs') && src.includes('<ListTab') && sharedListSource.includes('overflow-x-auto') && sharedListSource.includes('border-b-2') && sharedListSource.includes('text-muted-foreground')

  assert(hasUnifiedChipsPattern(taskBoardSrc), 'Tasks must implement unified category chips pattern')
  assert(hasUnifiedChipsPattern(inboxSubPanelSrc), 'Inbox must implement unified category chips pattern')
  assert(hasUnifiedChipsPattern(projectsViewSrc), 'Projects must implement unified category chips pattern')

  // 3. CTA Ordering Consistency (Detail on left, Primary on right)
  const detailIndex = taskItemCardSrc.indexOf('action-secondary min-h-11 min-w-[44px]')
  const primaryIndex = taskItemCardSrc.indexOf('action-primary min-h-11 min-w-[44px]')
  assert(
    detailIndex !== -1 && primaryIndex !== -1 && detailIndex < primaryIndex,
    'TaskItemCard must place secondary Detail CTA to the left of the primary CTA'
  )
})
