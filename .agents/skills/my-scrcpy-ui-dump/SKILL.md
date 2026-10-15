---
name: my-scrcpy-ui-dump
description: 结合 MyWebScrcpy 的 /api/ui/xml 与 /api/ui/page-info 判断页面，分析 UIAutomator dump，测试 XPath 并生成唯一定位表达式，为后续设备脚本提供依据。
---

# UI dump 分析

获取当前设备的 UI 快照，分析可验证的定位条件与操作前后状态，为后续脚本提供思路。此技能默认只读取与分析；不会自动唤醒、解锁、打开应用、点击或运行脚本。

## 获取

1. 使用 [my-scrcpy-use](../my-scrcpy-use/SKILL.md) 及其通用请求客户端。按该技能先读取当前 `/api/openapi.json` 和 `/api-docs.html`，核对 `/api/ui/xml` 与 `/api/ui/page-info` 的方法、参数、响应 schema 及引用。Token、TLS 和超时仍由同一客户端处理，不在此技能复制请求层。
2. 从用户或当前会话确认服务地址与设备 serial；不要把下面的演示设备作为默认目标。当前两个接口都是 `GET`，query 为 `serial`，200 响应为 JSON。XML 接口字段为 `serial`、`captured_at`、`xml`，可选 `display_size`；page-info 字段为 `package_name`、`activity`、`component`、`window_name`、`fetched_at`，可选 `display`。后续接口定义发生变化时，先调整数据映射，不猜兼容字段。
3. 按“page-info → XML → page-info”顺序采集同一设备，每次单独保存所有原始响应、提取的 XML 和分析结果；任一请求失败时停止，不分析旧文件。项目根目录下的示例：

   ```bash
   capture_dir="$(mktemp -d /tmp/my-scrcpy-ui-XXXXXX)"
   python3 .agents/skills/my-scrcpy-use/scripts/request_client.py \
     --base-url http://127.0.0.1:1989 /api/ui/page-info \
     --query serial=10.0.0.30:5555 --output "$capture_dir/page-before.json" && \
   python3 .agents/skills/my-scrcpy-use/scripts/request_client.py \
     --base-url http://127.0.0.1:1989 /api/ui/xml \
     --query serial=10.0.0.30:5555 --timeout 45 \
     --output "$capture_dir/response.json" && \
   python3 .agents/skills/my-scrcpy-use/scripts/request_client.py \
     --base-url http://127.0.0.1:1989 /api/ui/page-info \
     --query serial=10.0.0.30:5555 --output "$capture_dir/page-after.json" && \
   python3 .agents/skills/my-scrcpy-ui-dump/scripts/analyze_dump.py \
     "$capture_dir/response.json" --expected-serial 10.0.0.30:5555 \
     --page-info-before "$capture_dir/page-before.json" \
     --page-info-after "$capture_dir/page-after.json" \
     --xml-output "$capture_dir/dump.xml" --output "$capture_dir/analysis.json"
   ```

   上述地址和设备来自创建技能时的会话，后续替换为已确认的目标。原始快照含页面数据，留在本地分析，不作为测试样本提交。
4. 检查实际 serial、抓取时间、XML 格式及节点数量。400、422、502 等错误按当前规范和实际响应解释。不要通过备用 ADB 命令掩盖接口失败；同一服务的认证与网络问题在请求层解决。

## 分析与定位

[分析脚本](scripts/analyze_dump.py) 只解析本地 JSON 响应，返回节点层级、原始属性、标记为可操作的节点和定位候选。内部节点编号、树路径及匹配次数仅用于本次快照，不能当作跨页面稳定定位条件。

先读取 `page_context` 判断当前页面证据：

