# Python 生成界面验收

选定保留现有生成按钮与 Python 页，复用助手活动组件，生成与聊天状态独立。原型为 prototypes/cases/python-generation-v1，使用模拟数据；正式工作台验收使用真实生成路由、受控模型及设备接口，不等同于真机操作。

| 截图 | 证据范围 |
| --- | --- |
| [workbench-sdk-success.png](workbench-sdk-success.png) | 正式工作台：保存有效 SDK Python 后切换源码；生成通知只显示一次 |
| [workbench-sdk-run.png](workbench-sdk-run.png) | 正式工作台：实际 Python 进程、受控 HTTP 返回 Browser Phone，任务 completed/退出码 0 与执行日志关联 |
| [workbench-sdk-mobile.png](workbench-sdk-mobile.png) | 正式工作台手机宽度：任务/文档/助手分别切换，Python 可查看与运行 |
| [prototype-sdk-mobile.png](prototype-sdk-mobile.png) | 公网模拟原型 390×844：长源码独立滚动，不覆盖活动区 |

正式界面操作了成功、缺信息、模型错误、停止与编辑过期。停止与失败保留最后成功源码，保存改动后禁用旧产物运行。首次临时验收夹具未注册记录路由，补齐真实 taskmanager 路由后复验已完成任务与日志；没有修改正式记录功能来适配夹具。

公网原型操作成功、缺信息、请求失败和生成时编辑保护，验证主要 JS/CSS、动态组件加载。浏览器旧临时 API 退出后产生的轮询警告属于已停止验收夹具；新夹具页面与公网原型另行核对。小屏测试修正了原型长源码溢出，不将模拟状态当作真实 Pi 生成或设备动作证据。

[临时公网原型](https://fee-interesting-biggest-tire.trycloudflare.com/#python-generation-v1)。电脑与服务保持运行时可访问。运行资料和停止脚本在本机 prototypes/artifacts/remote-preview/20261013-python-generation-sdk；仅停止本次创建的两个进程，保留已有 5174/5175 预览。

```bash
bash prototypes/artifacts/remote-preview/20261013-python-generation-sdk/stop.sh
```

公网起初被 Vite preview 的具体域名名单拒绝；为本次独立预览配置添加准确 hostname 后返回正常页面，未放行所有域名、未重启已有服务。正式验收 API 为临时测试进程，交付后关闭；公网模拟原型保留。

最终核对：公网页面返回 HTTP 200，键盘 Enter 生成成功，当前验收页面没有新的警告/错误；临时正式验收进程已正常关闭并返回测试通过。公网模拟预览继续保留。
