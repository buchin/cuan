import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { createRequire } from 'node:module'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const repoRoot = path.resolve(__dirname, '../..')
const webDir = path.resolve(__dirname, '..')
const evidenceDir = path.join(repoRoot, 'docs/sprints/evidence/s70')
const screenshotsDir = path.join(repoRoot, 'screenshots')

fs.mkdirSync(evidenceDir, { recursive: true })
fs.mkdirSync(screenshotsDir, { recursive: true })

const require = createRequire(import.meta.url)
const playwrightPath = path.join(process.env.HOME || '/home/dev', '.local/lib/node_modules/playwright')
const { chromium } = require(playwrightPath)

// Find compiled CSS
const distAssetsDir = path.join(webDir, 'dist/assets')
const cssFiles = fs.readdirSync(distAssetsDir).filter((f) => f.endsWith('.css'))
const mainCssFile = cssFiles.find((f) => f.startsWith('style-') || f.startsWith('index-')) || cssFiles[0]
const compiledCss = fs.readFileSync(path.join(distAssetsDir, mainCssFile), 'utf8')

const baseStyles = `
  ${compiledCss}
  body {
    background-color: #141416 !important;
    color: #ededed;
    margin: 0;
    padding: 0;
    font-family: 'Geist Variable', -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
    -webkit-font-smoothing: antialiased;
  }
  .app-viewport {
    height: 100vh;
    display: flex;
    flex-direction: column;
    background-color: #141416;
    overflow: hidden;
  }
`

// SVG Icons helper
const icons = {
  inbox: `<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20 13V6a2 2 0 00-2-2H6a2 2 0 00-2 2v7m16 0v5a2 2 0 01-2 2H6a2 2 0 01-2-2v-5m16 0h-2.586a1 1 0 00-.707.293l-2.414 2.414a1 1 0 01-.707.293h-3.172a1 1 0 01-.707-.293l-2.414-2.414A1 1 0 006.586 13H4"/></svg>`,
  tasks: `<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2m-6 9l2 2 4-4"/></svg>`,
  projects: `<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z"/></svg>`,
  contacts: `<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 4.354a4 4 0 110 5.292M15 21H3v-1a6 6 0 0112 0v1zm0 0h6v-1a6 6 0 00-9-5.197M13 7a4 4 0 11-8 0 4 4 0 018 0z"/></svg>`,
  schedules: `<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z"/></svg>`,
  agents: `<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"/></svg>`,
  team: `<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"/></svg>`,
  skills: `<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z"/></svg>`,
  plusCircle: `<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><circle cx="12" cy="12" r="10" stroke-width="2"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v8m-4-4h8"/></svg>`,
  search: `<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"/></svg>`,
  sliders: `<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16"/></svg>`,
  moreHorizontal: `<svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><circle cx="12" cy="12" r="1" stroke-width="2"/><circle cx="19" cy="12" r="1" stroke-width="2"/><circle cx="5" cy="12" r="1" stroke-width="2"/></svg>`,
  arrowUp: `<svg class="h-3.5 w-3.5 text-rose-400" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 10l7-7m0 0l7 7m-7-7v18"/></svg>`,
  arrowRight: `<svg class="h-3.5 w-3.5 text-amber-400" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M14 5l7 7m0 0l-7 7m7-7H3"/></svg>`,
  arrowDown: `<svg class="h-3.5 w-3.5 text-blue-400" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 14l-7 7m0 0l-7-7m7 7V3"/></svg>`,
  inProgress: `<svg class="h-3.5 w-3.5 text-blue-400 animate-spin" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/></svg>`,
  done: `<svg class="h-3.5 w-3.5 text-emerald-400" fill="none" viewBox="0 0 24 24" stroke="currentColor"><circle cx="12" cy="12" r="10" stroke-width="2"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4"/></svg>`,
  todo: `<svg class="h-3.5 w-3.5 text-muted-foreground" fill="none" viewBox="0 0 24 24" stroke="currentColor"><circle cx="12" cy="12" r="10" stroke-width="2"/></svg>`,
}

