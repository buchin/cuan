import {
  registerExtension,
  enableExtension,
  type FenceProps,
  type ViewProps,
} from './sdk'

// Explicitly opt in from a trusted build/bootstrap. Never enable from message
// content, URL parameters, or arbitrary downloaded JavaScript.
export function installExampleExtension() {
  registerExtension({
    manifest: {
      id: 'example.productivity',
      name: 'Productivity example',
      version: '1.0.0',
      api_version: 1,
      capabilities: ['markdown:callout', 'view:project', 'parser:brand-token'],
    },
    fences: [
      {
        language: 'callout',
        component: ({ code }: FenceProps) => (
          <aside
            role="note"
            className="rounded-lg border-l-4 border-primary bg-muted p-3 text-sm whitespace-pre-wrap"
          >
            {code}
          </aside>
        ),
      },
    ],
    parsers: [
      {
        id: 'brand-token',
        plugin:
          () =>
          (tree: { type?: string; value?: string; children?: unknown[] }) => {
            const visit = (node: {
              type?: string
              value?: string
              children?: unknown[]
            }) => {
              if (node.type === 'text' && node.value)
                node.value = node.value.replace(/:cuan:/g, 'Cuan')
              for (const child of node.children || [])
                if (child && typeof child === 'object')
                  visit(child as typeof node)
            }
            visit(tree)
          },
      },
    ],
    projectViews: [
      {
        id: 'kanban',
        label: 'Kanban',
        component: ({ tasks, openTask }: ViewProps) => (
          <div className="grid gap-3 md:grid-cols-3">
            {[
              ['Ready', ['backlog', 'todo']],
              ['In progress', ['doing']],
              ['Result ready', ['review']],
              ['Needs attention', ['blocked']],
              ['Completed', ['done']],
            ].map(([label, statuses]) => (
              <section
                key={label as string}
                className="min-w-0 rounded-lg border bg-muted/20 p-3"
              >
                <h4 className="mb-3 text-sm font-semibold">{label}</h4>
                <div className="space-y-2">
                  {tasks
                    .filter((task) =>
                      (statuses as string[]).includes(task.status),
                    )
                    .map((task) => (
                      <button
                        key={task.id}
                        onClick={() => openTask(task.id)}
                        className="block min-h-11 w-full rounded-md border bg-background p-3 text-left text-sm"
                      >
                        <span className="block break-words">{task.title}</span>
                        <span className="mt-1 block text-xs text-muted-foreground">
                          {task.status}
                        </span>
                      </button>
                    ))}
                </div>
              </section>
            ))}
          </div>
        ),
      },
    ],
  })
  enableExtension('example.productivity')
}
