package scrcpy

// scrcpy 4.0 协议常量。所有多字节字段为大端序。
// 来源：scrcpy 源码 control_msg.c / device_msg.c / Streamer.java / VideoCodec.java，
// 已用 develop.md + 客户端 demuxer.c 交叉验证。

const ServerVersion = "4.0" // 启动 server 时第一个参数，设备端严格比对

// 设备端 jar 路径
const DeviceServerPath = "/data/local/tmp/scrcpy-server.jar"

// scrcpy 默认隧道端口
const DefaultTunnelPort = 27183

// 控制消息类型 (client -> device)
const (
	TypeInjectKeyCode        = 0
	TypeInjectText           = 1
	TypeInjectTouchEvent     = 2
	TypeInjectScrollEvent    = 3
	TypeBackOrScreenOn       = 4
	TypeExpandNotification   = 5
	TypeExpandSettings       = 6
	TypeCollapsePanels       = 7
	TypeGetClipboard         = 8
	TypeSetClipboard         = 9
	TypeSetDisplayPower      = 10
	TypeRotateDevice         = 11
)

// 触摸 action (AMOTION_EVENT_ACTION_*)
const (
	ActionDown         = 0
	ActionUp           = 1
	ActionMove         = 2
	ActionScroll       = 8
	ActionButtonPress  = 11
	ActionButtonRelease = 12
)

// 按键 action (AKEY_EVENT_ACTION_*)
const (
	KeyActionDown    = 0
	KeyActionUp      = 1
	KeyActionMultiple = 2
)

// 鼠标按钮位掩码 (AMOTION_EVENT_BUTTON_*)
const (
	ButtonPrimary    = 0x1
	ButtonSecondary  = 0x2
	ButtonTertiary   = 0x4
	ButtonBack       = 0x8
	ButtonForward    = 0x10
)

// 帧头 pts+flags 的位定义 (Streamer.writeFrameMeta)
const (
	FlagSessionPacket = uint64(1) << 63 // 会话包 (旋转/尺寸变化)，无 payload
	FlagConfigPacket  = uint64(1) << 62 // codec config 包 (SPS/PPS)，PTS=0
	FlagKeyFrame      = uint64(1) << 61 // 关键帧
	PTSMask           = uint64(0x1FFFFFFFFFFFFFFF) // 低 61 位
)

// video codec id (4 字节 ASCII，按 u32 BE 读)
const (
	CodecIDH264 = 0x68323634 // "h264"
	CodecIDH265 = 0x68323635 // "h265"
	CodecIDAV1  = 0x00617631 // "av1"
)

// PacketHeaderSize 每个媒体包/会话包的 12 字节头
const PacketHeaderSize = 12

// 触摸消息指针 ID 约定 (负值用 u64 补码表示)
const (
	PointerIDMouse = 0xFFFFFFFFFFFFFFFF // -1 鼠标
	PointerIDFinger = 0xFFFFFFFFFFFFFFFE // -2 通用手指
)