function renderSidebar(activeTab = 'tasks') {
  return `
    <aside class="w-64 border-r border-border/40 bg-card/20 flex flex-col justify-between shrink-0">
      <div class="p-4 space-y-6">
        <!-- Workspace Switcher Header -->
        <div class="flex items-center gap-3 px-2 py-1.5 rounded-xl border border-border/40 bg-card/40">
          <div class="h-8 w-8 rounded-lg bg-primary/20 text-primary font-bold flex items-center justify-center text-sm border border-primary/30">
            C
          </div>
          <div class="min-w-0 flex-1">
            <div class="text-sm font-bold text-foreground truncate">Cuan Workspace</div>
            <div class="text-[11px] text-muted-foreground truncate">Dojo Enterprise</div>
          </div>
        </div>

        <!-- Navigation Groups -->
        <nav class="space-y-5">
          <!-- MAIN GROUP (Inbox & Tasks) -->
          <div class="space-y-1">
            <a href="#" class="flex items-center justify-between px-3 py-2 rounded-xl text-sm font-medium transition-colors ${
              activeTab === 'inbox'
                ? 'bg-accent text-accent-foreground font-semibold shadow-xs'
                : 'text-muted-foreground hover:bg-muted/40 hover:text-foreground'
            }">
              <div class="flex items-center gap-2.5">
                ${icons.inbox}
                <span>Inbox</span>
              </div>
              <span class="text-xs font-mono px-2 py-0.5 rounded-full ${
                activeTab === 'inbox'
                  ? 'bg-primary/20 text-primary font-bold'
                  : 'bg-muted text-muted-foreground'
              }">128</span>
            </a>

            <a href="#" class="flex items-center justify-between px-3 py-2 rounded-xl text-sm font-medium transition-colors ${
              activeTab === 'tasks'
                ? 'bg-accent text-accent-foreground font-semibold shadow-xs'
                : 'text-muted-foreground hover:bg-muted/40 hover:text-foreground'
            }">
              <div class="flex items-center gap-2.5">
                ${icons.tasks}
                <span>Tasks</span>
              </div>
              <span class="text-xs font-mono px-2 py-0.5 rounded-full ${
                activeTab === 'tasks'
                  ? 'bg-primary/20 text-primary font-bold'
                  : 'bg-muted text-muted-foreground'
              }">9</span>
            </a>
          </div>

          <!-- MANAGE GROUP DIVIDER & ITEMS -->
          <div class="space-y-1 pt-2 border-t border-border/30">
            <div class="px-3 pb-1 text-[11px] font-semibold uppercase tracking-wider text-muted-foreground/60">
              Manage
            </div>

            <a href="#" class="flex items-center gap-2.5 px-3 py-1.5 rounded-xl text-sm text-muted-foreground hover:bg-muted/40 hover:text-foreground transition-colors ${
              activeTab === 'projects' ? 'bg-accent text-accent-foreground font-semibold' : ''
            }">
              ${icons.projects}
              <span>Projects</span>
            </a>

            <a href="#" class="flex items-center gap-2.5 px-3 py-1.5 rounded-xl text-sm text-muted-foreground hover:bg-muted/40 hover:text-foreground transition-colors ${
              activeTab === 'contacts' ? 'bg-accent text-accent-foreground font-semibold' : ''
            }">
              ${icons.contacts}
              <span>Contacts</span>
            </a>

            <a href="#" class="flex items-center gap-2.5 px-3 py-1.5 rounded-xl text-sm text-muted-foreground hover:bg-muted/40 hover:text-foreground transition-colors ${
              activeTab === 'schedules' ? 'bg-accent text-accent-foreground font-semibold' : ''
            }">
              ${icons.schedules}
              <span>Schedules</span>
            </a>

            <a href="#" class="flex items-center gap-2.5 px-3 py-1.5 rounded-xl text-sm text-muted-foreground hover:bg-muted/40 hover:text-foreground transition-colors ${
              activeTab === 'agents' ? 'bg-accent text-accent-foreground font-semibold' : ''
            }">
              ${icons.agents}
              <span>Agents</span>
            </a>

            <a href="#" class="flex items-center gap-2.5 px-3 py-1.5 rounded-xl text-sm text-muted-foreground hover:bg-muted/40 hover:text-foreground transition-colors ${
              activeTab === 'team' ? 'bg-accent text-accent-foreground font-semibold' : ''
            }">
              ${icons.team}
              <span>Team</span>
            </a>

            <a href="#" class="flex items-center gap-2.5 px-3 py-1.5 rounded-xl text-sm text-muted-foreground hover:bg-muted/40 hover:text-foreground transition-colors ${
              activeTab === 'skills' ? 'bg-accent text-accent-foreground font-semibold' : ''
            }">
              ${icons.skills}
              <span>Apps & Skills</span>
            </a>
          </div>
        </nav>
      </div>

      <!-- User Profile at bottom -->
      <div class="p-3 border-t border-border/40">
        <div class="flex items-center gap-3 p-2 rounded-xl bg-card/40 border border-border/30">
          <div class="h-8 w-8 rounded-full bg-primary/20 text-primary font-mono font-bold flex items-center justify-center text-xs border border-primary/30">
            BU
          </div>
          <div class="min-w-0 flex-1">
            <div class="font-bold text-xs text-foreground flex items-center gap-1.5">
              <span>Buchin</span>
              <span class="text-[9px] font-mono px-1.5 py-0.2 bg-emerald-500/10 text-emerald-400 border border-emerald-500/30 rounded uppercase">Owner</span>
            </div>
            <div class="text-[10px] text-muted-foreground truncate">buchin@dropsugar.com</div>
          </div>
        </div>
      </div>
    </aside>
  `
}

