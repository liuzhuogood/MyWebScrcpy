class Device {
  const Device({
    required this.serial,
    required this.state,
    this.alias,
    this.model,
    this.product,
  });

  final String serial;
  final String state;
  final String? alias;
  final String? model;
  final String? product;

  String get title => alias?.trim().isNotEmpty == true
      ? alias!.trim()
      : (model?.trim().isNotEmpty == true ? model!.trim() : serial);

  factory Device.fromJson(Map<String, dynamic> json) {
    return Device(
      serial: json['serial'] as String,
      state: json['state'] as String,
      alias: json['alias'] as String?,
      model: json['model'] as String?,
      product: json['product'] as String?,
    );
  }
}
