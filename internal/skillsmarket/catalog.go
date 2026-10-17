package skillsmarket

import (
	"path/filepath"
	"strings"
)

// SupportedTools returns the list of all supported AI tools.
var SupportedTools = []AiToolID{
	ToolCodex,
	ToolClaudeCode,
	ToolWorkbuddy,
}

// GetTargetSkillsRelPath returns the relative directory under user home for a tool.
func GetTargetSkillsRelPath(toolID AiToolID) (string, bool) {
	switch toolID {
	case ToolCodex:
		return filepath.Join(".agents", "skills"), true
	case ToolClaudeCode:
		return filepath.Join(".claude", "skills"), true
	case ToolWorkbuddy:
		return filepath.Join(".workbuddy", "skills"), true
	default:
		return "", false
	}
}

// GetTargetSkillsPath returns the absolute directory under the specified homeDir for a tool.
func GetTargetSkillsPath(homeDir string, toolID AiToolID) string {
	rel, ok := GetTargetSkillsRelPath(toolID)
	if !ok {
		return ""
	}
	return filepath.Join(homeDir, rel)
}

// AllAiTools returns all supported AI assistants with paths populated for the given homeDir.
func AllAiTools(homeDir string) []AiTool {
	return []AiTool{
		{
			ID:              ToolCodex,
			Name:            "Codex",
			Subtitle:        "查看接入步骤",
			GuideTitle:      "接入 Codex",
			GuideBadge:      "接入指南",
			Step1Title:      "准备设备",
			Step1Desc:       "连接手机并确认可操作",
			Step2Title:      "安装基础技能",
			Step2Desc:       "将设备操作能力加入 Codex",
			Step2ActionText: "安装基础技能",
			Step3Title:      "开始使用",
			Step3Desc:       "在 Codex 中描述你的任务",
			Step3ActionText: "复制示例",
			SamplePrompt:    "查看当前手机页面，并告诉我可以执行哪些操作",
			IconType:        "codex",
			TargetPath:      GetTargetSkillsPath(homeDir, ToolCodex),
			TargetRelPath:   filepath.Join(".agents", "skills"),
			OfficialDocURL:  "https://developers.openai.com/codex/skills",
		},
		{
			ID:              ToolClaudeCode,
			Name:            "Claude Code",
			Subtitle:        "查看接入步骤",
			GuideTitle:      "接入 Claude Code",
			GuideBadge:      "接入指南",
			Step1Title:      "准备设备",
			Step1Desc:       "连接手机并确认可操作",
			Step2Title:      "安装基础技能",
			Step2Desc:       "将设备操作能力加入 Claude Code",
			Step2ActionText: "安装基础技能",
			Step3Title:      "开始使用",
			Step3Desc:       "在 Claude Code 中描述你的任务",
			Step3ActionText: "复制示例",
			SamplePrompt:    "检查手机连接状态，识别屏幕并进入最近应用列表",
			IconType:        "claude-code",
			TargetPath:      GetTargetSkillsPath(homeDir, ToolClaudeCode),
			TargetRelPath:   filepath.Join(".claude", "skills"),
			OfficialDocURL:  "https://code.claude.com/docs/en/skills",
		},
		{
			ID:              ToolWorkbuddy,
			Name:            "WorkBuddy",
			Subtitle:        "查看接入步骤",
			GuideTitle:      "接入 WorkBuddy",
			GuideBadge:      "接入指南",
			Step1Title:      "准备设备",
			Step1Desc:       "连接手机并确认可操作",
			Step2Title:      "安装基础技能",
			Step2Desc:       "将设备操作能力加入 WorkBuddy",
			Step2ActionText: "安装基础技能",
			Step3Title:      "开始使用",
			Step3Desc:       "在 WorkBuddy 中描述你的任务",
			Step3ActionText: "复制示例",
			SamplePrompt:    "查看当前手机屏幕，帮我找出未读通知并给出概要",
			IconType:        "workbuddy",
			TargetPath:      GetTargetSkillsPath(homeDir, ToolWorkbuddy),
			TargetRelPath:   filepath.Join(".workbuddy", "skills"),
			OfficialDocURL:  "https://www.codebuddy.cn/docs/workbuddy/From-Beginner-to-Expert-Guide/Function-Description/Skills-Market",
		},
	}
}