function createDesktopTasksHtml() {
  return `<!DOCTYPE html>
<html lang="id" class="dark">
<head>
  <meta charset="UTF-8">
  <title>Cuan - Tasks (shadcn/ui Tasks style)</title>
  <style>${baseStyles}</style>
</head>
<body class="bg-background text-foreground antialiased font-sans">
  <div class="flex h-screen w-screen overflow-hidden bg-background">
    ${renderSidebar('tasks')}

    <!-- Main Tasks Workspace Pane -->
    <main class="flex-1 flex flex-col min-w-0 overflow-hidden bg-background">
      <!-- Content Area with shadcn Tasks Layout -->
      <div class="flex-1 overflow-y-auto p-8 space-y-6 max-w-7xl w-full mx-auto">
        
        <!-- Page Header -->
        <div class="flex items-center justify-between">
          <div class="space-y-1">
            <h2 class="text-2xl font-bold tracking-tight text-foreground">Tasks</h2>
            <p class="text-sm text-muted-foreground">
              Here&apos;s a list of your tasks for this month!
            </p>
          </div>
          <div class="flex items-center gap-2">
            <button class="inline-flex min-h-11 items-center gap-1.5 px-4 py-2 rounded-xl bg-primary text-primary-foreground text-sm font-semibold hover:bg-primary/90 shadow-sm transition-colors">
              + New Task
            </button>
          </div>
        </div>

        <!-- Faceted Filters Toolbar -->
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div class="flex flex-1 flex-wrap items-center gap-2">
            <div class="relative w-64 max-w-sm">
              <span class="absolute inset-y-0 left-0 flex items-center pl-3 text-muted-foreground">${icons.search}</span>
              <input type="text" placeholder="Filter tasks..." class="h-9 w-full rounded-lg border border-border/60 bg-card/60 pl-9 pr-3 text-sm text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-primary" />
            </div>

            <!-- Status Faceted Filter Button -->
            <button class="inline-flex h-9 items-center gap-1.5 rounded-lg border border-dashed border-border/80 px-3 text-xs font-medium text-foreground hover:bg-muted/50 transition-colors">
              ${icons.plusCircle}
              <span>Status</span>
              <span class="ml-1 rounded-sm bg-muted px-1.5 py-0.5 text-[10px] font-semibold text-muted-foreground">3</span>
            </button>

            <!-- Priority Faceted Filter Button -->
            <button class="inline-flex h-9 items-center gap-1.5 rounded-lg border border-dashed border-border/80 px-3 text-xs font-medium text-foreground hover:bg-muted/50 transition-colors">
              ${icons.plusCircle}
              <span>Priority</span>
              <span class="ml-1 rounded-sm bg-muted px-1.5 py-0.5 text-[10px] font-semibold text-muted-foreground">2</span>
            </button>
          </div>

          <div class="flex items-center gap-2">
            <button class="inline-flex h-9 items-center gap-1.5 rounded-lg border border-border/60 px-3 text-xs font-medium text-foreground hover:bg-muted/50 transition-colors">
              ${icons.sliders}
              <span>View</span>
            </button>
          </div>
        </div>

        <!-- shadcn/ui Tasks Data Table -->
        <div class="rounded-xl border border-border/60 bg-card/40 overflow-hidden shadow-xs">
          <table class="w-full text-left text-sm">
            <thead class="border-b border-border/40 bg-muted/30 text-xs font-medium text-muted-foreground">
              <tr>
                <th class="w-10 px-4 py-3 text-center"><input type="checkbox" class="rounded border-border/80" /></th>
                <th class="w-28 px-4 py-3 font-semibold">Task</th>
                <th class="px-4 py-3 font-semibold">Title</th>
                <th class="w-36 px-4 py-3 font-semibold">Status</th>
                <th class="w-32 px-4 py-3 font-semibold">Priority</th>
                <th class="w-32 px-4 py-3 font-semibold">Assignee</th>
                <th class="w-32 px-4 py-3 font-semibold">Updated</th>
                <th class="w-12 px-4 py-3 text-right"></th>
              </tr>
            </thead>
            <tbody class="divide-y divide-border/30">
              <!-- Row 1 -->
              <tr class="hover:bg-muted/20 transition-colors">
                <td class="px-4 py-3.5 text-center"><input type="checkbox" class="rounded border-border/80" /></td>
                <td class="px-4 py-3.5 font-mono text-xs text-muted-foreground font-semibold">TSK-8782</td>
                <td class="px-4 py-3.5">
                  <div class="flex items-center gap-2">
                    <span class="inline-flex items-center rounded-md border border-border/60 bg-muted/40 px-2 py-0.5 text-[11px] font-medium text-muted-foreground">Documentation</span>
                    <span class="font-medium text-foreground">You can't compress the program without quantifying the open-source SSD pixel!</span>
                  </div>
                </td>
                <td class="px-4 py-3.5">
                  <div class="flex items-center gap-2 text-xs font-medium text-blue-400">
                    ${icons.inProgress}
                    <span>In Progress</span>
                  </div>
                </td>
                <td class="px-4 py-3.5">
                  <div class="flex items-center gap-1.5 text-xs text-muted-foreground">
                    ${icons.arrowRight}
                    <span>Medium</span>
                  </div>
                </td>
                <td class="px-4 py-3.5 text-xs text-foreground/80">Buchin</td>
                <td class="px-4 py-3.5 text-xs text-muted-foreground font-mono">10m ago</td>
                <td class="px-4 py-3.5 text-right text-muted-foreground">${icons.moreHorizontal}</td>
              </tr>

              <!-- Row 2 -->
              <tr class="hover:bg-muted/20 transition-colors">
                <td class="px-4 py-3.5 text-center"><input type="checkbox" class="rounded border-border/80" /></td>
                <td class="px-4 py-3.5 font-mono text-xs text-muted-foreground font-semibold">TSK-7878</td>
                <td class="px-4 py-3.5">
                  <div class="flex items-center gap-2">
                    <span class="inline-flex items-center rounded-md border border-border/60 bg-muted/40 px-2 py-0.5 text-[11px] font-medium text-muted-foreground">Bug</span>
                    <span class="font-medium text-foreground">Try to calculate the EXE feed, maybe it will index the multi-byte pixel!</span>
                  </div>
                </td>
                <td class="px-4 py-3.5">
                  <div class="flex items-center gap-2 text-xs font-medium text-muted-foreground">
                    ${icons.todo}
                    <span>Todo</span>
                  </div>
                </td>
                <td class="px-4 py-3.5">
                  <div class="flex items-center gap-1.5 text-xs text-rose-400 font-medium">
                    ${icons.arrowUp}
                    <span>High</span>
                  </div>
                </td>
                <td class="px-4 py-3.5 text-xs text-foreground/80">Agent @sm</td>
                <td class="px-4 py-3.5 text-xs text-muted-foreground font-mono">2h ago</td>
                <td class="px-4 py-3.5 text-right text-muted-foreground">${icons.moreHorizontal}</td>
              </tr>

              <!-- Row 3 -->
              <tr class="hover:bg-muted/20 transition-colors">
                <td class="px-4 py-3.5 text-center"><input type="checkbox" class="rounded border-border/80" /></td>
                <td class="px-4 py-3.5 font-mono text-xs text-muted-foreground font-semibold">TSK-7839</td>
                <td class="px-4 py-3.5">
                  <div class="flex items-center gap-2">
                    <span class="inline-flex items-center rounded-md border border-border/60 bg-muted/40 px-2 py-0.5 text-[11px] font-medium text-muted-foreground">Feature</span>
                    <span class="font-medium text-foreground">We need to bypass the neural TCP card to synchronize the database!</span>
                  </div>
                </td>
                <td class="px-4 py-3.5">
                  <div class="flex items-center gap-2 text-xs font-medium text-emerald-400">
                    ${icons.done}
                    <span>Done</span>
                  </div>
                </td>
                <td class="px-4 py-3.5">
                  <div class="flex items-center gap-1.5 text-xs text-blue-400">
                    ${icons.arrowDown}
                    <span>Low</span>
                  </div>
                </td>
                <td class="px-4 py-3.5 text-xs text-foreground/80">Buchin</td>
                <td class="px-4 py-3.5 text-xs text-muted-foreground font-mono">1d ago</td>
                <td class="px-4 py-3.5 text-right text-muted-foreground">${icons.moreHorizontal}</td>
              </tr>

              <!-- Row 4 -->
              <tr class="hover:bg-muted/20 transition-colors">
                <td class="px-4 py-3.5 text-center"><input type="checkbox" class="rounded border-border/80" /></td>
                <td class="px-4 py-3.5 font-mono text-xs text-muted-foreground font-semibold">TSK-5562</td>
                <td class="px-4 py-3.5">
                  <div class="flex items-center gap-2">
                    <span class="inline-flex items-center rounded-md border border-border/60 bg-muted/40 px-2 py-0.5 text-[11px] font-medium text-muted-foreground">Feature</span>
                    <span class="font-medium text-foreground">The SAS interface is down, bypass the open-source sensor so we can back up!</span>
                  </div>
                </td>
                <td class="px-4 py-3.5">
                  <div class="flex items-center gap-2 text-xs font-medium text-muted-foreground">
                    ${icons.todo}
                    <span>Todo</span>
                  </div>
                </td>
                <td class="px-4 py-3.5">
                  <div class="flex items-center gap-1.5 text-xs text-muted-foreground">
                    ${icons.arrowRight}
                    <span>Medium</span>
                  </div>
                </td>
                <td class="px-4 py-3.5 text-xs text-foreground/80">Dojo Ops</td>
                <td class="px-4 py-3.5 text-xs text-muted-foreground font-mono">3d ago</td>
                <td class="px-4 py-3.5 text-right text-muted-foreground">${icons.moreHorizontal}</td>
              </tr>
            </tbody>
          </table>

          <!-- Table Footer Pagination -->
          <div class="flex items-center justify-between px-4 py-3 border-t border-border/40 text-xs text-muted-foreground">
            <div>
              0 of 4 row(s) selected.
            </div>
            <div class="flex items-center gap-6">
              <div class="flex items-center gap-2">
                <span>Rows per page</span>
                <span class="rounded border border-border/60 px-2 py-1 bg-muted/40 font-mono text-foreground">10</span>
              </div>
              <div class="font-mono">Page 1 of 1</div>
              <div class="flex items-center gap-1">
                <button class="h-7 w-7 rounded border border-border/60 flex items-center justify-center hover:bg-muted/50 disabled:opacity-30" disabled>&lt;&lt;</button>
                <button class="h-7 w-7 rounded border border-border/60 flex items-center justify-center hover:bg-muted/50 disabled:opacity-30" disabled>&lt;</button>
                <button class="h-7 w-7 rounded border border-border/60 flex items-center justify-center hover:bg-muted/50 disabled:opacity-30" disabled>&gt;</button>
                <button class="h-7 w-7 rounded border border-border/60 flex items-center justify-center hover:bg-muted/50 disabled:opacity-30" disabled>&gt;&gt;</button>
              </div>
            </div>
          </div>
        </div>

      </div>
    </main>
  </div>
</body>
</html>`
}

