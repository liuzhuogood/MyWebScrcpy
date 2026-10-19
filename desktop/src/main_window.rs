use serde::{Deserialize, Serialize};
use std::sync::Mutex;
use tauri::{AppHandle, Emitter, Manager, WebviewWindow};

#[derive(Clone, Debug, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct MainWindowState {
    pub always_on_top: bool,
    pub opacity: u8,
    pub click_through: bool,
}

impl Default for MainWindowState {
    fn default() -> Self {
        Self {
            always_on_top: false,
            opacity: 100,
            click_through: false,
        }
    }
}

#[derive(Default)]
pub struct MainWindowManager {
    state: Mutex<MainWindowState>,
}

pub fn render_opacity(window: &WebviewWindow, opacity: u8) -> Result<(), String> {
    let alpha = f64::from(opacity) / 100.0;
    let script = if opacity < 100 {
        format!(
            "document.documentElement.classList.add('desktop-translucent'); document.documentElement.style.setProperty('--window-opacity', '{alpha}');"
        )
    } else {
        "document.documentElement.classList.remove('desktop-translucent'); document.documentElement.style.removeProperty('--window-opacity');".to_string()
    };
    window.eval(&script).map_err(|e| e.to_string())
}

pub fn get_state(app: &AppHandle) -> MainWindowState {
    app.state::<MainWindowManager>()
        .state
        .lock()
        .map(|s| s.clone())
        .unwrap_or_default()
}

fn notify_state(window: &WebviewWindow, state: &MainWindowState) {
    let _ = window.emit("main-window-effects-changed", state);
}

pub fn set_always_on_top_internal(app: &AppHandle, enabled: bool) -> Result<(), String> {
    let window = app
        .get_webview_window("main")
        .ok_or_else(|| "主窗口不存在".to_string())?;
    window.set_always_on_top(enabled).map_err(|e| e.to_string())?;
    let mgr = app.state::<MainWindowManager>();
    let state = {
        let mut s = mgr.state.lock().map_err(|e| e.to_string())?;
        s.always_on_top = enabled;
        s.clone()
    };
    notify_state(&window, &state);
    eprintln!("[Desktop] 主窗口置顶: {enabled}");
    Ok(())
}

pub fn set_opacity_internal(app: &AppHandle, opacity: u8) -> Result<(), String> {
    validate_opacity(opacity)?;
    let window = app
        .get_webview_window("main")
        .ok_or_else(|| "主窗口不存在".to_string())?;
    render_opacity(&window, opacity)?;
    let mgr = app.state::<MainWindowManager>();
    let state = {
        let mut s = mgr.state.lock().map_err(|e| e.to_string())?;
        s.opacity = opacity;
        s.clone()
    };
    notify_state(&window, &state);
    eprintln!("[Desktop] 主窗口透明度: {opacity}%");
    Ok(())
}

pub fn set_click_through_internal(app: &AppHandle, enabled: bool) -> Result<(), String> {
    let window = app
        .get_webview_window("main")
        .ok_or_else(|| "主窗口不存在".to_string())?;
    window
        .set_ignore_cursor_events(enabled)
        .map_err(|e| e.to_string())?;
    let mgr = app.state::<MainWindowManager>();
    let state = {
        let mut s = mgr.state.lock().map_err(|e| e.to_string())?;
        s.click_through = enabled;
        s.clone()
    };
    notify_state(&window, &state);
    eprintln!("[Desktop] 主窗口鼠标穿透: {enabled}");
    Ok(())
}

pub fn restore(app: &AppHandle) -> Result<(), String> {
    let window = match app.get_webview_window("main") {
        Some(w) => w,
        None => return Ok(()),
    };
    let _ = window.set_ignore_cursor_events(false);
    let _ = window.set_always_on_top(false);
    let _ = render_opacity(&window, 100);
    let mgr = app.state::<MainWindowManager>();
    let state = {
        let mut s = mgr.state.lock().map_err(|e| e.to_string())?;
        s.always_on_top = false;
        s.opacity = 100;
        s.click_through = false;
        s.clone()
    };
    notify_state(&window, &state);
    eprintln!("[Desktop] 主窗口状态已恢复");
    Ok(())
}

// 供菜单事件调用的便捷包装
pub fn toggle_always_on_top(app: &AppHandle) {
    let current = get_state(app).always_on_top;
    if let Err(e) = set_always_on_top_internal(app, !current) {
        eprintln!("[Desktop] 切换主窗口置顶失败: {e}");
    }
}

pub fn toggle_click_through(app: &AppHandle) {
    let current = get_state(app).click_through;
    if let Err(e) = set_click_through_internal(app, !current) {
        eprintln!("[Desktop] 切换主窗口鼠标穿透失败: {e}");
    }
}

pub fn set_opacity(app: &AppHandle, opacity: u8) {
    if let Err(e) = set_opacity_internal(app, opacity) {
        eprintln!("[Desktop] 设置主窗口透明度失败: {e}");
    }
}

// Tauri IPC Commands
#[tauri::command]
pub async fn get_main_window_state(app: AppHandle) -> Result<MainWindowState, String> {
    Ok(get_state(&app))
}

#[tauri::command]
pub async fn set_main_window_opacity(app: AppHandle, opacity: u8) -> Result<(), String> {
    set_opacity_internal(&app, opacity)
}

#[tauri::command]
pub async fn set_main_window_click_through(app: AppHandle, enabled: bool) -> Result<(), String> {
    set_click_through_internal(&app, enabled)
}

pub fn validate_opacity(opacity: u8) -> Result<(), String> {
    if opacity > 100 {
        return Err("透明度必须在 0%～100% 之间".into());
    }
    Ok(())
}

#[tauri::command]
pub async fn set_main_window_always_on_top(app: AppHandle, always_on_top: bool) -> Result<(), String> {
    set_always_on_top_internal(&app, always_on_top)
}

#[tauri::command]
pub async fn restore_main_window(app: AppHandle) -> Result<(), String> {
    restore(&app)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn default_state_is_normal() {
        let state = MainWindowState::default();
        assert!(!state.always_on_top);
        assert_eq!(state.opacity, 100);
        assert!(!state.click_through);
    }

    #[test]
    fn test_validate_opacity() {
        assert!(validate_opacity(0).is_ok());
        assert!(validate_opacity(50).is_ok());
        assert!(validate_opacity(100).is_ok());
        assert!(validate_opacity(101).is_err());
        assert!(validate_opacity(255).is_err());
    }
}
