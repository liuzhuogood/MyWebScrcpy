import type { AiTool, AiToolId, InstalledMap, SkillItem } from './types'

export const aiToolMap: Record<AiToolId, AiTool> = {
  workbuddy: {
    id: 'workbuddy',
    name: 'WorkBuddy',
    subtitle: '查看接入步骤',
    guideTitle: '接入 WorkBuddy',
    guideBadge: '接入指南',
    step1Title: '准备设备',
    step1Desc: '连接手机并确认可操作',
    step2Title: '安装基础技能',
    step2Desc: '将设备操作能力加入 WorkBuddy',
    step2ActionText: '下载技能包',
    step3Title: '开始使用',
    step3Desc: '在 WorkBuddy 中描述你的任务',
    step3ActionText: '复制示例',
    samplePrompt: '查看当前手机屏幕，帮我找出未读通知并给出概要',
    iconType: 'workbuddy',
  },
  codex: {
    id: 'codex',
    name: 'Codex',
    subtitle: '查看接入步骤',
    guideTitle: '接入 Codex',
    guideBadge: '接入指南',
    step1Title: '准备设备',
    step1Desc: '连接手机并确认可操作',
    step2Title: '安装基础技能',
    step2Desc: '将设备操作能力加入 Codex',
    step2ActionText: '下载技能包',
    step3Title: '开始使用',
    step3Desc: '在 Codex 中描述你的任务',
    step3ActionText: '复制示例',
    samplePrompt: '查看当前手机页面，并告诉我可以执行哪些操作',
    iconType: 'codex',
  },
  'claude-code': {
    id: 'claude-code',
    name: 'Claude Code',
    subtitle: '查看接入步骤',
    guideTitle: '接入 Claude Code',
    guideBadge: '接入指南',
    step1Title: '准备设备',
    step1Desc: '连接手机并确认可操作',
    step2Title: '安装基础技能',
    step2Desc: '将设备操作能力加入 Claude Code',
    step2ActionText: '下载技能包',
    step3Title: '开始使用',
    step3Desc: '在 Claude Code 中描述你的任务',
    step3ActionText: '复制示例',
    samplePrompt: '检查手机连接状态，识别屏幕并进入最近应用列表',
    iconType: 'claude-code',
  },
}

export const aiTools: AiTool[] = [
  aiToolMap.workbuddy,
  aiToolMap.codex,
  aiToolMap['claude-code'],
]

export const skills: SkillItem[] = [
  {
    id: 'kuaishou-daily',
    name: '快手日常助手',
    app: '快手',
    category: 'short-video',
    categoryLabel: '短视频',
    description: '页面识别、内容浏览与日常任务',
    iconType: 'kuaishou',
    details: {
      summary: '专为快手 APP 优化的自动化技能，支持画面检测与常规浏览操作。',
      capabilities: [
        '页面状态与组件特征识别',
        '短视频内容上下滑动与停留浏览',
        '日常活动打卡与界面流转辅助',
      ],
    },
  },
  {
    id: 'wechat-helper',
    name: '微信消息助手',
    app: '微信',
    category: 'social',
    categoryLabel: '社交',
    description: '查找会话、读取消息与草拟回复',
    iconType: 'wechat',
    details: {
      summary: '帮助在手机微信中高效查找特定聊天会话、整理最新消息并协助草拟回复文本。',
      capabilities: [
        '搜索指定联系人或群组会话',
        '读取并解析当前会话最新未读消息',
        '结合对话上下文草拟回复文本供人工确认',
      ],
    },
  },
  {
    id: 'device-base',
    name: '手机操作基础',
    app: '通用设备',
    category: 'productivity',
    categoryLabel: '基础',
    description: '截图、识别页面与点击输入',
    iconType: 'device',
    details: {
      summary: '提供通用的 Android 设备底层控制能力，作为各应用专项技能的基础依赖。',
      capabilities: [
        '实时屏幕捕获与分辨率自适应',
        'UI 控件层级树提取与精确定位',
        '模拟点击、长按、滑动与文字键入',
      ],
    },
  },
]

export const initialInstalledMap: InstalledMap = {
  'device-base': ['codex'],
  'kuaishou-daily': [],
  'wechat-helper': [],
}