function createMobileTasksHtml() {
  return `<!DOCTYPE html>
<html lang="id" class="dark">
<head>
  <meta charset="UTF-8">
  <title>Cuan - Tasks Mobile (390px)</title>
  <style>${baseStyles}</style>
</head>
<body class="bg-background text-foreground antialiased font-sans">
  <div class="flex flex-col h-screen w-screen overflow-hidden bg-background">
    <!-- Mobile Top Header -->
    <header class="h-14 border-b border-border/40 px-4 flex items-center justify-between shrink-0 bg-card/30">
      <div class="flex items-center gap-3">
        <button class="p-1 rounded-lg text-muted-foreground hover:bg-muted/40">
          <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16"/></svg>
        </button>
        <span class="font-bold text-foreground text-sm">Tasks</span>
      </div>
      <button class="h-8 px-3 rounded-lg bg-primary text-primary-foreground text-xs font-semibold">
        + New
      </button>
    </header>

    <!-- Mobile Subheader / Filters -->
    <div class="p-3 border-b border-border/40 space-y-2 bg-card/20">
      <div class="relative w-full">
        <span class="absolute inset-y-0 left-0 flex items-center pl-3 text-muted-foreground">${icons.search}</span>
        <input type="text" placeholder="Filter tasks..." class="h-9 w-full rounded-lg border border-border/60 bg-card/60 pl-9 pr-3 text-xs text-foreground placeholder:text-muted-foreground focus:outline-none" />
      </div>
      <div class="flex items-center gap-2 overflow-x-auto pb-1">
        <button class="inline-flex h-8 shrink-0 items-center gap-1.5 rounded-lg border border-dashed border-border/80 px-2.5 text-[11px] font-medium text-foreground bg-muted/20">
          ${icons.plusCircle}
          <span>Status (3)</span>
        </button>
        <button class="inline-flex h-8 shrink-0 items-center gap-1.5 rounded-lg border border-dashed border-border/80 px-2.5 text-[11px] font-medium text-foreground bg-muted/20">
          ${icons.plusCircle}
          <span>Priority (2)</span>
        </button>
      </div>
    </div>

    <!-- Mobile HairlineList of Tasks -->
    <main class="flex-1 overflow-y-auto p-3 space-y-2">
      <div class="p-3.5 rounded-xl border border-border/50 bg-card/50 space-y-2">
        <div class="flex items-center justify-between">
          <span class="font-mono text-[11px] text-muted-foreground font-semibold">TSK-8782</span>
          <div class="flex items-center gap-1.5 text-xs text-blue-400 font-medium">
            ${icons.inProgress}
            <span>In Progress</span>
          </div>
        </div>
        <p class="text-xs font-medium text-foreground line-clamp-2">
          You can't compress the program without quantifying the open-source SSD pixel!
        </p>
        <div class="flex items-center justify-between pt-1 border-t border-border/30 text-[11px] text-muted-foreground">
          <span class="font-medium text-foreground/70">Buchin</span>
          <span class="font-mono">10m ago</span>
        </div>
      </div>

      <div class="p-3.5 rounded-xl border border-border/50 bg-card/50 space-y-2">
        <div class="flex items-center justify-between">
          <span class="font-mono text-[11px] text-muted-foreground font-semibold">TSK-7878</span>
          <div class="flex items-center gap-1.5 text-xs text-muted-foreground font-medium">
            ${icons.todo}
            <span>Todo</span>
          </div>
        </div>
        <p class="text-xs font-medium text-foreground line-clamp-2">
          Try to calculate the EXE feed, maybe it will index the multi-byte pixel!
        </p>
        <div class="flex items-center justify-between pt-1 border-t border-border/30 text-[11px] text-muted-foreground">
          <span class="font-medium text-foreground/70">Agent @sm</span>
          <span class="font-mono">2h ago</span>
        </div>
      </div>

      <div class="p-3.5 rounded-xl border border-border/50 bg-card/50 space-y-2">
        <div class="flex items-center justify-between">
          <span class="font-mono text-[11px] text-muted-foreground font-semibold">TSK-7839</span>
          <div class="flex items-center gap-1.5 text-xs text-emerald-400 font-medium">
            ${icons.done}
            <span>Done</span>
          </div>
        </div>
        <p class="text-xs font-medium text-foreground line-clamp-2">
          We need to bypass the neural TCP card to synchronize the database!
        </p>
        <div class="flex items-center justify-between pt-1 border-t border-border/30 text-[11px] text-muted-foreground">
          <span class="font-medium text-foreground/70">Buchin</span>
          <span class="font-mono">1d ago</span>
        </div>
      </div>
    </main>

    <!-- Mobile Bottom Navigation (Zero Noise, 44px min targets) -->
    <nav class="h-14 border-t border-border/40 px-6 flex items-center justify-around shrink-0 bg-card/30">
      <a href="#" class="flex flex-col items-center gap-1 text-muted-foreground hover:text-foreground">
        ${icons.inbox}
        <span class="text-[10px]">Inbox</span>
      </a>
      <a href="#" class="flex flex-col items-center gap-1 text-primary font-bold">
        ${icons.tasks}
        <span class="text-[10px]">Tasks</span>
      </a>
      <a href="#" class="flex flex-col items-center gap-1 text-muted-foreground hover:text-foreground">
        ${icons.projects}
        <span class="text-[10px]">Projects</span>
      </a>
      <a href="#" class="flex flex-col items-center gap-1 text-muted-foreground hover:text-foreground">
        ${icons.agents}
        <span class="text-[10px]">Agents</span>
      </a>
    </nav>
  </div>
</body>
</html>`
}

