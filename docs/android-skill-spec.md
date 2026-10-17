# 安卓自动化技能规范

单个技能目录、技能说明目录、上架评审格式与版本规则。单源版本在`skill.yaml`。

## 1. 技能目录

```text
skill-<任务名>/
  SKILL.md
  skill.yaml
  rules/
  wiki/
  templates/            # 按需
  display/
    manifest.yaml
    intro.md
    changelog.md
```

约束：`rules/` 保存业务目标、页面判断、边界与成功条件；`wiki/` 保存经验证的页面知识与恢复经验；通用操作与观察脚本由 `my-scrcpy-common` 统一提供，应用技能不包含专用业务脚本与代码生成要求；任务执行与证据记录保存在本地工作目录 `~/.mywebscrcppy/tasks/<task-id>/` 中。

## 2. 说明目录与评审格式

`assets/`只放文件，`display/`只写文案并相对路径引用`assets/`。
`intro.md`只讲用户用法，不写实现细节。

`display/manifest.yaml`字段全部必填，缺字段或超长打回：

```yaml
name: dingtalk-punch
version: 1.2.3
title: 钉钉自动打卡
subtitle: 一句话卖点
category: 考勤/效率
icon: ../assets/icon/icon-1024.png
screenshots:
  - ../assets/screenshots/01-首页.png
supports:
  min_android: 10
  serial_types: [wifi, usb]
permissions: [ocr, location-mock]
risk_level: low
price: free
```

资源要求：`icon-1024.png`为1024x1024 PNG；截图3-6张，1080x2400原图；
`demo.mp4`可选，不超过60秒。

## 3. 版本

`skill.yaml`的`version`为唯一真相，`display/manifest.yaml`必须一致：

```yaml
name: dingtalk-punch
version: 1.2.3
compat:
  executor: ">=1.0.0"
  backend_api: ">=2.5.0"
  min_android: 10
```

`MAJOR.MINOR.PATCH`：`MAJOR`为不兼容变更（参数改名/删除、前置条件变、
新增高危权限、上报字段变）；`MINOR`为兼容加功能（新增`rules/`分支、
新增可选参数、新增机型适配）；`PATCH`为修错（定位阈值、等待时间、
`dryrun.yaml`限额、文案错字）。

灰度后缀`1.3.0-dryrun.1`只走`dryrun/`，不许上架。规则变更后必须重生成脚本
并升版本；`PATCH`可直接发，`MINOR/MAJOR`重跑`dryrun`并更新`display/`。
发布按版本倒序记`display/changelog.md`，并打`git tag skill-<name>-vX.Y.Z`。
