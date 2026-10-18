# MyWebScrcpy Mobile

Android and iOS client for connecting to an existing MyWebScrcpy server. The app provides a small native server/device chooser and opens the existing `/player?serial=...` page in a WebView. Video decoding, WebSocket transport, and device control remain in the existing web client and Go server.

## Requirements

- Flutter 3.29 or newer and Dart 3.7 or newer
- Android SDK for Android builds
- Full Xcode installation for iOS builds (Command Line Tools alone are insufficient)

## Run

```sh
cd mobile
flutter pub get
flutter run
```

Enter the HTTPS base URL of a reachable MyWebScrcpy server, including its port when needed. The app requests `GET /api/devices`, then opens an online device at `/player?serial=<serial>`.

The configured server address is stored locally with SharedPreferences. Use HTTPS because the existing web player uses WebCodecs, which is available only in secure contexts (HTTPS or localhost). The app accepts certificate errors only for the configured server host and port, including self-signed certificates. The server currently exposes its device list without an app-specific login flow, so deploy it only on a network boundary you trust.

The WebView relies on the target platform's WebCodecs implementation for the current H.264/H.265/AV1 stream. Validate supported codecs on the actual Android System WebView and iOS WKWebView versions in the supported device matrix. This initial project does not add a native decoder or change the server protocol.
