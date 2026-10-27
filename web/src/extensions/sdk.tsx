import {
  Component,
  useSyncExternalStore,
  type ComponentProps,
  type ComponentType,
  type ReactNode,
} from 'react'
import type ReactMarkdown from 'react-markdown'

export const EXTENSION_API_VERSION = 1
export type ExtensionManifest = {
  id: string
  name: string
  version: string
  api_version: number
  capabilities: string[]
}
export type FenceProps = { code: string; language: string }
export type ViewProps = {
  projectId: string
  tasks: { id: string; title: string; status: string }[]
  openTask: (id: string) => void
}
type RemarkPlugin = NonNullable<
  ComponentProps<typeof ReactMarkdown>['remarkPlugins']
>[number]
export type FrontendExtension = {
  manifest: ExtensionManifest
  fences?: { language: string; component: ComponentType<FenceProps> }[]
  parsers?: { id: string; plugin: RemarkPlugin }[]
  projectViews?: {
    id: string
    label: string
    component: ComponentType<ViewProps>
  }[]
}
const registered = new Map<string, FrontendExtension>()
const enabled = new Set<string>()
const subscribers = new Set<() => void>()
let revision = 0
function changed() {
  revision++
  subscribers.forEach((notify) => notify())
}
export function registerExtension(extension: FrontendExtension) {
  const m = extension.manifest
  if (
    !/^[a-z][a-z0-9]*(?:[.-][a-z0-9]+)+$/.test(m.id) ||
    !m.name ||
    m.name.length > 80 ||
    !/^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(?:-[a-zA-Z0-9.-]+)?$/.test(
      m.version,
    ) ||
    m.api_version !== EXTENSION_API_VERSION ||
    registered.has(m.id) ||
    registered.size >= 32 ||
    new Set(m.capabilities).size !== m.capabilities.length ||
    m.capabilities.some((capability) => !/^(filter|validate|event|connector|action|markdown|parser|view|search):[a-z][a-z0-9_.-]{0,100}$/.test(capability))
  )
    throw new Error('Invalid or duplicate extension manifest')
  const requireCapability = (capability: string) => {
    if (!m.capabilities.includes(capability))
      throw new Error('Undeclared extension capability: ' + capability)
  }
  const languages = new Set<string>(),
    views = new Set<string>(),
    parsers = new Set<string>()
  for (const fence of extension.fences || []) {
    requireCapability('markdown:' + fence.language)
    if (
      !/^[a-z][a-z0-9-]{0,40}$/.test(fence.language) ||
      languages.has(fence.language) ||
      typeof fence.component !== 'function'
    )
      throw new Error('Invalid fence renderer')
    languages.add(fence.language)
  }
  for (const view of extension.projectViews || []) {
    requireCapability('view:project')
    if (
      !/^[a-z][a-z0-9-]{0,40}$/.test(view.id) ||
      !view.label ||
      view.label.length > 80 ||
      views.has(view.id) ||
      typeof view.component !== 'function'
    )
      throw new Error('Invalid project view')
    views.add(view.id)
  }
  for (const parser of extension.parsers || []) {
    requireCapability('parser:' + parser.id)
    if (
      !/^[a-z][a-z0-9-]{0,40}$/.test(parser.id) ||
      parsers.has(parser.id) ||
      (typeof parser.plugin !== 'function' && !Array.isArray(parser.plugin))
    )
      throw new Error('Invalid Markdown parser')
    parsers.add(parser.id)
  }
  registered.set(m.id, {
    manifest: { ...m, capabilities: [...m.capabilities] },
    fences: extension.fences?.map((f) => ({ ...f })),
    projectViews: extension.projectViews?.map((v) => ({ ...v })),
    parsers: extension.parsers?.map((p) => ({ ...p })),
  })
  changed()
}
export function enableExtension(id: string) {
  const candidate = registered.get(id)
  if (!candidate) throw new Error('Unknown extension')
  for (const existing of activeExtensions())
    if (existing.manifest.id !== id)
      for (const fence of candidate.fences || [])
        if (existing.fences?.some((item) => item.language === fence.language))
          throw new Error('Renderer language already enabled')
  enabled.add(id)
  changed()
}
export function disableExtension(id: string) {
  enabled.delete(id)
  changed()
}
export function unregisterExtension(id: string) {
  enabled.delete(id)
  registered.delete(id)
  changed()
}
export function activeExtensions() {
  return Array.from(registered.values())
    .filter((extension) => enabled.has(extension.manifest.id))
    .sort((a, b) => a.manifest.id < b.manifest.id ? -1 : a.manifest.id > b.manifest.id ? 1 : 0)
}
export function listExtensions() {
  return Array.from(registered.values()).map((extension) => ({
    manifest: {
      ...extension.manifest,
      capabilities: [...extension.manifest.capabilities],
    },
    enabled: enabled.has(extension.manifest.id),
  }))
}
export function useExtensionRevision() {
  return useSyncExternalStore(
    (notify) => {
      subscribers.add(notify)
      return () => subscribers.delete(notify)
    },
    () => revision,
    () => revision,
  )
}
export function markdownParsers() {
  return activeExtensions().flatMap(
    (extension) => extension.parsers?.map((parser) => parser.plugin) || [],
  )
}
export function fenceRenderer(language: string) {
  for (const extension of activeExtensions()) {
    const found = extension.fences?.find(
      (renderer) => renderer.language === language,
    )
    if (found) return found.component
  }
}
export function projectViews() {
  return activeExtensions().flatMap((extension) =>
    (extension.projectViews || []).map((view) => ({
      ...view,
      id: extension.manifest.id + ':' + view.id,
    })),
  )
}

// Trusted installed components may fail, while message text remains untrusted.
// Unknown/failed fenced renderers always retain readable original code.
export class ExtensionBoundary extends Component<
  { children: ReactNode; fallback: ReactNode; resetKey: string },
  { failed: boolean }
> {
  state = { failed: false }
  static getDerivedStateFromError() {
    return { failed: true }
  }
  componentDidUpdate(previous: Readonly<{ resetKey: string }>) {
    if (previous.resetKey !== this.props.resetKey && this.state.failed)
      this.setState({ failed: false })
  }
  render() {
    return this.state.failed ? this.props.fallback : this.props.children
  }
}
