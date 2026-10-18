import { Bot, Cpu, Terminal } from 'lucide-react'

export function AgentsView() {
  const agents = [
    {
      id: 'agent-general-runner',
      name: 'General Task Runner',
      role: 'Headless Task Implementer',
      status: 'idle',
      capabilities: ['Go', 'TypeScript', 'Node.js', 'Docker', 'Shell'],
      model: 'In-Process Go Runner & Claude/Gemini CLI',
    },
    {
      id: 'agent-content-writer',
      name: 'Content & Scriptwriter',
      role: 'Creator Blueprint Assistant',
      status: 'idle',
      capabilities: ['SEO', 'Drafting', 'WordPress REST', 'Video Scripts'],
      model: 'Gemini 2.5 Flash Vault',
    },
    {
      id: 'agent-webmaster',
      name: 'WordPress AI Webmaster',
      role: 'Site Optimizer & Script Injector',
      status: 'idle',
      capabilities: ['WordPress Config', 'MySQL', 'Caddy SSL', 'Plugins'],
      model: 'Built-in Webmaster Engine',
    },
  ]

  return (
    <div className="max-w-5xl mx-auto space-y-6">
      <div>
        <h1 className="text-xl font-bold text-foreground flex items-center gap-2">
          <Bot className="h-5 w-5 text-primary" />
          <span>AI Agents Hub</span>
        </h1>
        <p className="text-xs text-muted-foreground mt-0.5">
          Dedicated AI worker agents available on your server to execute project tasks.
        </p>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        {agents.map((agent) => (
          <div
            key={agent.id}
            className="border border-border bg-card p-5 space-y-4 rounded-lg shadow-sm"
          >
            <div className="flex items-center justify-between">
              <div className="h-8 w-8 bg-primary/10 border border-primary/20 flex items-center justify-center text-primary rounded-md">
                <Cpu className="h-4 w-4" />
              </div>
              <span className="flex items-center gap-1.5 text-[10px] uppercase font-bold text-green-600">
                <div className="h-2 w-2 rounded-full bg-green-500" />
                {agent.status}
              </span>
            </div>

            <div>
              <h3 className="font-bold text-sm text-foreground">{agent.name}</h3>
              <p className="text-[11px] text-muted-foreground mt-0.5">{agent.role}</p>
            </div>

            <div className="space-y-1.5 text-xs pt-2 border-t border-border">
              <div className="text-[10px] text-muted-foreground uppercase tracking-wider font-semibold">
                Engine / Model
              </div>
              <p className="text-[11px] text-foreground/80">{agent.model}</p>
            </div>

            <div className="space-y-1.5 text-xs">
              <div className="text-[10px] text-muted-foreground uppercase tracking-wider font-semibold">
                Skills & Capabilities
              </div>
              <div className="flex flex-wrap gap-1">
                {agent.capabilities.map((cap) => (
                  <span
                    key={cap}
                    className="text-[9px] px-1.5 py-0.5 bg-muted text-muted-foreground rounded"
                  >
                    {cap}
                  </span>
                ))}
              </div>
            </div>
          </div>
        ))}
      </div>

      <div className="p-6 border border-border bg-muted/30 rounded-lg">
        <h3 className="text-xs font-bold text-foreground mb-2 flex items-center gap-2">
          <Terminal className="h-4 w-4 text-primary" />
          <span>How In-Task Agent Execution Works</span>
        </h3>
        <p className="text-xs text-muted-foreground leading-relaxed">
          In Cuan, tasks are not dead checklist items. When you click <strong>&#34;Run Agent&#34;</strong> on any task card or from the side-peek drawer, the task prompt and Definition of Done (DoD) criteria are dispatched to the designated AI worker. The agent allocates an isolated workspace sandbox, inspects files, runs targeted tests, and streams terminal execution logs directly to your dashboard.
        </p>
      </div>
    </div>
  )
}
