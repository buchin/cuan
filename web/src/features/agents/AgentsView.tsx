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
        <h1 className="text-xl font-bold text-[#1a1b26] dark:text-[#c0caf5] flex items-center gap-2">
          <Bot className="h-5 w-5 text-[#7aa2f7]" />
          <span>AI Agents Hub</span>
        </h1>
        <p className="text-xs text-[#565f89] mt-0.5">
          Dedicated AI worker agents available on your server to execute project tasks.
        </p>
      </div>

      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        {agents.map((agent) => (
          <div
            key={agent.id}
            className="border border-[#cfd6e6] dark:border-[#24283b] bg-[#e2e7f1] dark:bg-[#16161e] p-5 space-y-4"
          >
            <div className="flex items-center justify-between">
              <div className="h-8 w-8 bg-[#7aa2f7]/10 border border-[#7aa2f7]/20 flex items-center justify-center text-[#7aa2f7]">
                <Cpu className="h-4 w-4" />
              </div>
              <span className="flex items-center gap-1.5 text-[10px] uppercase font-bold text-[#9ece6a]">
                <div className="h-2 w-2 rounded-full bg-[#9ece6a]" />
                {agent.status}
              </span>
            </div>

            <div>
              <h3 className="font-bold text-sm text-[#1a1b26] dark:text-[#c0caf5]">
                {agent.name}
              </h3>
              <p className="text-[11px] text-[#565f89] mt-0.5">{agent.role}</p>
            </div>

            <div className="space-y-1.5 text-xs pt-2 border-t border-[#cfd6e6] dark:border-[#24283b]">
              <div className="text-[10px] text-[#565f89] uppercase tracking-wider font-semibold">
                Engine / Model
              </div>
              <p className="text-[11px] text-[#343b58] dark:text-[#a9b1d6]">{agent.model}</p>
            </div>

            <div className="space-y-1.5 text-xs">
              <div className="text-[10px] text-[#565f89] uppercase tracking-wider font-semibold">
                Skills & Capabilities
              </div>
              <div className="flex flex-wrap gap-1">
                {agent.capabilities.map((cap) => (
                  <span
                    key={cap}
                    className="text-[9px] px-1.5 py-0.5 bg-[#cfd6e6] dark:bg-[#24283b] text-[#565f89]"
                  >
                    {cap}
                  </span>
                ))}
              </div>
            </div>
          </div>
        ))}
      </div>

      <div className="p-6 border border-[#cfd6e6] dark:border-[#24283b] bg-[#f0f3f8] dark:bg-[#1f2335]/30">
        <h3 className="text-xs font-bold text-[#1a1b26] dark:text-[#c0caf5] mb-2 flex items-center gap-2">
          <Terminal className="h-4 w-4 text-[#7aa2f7]" />
          <span>How In-Task Agent Execution Works</span>
        </h3>
        <p className="text-xs text-[#565f89] leading-relaxed">
          In Cuan, tasks are not dead checklist items. When you click <strong>"Run Agent"</strong> on any task card or from the side-peek drawer, the task prompt and Definition of Done (DoD) criteria are dispatched to the designated AI worker. The agent allocates an isolated workspace sandbox, inspects files, runs targeted tests, and streams terminal execution logs directly to your dashboard.
        </p>
      </div>
    </div>
  )
}
