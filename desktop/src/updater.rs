use crate::{backend, update_backup, State};
use serde::Serialize;
use std::sync::atomic::{AtomicBool, Ordering};
use tauri::{ipc::Channel, AppHandle, Manager};
use tauri_plugin_updater::UpdaterExt;

static UPGRADING: AtomicBool = AtomicBool::new(false);
struct UpgradeGuard;
impl Drop for UpgradeGuard {
    fn drop(&mut self) {
        UPGRADING.store(false, Ordering::Release);
    }
}

#[derive(Serialize)]
pub struct CheckResult {
    current_version: String,
    latest_version: String,
    has_update: bool,
    can_upgrade: bool,
    runtime_mode: &'static str,
    update_method: &'static str,
    update_reason: String,
    release_notes: String,
    published_at: String,
    asset_name: String,
    asset_size: u64,
    download_url: String,
}
#[derive(Clone, Serialize)]
pub struct Progress {
    state: &'static str,
    percent: f64,
    downloaded: u64,
    total: u64,
    speed: &'static str,
}
fn builder(app: &AppHandle) -> Result<tauri_plugin_updater::Updater, String> {
    let key = option_env!("MYWEBSCRCPY_UPDATER_PUBLIC_KEY")
        .filter(|value| !value.is_empty())
        .ok_or("此安装方式请下载完整安装包更新")?;
    #[cfg(target_os = "linux")]
    if std::env::var_os("APPIMAGE").is_none() {
        return Err("此安装方式请通过完整安装包更新".into());
    }
    let endpoint = tauri::Url::parse(
        "https://github.com/liuzhuogood/MyWebScrcpy/releases/latest/download/desktop-latest.json",
    )
    .map_err(|e| e.to_string())?;
    app.updater_builder()
        .pubkey(key)
 .timeout(std::time::Duration::from_secs(30))
        .endpoints(vec![endpoint])
        .map_err(|e| e.to_string())?
        .build()
        .map_err(|e| e.to_string())
}
#[tauri::command]
pub async fn desktop_check_update(app: AppHandle) -> Result<CheckResult, String> {
    let current = app.package_info().version.to_string();
    let mut result = CheckResult {
        current_version: format!("v{current}"),
        latest_version: format!("v{current}"),
        has_update: false,
        can_upgrade: false,
        runtime_mode: "desktop",
        update_method: "app",
        update_reason: String::new(),
        release_notes: String::new(),
        published_at: String::new(),
        asset_name: String::new(),
        asset_size: 0,
        download_url: String::new(),
    };
    let updater = match builder(&app) {
        Ok(value) => value,
        Err(reason) => {
            result.update_reason = reason;
            return Ok(result);
        }
    };
    let checked=match updater.check().await {
        Ok(value)=>value,
        Err(tauri_plugin_updater::Error::ReleaseNotFound)=>{result.update_reason="暂未获取到可用的桌面更新包，请稍后重试".into();return Ok(result)},
        Err(error)=>return Err(error.to_string()),
    };
    if let Some(update) = checked {
        result.latest_version = format!("v{}", update.version.trim_start_matches('v'));
        result.has_update = true;
        result.can_upgrade = true;
        result.release_notes = update.body.unwrap_or_default();
        result.published_at = update.date.map(|date| date.to_string()).unwrap_or_default();
    }
    Ok(result)
}
#[tauri::command]
pub async fn desktop_upgrade(app: AppHandle, on_progress: Channel<Progress>) -> Result<(), String> {
    if UPGRADING.swap(true, Ordering::AcqRel) {
        return Err("更新正在进行".into());
    }
    let _guard = UpgradeGuard;
    let update = builder(&app)?
        .check()
        .await
        .map_err(|e| e.to_string())?
        .ok_or("没有可安装的新版本")?;
    let mut downloaded = 0;
    let bytes = update
        .download(
            |chunk, total| {
                downloaded += chunk as u64;
                let total = total.unwrap_or(0);
                let percent = if total > 0 {
                    (downloaded as f64 / total as f64 * 100.0).min(100.0)
                } else {
                    0.0
                };
                let _ = on_progress.send(Progress {
                    state: "downloading",
                    percent,
                    downloaded,
                    total,
                    speed: "",
                });
            },
            || {},
        )
        .await
        .map_err(|e| e.to_string())?;
    let ready = {
        let state = app.state::<State>();
        let backend = state.backend.lock().map_err(|e| e.to_string())?;
        backend.as_ref().ok_or("服务未运行")?.ready.clone()
    };
    backend::request(&ready, "POST", "/api/desktop/update-prepare")?;
    let _ = on_progress.send(Progress {
        state: "applying",
        percent: 100.0,
        downloaded: bytes.len() as u64,
        total: bytes.len() as u64,
        speed: "",
    });
    // Stop the old child before replacing any part of the complete application.
    {
        let state = app.state::<State>();
        state.backend.lock().map_err(|e| e.to_string())?.take();
    }
    let backup = match update_backup::Backup::create(&app) {
        Ok(value) => value,
        Err(error) => {
            eprintln!("[Updater] 备份失败: {error}");
            app.restart();
        }
    };
    if let Err(error) = backup.arm(&format!("v{}", update.version.trim_start_matches('v'))) {
        eprintln!("[Updater] 更新监护进程启动失败: {error}");
        app.restart();
    }
    if let Err(error) = update.install(bytes) {
        backup.cancel();
        eprintln!("[Updater] 完整 App 安装失败: {error}");
        app.restart();
    }
    if let Err(error) = backup.installed() {
        eprintln!("[Updater] 安装提交标记写入失败: {error}");
        app.restart();
    }
    // Relaunch creates a fresh backend, token and origin through the normal setup path.
    app.restart();
}

pub fn prepare_startup(data: &std::path::Path) -> Result<(), String> {
    if let Some(receipt) = update_backup::pending_receipt(data)? {
        std::fs::write(
            format!("{}.pids", receipt.display()),
            serde_json::to_vec(&serde_json::json!({"host":std::process::id(),"backend":0}))
                .map_err(|e| e.to_string())?,
        )
        .map_err(|e| e.to_string())?;
    }
    Ok(())
}
pub fn record_backend_pid(data: &std::path::Path, pid: u32) -> Result<(), String> {
    if let Some(receipt) = update_backup::pending_receipt(data)? {
        std::fs::write(
            format!("{}.pids", receipt.display()),
            serde_json::to_vec(&serde_json::json!({"host":std::process::id(),"backend":pid}))
                .map_err(|e| e.to_string())?,
        )
        .map_err(|e| e.to_string())?;
    }
    Ok(())
}
