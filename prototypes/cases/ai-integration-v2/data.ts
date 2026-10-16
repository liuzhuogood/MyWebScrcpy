import type { AiTool, AiToolId, InstalledMap, SkillItem } from './types'
import deviceBaseZipUrl from './assets/device-base-skill-example.zip?url'
import kuaishouZipUrl from './assets/kuaishou-daily-skill-example.zip?url'
import wechatZipUrl from './assets/wechat-helper-skill-example.zip?url'

export { deviceBaseZipUrl }
export const deviceBaseZipFileName = 'device-base-skill-example.zip'

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
    zipDownloadUrl: kuaishouZipUrl,
    zipFileName: 'kuaishou-daily-skill-example.zip',
    details: {
      overview: '专为快手短视频客户端定制的界面识别与交互辅助技能，支持在已授权的移动设备上完成推荐流浏览、特定组件识别与活动任务辅助流转。',
      capabilities: [
        {
          title: '页面状态与组件特征识别',
          description: '分析快手当前页面结构，定位短视频滑动容器、作者信息、互动操作及浮层弹窗。',
        },
        {
          title: '短视频内容上下滑动与停留浏览',
          description: '模拟自然手势切换视频，支持设置单条视频停留时长与连续浏览批次。',
        },
        {
          title: '日常活动打卡与界面流转辅助',
          description: '按预设路径进入日常福利与签到中心，辅助定位并触发打卡动作。',
        },
      ],
      scenarios: [
        '按指定时长和批次自动浏览快手推荐流并记录页面状态',
        '定时巡检快手日常签到入口并完成例行福利领取',
        '自动化测试快手关键流转节点的响应与界面渲染情况',
      ],
      samplePrompts: [
        '打开手机快手，帮我浏览推荐页前 5 条视频并在每条停留 8 秒',
        '检查快手当前页面是否包含“签到”按钮，如果有则点击签到',
        '在快手页面向下滑动一段距离，识别当前正在播放的视频文案',
      ],
      prerequisites: [
        '手机已安装快手 APP 且处于正常登录与解锁状态',
        'MyWebScrcpy 连接正常，设备控制权限完备',
        '已在外部 AI 助手安装基础技能 (device-base)',
      ],
      usageSteps: [
        '将快手日常助手技能安装到你的 AI 助手，或直接下载技能包手动配置',
        '确保手机处于快手首页或前台准备状态',
        '在 AI 助手对话框输入任务指令，助手将结合屏幕状态自动执行操作',
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
    zipDownloadUrl: wechatZipUrl,
    zipFileName: 'wechat-helper-skill-example.zip',
    details: {
      overview: '协助在手机微信中快速检索目标联系人或群组会话，提取最新收到的文本消息并结合上下文草拟回复。本技能仅读取和草拟回复，绝不自动向外发送任何消息，所有发出内容均须由人工在微信内最终确认。',
      capabilities: [
        {
          title: '搜索指定联系人或群组会话',
          description: '在微信主界面搜索栏精准匹配联系人备注、昵称或群聊名称并打开对应对话。',
        },
        {
          title: '读取并解析当前会话最新消息',
          description: '提取对话窗口中最近数条未读或最新文本内容，区分发信人与消息时间。',
        },
        {
          title: '结合对话上下文草拟回复建议',
          description: '在外部 AI 助手中分析来信诉求并生成得体的回复文本，供用户审阅采纳。',
        },
      ],
      scenarios: [
        '会议或专注工作期间，委托助手汇总特定工作群的最新讨论要点',
        '面对重要业务咨询或客户留言，快速生成多种风格的草拟回复供人工挑选',
        '排查是否遗漏了关键联系人在最近数小时内的未读留言',
      ],
      samplePrompts: [
        '找到微信中联系人“产品对接群”，帮我读取最后 3 条消息',
        '查看当前微信聊天页面的未读消息，并草拟一条礼貌确认的回复',
        '检索微信联系人列表，确认是否收到了来自“运维告警”的通知',
      ],
      prerequisites: [
        '手机已登录微信且处于前台可用状态，屏幕保持点亮',
        '微信界面语言为中文，系统字体缩放为标准大小',
        '遵守安全隐私规范，所有消息外发操作必须由用户在微信端人工核验并手动发送',
      ],
      usageSteps: [
        '将微信消息助手技能安装至指定的 AI 助手平台',
        '在 AI 助手会话中指定需要查看的联系人或群聊名称',
        '审阅 AI 助手读取的内容与生成的草拟回复，由人工确认后复制或发送',
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
    zipDownloadUrl: deviceBaseZipUrl,
    zipFileName: 'device-base-skill-example.zip',
    details: {
      overview: '提供通用的 Android 设备底层屏幕捕获、控件层级树提取（UI Dump）与精准输入模拟能力，是所有应用专属自动化技能的核心底座。',
      capabilities: [
        {
          title: '实时屏幕捕获与分辨率映射',
          description: '以低延迟抓取设备当前画面，并根据物理屏幕分辨率与缩放比精准对齐操作坐标。',
        },
        {
          title: 'UI 控件层级树提取与精确定位',
          description: '抓取 Android UIAutomator XML 控件树，自动生成鲁棒、唯一的控件选择器。',
        },
        {
          title: '模拟点击、长按、滑动与文字键入',
          description: '支持精确坐标点击、手势滑动轨迹模拟及软键盘文本注入等原子操作。',
        },
      ],
      scenarios: [
        '为 WorkBuddy、Codex 或 Claude Code 提供跨应用的底层手机操控能力',
        '快速捕获手机当前屏幕状态，供多模态大模型进行界面视觉分析',
        '执行返回桌面、切换多任务、调节音量等通用系统级手势与按键操作',
      ],
      samplePrompts: [
        '截取当前手机屏幕并分析前台运行的是哪个应用',
        '识别屏幕中间的“确定”按钮坐标并执行一次点击',
        '返回手机主屏幕，向右滑动一屏并打开设置应用',
      ],
      prerequisites: [
        'Android 设备已开启 USB 调试或局域网无线调试模式',
        'MyWebScrcpy 服务处于运行状态且设备连接显示就绪',
        '授权允许模拟按键与无障碍/投屏相关权限',
      ],
      usageSteps: [
        '在外部 AI 助手接入步骤中下载并安装此基础技能包',
        '连接目标手机并验证 MyWebScrcpy 正常显示屏幕流',
        '在 AI 助手内直接发出屏幕识别与手势控制指令',
      ],
    },
  },
]

export const initialInstalledMap: InstalledMap = {
  'device-base': ['codex'],
  'kuaishou-daily': [],
  'wechat-helper': [],
}
