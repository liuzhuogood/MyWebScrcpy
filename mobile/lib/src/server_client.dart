import 'dart:convert';
import 'dart:io';

import 'package:http/http.dart' as http;
import 'package:http/io_client.dart';

import 'device.dart';

class ServerClient {
  ServerClient(this.baseUri);

  final Uri baseUri;

  Future<List<Device>> fetchDevices() async {
    final uri = baseUri.replace(path: '/api/devices', query: null, fragment: null);
    final nativeClient = HttpClient()
      ..badCertificateCallback = (_, host, port) =>
          host == baseUri.host && port == baseUri.port;
    final client = IOClient(nativeClient);
    late final http.Response response;
    try {
      response = await client.get(uri).timeout(const Duration(seconds: 12));
    } finally {
      client.close();
    }
    if (response.statusCode != 200) {
      final detail = utf8.decode(response.bodyBytes, allowMalformed: true).trim();
      throw ServerException(
        detail.isEmpty ? '服务端返回 ${response.statusCode}' : detail,
      );
    }

    final decoded = jsonDecode(utf8.decode(response.bodyBytes));
    if (decoded is! List) {
      throw const ServerException('设备列表响应格式不正确');
    }
    return decoded
        .map((entry) => Device.fromJson(entry as Map<String, dynamic>))
        .toList(growable: false);
  }
}

class ServerException implements Exception {
  const ServerException(this.message);
  final String message;

  @override
  String toString() => message;
}
