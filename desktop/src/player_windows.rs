use std::{
    collections::HashMap,
    sync::{
        atomic::{AtomicU64, Ordering},
        Mutex,
    },
};
use tauri::{AppHandle, Manager, WebviewWindow};
use tauri_plugin_global_shortcut::{GlobalShortcutExt, ShortcutState};

#[derive(Clone, serde::Serialize)]
#[serde(rename_all = "camelCase")]
pub struct PlayerWindow {
    pub serial: String,
    pub opacity: u8,
    pub click_through: bool,
    pub revision: u64,
}

#[derive(Default)]
pub struct PlayerWindows {
    windows: Mutex<HashMap<String, PlayerWindow>>,
    shortcut: Mutex<Option<String>>,
    recovery_generation: AtomicU64,
}

#[derive(serde::Serialize)]
#[serde(rename_all = "camelCase")]
pub struct Snapshot {
    windows: Vec<PlayerWindow>,
    recovery_shortcut: Option<String>,
    recovery_generation: u64,
}

fn validate_opacity(opacity: u8, recovery_ready: bool) -> Result<(), String> {
    if opacity > 100 {
        return Err("透明度必须在 0%～100% 之间".into());
    }
    if opacity < 100 && !recovery_ready {
        return Err("恢复快捷键不可用，暂不能降低透明度".into());
    }
    Ok(())
}

fn render_opacity(window: &WebviewWindow, opacity: u8) -> Result<(), String> {
    // Applied by the native recovery path too, even if the Vue component is unmounted.
    window
        .eval(format!(
            "document.documentElement.style.setProperty('--player-opacity', '{}')",
            f64::from(opacity) / 100.0
        ))
        .map_err(|error| error.to_string())
}

pub fn install(app: &AppHandle) {
    if let Err(error) = app.plugin(tauri_plugin_global_shortcut::Builder::new().build()) {
        eprintln!("[Desktop] 全局快捷键不可用，禁用透明度与鼠标穿透: {error}");
        return;
    }
    // The second binding is an explicit alternative when Alt+P is occupied.
    for (binding, display) in [
        (
            "Alt+P",
            if cfg!(target_os = "macos") {
                "Option + P"
            } else {
                "Alt + P"
            },
        ),
        (
            "Control+Alt+P",
            if cfg!(target_os = "macos") {
                "Control + Option + P"
            } else {
                "Ctrl + Alt + P"
            },
        ),
    ] {
        match app.global_shortcut().on_shortcut(binding, |app, _, event| {
            if event.state() == ShortcutState::Pressed {
                app.state::<PlayerWindows>()
                    .recovery_generation
                    .fetch_add(1, Ordering::Relaxed);
                eprintln!("[Desktop] 全局快捷键恢复小窗");
                restore_all(app);
            }
        }) {
            Ok(()) => {
                *app.state::<PlayerWindows>().shortcut.lock().unwrap() = Some(display.into());
                eprintln!("[Desktop] 小窗恢复快捷键已注册: {binding}");
                return;
            }
            Err(error) => eprintln!("[Desktop] 小窗恢复快捷键注册失败 ({binding}): {error}"),
        }
    }
}

pub fn track(app: &AppHandle, label: &str, serial: &str) -> Result<(), String> {
    app.state::<PlayerWindows>()
        .windows
        .lock()
        .map_err(|e| e.to_string())?
        .insert(
            label.into(),
            PlayerWindow {
                serial: serial.into(),
                opacity: 100,
                click_through: false,
                revision: 0,
            },
        );
    Ok(())
}

pub fn forget(app: &AppHandle, label: &str) {
    if let Ok(mut windows) = app.state::<PlayerWindows>().windows.lock() {
        windows.remove(label);
    }
}

#[tauri::command]
pub async fn list_player_windows(app: AppHandle) -> Result<Snapshot, String> {
    let state = app.state::<PlayerWindows>();
    let recovery_shortcut = state.shortcut.lock().map_err(|e| e.to_string())?.clone();
    let windows = state
        .windows
        .lock()
        .map_err(|e| e.to_string())?
        .values()
        .cloned()
        .collect();
    Ok(Snapshot {
        windows,
        recovery_shortcut,
        recovery_generation: state.recovery_generation.load(Ordering::Relaxed),
    })
}

