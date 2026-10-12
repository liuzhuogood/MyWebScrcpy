package scheduler

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"mywebscrcpy/internal/db"
	"mywebscrcpy/internal/mdscript"
)

// EntryInfo 前端或外部查询的调度项视图
type EntryInfo struct {
	ScriptID                 int64    `json:"script_id"`
	ScriptName               string   `json:"script_name"`
	CronExpr                 string   `json:"cron_expr"`
	TargetDevices            []string `json:"target_devices"`
	NeedsDeviceConfiguration bool     `json:"needs_device_configuration"`
	IsEnabled                bool     `json:"is_enabled"`
	NextRunAt                int64    `json:"next_run_at"`
	LastRunAt                int64    `json:"last_run_at"`
	LastStatus               string   `json:"last_status"`
}

// Scheduler 内嵌 Cron 调度器
type Scheduler struct {
	database   *db.DB
	executor   *mdscript.Executor
	cronEngine *cron.Cron
	mu         sync.Mutex
	entryIDs   map[int64]cron.EntryID
}

func NewScheduler(database *db.DB, executor *mdscript.Executor) *Scheduler {
	return &Scheduler{
		database:   database,
		executor:   executor,
		cronEngine: cron.New(cron.WithSeconds()), // 支持 6 段带秒表达式，也可兼容标准 5 段
		entryIDs:   make(map[int64]cron.EntryID),
	}
}

// Start 启动调度器并全量注册已启用脚本
func (s *Scheduler) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.cronEngine.Start()
	log.Println("[Scheduler] 内嵌 Cron 调度引擎已启动")
	return s.reloadAllLocked()
}

// Stop 停止调度器
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	ctx := s.cronEngine.Stop()
	<-ctx.Done()
	log.Println("[Scheduler] 内嵌 Cron 调度引擎已安全停止")
}

// ReloadAll 全量重新扫描数据库注册
func (s *Scheduler) ReloadAll() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.reloadAllLocked()
}

func (s *Scheduler) reloadAllLocked() error {
	// 清空当前全部注册项
	for _, entryID := range s.entryIDs {
		s.cronEngine.Remove(entryID)
	}
	s.entryIDs = make(map[int64]cron.EntryID)

	list, err := s.database.ListMarkdownScripts()
	if err != nil {
		return fmt.Errorf("读取脚本列表失败: %w", err)
	}

	for _, item := range list {
		if item.IsEnabled == 1 && item.ScheduleCron != "" && len(item.TargetDevices) > 0 {
			s.registerScriptLocked(item)
		}
	}
	log.Printf("[Scheduler] 全量加载完成，当前注册活跃调度项: %d 个", len(s.entryIDs))
	return nil
}

// ReloadScript 单个脚本热重载 (新增/修改/切换开关)
func (s *Scheduler) ReloadScript(scriptID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 1. 若旧项存在，先移除
	if oldEntryID, ok := s.entryIDs[scriptID]; ok {
		s.cronEngine.Remove(oldEntryID)
		delete(s.entryIDs, scriptID)
	}

	// 2. 从数据库读取最新脚本配置
	item, err := s.database.GetMarkdownScript(scriptID)
	if err != nil {
		return fmt.Errorf("获取脚本失败: %w", err)
	}
	if item == nil {
		return nil // 脚本已删除
	}

	// 3. 若启用且配置了 Cron，则注册
	if item.IsEnabled == 1 && item.ScheduleCron != "" && len(item.TargetDevices) > 0 {
		s.registerScriptLocked(*item)
	}
	return nil
}

// RemoveScript 脚本删除时移除调度项
func (s *Scheduler) RemoveScript(scriptID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entryID, ok := s.entryIDs[scriptID]; ok {
		s.cronEngine.Remove(entryID)
		delete(s.entryIDs, scriptID)
		log.Printf("[Scheduler] 脚本 %d 调度项已注销", scriptID)
	}
}

// registerScriptLocked 内部注册逻辑
func (s *Scheduler) registerScriptLocked(item db.MarkdownScriptRecord) {
	cronExpr := item.ScheduleCron
	// robfig/cron/v3 WithSeconds 模式：若是标准 5 段表达式，前面自动补 0 秒
	fields := len(cronFields(cronExpr))
	standardExpr := cronExpr
	if fields == 5 {
		standardExpr = "0 " + cronExpr
	}

	scriptID := item.ID
	entryID, err := s.cronEngine.AddFunc(standardExpr, func() {
		s.triggerWorker(scriptID)
	})
	if err != nil {
		log.Printf("[Scheduler] 注册脚本 %d (「%s」) Cron 表达式失败 [%s]: %v", item.ID, item.Name, item.ScheduleCron, err)
		return
	}

	s.entryIDs[scriptID] = entryID
	log.Printf("[Scheduler] 成功注册调度项: 脚本 %d (「%s」), Cron: %s (EntryID: %d)", item.ID, item.Name, item.ScheduleCron, entryID)
}

// triggerWorker 到点触发执行单元 (核心：版本自动编译 -> 设备排他锁判定 -> 启动任务管理器)
func (s *Scheduler) triggerWorker(scriptID int64) {
	nowMs := time.Now().UnixMilli()

	script, err := s.database.GetMarkdownScript(scriptID)
	if err != nil || script == nil {
		log.Printf("[Scheduler] 触发执行失败: 找不到脚本 %d", scriptID)
		return
	}

	if script.IsEnabled != 1 {
		log.Printf("[Scheduler] 脚本 %d (「%s」) 已停用，跳过本次触发", scriptID, script.Name)
		return
	}

	if len(script.TargetDevices) == 0 {
		log.Printf("[Scheduler] script=%d pending device configuration", scriptID)
		return
	}
	results, err := s.executor.ExecuteWithTrigger(scriptID, "schedule")
	if err != nil {
		log.Printf("[Scheduler] script=%d failed: %v", scriptID, err)
		_ = s.database.UpdateScriptRunStatus(scriptID, "failed", nowMs)
		return
	}
	log.Printf("[Scheduler] script=%d dispatched devices=%d", scriptID, len(results))
}

// GetScheduleEntries 获取所有调度计划与下次执行时间
func (s *Scheduler) GetScheduleEntries() ([]EntryInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	scripts, err := s.database.ListMarkdownScripts()
	if err != nil {
		return nil, err
	}

	var res []EntryInfo
	for _, sc := range scripts {
		info := EntryInfo{
			ScriptID:                 sc.ID,
			ScriptName:               sc.Name,
			CronExpr:                 sc.ScheduleCron,
			TargetDevices:            sc.TargetDevices,
			NeedsDeviceConfiguration: sc.NeedsDeviceConfiguration,
			IsEnabled:                sc.IsEnabled == 1,
			LastRunAt:                sc.LastRunAt,
			LastStatus:               sc.LastRunStatus,
		}

		if entryID, ok := s.entryIDs[sc.ID]; ok {
			entry := s.cronEngine.Entry(entryID)
			if !entry.Next.IsZero() {
				info.NextRunAt = entry.Next.UnixMilli()
			}
		}

		res = append(res, info)
	}
	return res, nil
}

func cronFields(s string) []string {
	var f []string
	var cur []rune
	for _, r := range s {
		if r == ' ' || r == '\t' {
			if len(cur) > 0 {
				f = append(f, string(cur))
				cur = nil
			}
		} else {
			cur = append(cur, r)
		}
	}
	if len(cur) > 0 {
		f = append(f, string(cur))
	}
	return f
}
