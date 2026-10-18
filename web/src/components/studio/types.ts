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
  project_ids?: string[]
  project_names?: string[]
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
  scheduled_at?: string | null
  published_at?: string | null
  publication_url?: string
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

// ============================================================================
// Sprint 11 (S11.4): Modular Workspace, ZIP Inspector & Publishing Types
// ============================================================================

export interface ProjectItemDTO {
  id: string
  name: string
  repo_url?: string
  repo_owner?: string
  repo_name?: string
  default_branch?: string
  clone_status?: string
  project_type?: string
  metadata?: Record<string, any>
  enabled_modules?: string[]
  source_type?: 'blank' | 'template' | 'zip' | string
  source_metadata?: Record<string, any>
  is_archived?: boolean
  status?: string
  created_at?: string
  updated_at?: string
}

export interface TechStackInfoDTO {
  primary_stack: string
  framework: string
  language: string
  confidence: number
  description: string
  signatures: string[]
}

export interface SQLFileInfoDTO {
  path: string
  size: number
  dialect: string
}

export interface EnvVarInfoDTO {
  source_file?: string
  all_keys: string[]
  required_keys: string[]
  database_keys: string[]
  security_keys: string[]
}

export interface TaskTemplateDTO {
  title: string
  description: string
  priority: string
  tags: string[]
  dod_checklist: DoDItem[]
}

export interface InspectionResultDTO {
  archive_name?: string
  tech_stack: TechStackInfoDTO
  env: EnvVarInfoDTO
  sql_files: SQLFileInfoDTO[]
  total_files: number
  total_size: number
  root_prefix?: string
  recommended_modules: string[]
  recommended_tasks: TaskTemplateDTO[]
}

export interface CreateFromZipResponse {
  status: string
  project: ProjectItemDTO
  inspection?: InspectionResultDTO
  room: ProjectRoomDTO
  agent: AgentDTO
  tasks: ProjectTaskDTO[]
}

export interface WordPressIntegrationDTO {
  id: string
  project_id: string
  integration_type: string
  site_url: string
  username: string
  status: string
  created_at: string
  updated_at: string
}

export interface PublishTaskResult {
  success: boolean
  task_id: string
  post_id: number
  status: string
  publication_url: string
  published_at?: string
}

// ============================================================================
// Sprint 12 (S12.4): 1-Click Script Deployer & Database Provisioner Types
// ============================================================================

export interface ScriptDeploymentDTO {
  id: string
  project_id: string
  server_id?: string
  port: number
  domain: string
  has_env: boolean
  build_status: 'pending' | 'building' | 'ready' | 'failed' | string
  health: 'unknown' | 'healthy' | 'unhealthy' | string
  created_at: string
  updated_at: string
}

export interface ProvisionedDatabaseDTO {
  id: string
  project_id: string
  server_id?: string
  engine: 'sqlite' | 'mysql' | 'postgres' | string
  db_name: string
  db_user: string
  has_password: boolean
  status: 'provisioned' | 'ready' | 'failed' | string
  schema_imported: boolean
  created_at: string
  updated_at: string
}

export interface ProvisionDBPayload {
  engine: 'sqlite' | 'mysql' | 'postgres' | string
  db_name?: string
  db_user?: string
  db_password?: string
  server_id?: string
  sql_file?: string
  sql_content?: string
  import_schema?: boolean
}

export interface ProvisionDBResponse {
  status: string
  database: ProvisionedDatabaseDTO
  tables_count: number
  imported_tables: string[]
  total_statements: number
  execution_log: string
}

export interface GenerateEnvPayload {
  db_id?: string
  server_id?: string
  custom_vars?: Record<string, string>
  base_env?: string
}

export interface GenerateEnvResponse {
  status: string
  project_id: string
  deployment?: ScriptDeploymentDTO
  env_keys: string[]
  masked_env: Record<string, string>
}

export interface GetProjectEnvResponse {
  status: string
  project_id: string
  has_env: boolean
  env_keys: string[]
  masked_env: Record<string, string>
}

