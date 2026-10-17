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

export interface ProbeRecord {
  timestamp: string
  status_code: number
  response_time_ms: number
  is_success: boolean
  error?: string
}

export interface DeploymentHealthResponse {
  alive: boolean
  pid: number
  port: number
  port_responding: boolean
  error?: string
  deployment_id: string
  url: string
  status: 'healthy' | 'degraded' | 'unhealthy' | string
  last_checked_at?: string
  last_status_code: number
  response_time_ms: number
  consecutive_failures: number
  uptime_percent: number
  avg_latency_ms: number
  total_probes: number
  history?: ProbeRecord[]
}

export interface ServerMetricDTO {
  id: string
  server_id?: string
  deployment_id?: string
  cpu_percent: number
  memory_used_mb: number
  memory_total_mb: number
  memory_percent: number
  disk_used_gb: number
  disk_total_gb: number
  disk_percent: number
  recorded_at: string
}

export interface AlertChannelDTO {
  id: string
  project_id: string
  type: 'telegram' | 'discord' | 'webhook' | string
  name: string
  is_active: boolean
  masked_config?: Record<string, string>
  created_at: string
  updated_at: string
}

export interface AlertIncidentDTO {
  id: string
  project_id: string
  target_type: 'deployment' | 'server' | string
  target_id: string
  title: string
  message: string
  status: 'firing' | 'resolved' | string
  triggered_at: string
  resolved_at?: string
}

// Project Studio Discussions & Tasks Types (Sprint 7 S07.4)

export interface ProjectRoomDTO {
  id: string
  project_id: string
  title: string
  topic: string
  created_at: string
  updated_at: string
}

export interface ProjectMessageDTO {
  id: string
  room_id: string
  author_type: 'user' | 'agent' | 'system' | string
  author_id: string
  author_name: string
  content: string
  reply_to_id?: string | null
  created_at: string
  updated_at: string
}

export interface DoDItem {
  id?: string
  text: string
  completed: boolean
}

export interface ProjectTaskDTO {
  id: string
  project_id: string
  room_id?: string | null
  title: string
  description: string
  status: 'backlog' | 'doing' | 'review' | 'done' | string
  priority: 'low' | 'normal' | 'high' | 'urgent' | string
  dod_checklist: DoDItem[]
  assignee_id?: string | null
  parent_task_id?: string | null
  batch_id?: string | null
  tags: string[]
  agent_output?: string
  execution_status?: 'idle' | 'running' | 'completed' | 'failed' | string
  created_at: string
  updated_at: string
}

export interface DistillTaskPayload {
  message_ids?: string[]
  title?: string
  batch_id?: string | null
  tags?: string[]
}

export interface DistillResult {
  task: ProjectTaskDTO
  distilled_from_messages: number
  explicit_dod_count: number
  inferred_dod_count: number
  execution_dispatch?: {
    dispatched: boolean
    task_id: string
    project_id: string
    runner_id?: string
    attempt_id?: string
    agent_name?: string
    triggered_at: string
    message: string
  }
}

export interface CreateProjectTaskPayload {
  id?: string
  room_id?: string | null
  title: string
  description: string
  status?: string
  priority?: string
  dod_checklist?: DoDItem[]
  assignee_id?: string | null
  parent_task_id?: string | null
  batch_id?: string | null
  tags?: string[]
}

export interface UpdateProjectTaskPayload {
  room_id?: string | null
  title?: string
  description?: string
  status?: string
  priority?: string
  dod_checklist?: DoDItem[]
  assignee_id?: string | null
  parent_task_id?: string | null
  batch_id?: string | null
  tags?: string[]
}

// Batch Campaign Studio Types (Sprint 8 S08.4)

export interface BatchCampaignDTO {
  id: string
  title: string
  description: string
  template_payload: {
    title_template?: string
    instructions?: string
    script?: string
    tags?: string[]
    dod_checklist?: DoDItem[]
    [key: string]: any
  }
  target_criteria: {
    project_ids?: string[]
    tags?: string[]
    all?: boolean
    exclude_ids?: string[]
    [key: string]: any
  }
  status: 'draft' | 'pending' | 'running' | 'completed' | 'failed' | 'paused' | 'canceled' | string
  created_at: string
  updated_at: string
}

