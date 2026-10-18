import 'package:flutter/material.dart';

import 'device.dart';
import 'player_page.dart';
import 'server_client.dart';
import 'server_settings.dart';

class DeviceListPage extends StatefulWidget {
  const DeviceListPage({super.key});

  @override
  State<DeviceListPage> createState() => _DeviceListPageState();
}

class _DeviceListPageState extends State<DeviceListPage> {
  final _urlController = TextEditingController();
  List<Device> _devices = const [];
  bool _loading = false;
  bool _configured = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    _restoreServer();
  }

  @override
  void dispose() {
    _urlController.dispose();
    super.dispose();
  }

  Future<void> _restoreServer() async {
    final saved = await ServerSettings.read();
    if (!mounted) return;
    if (saved != null && saved.isNotEmpty) {
      _urlController.text = saved;
      final uri = _parseBaseUri();
      if (uri == null || !_isSecureOrigin(uri)) {
        setState(() => _error = '投屏客户端需要 HTTPS 服务地址');
        return;
      }
      _configured = true;
      await _loadDevices();
    } else {
      setState(() {});
    }
  }

  Uri? _parseBaseUri() {
    final text = _urlController.text.trim();
    final uri = Uri.tryParse(text.contains('://') ? text : 'https://$text');
    if (uri == null || !uri.hasAuthority || uri.host.isEmpty) return null;
    if (uri.scheme != 'http' && uri.scheme != 'https') return null;
    if (uri.userInfo.isNotEmpty || uri.query.isNotEmpty || uri.fragment.isNotEmpty) {
      return null;
    }
    return uri.replace(path: '', query: null, fragment: null);
  }

  bool _isSecureOrigin(Uri uri) => uri.scheme == 'https';

  Future<void> _connect() async {
    final uri = _parseBaseUri();
    if (uri == null) {
      setState(() => _error = '请输入有效的 HTTPS 服务地址');
      return;
    }
    if (!_isSecureOrigin(uri)) {
      setState(() => _error = '投屏视频解码需要 HTTPS 服务地址');
      return;
    }
    _urlController.text = uri.toString().replaceFirst(RegExp(r'/$'), '');
    await ServerSettings.write(_urlController.text);
    if (!mounted) return;
    _configured = true;
    await _loadDevices();
  }

  Future<void> _loadDevices() async {
    final uri = _parseBaseUri();
    if (uri == null) return;
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final devices = await ServerClient(uri).fetchDevices();
      if (!mounted) return;
      setState(() => _devices = devices);
    } catch (error) {
      if (!mounted) return;
      setState(() => _error = error.toString());
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  Future<void> _editServer() async {
    setState(() {
      _configured = false;
      _devices = const [];
      _error = null;
    });
  }

  Future<void> _openPlayer(Device device) async {
    final uri = _parseBaseUri();
    if (uri == null) return;
    if (!_isSecureOrigin(uri)) {
      await showDialog<void>(
        context: context,
        builder: (context) => AlertDialog(
          title: const Text('需要安全连接'),
          content: const Text('投屏视频解码需要安全网页环境。请为服务端配置受信任的 HTTPS 证书后再连接。'),
          actions: [
            TextButton(onPressed: () => Navigator.pop(context), child: const Text('知道了')),
          ],
        ),
      );
      return;
    }
    if (!mounted) return;
    await Navigator.of(context).push(MaterialPageRoute<void>(
      builder: (_) => PlayerPage(baseUri: uri, device: device),
    ));
  }

  @override
  Widget build(BuildContext context) {
    final onlineDevices = _devices.where((device) => device.state == 'device').toList();
    return Scaffold(
      appBar: AppBar(
        title: const Text('MyWebScrcpy'),
        actions: [
          if (_configured)
            IconButton(
              tooltip: '切换服务端',
              onPressed: _editServer,
              icon: const Icon(Icons.dns_outlined),
            ),
          if (_configured)
            IconButton(
              tooltip: '刷新设备',
              onPressed: _loading ? null : _loadDevices,
              icon: const Icon(Icons.refresh),
            ),
        ],
      ),
      body: SafeArea(
        child: !_configured ? _buildServerForm() : _buildDevices(onlineDevices),
      ),
    );
  }

  Widget _buildServerForm() {
    return Center(
      child: SingleChildScrollView(
        padding: const EdgeInsets.all(24),
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 480),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              const Icon(Icons.screen_share_outlined, size: 56),
              const SizedBox(height: 20),
              Text('连接 MyWebScrcpy', style: Theme.of(context).textTheme.headlineSmall),
              const SizedBox(height: 8),
              const Text('输入运行 MyWebScrcpy 的服务端地址'),
              const SizedBox(height: 20),
              TextField(
                controller: _urlController,
                keyboardType: TextInputType.url,
                autocorrect: false,
                textInputAction: TextInputAction.go,
                onSubmitted: (_) => _connect(),
                decoration: const InputDecoration(
                  labelText: '服务端地址',
                  hintText: 'https://my-server:8080',
                  border: OutlineInputBorder(),
                ),
              ),
              if (_error != null) ...[
                const SizedBox(height: 12),
                Text(_error!, style: TextStyle(color: Theme.of(context).colorScheme.error)),
              ],
              const SizedBox(height: 16),
              FilledButton(
                onPressed: _loading ? null : _connect,
                child: _loading
                    ? const SizedBox.square(dimension: 18, child: CircularProgressIndicator(strokeWidth: 2))
                    : const Text('连接'),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildDevices(List<Device> devices) {
    if (_loading && _devices.isEmpty) return const Center(child: CircularProgressIndicator());
    if (_error != null) {
      return Center(
        child: Padding(
          padding: const EdgeInsets.all(24),
          child: Column(mainAxisSize: MainAxisSize.min, children: [
            const Icon(Icons.cloud_off_outlined, size: 44),
            const SizedBox(height: 12),
            Text(_error!, textAlign: TextAlign.center),
            const SizedBox(height: 16),
            FilledButton(onPressed: _loadDevices, child: const Text('重试')),
          ]),
        ),
      );
    }
    if (devices.isEmpty) {
      return RefreshIndicator(
        onRefresh: _loadDevices,
        child: ListView(children: const [
          SizedBox(height: 150),
          Icon(Icons.phone_android, size: 48),
          SizedBox(height: 16),
          Center(child: Text('没有在线设备')),
        ]),
      );
    }
    return RefreshIndicator(
      onRefresh: _loadDevices,
      child: ListView.separated(
        padding: const EdgeInsets.all(16),
        itemCount: devices.length,
        separatorBuilder: (_, __) => const SizedBox(height: 10),
        itemBuilder: (context, index) {
          final device = devices[index];
          return Card(
            clipBehavior: Clip.antiAlias,
            child: ListTile(
              contentPadding: const EdgeInsets.symmetric(horizontal: 18, vertical: 8),
              leading: const CircleAvatar(child: Icon(Icons.phone_android)),
              title: Text(device.title, maxLines: 1, overflow: TextOverflow.ellipsis),
              subtitle: Text(device.model?.isNotEmpty == true ? '${device.model} · ${device.serial}' : device.serial),
              trailing: const Icon(Icons.chevron_right),
              onTap: () => _openPlayer(device),
            ),
          );
        },
      ),
    );
  }
}