- 前后 `package_name`、`activity`、`component`、`window_name` 是各自字段的原始观察值，不能互相解析替代；`package_name` 表示聚焦应用，业务页面含义仍需 XML 中的文字、ID 和层级验证。不能仅凭 Activity 类名编造页面标题。
- `capture_within_page_info_window` 核对 XML 时间是否落在两次 `fetched_at` 之间，`changed_page_fields` / `changed_display_fields` 标记前后变化。不属于同一时间窗口或前后变化时，重新采集并停止基于旧页面提出可执行操作；前后一致也不等于原子快照。
- `focus_package_in_dump` 核对聚焦包是否出现在节点包名中。若 page-info 显示桌面或应用、XML 却只有 System UI，就分别报告焦点和覆盖层证据，不能声称目标业务页面已经可操作。若出现明确 keyguard 控件，可以说明 XML 中观察到了这些控件；是否锁屏需进一步证据，不能自动解锁。
- `display` 中的尺寸、密度和角度按当前定义读取。`display_size_matches` 比较其尺寸与 XML 的 `display_size`；缺失则保持未知，不用 page-info 补造 XML 坐标基准。XML 根的 rotation 与 page-info 旋转角度独立记录，未经确认不做单位换算。
- page-info 响应本身不含 serial，设备对应关系由采集时相同 query 参数保证；不能声称从响应验证了设备身份。单独分析旧 XML 时可省略页面文件，但只能输出该 XML 的观察，不能判断实时页面。

- 先看 `summary` 和 `warnings`：包名集合、节点数量、文本与描述数量、点击/长按/滚动标记。一个快照可能包含系统窗口和应用窗口，包名不能直接证明前台应用。空树或仅有 System UI 遮罩时，明确说明当前没有业务页面证据；不要推断设备一定锁屏或自动解锁。
- 沿 `parent_id` / `ancestor_ids` 理解节点所在容器、同级关系、可点击父节点及滚动区域。子文字节点可能不可点击，读取 `click_target_id` 确认可点击祖先，并核对该祖先是否对应整个列表行，避免点错。
- 分别评估实际存在的 `resource-id`、`content-desc`、`text`，辅助使用 `package`、`class` 和明确的父子/同级关系。不是字段名兜底，也不要编造缺失属性。`selector_candidates` 中的条件是分析数据，并非已实现的脚本 API。
- `matches_in_snapshot` 只表示当前快照中的匹配数量。优先评估非空 ID 与文字；重复 ID、相同文字、动态计数与列表复用都要加容器或关系限定。一次唯一不等于跨页面稳定，长期脚本应比较多次快照。
- `enabled`、`clickable`、`long-clickable`、`scrollable`、`checkable`、`checked`、`selected`、`password` 等属性按 XML 中的原值解释。缺失状态保留为未知，不视为默认可用。分析脚本只标记明确 `enabled="true"` 的操作节点；标记不证明点击或滚动一定成功。
- `bounds` 是 `[x1,y1][x2,y2]` 的矩形。中心点仅是候选坐标，需矩形非空、完整落在响应 `display_size` 内。缺失基准、越界或非法矩形时不给候选坐标；不从最大节点范围推算分辨率、不自动裁剪、不做未知缩放或旋转转换。坐标可能受遮挡影响，执行前重新抓取并结合截图核对。
- XML 不覆盖所有视觉内容。没有节点或缺少文字时，报告观察边界；需要视觉定位时可提出获取截图的下一步，具体接口仍从当前规范查询。密码节点只讨论结构，不复述其值。

## XPath 生成与测试

[XPath 工具](scripts/xpath_tools.py) 使用 lxml 的 XPath 1.0 引擎，在原始 XML 上执行表达式，不靠字符串计数判断唯一性。依赖见 [requirements.txt](scripts/requirements.txt)，在所选 Python 环境运行 `python3 -m pip install -r .agents/skills/my-scrcpy-ui-dump/scripts/requirements.txt`；已经安装时无需重复安装。

先从分析结果确定目标的本次 `node_id`，再生成表达式：

