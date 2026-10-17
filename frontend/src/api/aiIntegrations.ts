export type AiToolId = 'codex' | 'claude-code' | 'workbuddy'

export interface AiTool {
  id: AiToolId
  name: string
  subtitle: string
  guideTitle: string
  guideBadge: string
  step1Title: string
  step1Desc: string
  step2Title: string
  step2Desc: string
  step2ActionText: string
  step3Title: string
  step3Desc: string
  step3ActionText: string
  samplePrompt: string
  iconType: 'codex' | 'workbuddy' | 'claude-code'
  targetPath: string
  targetRelPath: string
  officialDocUrl: string
}

export type SkillCategory = 'all' | 'social' | 'short-video' | 'productivity'

export type SkillId = 'mywebscrcpy-device-base' | 'mywebscrcpy-kuaishou-daily' | 'mywebscrcpy-wechat-helper'

export interface SkillCapability {
  title: string
  description: string
}

export interface SkillDetails {
  overview: string
  capabilities: SkillCapability[]
  scenarios: string[]
  samplePrompts: string[]
  prerequisites: string[]
  usageSteps: string[]
}

export interface TargetStatus {
  installed: boolean
  status: 'installed' | 'not_installed' | 'conflict' | 'error'
  path: string
  error?: string
}

export interface SkillItem {
  id: SkillId
  name: string
  app: string
  category: 'social' | 'short-video' | 'productivity'
  categoryLabel: string
  description: string
  iconType: 'kuaishou' | 'wechat' | 'device'
  zipDownloadUrl: string
  zipFileName: string
  installedTargets: AiToolId[]
  targetStatuses: Record<AiToolId, TargetStatus>
  details?: SkillDetails
}

export interface SkillInstallTargetResult {
  target: AiToolId
  status: 'installed' | 'already_installed' | 'failed'
  path: string
  error?: string
}

export interface SkillInstallResponse {
  skillId: SkillId
  results: SkillInstallTargetResult[]
}

export class ApiError extends Error {
  readonly status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

async function request<T>(input: string, init: RequestInit = {}): Promise<T> {
  const res = await fetch(input, init)
  let data: unknown = null
  try {
    data = await res.json()
  }
  catch {
    // Non-JSON response
  }
  if (!res.ok) {
    let errorMsg = `HTTP ${res.status}`
    if (data && typeof data === 'object' && 'error' in data) {
      const errVal = (data as { error: unknown }).error
      if (typeof errVal === 'string' && errVal.trim()) {
        errorMsg = errVal.trim()
      }
    }
    throw new ApiError(res.status, errorMsg)
  }
  if (data === null || data === undefined) {
    throw new ApiError(res.status, '解析响应内容失败')
  }
  return data as T
}

export async function fetchAiIntegrations(): Promise<AiTool[]> {
  const res = await request<{ tools: AiTool[] }>('/api/ai-integrations')
  return res.tools
}

export async function fetchSkills(): Promise<SkillItem[]> {
  const res = await request<{ skills: SkillItem[] }>('/api/skills')
  return res.skills
}

export function fetchSkillDetail(id: string): Promise<SkillItem> {
  return request<SkillItem>(`/api/skills/${encodeURIComponent(id)}`)
}

export function installSkill(id: string, targets: AiToolId[]): Promise<SkillInstallResponse> {
  return request<SkillInstallResponse>(`/api/skills/${encodeURIComponent(id)}/install`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ targets }),
  })
}

export function getSkillDownloadUrl(id: string): string {
  return `/api/skills/${encodeURIComponent(id)}/download`
}

export async function downloadSkillZip(id: string, fileName?: string): Promise<void> {
  const res = await fetch(`/api/skills/${encodeURIComponent(id)}/download`)
  if (!res.ok) {
    let errorMsg = `下载失败 (HTTP ${res.status})`
    try {
      const errData = await res.json()
      if (errData && typeof errData === 'object' && 'error' in errData && typeof errData.error === 'string') {
        errorMsg = errData.error
      }
    }
    catch {
      // Non-JSON
    }
    throw new ApiError(res.status, errorMsg)
  }
  const ct = res.headers.get('Content-Type') || ''
  if (!ct.includes('application/zip') && !ct.includes('application/octet-stream')) {
    throw new Error('下载文件类型不正确')
  }
  const blob = await res.blob()
  const objectUrl = window.URL.createObjectURL(blob)
  try {
    const a = document.createElement('a')
    a.href = objectUrl
    a.download = fileName || `${id}.zip`
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
  }
  finally {
    window.URL.revokeObjectURL(objectUrl)
  }
}
