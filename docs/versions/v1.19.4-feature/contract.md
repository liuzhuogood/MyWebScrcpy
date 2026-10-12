# v1.19.4 接口与验收契约（实施契约）

日期：2026-10-12。用户已明确回复“开始正式实现，按当前设计推进”，作为本轮实施契约。

## 唯一关联与快照

`script_id → dispatch_id → task_id → recording_id / report revision / log seq` 为唯一关联链。数据由已有批次映射扩展，不允许前端从相近时间、当前设备列表、标题或最新录制推断。

| 对象 | 必须保存的字段 |
| --- | --- |
| 批次 | id、script_id、trigger（manual/schedule/legacy_unknown）、dispatch_state（dispatching/finished）、status、started_at、ended_at、error_msg |
| 批次快照 | availability（available/legacy_unavailable）、script_name、document、document_hash、compiled_hash、document_version、target_devices、recording、report；旧缺失字段明确 null |
| 设备运行 | 沿用 RunningTask 字段；新增 dispatch_id、preparation_state（preparing/ready/failed/finished），运行过程不引用当前编辑稿 |
| 录制归属 | recording_id、task_id、device_serial、owner_task_id、requested_at、started_at、ended_at；缺少归属就拒绝任务停止请求 |

旧批次没有来源时使用明确 `legacy_unknown`，不能猜为定时。当前未公开 preparation_state 不意味着预留时可以声明 Python 已运行；新页面优先显示“准备中”，只有启动确认后显示“运行中”。现有执行状态入口保留，新增映射由服务端统一提供。

## 运行详情示例

建议 `GET /api/tasks/{task_id}` 返回稳定对象。示例为完成的单台设备；时间为 Unix 毫秒，duration 从真实 started_at/ended_at 计算，PID 不作为产物关联键。

```json
{
  "task": {
    "id": "task_01",
    "dispatch_id": "dispatch_01",
    "script_id": 12,
    "script_name": "每日签到",
    "device_serial": "3A281F",
    "pid": 1234,
    "status": "completed",
    "preparation_state": "finished",
    "exit_code": 0,
    "error_msg": "",
    "started_at": 1791766800000,
    "ended_at": 1791766812000
  },
  "artifacts": {
    "log": {
      "availability": "available",
      "capture_state": "completed",
      "truncated": false,
      "last_seq": 7,
      "bytes": 864,
      "expires_at": 1792371612000,
      "error_msg": ""
    },
    "recording": {
      "state": "completed",
      "recording_id": "recording_01",
      "record_audio": false,
      "coverage_complete": true,
      "error_msg": ""
    },
    "report": {
      "state": "completed",
      "generation_revision": 1,
      "available_revision": 1,
      "can_retry": true,
      "error_msg": ""
    }
  }
}
```

`expires_at` 等没有值时明确 null，不省略后让前端猜默认值。旧记录的录制音频、覆盖完整性未知时也是 null。coverage_complete 只能由本次进程区间与实际视频区间验证，不能仅靠状态 completed 推断。

## 三类产物状态

| 对象 | 状态与页面行为 |
| --- | --- |
| 日志 availability | available：可查询；unavailable：历史未采集；expired：已清理，返回到期状态 |
| 日志 capture_state | waiting：准备中；running：持续输出；completed：采集已结束；degraded：写盘失败，提示不完整，不改变进程结果 |
| 日志 truncated | 独立布尔；达到上限仍排空输出，显示截断并停止持久写入普通输出，保留有界结束事件元数据 |
| 录制 state | unavailable：旧历史无关联；disabled：启动快照关闭；not_started：开启但设备预检未通过；preparing：等待创建；recording：录制中；stopping：收尾中；completed：可播放；failed：显示原因；expired：文件已清理 |
| 报告 state | unavailable：旧历史未生成；disabled：启动快照关闭；waiting：运行尚未结束；pending：已入队；running：模型调用中；completed：生成成功；failed：显示原因 |

接口按状态决定可用动作。运行中、旧历史无快照和已关闭的报告不能出现重新生成按钮；不能在用户修改当前头部后追溯开启旧运行的报告。报告引用日志过期时保留报告和引用行号，但取消跳转按钮并显示“原始日志已过期”，不让链接定位到其他运行。

再次生成时，`generation_revision` 是最新尝试，`available_revision` 是可阅读的最后成功修订。最新尝试失败仍展示旧成功报告和失败状态；两个字段分别表达事实，不让前端从空 markdown 猜测回退版本。GET report 默认读取 available_revision，另提供 generation 状态；revision 参数精确读取指定成功修订。