export interface ScriptDeployPayload {
  server_id?: string
  domain?: string
  port?: number
  stack?: string
  env_vars?: Record<string, string>
  run_install?: boolean
  run_build?: boolean
  run_migrate?: boolean
  post_install_cmds?: string[]
  timeout_seconds?: number
  health_path?: string
  skip_health_wait?: boolean
}

export interface SupervisedProcessStatus {
  project_id: string
  deployment_id: string
  pid?: number
  port: number
  domain: string
  status: 'stopped' | 'running' | 'crashed' | 'pending' | 'building' | 'ready' | 'failed' | string
  health: 'unknown' | 'healthy' | 'unhealthy' | string
  uptime_seconds?: number
  started_at?: string
  last_error?: string
}

export interface ScriptDeployResponse {
  deployment: ScriptDeploymentDTO
  build_result?: {
    success: boolean
    stack: string
    duration_ms: number
    command_log: string[]
  }
  process: SupervisedProcessStatus
  message: string
}

export interface ScriptDeployStatusResponse {
  deployment?: ScriptDeploymentDTO
  process?: SupervisedProcessStatus
}

export interface ScriptDeployLogEntry {
  id?: number | string
  deployment_id?: string
  stream?: 'stdout' | 'stderr' | 'system' | string
  message: string
  timestamp: string
}

// ============================================================================
// Sprint 13 (S13.4): Turnkey WordPress Cloud & AI Webmaster Engine Types
// ============================================================================

export interface WordPressDeploymentDTO {
  id: string
  project_id: string
  site_title: string
  admin_email: string
  admin_username: string
  has_admin_password: boolean
  db_name: string
  db_user: string
  has_db_password: boolean
  db_host: string
  db_port: number
  domain: string
  port: number
  table_prefix: string
  wp_version: string
  status: 'pending' | 'provisioning' | 'running' | 'stopped' | 'failed' | string
  health_status: 'healthy' | 'unhealthy' | 'unknown' | string
  permalink_structure: string
  created_at: string
  updated_at: string
}

export interface WordPressCredentialsDTO {
  project_id: string
  deployment_id: string
  site_title: string
  site_url: string
  admin_url: string
  admin_username: string
  admin_password: string
  db_name: string
  db_user: string
  db_password: string
  db_host: string
  db_port: number
  table_prefix?: string
  wp_version?: string
  permalink_structure?: string
}

export interface WordPressDeployPayload {
  site_title: string
  admin_email: string
  admin_username: string
  admin_password?: string
  domain?: string
  db_name?: string
  db_user?: string
  db_password?: string
  table_prefix?: string
  wp_version?: string
  permalink_structure?: string
  server_id?: string
  skip_health_wait?: boolean
}

export interface WordPressDeployResponse {
  deployment: WordPressDeploymentDTO
  public_dto?: WordPressDeploymentDTO
  admin_password_plain?: string
  db_password_plain?: string
  site_url: string
  admin_url: string
  port: number
  pid: number
  status: string
  health_status: string
  execution_logs: string[]
  message?: string
}

export interface WordPressAssetDTO {
  id: string
  deployment_id: string
  kind: 'theme' | 'plugin' | string
  slug: string
  name: string
  version: string
  is_active: boolean
  source_zip?: string
  installed_at: string
}

export interface WordPressScriptDTO {
  id: string
  name: string
  type: string
  position: 'head' | 'footer' | string
  content: string
  enabled: boolean
  updated_at?: string
}

export interface WordPressConfigTweaks {
  constants?: Record<string, any>
  tweaks?: Record<string, any>
  wp_debug?: boolean
  wp_debug_log?: boolean
  wp_debug_display?: boolean
  script_debug?: boolean
  wp_memory_limit?: string
  wp_max_memory_limit?: string
  [key: string]: any
}

export interface WordPressWebmasterActionDTO {
  id: string
  deployment_id: string
  action_type: 'inject_script' | 'config_tweak' | 'install_asset' | 'toggle_asset' | 'delete_asset' | string
  payload: Record<string, any>
  applied_at: string
}

export interface WordPressHealthResponse {
  project_id: string
  deployment_id: string
  health_status: string
  probe?: {
    alive: boolean
    status_code: number
    response_time_ms: number
    url: string
    is_healthy: boolean
    error?: string
  }
}


