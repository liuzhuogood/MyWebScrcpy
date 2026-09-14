# v1.9.0-feature 会话记录

## 2026-09-14

### 会话背景与需求提出
**用户**：
在现有的投屏与动作执行架构下，点击和滑动目前仅支持通过 scrcpy 控制 Socket 注入（即 SDK 模式）。但在某些特殊场景（例如游戏安全策略拦截应用层虚拟注入、系统安全输入界面、第三方定制 ROM 权限限制等），希望能够在调用点击和滑动接口时支持传入 `mode` 参数，允许指定使用 `adb` 方式（`adb shell input tap/swipe`）进行控制。

同时，用户提出了以下技术探讨点：
1. 是否可以支持更多控制模式，例如 AOA（OTG USB HID）、UHID、sendevent 等？
2. 如果 AOA 会与 scrcpy 冲突，能否通过“另起一个独立进程单独跑 AOA”的方式来解决？
3. 如何保证存量接口与调用者的向后兼容？
4. ADB 模式下的坐标系统如何处理？
5. 先完成完整的技术方案与版本四件套设计，不直接开发代码。

---

### 技术探讨与可行性深度分析

#### 1. 关于能否另起独立进程运行 AOA（Android Open Accessory）的研讨
- **分析过程**：
  - AOA 2.0 协议要求 Android 设备充当 USB Accessory（从设备模拟为 HID 键盘/鼠标等硬件外设）。
  - 在典型的使用场景中，手机通过**单根物理 USB 数据线**连接至宿主机。
  - 当外部独立进程发起 AOA 握手（向手机发送 Vendor Request 53 `ACCESSORY_START`）时，Android 系统的 Linux 内核驱动会主动卸载当前的 USB Gadget 配置（断开包含 ADB 的复合设备驱动），并重新配置为 Google Accessory 模式。
  - 这一模式切换会直接在物理层拉低并重新上拉 USB D+/D- 数据线（USB Bus Reset），导致宿主机操作系统检测到该 USB 端口设备被“拔出并重新枚举（Re-enumeration）”。
  - 结果：宿主机所有正在使用该 USB 设备的文件描述符和连接句柄瞬间失效（Broken Pipe / EOF）。正在运行的 ADB 守护连接被强行切断，MyWebScrcpy 的 scrcpy 视频流推流与控制连接立即断连崩溃。
- **技术结论**：
  - **单 USB 线缆下，AOA 与 ADB/Scrcpy 在物理总线层面互斥**。
  - “另起进程”属于操作系统进程层面的隔离，无法改变底层单根物理 USB 总线只能工作在一种设备模式的硬件事实。因此明确剔除 AOA 方案。

#### 2. 关于 UHID 模式的可行性分析
- **分析过程**：
  - scrcpy 官方支持的 UHID 模式通过向 `/dev/uhid` 注入相对位移的鼠标 HID 报文。
  - 相对位移对于精确的绝对坐标点击（如点击屏幕 (0.5, 0.3)）无法直接计算步长，无法满足精确自动化测试需求。
  - 若尝试模拟绝对坐标触控板/触摸屏（Digitizer），HID 报告描述符定义极其复杂，且不同厂商 Android 内核（尤其是部分魔改 ROM）对虚拟 HID 触控屏的支持存在兼容性裂痕。
- **技术结论**：
  - 剔除 UHID 方案，保持架构精简可靠。

#### 3. 关于 sendevent 模式的可行性分析
- **分析过程**：
  - 直接向 `/dev/input/event*` 写入二进制事件虽然延迟极低，但该节点在 Android 系统中权限为 `crw-rw---- root input`，必须依赖设备已获得 Root 权限。
  - 绝大多数商用设备和自动化生产设备均未 Root。
  - 此外，不同设备的触控 IC 量程（`ABS_MT_POSITION_X/Y`）极度碎片化（如 0~32767、0~4095），难以通用。
- **技术结论**：
  - 剔除 sendevent 方案。

#### 4. 架构方案收敛与设计原则
经过多维度权衡与评估，最终方案收敛为 **SDK 与 ADB 双模式架构**：
1. **模式定义**：
   - `sdk` 模式（默认）：保留现有 scrcpy 控制 Socket 注入，具有 1~3ms 极低延迟与拟人化曲线能力。
   - `adb` 模式：作为高穿透备选通道，调用系统内置的 `adb shell input tap/swipe`，穿透受限窗口。