## 日志游标与搜索

建议 `GET /api/tasks/{id}/logs` 返回：

```json
{
  "items": [
    {"task_id":"task_01","seq":4,"at_ms":1791766806412,"stream":"stdout","message":"等待签到按钮，重试 1/3","continued":false}
  ],
  "next_cursor": 7,
  "has_more": false,
  "complete": true,
  "truncated": false,
  "availability": "available"
}
```

- after=n：只读取 seq>n，升序返回；next_cursor 为本次扫描到的最高 seq，包含被来源或搜索条件排除的行，避免空结果重复扫描。执行中没有新增行时保留 n。
- before=n：只读取 seq<n，按倒序扫描后以升序返回显示，next_cursor 为扫描到的最低 seq，继续查更早内容；无更多扫描行时保持原游标并 has_more=false。
- 两个游标不能同时传；不传时读取最近一页，响应同时给出 oldest_seq 和 latest_seq 供前端分别向前/向后读取。示例为增量查询，故只包含 next_cursor。
- 搜索仅匹配当前运行原文，q 最大 200 字符；stream 只接受 all/stdout/stderr/system。查询每页有限扫描，空 items + has_more=true 表示尚有扫描范围，前端可继续请求，不能马上判定全局无匹配。
- 每行 seq 全生命周期稳定，筛选后仍显示原 seq，报告不引用筛选数组位置。相同 seq 返回到已加载窗口时去重，不能丢弃不同 seq 的相同文本。
- 数据不可用/已过期分别返回结构化原因；搜索 200 且 items=[] 属于无匹配，不能当成下载或存储失败。

## 异常处理矩阵

| 情况 | 任务 | 录制 | 报告 |
| --- | --- | --- | --- |
| 设备离线/占用 | 对应设备 skipped，其他设备继续 | 开启时 not_started，记录原因 | 开启时总结跳过事实 |
| 录制冲突/声音不可用 | 建议继续启动 | failed，不能接管别人录制 | 如实提及缺少视频，不能推断画面 |
| 录制创建超时 | 建议继续启动 | failed；迟到创建仅清理属于本任务的实例 | 记录请求超时事实 |
| Python 启动失败 | failed，释放已预留设备 | 停止本任务已建录制 | 开启时总结启动失败 |
| 写盘失败 | 运行中不强杀 | 独立状态 | 证据标记不完整 |
| 超时/手动终止 | 保留 failed/killed；确认退出 | 请求停止自己录制并异步封装 | 开启时在日志收尾后入队 |
| 视频封装耗时/失败 | 原结果不变 | stopping/failed | 不等待封装；保留生成时录制状态 |
| 模型未配置/请求失败 | 原结果不变 | 原结果不变 | failed，显式重试，不自动换供应商 |
| 重新生成失败 | 原结果不变 | 原结果不变 | 保留上次成功报告 |
| 编译失败 | 批次 failed，无设备运行实例 | 不创建录制 | 本版只显示确定性编译错误，不造设备报告 |

录制和报告错误各占自己的标签，不把同一错误在运行标题、日志顶部、标签详情中重复弹窗展示。系统日志仍记录事件便于诊断。

## 验收门槛

| 场景 | 必须可观察的结果 |
| --- | --- |
| 正常执行 | 进程 stdout/stderr、task_id 和录制 owner 一致；重启后按同一 ID 查看产物；AI 结论有有效引用 |
| 多设备部分异常 | 同一 dispatch 下只阻止异常设备；逐台产物不能交叉；修改设备配置不改变历史 |
| 超时与手动终止 | 进程组确已退出；最后日志可读；仅停止所属录制；报告引用正确终态 |
| 快速再次运行 | 不重复占用设备，不把上次收尾视频关联本次，不让旧响应覆盖新 UI |
| 资料缺失与过期 | 区分未采集/未开启/过期；报告可读但过期引用不可跳转；不能虚构历史配置 |
| 报告重复触发 | 并发重试只生成一个在途 revision；失败保留最后成功内容；无自动重跑脚本 |
| 界面与资源 | 编辑稿/助手会话返回后保留；小屏保留行号与时间；下载是真实文件，播放具备 Range 支持 |

设计推荐初值保持为日志 7 天、每运行 20 MiB、总量 1 GiB、AI 输入 64 KiB、模型并发 2。本轮按用户“按当前设计推进”的授权采用这些初值。