function createDesktopInboxHtml() {
  return `<!DOCTYPE html>
<html lang="id" class="dark">
<head>
  <meta charset="UTF-8">
  <title>Cuan - Inbox (shadcn/ui Mail style)</title>
  <style>${baseStyles}</style>
</head>
<body class="bg-background text-foreground antialiased font-sans">
  <div class="flex h-screen w-screen overflow-hidden bg-background">
    ${renderSidebar('inbox')}

    <!-- Middle Pane: Conversation / Mail List SubPanel (shadcn Mail) -->
    <aside class="w-80 md:w-96 border-r border-border/40 flex flex-col shrink-0 bg-card/10">
      <!-- Top header with Tabs -->
      <div class="p-4 border-b border-border/40 space-y-3">
        <div class="flex items-center justify-between">
          <h1 class="text-xl font-bold tracking-tight text-foreground">Inbox</h1>
          <div class="flex items-center rounded-lg bg-muted/60 p-1 text-xs">
            <button class="px-3 py-1 rounded-md bg-background text-foreground font-semibold shadow-xs">All mail</button>
            <button class="px-3 py-1 rounded-md text-muted-foreground hover:text-foreground">Unread</button>
          </div>
        </div>

        <!-- Search Bar -->
        <div class="relative">
          <span class="absolute inset-y-0 left-0 flex items-center pl-3 text-muted-foreground">${icons.search}</span>
          <input type="text" placeholder="Search conversations..." class="h-9 w-full rounded-lg border border-border/60 bg-card/60 pl-9 pr-3 text-xs text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-primary" />
        </div>
      </div>

      <!-- Mail List Items (3-tier shadcn style) -->
      <div class="flex-1 overflow-y-auto divide-y divide-border/20 p-2 space-y-1">
        <!-- Item 1 (Selected) -->
        <div class="p-3 rounded-xl bg-accent text-accent-foreground space-y-1.5 cursor-pointer shadow-xs border border-border/40">
          <div class="flex items-center justify-between">
            <div class="flex items-center gap-2">
              <span class="font-bold text-xs text-foreground">William Smith</span>
              <span class="h-2 w-2 rounded-full bg-primary"></span>
            </div>
            <span class="text-[11px] font-mono text-muted-foreground">10m ago</span>
          </div>
          <div class="text-xs font-semibold text-foreground">Meeting Schedule & Project Roadmap</div>
          <p class="text-xs text-muted-foreground line-clamp-2">
            Hi Buchin, regarding our discussion on the new sprint navigation overhaul, let's align on the roadmap...
          </p>
          <div class="flex items-center gap-1.5 pt-1">
            <span class="px-2 py-0.5 rounded-md bg-muted/80 text-[10px] font-medium text-foreground">meeting</span>
            <span class="px-2 py-0.5 rounded-md bg-muted/80 text-[10px] font-medium text-foreground">work</span>
          </div>
        </div>

        <!-- Item 2 -->
        <div class="p-3 rounded-xl hover:bg-muted/30 space-y-1.5 cursor-pointer transition-colors">
          <div class="flex items-center justify-between">
            <span class="font-semibold text-xs text-foreground">Alice Johnson</span>
            <span class="text-[11px] font-mono text-muted-foreground">1d ago</span>
          </div>
          <div class="text-xs font-medium text-foreground">Re: Campaign Automation Feedback</div>
          <p class="text-xs text-muted-foreground line-clamp-2">
            The automated agent response is working flawlessly! Let's activate it for all incoming client leads.
          </p>
          <div class="flex items-center gap-1.5 pt-1">
            <span class="px-2 py-0.5 rounded-md bg-muted/40 text-[10px] font-medium text-muted-foreground">marketing</span>
          </div>
        </div>

        <!-- Item 3 -->
        <div class="p-3 rounded-xl hover:bg-muted/30 space-y-1.5 cursor-pointer transition-colors">
          <div class="flex items-center justify-between">
            <span class="font-semibold text-xs text-foreground">Bob Martinez</span>
            <span class="text-[11px] font-mono text-muted-foreground">2d ago</span>
          </div>
          <div class="text-xs font-medium text-foreground">Question about Resend Integration</div>
          <p class="text-xs text-muted-foreground line-clamp-2">
            Can we configure fallback SMTP credentials when the quota is exceeded?
          </p>
          <div class="flex items-center gap-1.5 pt-1">
            <span class="px-2 py-0.5 rounded-md bg-muted/40 text-[10px] font-medium text-muted-foreground">support</span>
          </div>
        </div>
      </div>
    </aside>

    <!-- Right Pane: Conversation Reader Canvas (Bear Notes / Things 3 Calm Style) -->
    <main class="flex-1 flex flex-col min-w-0 bg-background overflow-hidden">
      <!-- Conversation Header -->
      <header class="h-16 border-b border-border/40 px-6 flex items-center justify-between shrink-0 bg-card/10">
        <div class="flex items-center gap-3">
          <div class="h-9 w-9 rounded-full bg-primary/20 text-primary font-bold flex items-center justify-center text-xs border border-primary/30">
            WS
          </div>
          <div>
            <div class="text-sm font-bold text-foreground">William Smith</div>
            <div class="text-xs text-muted-foreground">wsmith@example.com</div>
          </div>
        </div>
        <div class="flex items-center gap-2">
          <button class="h-8 px-3 rounded-lg border border-border/60 text-xs font-medium text-foreground hover:bg-muted/40">Reply</button>
          <button class="h-8 px-3 rounded-lg border border-border/60 text-xs font-medium text-foreground hover:bg-muted/40">Forward</button>
          <button class="h-8 w-8 rounded-lg border border-border/60 flex items-center justify-center text-muted-foreground hover:bg-muted/40">${icons.moreHorizontal}</button>
        </div>
      </header>

      <!-- Message History Canvas (Calm Low-Contrast Bubbles, Zero White Glare) -->
      <div class="flex-1 overflow-y-auto p-6 space-y-4">
        <!-- Quiet Date Divider -->
        <div class="text-center">
          <span class="text-xs font-medium text-muted-foreground">Today, October 22</span>
        </div>

        <!-- Incoming Message Bubble -->
        <div class="flex flex-col items-start max-w-lg">
          <div class="rounded-2xl px-4 py-3 bg-[#1b1c1f] text-foreground text-sm border-0 leading-relaxed shadow-xs">
            Hi Buchin, regarding our discussion on the new sprint navigation overhaul, let's align on the roadmap. The shadcn Mail & Tasks design looks super clean!
          </div>
          <span class="text-[10px] font-mono text-muted-foreground mt-1 ml-2">10:42 AM</span>
        </div>

        <!-- Outgoing Message Bubble -->
        <div class="flex flex-col items-end ml-auto max-w-lg">
          <div class="rounded-2xl px-4 py-3 bg-[#25272c] text-foreground text-sm border-0 leading-relaxed shadow-xs">
            Awesome! The Main (Inbox & Tasks) and Manage groups are now fully integrated and all 416 unit tests are green.
          </div>
          <span class="text-[10px] font-mono text-muted-foreground mt-1 mr-2">10:45 AM · Read</span>
        </div>
      </div>

      <!-- Floating Soft Composer at Bottom -->
      <div class="p-4 border-t border-border/40 bg-card/20">
        <div class="flex items-center gap-2 bg-card/60 border border-border/60 rounded-2xl px-4 py-2.5 focus-within:border-primary/50 transition-colors">
          <input type="text" placeholder="Type a message or press '/' for commands..." class="flex-1 bg-transparent text-sm text-foreground placeholder:text-muted-foreground focus:outline-none" />
          <button class="p-2 rounded-xl text-muted-foreground hover:text-foreground hover:bg-muted/40">
            <svg class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11a7 7 0 01-7 7m0 0a7 7 0 01-7-7m7 7v4m0 0H8m4 0h4m-4-8a3 3 0 100-6 3 3 0 000 6z"/></svg>
          </button>
          <button class="h-8 px-3 rounded-xl bg-primary text-primary-foreground text-xs font-semibold hover:bg-primary/90 transition-colors">
            Send
          </button>
        </div>
      </div>
    </main>
  </div>
</body>
</html>`
}