#[tauri::command]
pub async fn set_player_opacity(
    window: WebviewWindow,
    opacity: u8,
    expected_revision: u64,
) -> Result<(), String> {
    let state = window.state::<PlayerWindows>();
    validate_opacity(
        opacity,
        state.shortcut.lock().map_err(|e| e.to_string())?.is_some(),
    )?;
    let mut windows = state.windows.lock().map_err(|e| e.to_string())?;
    let player = windows
        .get_mut(window.label())
        .ok_or("仅设备小窗支持透明度调节")?;
    if player.revision != expected_revision {
        return Err("小窗已恢复，请重新调整".into());
    }
    render_opacity(&window, opacity)?;
    player.opacity = opacity;
    eprintln!("[Desktop] 小窗 {} 透明度: {opacity}%", window.label());
    Ok(())
}

#[tauri::command]
pub async fn set_player_click_through(
    window: WebviewWindow,
    enabled: bool,
    expected_revision: u64,
) -> Result<(), String> {
    let state = window.state::<PlayerWindows>();
    if enabled && state.shortcut.lock().map_err(|e| e.to_string())?.is_none() {
        return Err("恢复快捷键不可用，暂不能开启鼠标穿透".into());
    }
    let mut windows = state.windows.lock().map_err(|e| e.to_string())?;
    let player = windows
        .get_mut(window.label())
        .ok_or("仅设备小窗支持鼠标穿透")?;
    if player.revision != expected_revision {
        return Err("小窗已恢复，请重新调整".into());
    }
    window
        .set_ignore_cursor_events(enabled)
        .map_err(|e| e.to_string())?;
    player.click_through = enabled;
    eprintln!("[Desktop] 小窗 {} 鼠标穿透: {enabled}", window.label());
    Ok(())
}

pub fn restore(app: &AppHandle, label: &str) -> Result<(), String> {
    let state = app.state::<PlayerWindows>();
    let mut windows = state.windows.lock().map_err(|e| e.to_string())?;
    let player = windows.get_mut(label).ok_or("设备小窗已关闭")?;
    let window = app.get_webview_window(label).ok_or("设备小窗已关闭")?;
    // Invalidate slider/enable commands queued before this recovery.
    player.revision += 1;
    // Recover input first; a render failure must never prevent this step.
    window
        .set_ignore_cursor_events(false)
        .map_err(|e| e.to_string())?;
    player.click_through = false;
    render_opacity(&window, 100)?;
    player.opacity = 100;
    eprintln!("[Desktop] 小窗 {label} 已恢复");
    Ok(())
}

pub fn restore_all(app: &AppHandle) {
    let labels: Vec<String> = match app.state::<PlayerWindows>().windows.lock() {
        Ok(windows) => windows.keys().cloned().collect(),
        Err(error) => {
            eprintln!("[Desktop] 读取小窗失败: {error}");
            return;
        }
    };
    for label in labels {
        if let Err(error) = restore(app, &label) {
            eprintln!("[Desktop] 恢复小窗 {label} 失败: {error}");
        }
    }
}

#[tauri::command]
pub async fn restore_player_window(app: AppHandle, serial: String) -> Result<(), String> {
    let label = {
        let state = app.state::<PlayerWindows>();
        let windows = state.windows.lock().map_err(|e| e.to_string())?;
        windows
            .iter()
            .find(|(_, player)| player.serial == serial)
            .map(|(label, _)| label.clone())
            .ok_or("设备小窗已关闭")?
    };
    restore(&app, &label)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn opacity_boundaries_and_recovery_guard() {
        for opacity in [0, 1, 20, 99, 100] {
            assert!(validate_opacity(opacity, true).is_ok());
        }
        assert!(validate_opacity(101, true).is_err());
        assert!(validate_opacity(255, true).is_err());
        assert!(validate_opacity(0, false).is_err());
        assert!(validate_opacity(99, false).is_err());
        assert!(validate_opacity(100, false).is_ok());
    }
}
