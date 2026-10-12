use crate::backend;
use std::{
    fs,
    path::{Path, PathBuf},
    process::Stdio,
    time::{SystemTime, UNIX_EPOCH},
};
use tauri::{AppHandle, Manager};

pub struct Backup {
    pub directory: PathBuf,
    pub data: PathBuf,
    pub old_app: Option<PathBuf>,
    pub app: Option<PathBuf>,
}
impl Backup {
    pub fn create(app: &AppHandle) -> Result<Self, String> {
        let data = app.path().app_data_dir().map_err(|e| e.to_string())?;
        let stamp = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .map_err(|e| e.to_string())?
            .as_millis();
        let directory = data.join("update-backups").join(stamp.to_string());
        fs::create_dir_all(&directory).map_err(|e| e.to_string())?;
 #[cfg(unix)] { use std::os::unix::fs::PermissionsExt; fs::set_permissions(&directory,fs::Permissions::from_mode(0o700)).map_err(|e|e.to_string())?; }
        let status = backend::command(&backend::executable()?)
            .arg("--update-backup")
            .arg(&data)
            .arg(directory.join("runtime"))
            .status()
            .map_err(|e| e.to_string())?;
        if !status.success() {
            return Err("更新前数据备份失败".into());
        }
        let mut result = Self {
            directory,
            data,
            old_app: None,
            app: None,
        };
        fs::copy(backend::executable()?, result.directory.join("watchdog"))
            .map_err(|e| e.to_string())?;
        if cfg!(target_os = "macos") {
            let exe = std::env::current_exe().map_err(|e| e.to_string())?;
            let bundle = exe
                .ancestors()
                .find(|path| path.extension().is_some_and(|ext| ext == "app"))
                .ok_or("此运行方式请使用完整安装包更新")?;
            let backup = result.directory.join("previous.app");
            let status = std::process::Command::new("/usr/bin/ditto")
                .arg(bundle)
                .arg(&backup)
                .status()
                .map_err(|e| e.to_string())?;
            if !status.success() {
                return Err("完整 App 备份失败".into());
            }
            result.old_app = Some(backup);
            result.app = Some(bundle.to_path_buf());
        }
        Ok(result)
    }
    pub fn installed(&self) -> Result<(), String> {
        if self.app.is_some() {
            fs::write(self.directory.join("installed"), b"installed").map_err(|e| e.to_string())?;
        }
        Ok(())
    }
    pub fn cancel(&self) {
        let _ = fs::remove_file(self.data.join("update-pending.json"));
    }
    pub fn arm(&self, version: &str) -> Result<(), String> {
        let Some(app) = &self.app else { return Ok(()) };
        let receipt = self.directory.join("ready.json");
        let pending = self.data.join("update-pending.json");
        let plan = serde_json::json!({"app":app,"backup":self.old_app,"data":self.data,"receipt":receipt,"version":version,"pending":pending});
        let plan_file = self.directory.join("plan.json");
        fs::write(
            &plan_file,
            serde_json::to_vec(&plan).map_err(|e| e.to_string())?,
        )
        .map_err(|e| e.to_string())?;
        let helper = self.directory.join("watchdog");

        let log = fs::OpenOptions::new()
            .create(true)
            .append(true)
            .open(self.directory.join("watchdog.log"))
            .map_err(|e| e.to_string())?;
        fs::write(
            &pending,
            serde_json::to_vec(&plan).map_err(|e| e.to_string())?,
        )
        .map_err(|e| e.to_string())?;
        // Empty startup PIDs distinguish an executable that cannot launch at all.
        fs::write(
            receipt.with_extension("json.pids"),
            b"{\"host\":0,\"backend\":0}",
        )
        .map_err(|e| e.to_string())?;
        if let Err(error) = backend::command(&helper)
            .arg("--watch-app-update")
            .arg(plan_file)
            .stdin(Stdio::null())
            .stdout(Stdio::from(log.try_clone().map_err(|e| e.to_string())?))
            .stderr(Stdio::from(log))
            .spawn()
        {
            let _ = fs::remove_file(pending);
            return Err(error.to_string());
        }
        Ok(())
    }
}
pub fn pending_receipt(data: &Path) -> Result<Option<PathBuf>, String> {
    let file = data.join("update-pending.json");
    if !file.is_file() {
        return Ok(None);
    }
    let value: serde_json::Value =
        serde_json::from_slice(&fs::read(file).map_err(|e| e.to_string())?)
            .map_err(|e| e.to_string())?;
    Ok(Some(PathBuf::from(
        value
            .get("receipt")
            .and_then(|v| v.as_str())
            .ok_or("更新就绪路径缺失")?,
    )))
}