var rawSkills = []SkillDetail{
	{
		SkillSummary: SkillSummary{
			ID:               SkillDeviceBase,
			Name:             "手机操作基础",
			App:              "通用设备",
			Category:         "productivity",
			CategoryLabel:    "基础",
			Description:      "截图、识别页面与点击输入",
			IconType:         "device",
			ZipDownloadURL:   "/api/skills/mywebscrcpy-device-base/download",
			ZipFileName:      "mywebscrcpy-device-base.zip",
			InstalledTargets: []AiToolID{},
			TargetStatuses:   map[AiToolID]TargetStatus{},
		},
		Details: SkillDetails{
			Overview: "提供通用的 Android 设备底层屏幕捕获、控件层级树提取（UI Dump）与精准输入模拟能力，是所有应用专属自动化技能的核心底座。",
			Capabilities: []SkillCapability{
				{
					Title:       "实时屏幕捕获与分辨率映射",
					Description: "以低延迟抓取设备当前画面，并根据物理屏幕分辨率与缩放比精准对齐操作坐标。",
				},
				{
					Title:       "UI 控件层级树提取与精确定位",
					Description: "抓取 Android UIAutomator XML 控件树，自动生成鲁棒、唯一的控件选择器。",
				},
				{
					Title:       "模拟点击、长按、滑动与文字键入",
					Description: "支持精确坐标点击、手势滑动轨迹模拟及软键盘文本注入等原子操作。",
				},
			},
			Scenarios: []string{
				"为 WorkBuddy、Codex 或 Claude Code 提供跨应用的底层手机操控能力",
				"快速捕获手机当前屏幕状态，供多模态大模型进行界面视觉分析",
				"执行返回桌面、切换多任务、调节音量等通用系统级手势与按键操作",
			},
			SamplePrompts: []string{
				"截取当前手机屏幕并分析前台运行的是哪个应用",
				"识别屏幕中间的“确定”按钮坐标并执行一次点击",
				"返回手机主屏幕，向右滑动一屏并打开设置应用",
			},
			Prerequisites: []string{
				"Android 设备已开启 USB 调试或局域网无线调试模式",
				"MyWebScrcpy 服务处于运行状态且设备连接显示就绪",
				"授权允许模拟按键与无障碍/投屏相关权限",
			},
			UsageSteps: []string{
				"在外部 AI 助手接入步骤中下载并安装此基础技能包",
				"连接目标手机并验证 MyWebScrcpy 正常显示屏幕流",
				"在 AI 助手内直接发出屏幕识别与手势控制指令",
			},
		},
	},
	{
		SkillSummary: SkillSummary{
			ID:               SkillKuaishouDaily,
			Name:             "快手日常助手",
			App:              "快手",
			Category:         "short-video",
			CategoryLabel:    "短视频",
			Description:      "页面识别、内容浏览与日常任务",
			IconType:         "kuaishou",
			ZipDownloadURL:   "/api/skills/mywebscrcpy-kuaishou-daily/download",
			ZipFileName:      "mywebscrcpy-kuaishou-daily.zip",
			InstalledTargets: []AiToolID{},
			TargetStatuses:   map[AiToolID]TargetStatus{},
		},
		Details: SkillDetails{
			Overview: "专为快手短视频客户端定制的界面识别与交互辅助技能，支持在已授权的移动设备上完成推荐流浏览、特定组件识别与活动任务辅助流转。不宣称已实机回归。",
			Capabilities: []SkillCapability{
				{
					Title:       "页面状态与组件特征识别",
					Description: "分析快手当前页面结构，定位短视频滑动容器、作者信息、互动操作及浮层弹窗。",
				},
				{
					Title:       "短视频内容上下滑动与停留浏览",
					Description: "模拟自然手势切换视频，支持设置单条视频停留时长与连续浏览批次。",
				},
				{
					Title:       "日常活动打卡与界面流转辅助",
					Description: "按预设路径进入日常福利与签到中心，辅助定位并触发打卡动作。",
				},
			},
			Scenarios: []string{
				"按指定时长和批次自动浏览快手推荐流并记录页面状态",
				"定时巡检快手日常签到入口并完成例行福利领取",
				"自动化测试快手关键流转节点的响应与界面渲染情况",
			},
			SamplePrompts: []string{
				"打开手机快手，帮我浏览推荐页前 5 条视频并在每条停留 8 秒",
				"检查快手当前页面是否包含“签到”按钮，如果有则点击签到",
				"在快手页面向下滑动一段距离，识别当前正在播放的视频文案",
			},
			Prerequisites: []string{
				"手机已安装快手 APP 且处于正常登录与解锁状态",
				"MyWebScrcpy 连接正常，设备控制权限完备",
				"已在外部 AI 助手安装基础技能 (mywebscrcpy-device-base)",
			},
			UsageSteps: []string{
				"将快手日常助手技能安装到你的 AI 助手，或直接下载技能包手动配置",
				"确保手机处于快手首页或前台准备状态",
				"在 AI 助手对话框输入任务指令，助手将结合屏幕状态自动执行操作",
			},
		},
	},
	{
		SkillSummary: SkillSummary{
			ID:               SkillWechatHelper,
			Name:             "微信消息助手",
			App:              "微信",
			Category:         "social",
			CategoryLabel:    "社交",
			Description:      "查找会话、读取消息与草拟回复",
			IconType:         "wechat",
			ZipDownloadURL:   "/api/skills/mywebscrcpy-wechat-helper/download",
			ZipFileName:      "mywebscrcpy-wechat-helper.zip",
			InstalledTargets: []AiToolID{},
			TargetStatuses:   map[AiToolID]TargetStatus{},
		},
		Details: SkillDetails{
			Overview: "协助在手机微信中快速检索目标联系人或群组会话，提取最新收到的文本消息并结合上下文草拟回复。本技能仅读取和草拟回复，绝不自动向外发送任何消息，所有发出内容均须由人工在微信内最终确认并手动点击发送。",
			Capabilities: []SkillCapability{
				{
					Title:       "搜索指定联系人或群组会话",
					Description: "在微信主界面搜索栏精准匹配联系人备注、昵称或群聊名称并打开对应对话。",
				},
				{
					Title:       "读取并解析当前会话最新消息",
					Description: "提取对话窗口中最近数条未读或最新文本内容，区分发信人与消息时间。",
				},
				{
					Title:       "结合对话上下文草拟回复建议",
					Description: "在外部 AI 助手中分析来信诉求并生成得体的回复文本，供用户审阅采纳。",
				},
			},
			Scenarios: []string{
				"会议或专注工作期间，委托助手汇总特定工作群的最新讨论要点",
				"面对重要业务咨询或客户留言，快速生成多种风格的草拟回复供人工挑选",
				"排查是否遗漏了关键联系人在最近数小时内的未读留言",
			},
			SamplePrompts: []string{
				"找到微信中联系人“产品对接群”，帮我读取最后 3 条消息",
				"查看当前微信聊天页面的未读消息，并草拟一条礼貌确认的回复",
				"检索微信联系人列表，确认是否收到了来自“运维告警”的通知",
			},
			Prerequisites: []string{
				"手机已登录微信且处于前台可用状态，屏幕保持点亮",
				"遵守安全隐私规范，所有消息外发操作必须由用户在微信端人工核验并手动发送",
			},
			UsageSteps: []string{
				"将微信消息助手技能安装至指定的 AI 助手平台",
				"在 AI 助手会话中指定需要查看的联系人或群聊名称",
				"审阅 AI 助手读取的内容与生成的草拟回复，由人工确认后复制或发送",
			},
		},
	},
	{
		SkillSummary: SkillSummary{
			ID:               SkillCommon,
			Name:             "通用操作技能",
			App:              "通用设备",
			Category:         "productivity",
			CategoryLabel:    "通用",
			Description:      "观察页面、拟人化看视频、尝试返回及生命周期管理",
			IconType:         "device",
			ZipDownloadURL:   "/api/skills/my-scrcpy-common/download",
			ZipFileName:      "my-scrcpy-common.zip",
			InstalledTargets: []AiToolID{},
			TargetStatuses:   map[AiToolID]TargetStatus{},
		},
		Details: SkillDetails{
			Overview: "提供设备总状态观察、AI 视觉决策、拟人化随机看视频上滑、KEYCODE_ESCAPE/BACK 尝试返回及应用启停等共享操作能力，让应用专属技能聚焦规则与知识。",
			Capabilities: []SkillCapability{
				{
					Title:       "总状态与 AI 视觉决策",
					Description: "只读采集单轮设备画面、OCR 与多源模板匹配结果，按需调用视觉模型获取建议。",
				},
				{
					Title:       "拟人化看短视频",
					Description: "随机 5~15 秒停留与曲线轨迹向上滑动，受总时限与前台稳定性检查点约束。",
				},
				{
					Title:       "智能返回与前后差异分析",
					Description: "发送 KEYCODE_ESCAPE 或 KEYCODE_BACK，比对前后页面、Activity 与可见文本变化。",
				},
				{
					Title:       "应用生命周期管理",
					Description: "通过包名启动或安全重启前台应用，并在动作后闭环核对实际运行状态。",
				},
			},
			Scenarios: []string{
				"作为短视频刷金币、打卡签到等应用技能的通用底层支撑",
				"执行弹层关闭并确认页面是否真正退出的闭环决策",
				"通过视觉大模型辅助分析未知弹窗并获得下一步操作建议",
			},
			SamplePrompts: []string{
				"调用 my-scrcpy-common 观察当前设备状态，并提供下一步决策建议",
				"使用通用看视频脚本在快手停留观看 3 个视频",
				"尝试按 ESC 键返回并检查弹窗是否已关闭",
			},
			Prerequisites: []string{
				"设备处于已连接且解锁状态",
				"服务支持总状态与 ADB 指令接口",
			},
			UsageSteps: []string{
				"安装通用技能至当前使用的外部助手目录",
				"在应用技能或任务提示词中直接引用共享操作脚本",
			},
		},
	},
}