export interface RollupMetrics {
  campaign_id: string
  batch_id?: string
  total_target: number
  total_tasks: number
  pending_tasks: number
  running_tasks: number
  completed_tasks: number
  review_tasks: number
  failed_tasks: number
  success_percentage: number
  status: 'draft' | 'pending' | 'running' | 'completed' | 'failed' | 'review' | string
  updated_at: string
}

export interface CampaignEvent {
  type: string
  campaign_id: string
  timestamp: string
  data: any
}

export interface CampaignTaskResult {
  task_id: string
  project_id: string
  success: boolean
  status: string
  error?: string
  duration_ms: number
  log_output?: string
}

export interface CampaignRunResult {
  campaign_id: string
  total_tasks: number
  completed_tasks: number
  failed_tasks: number
  peak_concurrency: number
  duration_ms: number
  success: boolean
  results?: CampaignTaskResult[]
}

export interface CreateBatchCampaignPayload {
  title: string
  description: string
  template_payload?: any
  target_criteria?: any
  status?: string
}

// ============================================================================
// Sprint 9 (S09.4): Agents Hub & Terminal Types
// ============================================================================

export interface AgentDTO {
  id: string
  name: string
  avatar: string
  role: string
  system_prompt: string
  model_id: string
  provider: string
  status: 'active' | 'inactive' | 'archived' | string
  created_at: string
  updated_at: string
}

export interface CreateAgentPayload {
  id?: string
  name: string
  avatar?: string
  role?: string
  system_prompt?: string
  model_id?: string
  provider?: string
  status?: string
}

export interface UpdateAgentPayload {
  name?: string
  avatar?: string
  role?: string
  system_prompt?: string
  model_id?: string
  provider?: string
  status?: string
}

export interface AgentConnectionDTO {
  id: string
  user_id: string
  provider: string
  plan_hint: string
  status: 'active' | 'invalid' | 'disconnected' | string
  has_credentials: boolean
  created_at: string
  updated_at: string
}

export interface CreateAgentConnectionPayload {
  id?: string
  user_id?: string
  provider?: string
  api_key?: string
  credentials?: string
  plan_hint?: string
  status?: string
  verify_now?: boolean
}

export interface ModelInfo {
  id: string
  name: string
  display_name: string
  description?: string
  input_token_limit?: number
  output_token_limit?: number
  supported_generation_methods?: string[]
  is_default?: boolean
}

export interface ConnectionProbeResult {
  valid: boolean
  provider: string
  plan_hint?: string
  latency_ms: number
  available_models?: ModelInfo[]
  error_message?: string
  tested_at: string
}

export interface TerminalSessionSummary {
  id: string
  task_id: string
  subscribers: number
  created_at: string
  last_activity: string
  cols: number
  rows: number
  pid: number
  work_dir: string
}

// ============================================================================
// Sprint 10 (S10.4): Creator Blueprint & In-Task Agent Execution Types
// ============================================================================

export interface CreatorRoomTemplate {
  title: string
  topic: string
}

export interface CreatorTaskTemplate {
  title: string
  description: string
  priority: string
  dod_checklist: DoDItem[]
  tags: string[]
  execution_status: string
}

export interface CreatorAgentTemplate {
  name: string
  role: string
  avatar: string
  system_prompt: string
  model_id: string
  provider: string
}

export interface CreatorTemplate {
  id: string
  name: string
  description: string
  category: string
  icon: string
  room: CreatorRoomTemplate
  tasks: CreatorTaskTemplate[]
  agent: CreatorAgentTemplate
}

export interface InstantiateCreatorProjectPayload {
  template_id: string
  project_name: string
  repo_url?: string
  default_branch?: string
  token?: string
  metadata?: Record<string, any>
}

export interface InstantiateCreatorProjectResponse {
  status: string
  template_id: string
  project: {
    id: string
    name: string
    slug?: string
    repo_url?: string
    project_type?: string
    metadata?: Record<string, any>
    created_at?: string
    [key: string]: any
  }
  room: ProjectRoomDTO
  agent: AgentDTO
  tasks: ProjectTaskDTO[]
}

export interface ExecuteTaskAgentPayload {
  instructions?: string
  user_id?: string
  model_id?: string
  provider?: string
}

export interface ExecuteTaskAgentResponse {
  status: string
  execution_status: string
  task: ProjectTaskDTO
  agent?: AgentDTO
  agent_output: string
  dod_completed_count: number
  dod_total_count: number
  room_message_id?: string
}

