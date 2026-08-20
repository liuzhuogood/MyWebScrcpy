# 生产环境资产

## 环境概览

- 用途：对外提供 MyWebScrcpy 浏览器端 Android 投屏服务。
- 访问入口：`https://10.0.0.6:8080`（默认启用 HTTPS，浏览器打开即用）。
- 负责人：liuzhuo。
- 部署形态：直接运行二进制 `mywebscrcpy-linux-amd64`，由 systemd 托管，未使用 Docker。

## 服务器 / 主机

| 用途 | IP | SSH 端口 | 备注 |
| --- | --- | --- | --- |
| 应用服务器 | 10.0.0.6 | 22 | hostname `C93`，已配置免密登录 `ssh liuzhuo@10.0.0.6` |

### 主机规格

- 操作系统：Debian GNU/Linux 12 (bookworm)
- 内核：6.1.0-35-amd64（SMP PREEMPT_DYNAMIC）
- 架构：x86_64
- 登录账号：`liuzhuo`（家目录 `/home/liuzhuo`），服务以 `root` 运行

## 账号与服务

- 系统账号：`liuzhuo`（SSH 免密登录），服务进程以 `root` 运行。
- 二进制部署：
  - 运行路径：`/opt/mywebscrcpy/mywebscrcpy-linux-amd64`
  - 启动参数：`-https`（启用内置 HTTPS 证书）
  - 工作目录：`/opt/mywebscrcpy/`
- systemd 服务：`mywebscrcpy.service`（`loaded active running`，开机自启）
  - 进程示例：`root 82263 1 0 15:41 ? 00:00:10 /opt/mywebscrcpy/mywebscrcpy-linux-amd64 -https`
- 数据库：暂无（项目不依赖数据库）。
- 缓存：暂无。
- 消息队列：暂无。

### 服务运维

```bash
# 查看状态
systemctl status mywebscrcpy.service
# 重启 / 停止 / 启动
sudo systemctl restart mywebscrcpy.service
sudo systemctl stop mywebscrcpy.service
sudo systemctl start mywebscrcpy.service
# 查看日志
journalctl -u mywebscrcpy.service -f
```

## 第三方服务

- 暂无。

## 域名与端口

- 对外域名：暂无（当前通过内网 IP `10.0.0.6` 访问）。
- 服务端口：`8080`（HTTPS），监听 `*:8080`。
- 设备连接：通过本机 ADB 连接 Android 设备（USB 直连或网络 ADB）。

## 关键配置

- ADB：已安装 `Android Debug Bridge version 1.0.41`。
- Docker：未安装（当前为二进制直跑部署）。
- 环境变量：通过 systemd drop-in `/etc/systemd/system/mywebscrcpy.service.d/run-as-liuzhuo.conf` 注入。
- 文件管理对象：当前连接手机的 `/storage/emulated/0` 共享存储，由 ADB 执行文件操作；服务器不再维护用户文件根目录。
- 单文件上传上限：未显式设置，使用程序默认值 `268435456` 字节。
- HTTPS 证书：使用项目内置自签证书（`-https`）。
- 备用二进制：`/home/liuzhuo/mywebscrcpy-new`（待替换 / 升级用）。

## 部署方式

- 部署脚本：`scripts/deploy/deploy-prod.sh`
- 部署流程：本地构建 Linux amd64 二进制 → `scp` 上传 → 替换二进制 → `systemctl restart`。
- 无需编译环境（服务器上不安装 Go，二进制在本地 Mac 交叉编译）。

## 联系人 / 变更记录

- 负责人：liuzhuo。
- 最近变更：初始化生产环境资产；确认服务以二进制 + systemd 方式运行在 `10.0.0.6`。
- 2026-07-16：创建部署脚本 `scripts/deploy/deploy-prod.sh`。
- 2026-08-20：部署 `v1.3.0`；服务因 systemd 未设置相对路径根目录首次重启失败，随后修正运行配置并恢复 `active`。HTTPS 首页和投屏链路健康检查通过。
