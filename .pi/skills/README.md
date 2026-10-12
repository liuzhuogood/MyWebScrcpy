# 文档助手的 Pi 技能

将项目专用技能放在这个目录中，每个技能使用独立子目录：

```text
.pi/skills/
└── document-review/
    ├── SKILL.md
    └── references/
        └── rules.md
```

`SKILL.md` 的最小格式：

```markdown
---
name: document-review
description: 审核移动端自动化 Markdown 文档；当用户要求检查步骤或配置时使用。
---

# 文档审核

读取本次运行目录的 current.md，根据用户需求检查步骤与配置。
需要参考资料时，读取技能目录下的 references/rules.md。
给出修改建议，不直接修改文档或操作设备。
```

上面是格式示例，不是已安装的技能；添加时请写入实际存在的参考文件。

助手会在每次新请求启动 Pi 时扫描此目录。名称、描述和路径会加入模型上下文，完整正文按需读取。因此新增或修改技能后，下一次发送消息即可加载更新，无需再次修改代码。进行中的请求继续使用其已加载内容。

此目录的全部技能和参考资料会自动进入打包产物。启动时释放到可执行程序旁的 `.pi/skills/`，缺失文件自动补充，已有文件和用户新增技能保留。用户无需另外复制项目目录；以后可直接编辑程序旁的技能。更新应用不会覆盖已有技能修改。

技能目录按程序位置确定，与启动工作目录无关。Pi 的临时运行目录不变，不自动加载全局技能或项目 `.agents/skills/`。

技能可以携带说明和参考资料，但助手仍只启用 read、grep、find、ls。技能中要求执行 Shell、写文件或操作手机的指令不会使这些工具自动可用。

格式与按需加载规则见 [Pi 技能文档](https://github.com/earendil-works/pi/blob/main/packages/coding-agent/docs/skills.md)。