2. **向后兼容**：
   - 存量请求缺省 `mode` 字段时，默认走 `sdk` 模式，原有逻辑与参数（含 `humanize`）完全不受影响。
3. **坐标系统自动映射**：
   - 上层统一采用 0~1 归一化坐标。
   - `adb` 模式下后端自动依据设备屏幕物理分辨率进行像素换算与范围边界夹取，避免调用者繁琐计算。
4. **组件复用与串行保护**：
   - 复用工程中已成熟的 `internal/adbcommand/manager.go` 执行命令，免去重复 fork 进程的开销，并纳入 `devicegate` 串行排他管控。
   - 执行成功后同步广播 `touch.event` 保持投屏触控反馈动效。

---

### 输出交付
依据上述结论，在 `docs/versions/v1.9.0-feature/` 下创建完整的“版本四件套”文档：
1. `version.md`：版本摘要与核心亮点总结。
2. `design.md`：详细设计文档，含五种模式可行性对比矩阵、架构图、执行时序图、AOA 断流原理分析、接口设计与坐标映射规范。
3. `tasks.md`：覆盖规格设计、模型扩展、ADB 封装、分流改造、全量单测与文档更新的待办任务清单。
4. `session.md`：完整记录会话讨论脉络与架构收敛历程。

---

## 2026-09-15

### 实施授权与增量需求
**用户**：
1. 开启实现吧，做一下测试 /goal。
2. 要增加模板点击功能，也就是传模板名进行点击，自动点击模板中央，或者增加随机小范围偏移（参数选项，默认不偏移）。
3. 如果模板没有匹配，就返回报错信息。
4. 还有如果是 adb 点击，是不是可以实现像手指一样区域点击，帮我看看，可以的话就实现。

### 核心实现记录
1. **ADB 模式及物理手指区域点击落地**：
   - 检查现有 `internal/ws/humanize.go` 针对 SDK 模式的拟人化实现；
   - 在 `internal/ws/registry.go` 的 `deviceActionExecutor.executeADB` 中：
     - 当 `mode="adb"` 且 `humanize=true` 时，落点施加高斯微抖动（模拟物理手指接触面半径，通常在 5~8 像素左右），并生成 60~150ms 的真实物理按压时长；
     - 通过执行起点终点相同的 `adb shell input swipe fx fy fx fy <holdMS>` 触发 Android 底层输入系统的完整物理按压（ACTION_DOWN -> 保持 -> ACTION_UP），完美实现“像手指一样的区域点击”；
     - 当 `humanize=false` 时，执行标准精确单点 `adb shell input tap x y`；
     - 执行成功后同步调用 `touchPublisher.emit` 广播投屏触控反馈动效。
2. **模板自动匹配与点击功能 (POST /api/templates/click)**：
   - 在 `internal/template/model.go` 中增加 `ClickOptions`、`ClickRequest`、`ClickResult` 及 `ErrTemplateNotMatched`；
   - 在 `internal/template/storage.go` 中实现 `FindTemplate`，支持按模板 ID 或名称（大小写自适应）检索模板；
   - 在 `internal/template/service.go` 中实现 `ClickTemplate`：
     - 先查询当前最新匹配缓存，若无或未开启后台 worker 则触发即时抓屏 + 匹配；
     - 若未在当前画面匹配到该模板，返回 `ErrTemplateNotMatched`（HTTP 422 报错）；若模板不存在返回 404；
     - 自动计算匹配框几何中心 $(cX, cY)$；若开启 `random_offset=true`，则在模板矩形内部施加受限随机微偏移（确保 100% 落在目标框内）；
     - 提交动作执行器完成点击，支持 `mode="sdk"` 与 `mode="adb"`；
   - 在 `internal/template/handler.go` 中注册 `POST /api/templates/click` 路由，并在 `web/js/template-match.js` 模板卡片上增加“点击”交互按钮与提示。
3. **测试与规范验证**：
   - 编写 `internal/ws/adb_mode_test.go` 与 `internal/template/click_test.go`；
   - 更新 `docs/vision-integration.md` 与 `web/openapi.json` 并通过 `api_docs_test.go` 校验；
   - 全仓库运行 `go test -race -count=1 ./...`，全量测试 100% 通过，无竞态问题。

