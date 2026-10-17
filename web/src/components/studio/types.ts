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

export interface DNSInstructionDetail {
  type: string
  name: string
  target?: string
  value?: string
  ttl?: number
  description?: string
}

export interface CustomDomainInstructions {
  cname: DNSInstructionDetail
  txt: DNSInstructionDetail
  a: DNSInstructionDetail
}

export interface CustomDomainItem {
  id: string
  deployment_id: string
  domain: string
  dns_status: 'pending' | 'verified' | 'failed' | string
  ssl_status: 'pending' | 'active' | 'error' | 'failed' | string
  verification_token?: string
  last_checked_at?: string
  created_at: string
  updated_at: string
}

export interface DomainVerifyResult {
  status: string
  verified: boolean
  dns_status: string
  domain: CustomDomainItem
  result?: {
    domain: string
    verified: boolean
    method?: string
    message: string
    cname_target?: string
    txt_record?: string
    resolved_ips?: string[]
  }
  route_registered?: boolean
  message: string
  error?: string
}

export interface CreateDomainResult {
  status: string
  message: string
  domain: CustomDomainItem
  instructions: CustomDomainInstructions
}

