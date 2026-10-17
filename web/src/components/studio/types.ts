export interface DeploymentItem {
  id: string
  server_id?: string
  project_name: string
  slug: string
  source_type: 'zip' | 'git'
  source_ref: string
  runtime: 'php' | 'nodejs' | 'static' | 'custom'
  status: 'pending' | 'extracting' | 'building' | 'running' | 'stopped' | 'failed'
  internal_port: number
  domain: string
  env_vars: Record<string, any>
  process_pid: number
  created_at: string
  updated_at: string
}

export interface DeploymentLog {
  id: number
  deployment_id: string
  stream: 'stdout' | 'stderr' | 'system'
  message: string
  timestamp: string
}

export interface HealthStatus {
  alive: boolean
  pid: number
  port: number
  port_responding: boolean
  error?: string
}

export interface DeploymentResult {
  deployment: DeploymentItem
  detected_runtime: string
  extracted_path: string
  status: string
}

export interface ServerSpecs {
  os?: string
  distro?: string
  kernel?: string
  arch?: string
  cores?: number
  ram_mb?: number
  hostname?: string
  ping_ms?: number
  error?: string
  [key: string]: any
}

export interface ServerItem {
  id: string
  name: string
  host: string
  port: number
  username: string
  auth_type: string
  key_fingerprint?: string
  status: 'pending' | 'online' | 'unreachable' | 'error' | 'offline' | string
  last_ping_at?: string
  os_info?: ServerSpecs
  created_at: string
  updated_at: string
}

export interface TestServerResult {
  success: boolean
  status: string
  error?: string
  specs?: ServerSpecs
  server: ServerItem
}