// AllSkills returns all raw skill details.
func AllSkills() []SkillDetail {
	res := make([]SkillDetail, len(rawSkills))
	copy(res, rawSkills)
	return res
}

// KnownSkillIDs returns a list of all recognized skill IDs.
func KnownSkillIDs() []string {
	ids := make([]string, len(rawSkills))
	for i, s := range rawSkills {
		ids[i] = string(s.ID)
	}
	return ids
}

// IsKnownSkillID checks if the given ID is a valid skill ID.
func IsKnownSkillID(id string) bool {
	for _, s := range rawSkills {
		if string(s.ID) == id {
			return true
		}
	}
	return false
}

// GetSkill returns the SkillDetail for a skill ID.
func GetSkill(id string) (*SkillDetail, bool) {
	for _, s := range rawSkills {
		if string(s.ID) == id {
			clone := s
			return &clone, true
		}
	}
	return nil, false
}

// IsSkillDetailRoutePath checks if the URL path is a single-segment skill detail route path (/ai-integration/skills/:id).
func IsSkillDetailRoutePath(path string) bool {
	const prefix = "/ai-integration/skills/"
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	id := strings.TrimPrefix(path, prefix)
	return id != "" && !strings.Contains(id, "/")
}

// IsKnownDetailPath checks if the URL path is a confirmed skill detail route shell path.
func IsKnownDetailPath(path string) bool {
	return IsSkillDetailRoutePath(path)
}
