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

export interface ProjectObjective {
  key: string
  statement: string
  success_criteria: string[]
}

export interface ScopeItem {
  key: string
  kind: 'in_scope' | 'out_of_scope'
  statement: string
  rationale: string
}

export interface PlanAssumption {
  key: string
  statement: string
  owner_reference: string
  validation_date: string
  status: string
}

export interface PlanConstraint {
  key: string
  kind: string
  statement: string
  source_ref: string
  enforcement: string
}

export interface ResponsibilityAssignment {
  key: string
  role_key: string
  subject: string
  responsibility: string
}

export interface PlanDeliverable {
  key: string
  name: string
  work_product_id: string
  required_status: string
}

export interface PlanPhaseDefinition {
  key: string
  name: string
  planned_start: string
  planned_finish: string
  depends_on: string[]
}

export interface PlanMilestoneDefinition {
  key: string
  name: string
  phase_key: string
  target_date: string
  deliverable_keys: string[]
  acceptance_rule_keys: string[]
}

export interface PlanAcceptanceRule {
  key: string
  predicate_key: string
  parameters: Record<string, unknown>
  enforcement: string
}

export interface GenericPlanManifest {
  objectives: ProjectObjective[]
  scope_items: ScopeItem[]
  assumptions: PlanAssumption[]
  constraints: PlanConstraint[]
  responsibility_assignments: ResponsibilityAssignment[]
  deliverables: PlanDeliverable[]
  phases: PlanPhaseDefinition[]
  milestones: PlanMilestoneDefinition[]
  acceptance_rules: PlanAcceptanceRule[]
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

export interface PlanReviewCandidate {
  id: string
  login: string
  display_name: string
  can_review: boolean
  can_approve: boolean
}

export interface PlanReviewStatus {
  revision_id: string
  participants: {
    display_name: string
    role: 'reviewer' | 'approver'
    completed: boolean
    assigned_to_me: boolean
  }[]
  has_positive_review: boolean
}

export interface PlanApprovalEvidence {
  revision_id: string
  decision_kind: 'review' | 'approval'
  display_name: string
  decided_at: string
}

export interface ReviewRoleProject {
  id: string
  code: string
  name: string
}

export interface ReviewRoleUser {
  id: string
  login: string
  display_name: string
}

export interface ProjectReviewRoleBinding {
  id: string
  user_id: string
  login: string
  display_name: string
  role_key: 'project.reviewer' | 'project.approver'
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
