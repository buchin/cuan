import test from 'node:test'
import assert from 'node:assert'
import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)

const projectsApiPath = path.join(__dirname, '../src/api/projects.ts')
const readyPagePath = path.join(__dirname, '../src/features/tasks/TaskReadyPage.tsx')
const localesPath = path.join(__dirname, '../src/locales/id.json')
const routesProjectsPath = path.join(__dirname, '../../internal/server/routes_projects.go')
const inboxLifecyclePath = path.join(__dirname, '../../internal/server/inbox_lifecycle.go')
const launchDbPath = path.join(__dirname, '../../internal/task/launch_db.go')

const apiSource = fs.readFileSync(projectsApiPath, 'utf8')
const readySource = fs.readFileSync(readyPagePath, 'utf8')
const localesSource = fs.readFileSync(localesPath, 'utf8')
const routesSource = fs.readFileSync(routesProjectsPath, 'utf8')
const inboxSource = fs.readFileSync(inboxLifecyclePath, 'utf8')
const launchDbSource = fs.readFileSync(launchDbPath, 'utf8')

test('1. API: projectsApi includes reconcile method targeting /projects/:id/reconcile', () => {
  assert.ok(
    apiSource.includes('reconcile: async (id: string)'),
    'projects.ts must export reconcile method'
  )
  assert.ok(
    apiSource.includes('/reconcile'),
    'reconcile method must POST to /reconcile endpoint'
  )
})

test('2. TaskReadyPage: renders Bebaskan Antrean Project action button during queue waiting', () => {
  assert.ok(
    readySource.includes('projectsApi.reconcile'),
    'TaskReadyPage must invoke projectsApi.reconcile'
  )
  assert.ok(
    readySource.includes("t('Bebaskan Antrean Project')"),
    'TaskReadyPage must display translated button label'
  )
})

test('3. Localization: Indonesian dictionary contains Bebaskan Antrean Project translation', () => {
  const idDict = JSON.parse(localesSource)
  assert.strictEqual(
    idDict['Bebaskan Antrean Project'],
    'Bebaskan Antrean Project',
    'id.json must include translation for Bebaskan Antrean Project'
  )
})

test('4. Backend Routes: routes_projects.go registers POST /projects/{id}/reconcile', () => {
  assert.ok(
    routesSource.includes('pr.Post("/projects/{id}/reconcile", s.handleReconcileProject)'),
    'routes_projects.go must register /projects/{id}/reconcile route'
  )
  assert.ok(
    routesSource.includes('handleReconcileProject'),
    'routes_projects.go must implement handleReconcileProject handler'
  )
})

test('5. Watchdog Loop: inbox_lifecycle.go periodic scanner reconciles expired leases and orphan launches', () => {
  assert.ok(
    inboxSource.includes('ReconcileExpiredLeases'),
    'inbox_lifecycle.go periodic loop must call ReconcileExpiredLeases'
  )
  assert.ok(
    inboxSource.includes('ReconcileOrphanLaunches'),
    'inbox_lifecycle.go periodic loop must call ReconcileOrphanLaunches'
  )
})

test('6. Zombie & Terminal Task Purging: launch_db.go cleans orphan started launches and stale terminal tasks', () => {
  assert.ok(
    launchDbSource.includes("title LIKE 'Terminal · %'"),
    'launch_db.go ReconcileOrphanLaunches must auto-cancel stale queued terminal tasks'
  )
  assert.ok(
    launchDbSource.includes("Execution attempt is no longer active"),
    'launch_db.go ReconcileOrphanLaunches must clear zombie started launches'
  )
})
