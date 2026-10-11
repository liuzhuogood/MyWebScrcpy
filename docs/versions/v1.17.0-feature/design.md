# v1.17.0-feature 技术设计：内嵌 Cron 调度器与排期执行系统

更新日期：2026-10-11。

---

## 1. 系统架构图

```mermaid
flowchart TD
    subgraph Frontend [前端 Web 工作台]
        UI1[Markdown 头部 schedule 声明] --> UI2[保存或切换定时开关]
        UI3[下次执行倒计时看板]
    end

    subgraph Server [服务端 internal/scheduler]
        UI2 -->|API 触发| S_API[Scheduler API Handlers]
        S_API -->|热重载| S_Pool[Cron 调度器池 robfig/cron]
        S_Pool -->|定时时刻触发| Worker[Trigger Worker]
        
        Worker --> CheckVer{检查版本一致性}
        CheckVer -- 待更新 --> Compile[Compiler.Compile 自动生成稳定 Python]
        CheckVer -- 一致 --> CheckLock{检查任务管理器设备锁}
        Compile --> CheckLock
        
        CheckLock -- 设备正忙 --> Skip[记录 Skipped 状态与日志]
        CheckLock -- 设备空闲 --> Exec[TaskManager.StartTask]
    end

    subgraph TaskEngine [任务生命周期 internal/taskmanager]
        Exec --> PID[登记真实系统 PID 锁定设备]
        PID --> Run[Python 隔离子进程执行]
        Run --> Finish[进程退出, 释放设备锁, 记录执行结果]
    end
```

---

## 2. 调度执行状态机与防重叠设计

每次调度器到点被触发时，严格遵循以下确定性状态机：

```mermaid
stateDiagram-v2
    [*] --> Scheduled: 到达 Cron 预定时刻
    Scheduled --> ResolvingTarget: 解析目标设备 target_device
    ResolvingTarget --> DeviceOffline: 目标设备不在线
    DeviceOffline --> Skipped
    ResolvingTarget --> CheckingLock: 目标设备在线
    CheckingLock --> DeviceBusy: taskManager.GetActiveTaskByDevice != nil
    DeviceBusy --> Skipped: 触发防并发保护, 跳过本周期
    CheckingLock --> DeviceAvailable: 目标设备完全空闲
    DeviceAvailable --> CompilingIfNeeded: 校验版本与 Python 产物
    CompilingIfNeeded --> DispatchTask: TaskManager.StartTask
    DispatchTask --> Running: 施加设备排他锁并执行
    Running --> Completed: 执行成功, 自动释放锁
    Running --> Failed: 异常/超时退出, 自动释放锁
    Skipped --> [*]
    Completed --> [*]
    Failed --> [*]
```

---

## 3. 核心数据结构

### 3.1 调度项元数据 (ScheduleEntry)

```go
type ScheduleEntry struct {
    ScriptID     int64     `json:"script_id"`
    ScriptName   string    `json:"script_name"`
    CronExpr     string    `json:"cron_expr"`
    TargetDevice string    `json:"target_device"`
    IsEnabled    bool      `json:"is_enabled"`
    NextRunAt    int64     `json:"next_run_at"`
    LastRunAt    int64     `json:"last_run_at"`
    LastStatus   string    `json:"last_status"` // "success", "failed", "skipped"
}
```

### 3.2 数据库支持 (markdown_scripts 扩展字段)

```sql
ALTER TABLE markdown_scripts ADD COLUMN last_run_at INTEGER NOT NULL DEFAULT 0;
ALTER TABLE markdown_scripts ADD COLUMN last_run_status TEXT NOT NULL DEFAULT '';
```

---

## 4. API 规范

### 4.1 获取当前所有调度计划
- **方法与路径**：`GET /api/scheduler/entries`
- **返回**：
  ```json
  {
    "entries": [
      {
        "script_id": 1,
        "script_name": "真机签到",
        "cron_expr": "*/30 * * * *",
        "target_device": "10.0.0.30:5555",
        "is_enabled": true,
        "next_run_at": 1791718800000,
        "last_run_at": 1791717000000,
        "last_status": "success"
      }
    ]
  }
  ```

### 4.2 一键启停单个脚本的定时调度
- **方法与路径**：`POST /api/md-scripts/{id}/toggle-schedule`
- **请求体**：
  ```json
  {
    "enabled": false
  }
  ```
- **返回**：
  ```json
  {
    "ok": true,
    "script_id": 1,
    "is_enabled": 0
  }
  ```
