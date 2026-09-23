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
}

export interface WorkProductSummary {
  id: string
  code: string
  type: string
  title: string
  status: string
}

export interface WorkProductRevisionView {
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