```bash
python3 .agents/skills/my-scrcpy-ui-dump/scripts/xpath_tools.py \
  "$capture_dir/response.json" --expected-serial 10.0.0.30:5555 --node-id 1 \
  --page-info-before "$capture_dir/page-before.json" --page-info-after "$capture_dir/page-after.json"
```

省略 `--node-id` 时为所有节点生成。`selected_xpath` 只接受实际恰好命中该目标节点的表达式；`tested_candidates` 保留已测试候选与匹配数。找不到唯一条件时返回空结果和非零退出码，不用全局序号或 `(表达式)[1]` 掩盖多匹配。

生成顺序为：`@resource-id`、`@text` 及二者组合（必要时加实际包名/类名）→ 相邻兄弟特征及祖先/容器内特征 → 自身 `@content-desc` → `@bounds`。UIAutomator 的 ID 属性是 `resource-id`，文字属性是 `text`，不能凭空改用 `@id` 或 `text()`。相邻和容器锚点取真实的 ID、文字或描述，自动搜索最近兄弟及最多三级祖先；未找到时可根据页面证据手工构造更远关系再测试。`bounds` 仅在矩形及屏幕基准有效时生成，标为 `geometry`，跨快照需重新验证。关系表达式中的 `[1]` 表示最近兄弟/祖先的相对关系，必须核对其页面语义。

测试用户给定或手工优化的表达式，可重复传 `--xpath`：

```bash
python3 .agents/skills/my-scrcpy-ui-dump/scripts/xpath_tools.py \
  "$capture_dir/response.json" --expected-serial 10.0.0.30:5555 --node-id 1 \
  --page-info-before "$capture_dir/page-before.json" --page-info-after "$capture_dir/page-after.json" \
  --xpath "//node[@resource-id='com.android.systemui:id/scrim_behind']"
```

此例 ID 来自创建技能时的真实快照，后续替换为当前目标属性。结果包含 `match_count`、`matched_node_ids` 和 `unique`；指定 `--node-id` 时还检查 `verified_target`，防止唯一但命中了别的控件。零匹配、多匹配、目标不符和语法错误都返回非零退出码。仅接受 UI `node` 元素集合，标量、属性与字符串结果不能作为元素定位。文本中的单双引号由生成器编码，不手工拼接未经转义的 XPath。

定位唯一不代表可点击或业务页面正确：用 `page_context` 确认页面证据，用匹配节点及分析结果的 `click_target_id` 确认操作对象，必要时为可点击父节点另生成并验证 XPath。工具退出码只表示 XPath 定位测试，页面结论单独查看；页面信息冲突时不能据此执行脚本。把同一表达式在新的 dump 上重复测试，才可评估定位稳定性；执行脚本所用运行时也需支持这些 XPath 语法。

## 为脚本输出思路

针对用户目标给出少量有证据的建议；没有目标时概述可用定位点与信息缺口，不臆造业务流程。报告包含：

1. 快照来源：服务、serial、XML 及前后 page-info 时间、屏幕基准与文件链接。
2. 页面证据：聚焦包名、Activity、组件、窗口与 XML 关键文字/ID/类名、容器关系和状态是否一致；页面含义的推测单独注明。
3. 定位方案：准确条件、当前匹配次数、重复或动态属性风险、必要的父子关系；坐标仅作需再验证的候选。
4. 流程思路：进入页面的已知前置条件 → 等待目标出现且满足状态 → 唯一定位 → 拟执行动作 → 用新快照验证页面或控件状态变化。等待和重试要有时间/次数上限，未找到、多匹配或验证失败时停止并保存证据。
5. 验证边界：哪些已从 XML 确认，哪些仍需实际操作验证。用户只要分析时输出步骤或伪代码；正式脚本使用的函数与接口先核实，不能把定位候选伪装成现有 SDK 方法。

执行动作后，只有在已获授权且动作确已发生时获取新快照比较。对比语义属性、容器关系和页面状态，不用本次编号或结构路径断言同一节点。避免把旧坐标、时间或设备 ID 固化到正式脚本。
