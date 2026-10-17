# 前置条件

1. 从运行中服务读取 OpenAPI，核对设备、页面信息、XML、原分辨率截图、OCR上传、录像与任务上报接口，按当前字段读取。
2. 设备必须由用户指定且处于 device 状态。真机验证目标曾为 10.0.0.30:5555，脚本不内置此目标。
3. 当前源码与运行中Schema均无独立解锁能力；使用已确认的公开SDK ADB命令 input keyevent KEYCODE_WAKEUP、wm dismiss-keyguard 唤醒并请求解除锁屏，以实际前台页面验证。不输入密码，不调用未定义接口或SDK方法。
4. SDK 未提供应用启动能力，先核对 com.kuaishou.nebula 已安装，再通过 SDK ADB monkey 单次启动，以实际前台页面核验结果。
5. 业务前开始本次录像；遇到已有录像冲突停止，不抢停用户录像。退出时只停止自己的录像，下载并验证可解码。
6. 通过 my-scrcpy-use/scripts/run-with-env.py 注册环境；所有证据在本次执行目录。服务与OCR须已可用，不自动重启。