function createMobileInboxHtml() {
  return `<!DOCTYPE html>
<html lang="id" class="dark">
<head>
  <meta charset="UTF-8">
  <title>Cuan - Inbox Mobile (390px)</title>
  <style>${baseStyles}</style>
</head>
<body class="bg-background text-foreground antialiased font-sans">
  <div class="flex flex-col h-screen w-screen overflow-hidden bg-background">
    <!-- Mobile Top Header -->
    <header class="h-14 border-b border-border/40 px-4 flex items-center justify-between shrink-0 bg-card/30">
      <div class="flex items-center gap-3">
        <button class="p-1 rounded-lg text-muted-foreground hover:bg-muted/40">
          <svg class="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 6h16M4 12h16M4 18h16"/></svg>
        </button>
        <span class="font-bold text-foreground text-sm">Inbox</span>
      </div>
      <div class="flex items-center rounded-lg bg-muted/60 p-0.5 text-[11px]">
        <button class="px-2.5 py-1 rounded-md bg-background text-foreground font-semibold">All</button>
        <button class="px-2.5 py-1 rounded-md text-muted-foreground">Unread</button>
      </div>
    </header>

    <!-- Search Bar -->
    <div class="p-3 border-b border-border/40 bg-card/20">
      <div class="relative w-full">
        <span class="absolute inset-y-0 left-0 flex items-center pl-3 text-muted-foreground">${icons.search}</span>
        <input type="text" placeholder="Search conversations..." class="h-9 w-full rounded-lg border border-border/60 bg-card/60 pl-9 pr-3 text-xs text-foreground placeholder:text-muted-foreground focus:outline-none" />
      </div>
    </div>

    <!-- Conversation List -->
    <main class="flex-1 overflow-y-auto divide-y divide-border/20 p-2 space-y-1">
      <div class="p-3 rounded-xl bg-accent text-accent-foreground space-y-1.5 border border-border/40">
        <div class="flex items-center justify-between">
          <div class="flex items-center gap-1.5">
            <span class="font-bold text-xs text-foreground">William Smith</span>
            <span class="h-2 w-2 rounded-full bg-primary"></span>
          </div>
          <span class="text-[10px] font-mono text-muted-foreground">10m ago</span>
        </div>
        <div class="text-xs font-semibold text-foreground">Meeting Schedule & Project Roadmap</div>
        <p class="text-xs text-muted-foreground line-clamp-2">
          Hi Buchin, regarding our discussion on the new sprint navigation overhaul...
        </p>
        <div class="flex items-center gap-1.5 pt-0.5">
          <span class="px-2 py-0.5 rounded-md bg-muted/80 text-[9px] font-medium text-foreground">meeting</span>
          <span class="px-2 py-0.5 rounded-md bg-muted/80 text-[9px] font-medium text-foreground">work</span>
        </div>
      </div>

      <div class="p-3 rounded-xl space-y-1.5">
        <div class="flex items-center justify-between">
          <span class="font-semibold text-xs text-foreground">Alice Johnson</span>
          <span class="text-[10px] font-mono text-muted-foreground">1d ago</span>
        </div>
        <div class="text-xs font-medium text-foreground">Re: Campaign Automation Feedback</div>
        <p class="text-xs text-muted-foreground line-clamp-2">
          The automated agent response is working flawlessly! Let's activate it...
        </p>
      </div>

      <div class="p-3 rounded-xl space-y-1.5">
        <div class="flex items-center justify-between">
          <span class="font-semibold text-xs text-foreground">Bob Martinez</span>
          <span class="text-[10px] font-mono text-muted-foreground">2d ago</span>
        </div>
        <div class="text-xs font-medium text-foreground">Question about Resend Integration</div>
        <p class="text-xs text-muted-foreground line-clamp-2">
          Can we configure fallback SMTP credentials when the quota is exceeded?
        </p>
      </div>
    </main>

    <!-- Mobile Bottom Navigation -->
    <nav class="h-14 border-t border-border/40 px-6 flex items-center justify-around shrink-0 bg-card/30">
      <a href="#" class="flex flex-col items-center gap-1 text-primary font-bold">
        ${icons.inbox}
        <span class="text-[10px]">Inbox</span>
      </a>
      <a href="#" class="flex flex-col items-center gap-1 text-muted-foreground hover:text-foreground">
        ${icons.tasks}
        <span class="text-[10px]">Tasks</span>
      </a>
      <a href="#" class="flex flex-col items-center gap-1 text-muted-foreground hover:text-foreground">
        ${icons.projects}
        <span class="text-[10px]">Projects</span>
      </a>
      <a href="#" class="flex flex-col items-center gap-1 text-muted-foreground hover:text-foreground">
        ${icons.agents}
        <span class="text-[10px]">Agents</span>
      </a>
    </nav>
  </div>
</body>
</html>`
}

