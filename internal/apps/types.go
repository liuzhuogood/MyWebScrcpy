package apps

// InstallOptions APK 安装参数选项。
type InstallOptions struct {
	ReplaceExisting  bool `json:"replace_existing"`  // -r: 保留数据覆盖安装
	GrantPermissions bool `json:"grant_permissions"` // -g: 授予所有运行时权限
	AllowDowngrade   bool `json:"allow_downgrade"`   // -d: 允许版本降级
}

// InstallResult 安装执行结果。
type InstallResult struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
	Output  string `json:"output,omitempty"`
}
