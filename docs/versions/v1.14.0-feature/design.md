# v1.14.0-feature 设计文档：手动添加设备与 SQLite 持久化

## 1. 背景与目标

当前项目主页的设备列表完全依赖后台定时执行 `adb devices -l` 轮询发现。对于局域网无线调试设备（TCP/IP 设备），若服务重启或网络闪断，设备将直接丢失。
本版本目标：
1. 引入 SQLite 数据库持久化存储手动添加的设备。
2. 设备列表融合展示「自动发现」与「手动添加」两类设备。
3. 后台服务对配置了自动重连的手动设备执行定时守护与重试连接（`adb connect`）。
4. 前端提供直观的来源标识、添加设备弹窗、手动设备删除与快速重连功能。

## 2. 架构设计

### 2.1 数据库选型
采用纯 Go 实现的 SQLite 驱动 `modernc.org/sqlite`：
- **优势**：不依赖 CGO，无需本地 C 编译器，能够完全保留当前项目的跨平台无依赖交叉编译能力（Linux amd64/arm64, Windows, macOS）。
- **文件位置**：默认位于 `data/mywebscrcpy.db`，跟随应用数据目录。

### 2.2 数据模型 (Table: `devices`)
```sql
CREATE TABLE IF NOT EXISTS devices (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    serial              TEXT NOT NULL UNIQUE,          -- IP:Port 或 设备序列号
    source              TEXT NOT NULL DEFAULT 'manual',-- 'manual'(手动添加) / 'auto'(自动发现)
    alias               TEXT NOT NULL DEFAULT '',      -- 用户别名/备注
    state               TEXT NOT NULL DEFAULT 'offline',-- ADB 状态: 'device', 'offline', 'connecting' 等
    product             TEXT NOT NULL DEFAULT '',
    model               TEXT NOT NULL DEFAULT '',
    device_name         TEXT NOT NULL DEFAULT '',
    transport_id        TEXT NOT NULL DEFAULT '',
    auto_reconnect      INTEGER NOT NULL DEFAULT 1,    -- 是否断线自动重试连接 (1/0)
    last_connected_at   INTEGER NOT NULL DEFAULT 0,    -- 最近一次连通成功的时间戳 (Unix ms)
    created_at          INTEGER NOT NULL,              -- 创建时间戳 (Unix ms)
    updated_at          INTEGER NOT NULL               -- 更新时间戳 (Unix ms)
);

CREATE INDEX IF NOT EXISTS idx_devices_source ON devices(source);
CREATE INDEX IF NOT EXISTS idx_devices_state ON devices(state);
```

### 2.3 业务流与重连守护
1. **聚合查询**：
   - 定时执行 `adb devices -l` 获取物理在线设备。
   - 读取 SQLite 中已保存的手动设备列表。
   - 聚合状态：若手动设备在 adb devices 中存在且状态为 `device`，更新其状态为 `device` 并更新元数据（product/model）与最后连通时间；若不存在则标记为 `offline`。
2. **后台重连守护**：
   - 守护协程定期（如每 10-15s）检查所有 `source == 'manual'` 且 `auto_reconnect == 1` 但当前为 `offline` 的设备。
   - 异步调用 `adb connect <serial>`。
3. **API 规范**：
   - `GET /api/devices`：返回融合后的设备列表。
   - `POST /api/devices`：添加手动设备并立即尝试 `adb connect`，写入数据库。
   - `DELETE /api/devices/{serial}`：删除手动设备（仅允许删除 manual 来源）。
   - `POST /api/devices/{serial}/connect`：手动触发一次连接。
   - `PATCH /api/devices/{serial}`：更新设备别名或自动重连设置。