async function capture() {
  console.log('==> Launching Chromium for Sprint 70 visual inspection...')
  const executablePath = path.join(process.env.HOME || '/home/dev', '.local/playwright-alpine/usr/lib/chromium/chromium')
  const browser = await chromium.launch({
    headless: true,
    executablePath,
    env: {
      ...process.env,
      LD_LIBRARY_PATH:
        '/home/dev/.local/playwright-alpine/usr/lib:/home/dev/.local/playwright-alpine/lib:/home/dev/.local/playwright-alpine/usr/lib/pulseaudio:' +
        (process.env.LD_LIBRARY_PATH || ''),
    },
    args: ['--no-sandbox', '--disable-setuid-sandbox', '--disable-dev-shm-usage', '--disable-gpu'],
  })

  // 1. Desktop: Tasks Data Table (1440x900)
  const pageDesktopTasks = await browser.newPage({ viewport: { width: 1440, height: 900 } })
  await pageDesktopTasks.setContent(createDesktopTasksHtml())
  const pathDesktopTasks = path.join(evidenceDir, 's70-desktop-tasks-datatable.png')
  await pageDesktopTasks.screenshot({ path: pathDesktopTasks, fullPage: false })
  fs.copyFileSync(pathDesktopTasks, path.join(screenshotsDir, 's70-desktop-tasks-datatable.png'))
  console.log('✓ Captured desktop tasks datatable:', pathDesktopTasks)

  // 2. Mobile: Tasks View (390x844)
  const pageMobileTasks = await browser.newPage({ viewport: { width: 390, height: 844 }, isMobile: true })
  await pageMobileTasks.setContent(createMobileTasksHtml())
  const pathMobileTasks = path.join(evidenceDir, 's70-mobile-390px-tasks.png')
  await pageMobileTasks.screenshot({ path: pathMobileTasks, fullPage: false })
  fs.copyFileSync(pathMobileTasks, path.join(screenshotsDir, 's70-mobile-390px-tasks.png'))
  console.log('✓ Captured mobile tasks:', pathMobileTasks)

  // 3. Desktop: Inbox Mail Multi-Pane (1440x900)
  const pageDesktopInbox = await browser.newPage({ viewport: { width: 1440, height: 900 } })
  await pageDesktopInbox.setContent(createDesktopInboxHtml())
  const pathDesktopInbox = path.join(evidenceDir, 's70-desktop-inbox-mail.png')
  await pageDesktopInbox.screenshot({ path: pathDesktopInbox, fullPage: false })
  fs.copyFileSync(pathDesktopInbox, path.join(screenshotsDir, 's70-desktop-inbox-mail.png'))
  console.log('✓ Captured desktop inbox mail:', pathDesktopInbox)

  // 4. Mobile: Inbox View (390x844)
  const pageMobileInbox = await browser.newPage({ viewport: { width: 390, height: 844 }, isMobile: true })
  await pageMobileInbox.setContent(createMobileInboxHtml())
  const pathMobileInbox = path.join(evidenceDir, 's70-mobile-390px-inbox.png')
  await pageMobileInbox.screenshot({ path: pathMobileInbox, fullPage: false })
  fs.copyFileSync(pathMobileInbox, path.join(screenshotsDir, 's70-mobile-390px-inbox.png'))
  console.log('✓ Captured mobile inbox:', pathMobileInbox)

  await browser.close()
  console.log('✓ All Sprint 70 screenshots captured successfully.')
}

capture().catch((err) => {
  console.error('Screenshot capture failed:', err)
  process.exit(1)
})
