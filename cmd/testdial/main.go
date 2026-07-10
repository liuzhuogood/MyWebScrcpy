// testdial: 独立测试 scrcpy server 启动 + 连接握手
package main

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"time"
)

func main() {
	serial := "10.0.0.104:5555"
	adb := "/Users/liuzhuo/Library/Android/sdk/platform-tools/adb"
	scid := "8b9c0d1e"
	socketName := "scrcpy_" + scid

	// 0. push jar
	fmt.Println("=== 0. push jar ===")
	out, err := exec.Command(adb, "-s", serial, "push",
		"/opt/homebrew/Cellar/scrcpy/4.0/bin/scrcpy-server",
		"/data/local/tmp/scrcpy-server.jar").CombinedOutput()
	fmt.Printf("push: err=%v out=%s\n", err, string(out))

	// 1. forward
	fmt.Println("=== 1. forward ===")
	local := "tcp:27183"
	remote := "localabstract:" + socketName
	out, err = exec.Command(adb, "-s", serial, "forward", local, remote).CombinedOutput()
	fmt.Printf("forward: %s err=%v out=%s\n", local, err, string(out))

	// 2. 启动 server (用 exec.Command, 异步)
	fmt.Println("=== 2. start server ===")
	cmd := exec.Command(adb, "-s", serial, "shell",
		"CLASSPATH=/data/local/tmp/scrcpy-server.jar app_process / com.genymobile.scrcpy.Server 4.0 "+
			"scid="+scid+" log_level=debug audio=false tunnel_forward=true")
	// 把 stdout/stderr 都打印出来
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		fmt.Println("start error:", err)
		return
	}
	fmt.Println("server PID:", cmd.Process.Pid)

	// 3. 等待并 dial (重试)
	fmt.Println("=== 3. dial ===")
	var videoConn net.Conn
	for i := 0; i < 20; i++ {
		time.Sleep(300 * time.Millisecond)
		videoConn, err = net.DialTimeout("tcp", "127.0.0.1:27183", 2*time.Second)
		if err == nil {
			fmt.Printf("dial %d: 连接成功 (remote=%v)\n", i+1, videoConn.RemoteAddr())
			break
		}
		fmt.Printf("dial %d: %v\n", i+1, err)
	}
	if videoConn == nil {
		fmt.Println("无法连接，检查 server 输出")
		cmd.Process.Kill()
		return
	}
	defer videoConn.Close()

	// 4. 读 dummy byte
	fmt.Println("=== 4. 读 dummy byte ===")
	dummy := make([]byte, 1)
	videoConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	n, err := io.ReadFull(videoConn, dummy)
	fmt.Printf("dummy: n=%d byte=0x%02x err=%v\n", n, dummy[0], err)
	if err != nil {
		fmt.Println("读 dummy 失败，server 可能崩溃")
		cmd.Process.Kill()
		return
	}
	videoConn.SetReadDeadline(time.Time{})

	// 5. 读 codec id
	fmt.Println("=== 5. 读 codec id ===")
	var codecBuf [4]byte
	n, err = io.ReadFull(videoConn, codecBuf[:])
	codecID := binary.BigEndian.Uint32(codecBuf[:])
	fmt.Printf("codec: n=%d id=0x%x (%q) err=%v\n", n, codecID, string(codecBuf[:]), err)

	// 6. 读 session packet (12 字节)
	fmt.Println("=== 6. 读 session packet ===")
	hdr := make([]byte, 12)
	n, err = io.ReadFull(videoConn, hdr)
	ptsFlags := binary.BigEndian.Uint64(hdr[0:8])
	w := binary.BigEndian.Uint32(hdr[4:8])
	h := binary.BigEndian.Uint32(hdr[8:12])
	fmt.Printf("session: n=%d flags=0x%x width=%d height=%d err=%v\n", n, ptsFlags, w, h, err)

	// 7. 读第一帧
	fmt.Println("=== 7. 读第一帧 ===")
	n, err = io.ReadFull(videoConn, hdr)
	ptsFlags = binary.BigEndian.Uint64(hdr[0:8])
	pktLen := binary.BigEndian.Uint32(hdr[8:12])
	fmt.Printf("frame header: flags=0x%x len=%d err=%v\n", ptsFlags, pktLen, err)
	if pktLen > 0 && pktLen < 1000000 {
		payload := make([]byte, pktLen)
		n, err = io.ReadFull(videoConn, payload)
		fmt.Printf("frame payload: %d bytes, first 16: %x err=%v\n", n, payload[:16], err)
	}

	fmt.Println("=== 成功! server 工作正常 ===")
	cmd.Process.Kill()
}
