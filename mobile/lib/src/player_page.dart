import 'dart:async';

import 'package:flutter/material.dart';
import 'package:webview_flutter_android/webview_flutter_android.dart';
import 'package:webview_flutter/webview_flutter.dart';
import 'package:webview_flutter_wkwebview/webview_flutter_wkwebview.dart';

import 'device.dart';

class PlayerPage extends StatefulWidget {
  const PlayerPage({super.key, required this.baseUri, required this.device});

  final Uri baseUri;
  final Device device;

  @override
  State<PlayerPage> createState() => _PlayerPageState();
}

class _PlayerPageState extends State<PlayerPage> {
  late final WebViewController _controller;
  bool _loading = true;
  String? _error;

  @override
  void initState() {
    super.initState();
    final playerUri = widget.baseUri.replace(
      path: '/player',
      queryParameters: {'serial': widget.device.serial},
    );
    _controller = WebViewController()
      ..setJavaScriptMode(JavaScriptMode.unrestricted)
      ..setBackgroundColor(const Color(0xFF08090B))
      ..setNavigationDelegate(NavigationDelegate(
        onPageStarted: (_) {
          if (mounted) setState(() { _loading = true; _error = null; });
        },
        onPageFinished: (_) {
          if (mounted) setState(() => _loading = false);
        },
        onWebResourceError: (error) {
          if (mounted && error.isForMainFrame == true) {
            setState(() { _loading = false; _error = error.description; });
          }
        },
        onSslAuthError: (error) {
          final sameServer = switch (error.platform) {
            AndroidSslAuthError(:final url) => _matchesServer(Uri.tryParse(url)),
            WebKitSslAuthError(:final host, :final port) =>
              host == widget.baseUri.host && port == widget.baseUri.port,
            _ => false,
          };
          unawaited(sameServer ? error.proceed() : error.cancel());
        },
        onNavigationRequest: (request) {
          final destination = Uri.tryParse(request.url);
          if (destination == null ||
              destination.scheme != widget.baseUri.scheme ||
              destination.host != widget.baseUri.host ||
              destination.port != widget.baseUri.port) {
            return NavigationDecision.prevent;
          }
          return NavigationDecision.navigate;
        },
      ))
      ..loadRequest(playerUri);
  }

  bool _matchesServer(Uri? uri) => uri != null &&
      uri.host == widget.baseUri.host &&
      uri.port == widget.baseUri.port;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: const Color(0xFF08090B),
      appBar: AppBar(
        title: Text(widget.device.title, maxLines: 1, overflow: TextOverflow.ellipsis),
        backgroundColor: const Color(0xFF08090B),
        foregroundColor: Colors.white,
        actions: [
          IconButton(
            tooltip: '重新加载',
            onPressed: () => _controller.reload(),
            icon: const Icon(Icons.refresh),
          ),
        ],
      ),
      body: SafeArea(
        top: false,
        child: Stack(children: [
          WebViewWidget(controller: _controller),
          if (_loading) const LinearProgressIndicator(minHeight: 2),
          if (_error != null)
            Positioned.fill(
              child: ColoredBox(
                color: const Color(0xFF08090B),
                child: Center(
                  child: Padding(
                    padding: const EdgeInsets.all(24),
                    child: Column(mainAxisSize: MainAxisSize.min, children: [
                      const Icon(Icons.wifi_off, color: Colors.white70, size: 40),
                      const SizedBox(height: 12),
                      Text(_error!, textAlign: TextAlign.center, style: const TextStyle(color: Colors.white)),
                      const SizedBox(height: 16),
                      FilledButton(onPressed: _controller.reload, child: const Text('重试')),
                    ]),
                  ),
                ),
              ),
            ),
        ]),
      ),
    );
  }
}
