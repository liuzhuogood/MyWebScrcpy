package scrcpy

import "encoding/binary"

// uhid.go — UHID 虚拟 HID 设备控制消息构造。
//
// 字节布局对照 scrcpy 源码 app/src/control_msg.c 的 sc_control_msg_serialize，
// 设备端解析见 server .../control/ControlMessageReader.java：
//
//	UHID_CREATE: id(2) vendorId(2) productId(2) nameLen(1) name(n) descLen(2) reportDesc(m)
//	UHID_INPUT:  id(2) size(2) data(m)
//	UHID_DESTROY: id(2)
//
// 设备端 UhidManager 把 reportDesc 写入 /dev/uhid (UHID_CREATE2)，后续每条
// data 作为一份 HID input report 注入，经 Linux input 子系统进入 Android，
// 事件源是虚拟硬件设备而非 InputManager 注入，因此更难被反自动化检测识别。

// UhidCreate 构造创建虚拟 HID 设备的消息。
// name 为空时设备端默认命名 "scrcpy"。
func UhidCreate(id, vendorID, productID uint16, name string, reportDesc []byte) []byte {
	if len(name) > 127 {
		name = name[:127]
	}
	if len(reportDesc) > 0xFFFF {
		reportDesc = reportDesc[:0xFFFF]
	}
	b := make([]byte, 8+len(name)+2+len(reportDesc))
	b[0] = TypeUhidCreate
	binary.BigEndian.PutUint16(b[1:3], id)
	binary.BigEndian.PutUint16(b[3:5], vendorID)
	binary.BigEndian.PutUint16(b[5:7], productID)
	b[7] = byte(len(name))
	copy(b[8:], name)
	pos := 8 + len(name)
	binary.BigEndian.PutUint16(b[pos:pos+2], uint16(len(reportDesc)))
	copy(b[pos+2:], reportDesc)
	return b
}

// UhidInput 构造一条 HID input report 注入消息，data 为完整 report 内容。
func UhidInput(id uint16, data []byte) []byte {
	if len(data) > 0xFFFF {
		data = data[:0xFFFF]
	}
	b := make([]byte, 5+len(data))
	b[0] = TypeUhidInput
	binary.BigEndian.PutUint16(b[1:3], id)
	binary.BigEndian.PutUint16(b[3:5], uint16(len(data)))
	copy(b[5:], data)
	return b
}

// UhidDestroy 构造销毁虚拟 HID 设备的消息。
func UhidDestroy(id uint16) []byte {
	b := make([]byte, 3)
	b[0] = TypeUhidDestroy
	binary.BigEndian.PutUint16(b[1:3], id)
	return b
}
