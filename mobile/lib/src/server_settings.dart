import 'package:shared_preferences/shared_preferences.dart';

class ServerSettings {
  static const _key = 'server_base_url';

  static Future<String?> read() async {
    final preferences = await SharedPreferences.getInstance();
    return preferences.getString(_key);
  }

  static Future<void> write(String value) async {
    final preferences = await SharedPreferences.getInstance();
    await preferences.setString(_key, value);
  }
}
