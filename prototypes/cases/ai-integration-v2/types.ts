export type AiToolId = 'workbuddy' | 'codex' | 'claude-code'

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
}

export type SkillCategory = 'all' | 'social' | 'short-video' | 'productivity'

export type SkillId = 'kuaishou-daily' | 'wechat-helper' | 'device-base'

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
  details: SkillDetails
}

export type InstalledMap = Record<SkillId, AiToolId[]>
