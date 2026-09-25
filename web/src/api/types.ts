// Типи відповідей REST API DELMOS (server/*_handlers.go). Дублює форму JSON,
// а не бізнес-правила: сервер лишається єдиним джерелом істини для дозволів,
// станів і хешів (docs/architecture/gui/README.md, правило 2).

export interface LoginResponse {
  csrf_token: string
  expires_at: string
  user: UserView
}

export interface UserView {
  id: string
  login: string
  display_name: string
  permissions: string[]
}

export interface ProjectSummary {
  id: string
  code: string
  name: string
  status: string
}

export interface PlanView {
  code: string
  type: string
  title: string
  status: string
  revision_number: number
  body: string
  metadata: Record<string, unknown>
  payload_hash: string
}

export interface ProjectDetail {
  id: string
  code: string
  name: string
  description: string
  status: string
  plan: PlanView
  permissions?: string[]
}

export interface GenericPlanManifest {
  objectives: unknown[]
  scope_items: unknown[]
  assumptions: unknown[]
  constraints: unknown[]
  responsibility_assignments: unknown[]
  deliverables: unknown[]
  phases: unknown[]
  milestones: unknown[]
  acceptance_rules: unknown[]
  governance: { change_control_required: boolean }
  extensions: Record<string, unknown>
}

export interface PlanApplied {
  effective_plan_revision_id: string
  config_generation: number
  row_version: number
}

export interface ProjectPhase {
  phase_key: string
  name: string
  planned_start: string
  planned_finish: string
  status: 'not_started' | 'active' | 'completed'
  depends_on: string[]
}

export interface ProjectMilestone {
  milestone_key: string
  phase_key: string
  name: string
  target_date: string
  status: 'pending' | 'passed' | 'failed' | 'waived'
  deliverable_keys: string[]
  acceptance_rule_keys: string[]
}

export interface PlanDetailView {
  work_product_id: string
  code: string
  title: string
  status: string
  row_version: number
  revision_id: string
  revision_number: number
  payload_hash: string
  body: string
  template_key: string
  template_version: number
  manifest: GenericPlanManifest
  permissions?: string[]
  effective_plan_revision_id?: string
  config_generation?: number
  phases?: ProjectPhase[]
  milestones?: ProjectMilestone[]
}

export interface Stakeholder {
  id: string
  project_id: string
  kind: 'user' | 'organization' | 'external_party'
  name: string
  contact_ref: string
  interest: string
  created_at: string
}

export interface ProjectRisk {
  id: string
  project_id: string
  title: string
  description: string
  status: 'identified' | 'analyzed' | 'mitigated' | 'closed'
  impact: 'low' | 'medium' | 'high' | 'critical'
  likelihood: 'low' | 'medium' | 'high'
  response_strategy: string
  owner_ref: string
  created_at: string
}

export interface WorkProductSummary {
  id: string
  code: string
  type: string
  title: string
  status: string
}

export interface WorkProductRevisionView {
  revision_id: string
  revision_number: number
  body: string
  metadata: Record<string, unknown>
  payload_hash: string
  content_hash: string
}

export interface WorkProductView {
  id: string
  code: string
  type: string
  title: string
  status: string
  row_version: number
  latest_revision: WorkProductRevisionView
  permissions?: string[]
}

export const CORE_WORK_PRODUCT_TYPES = [
  'requirement',
  'architecture',
  'test_spec',
  'report',
  'record',
] as const

export interface RepositoryBindingView {
  remote_url: string
  default_branch: string
  status: string
  last_error?: string
}

export type ComponentHealthStatus = 'green' | 'yellow' | 'red'

export interface ComponentHealth {
  id: string
  name: string
  status: ComponentHealthStatus
  message: string
}

export interface BootStatusResponse {
  status: ComponentHealthStatus
  version: string
  components: ComponentHealth[]
}
