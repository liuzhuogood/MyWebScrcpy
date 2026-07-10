# MyWebScrcpy

Browser-based Android screen mirroring powered by Go + WebCodecs. No client installation needed — just open a browser to mirror and control your Android device.

[中文版](README.md)

## Screenshots

**Device List**

![Device List](screenshots/device-list.png)

**Screen Mirroring & Control**

![Screen Mirroring](screenshots/player.png)

## Features

- Pure browser, no client installation required
- WebCodecs hardware decoding for low latency
- H.264 / H.265 / AV1 codec support
- Mouse control: tap, drag, scroll, right-click for back
- Keyboard input: text injection, shortcut keys
- Touch support (mobile browsers)
- One-click screen rotation
- Fullscreen mode (iOS pseudo-fullscreen supported)
- Screen-off detection
- Auto-reconnect
- Single binary with embedded scrcpy-server and web assets

## How It Works

```
Browser ──WebSocket──▶ Go Server ──ADB Forward──▶ scrcpy-server (device)
  │                       │
  │  H.264 video frames   │  Control messages passthrough
  │  ◀──────────────────  │  ──────────────────────▶
  │                       │
WebCodecs decode        app_process launch
Canvas render           video encode + control inject
```

The Go backend handles:
1. Push embedded scrcpy-server jar to device via ADB
2. Establish ADB forward tunnel
3. Start scrcpy server process on device
4. Bidirectional forwarding of video frames and control messages between browser and device via WebSocket

## Requirements

- Go 1.21+
- ADB (Android Debug Bridge)
- Chrome 94+ (WebCodecs support required)
- Android device with USB debugging enabled or connected via network ADB

## Quick Start

```bash
# Clone
git clone https://github.com/liuzhuogood/MyWebScrcpy.git
cd MyWebScrcpy

# Build
go build -o mywebscrcpy .

# Run
./mywebscrcpy
```

Open `http://localhost:8080` in your browser, click a device to start mirroring.

### Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | HTTP listen port | `8080` |
| `ANDROID_HOME` | ADB path lookup | System PATH |

## Controls

| Action | Description |
|--------|-------------|
| Left click | Tap / drag |
| Right click | Back button |
| Scroll wheel | Scroll page |
| Keyboard | Text input |
| Toolbar | Home, Back, Recents, Power, Rotate, Fullscreen |

## Project Structure

```
MyWebScrcpy/
├── main.go                    # Entry point, HTTP server
├── assets/
│   └── scrcpy-server          # scrcpy server jar (embedded)
├── internal/
│   ├── device/manager.go      # ADB device management
│   ├── scrcpy/
│   │   ├── server.go          # scrcpy server lifecycle
│   │   ├── connection.go      # TCP connection + frame reading
│   │   ├── protocol.go        # scrcpy 4.0 protocol constants
│   │   └── control.go         # Control message packing
│   └── ws/hub.go              # WebSocket management
└── web/
    ├── index.html             # Device list page
    ├── player.html            # Screen mirroring player
    ├── css/style.css
    └── js/
        ├── decoder.js         # WebCodecs H.264 decoder
        └── control.js         # Browser-side control message packing
```

## Tech Stack

- **Backend**: Go + gorilla/websocket
- **Frontend**: Vanilla JS + WebCodecs API + Canvas
- **Protocol**: scrcpy 4.0
- **Video Codec**: H.264 (Baseline)

## License

MIT
