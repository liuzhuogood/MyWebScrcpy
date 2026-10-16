export type AiToolId = 'codex' | 'workbuddy' | 'claude-code'

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

export interface SkillItem {
  id: SkillId
  name: string
  app: string
  category: 'social' | 'short-video' | 'productivity'
  categoryLabel: string
  description: string
  iconType: 'kuaishou' | 'wechat' | 'device'
  details: {
    summary: string
    capabilities: string[]
  }
}

export type InstalledMap = Record<SkillId, AiToolId[]>
