use crate::State;
use tauri::{AppHandle, Manager};
use tauri_plugin_opener::OpenerExt;

#[tauri::command]
pub fn open_browser_link(app: AppHandle, url: String) -> Result<(), String> {
    let destination = tauri::Url::parse(&url).map_err(|error| error.to_string())?;
    let origin = tauri::Url::parse(&app.state::<State>().url)
        .map_err(|error| error.to_string())?;
    if destination.scheme() != "https" && destination.origin() != origin.origin() {
        return Err("不支持的浏览器链接".into());
    }
    app.opener().open_url(destination.as_str(), None::<&str>).map_err(|error| {
        eprintln!("[Desktop] 打开浏览器链接失败: {error}");
        error.to_string()
    })
}

#[tauri::command]
pub fn open_device_window(
    app: AppHandle,
    serial: String,
    title: Option<String>,
    width: Option<f64>,
    height: Option<f64>,
    borderless: Option<bool>,
) -> Result<(), String> {
    let state = app.state::<State>();
    let safe_serial = serial.replace(|c: char| !c.is_alphanumeric(), "_");
    let label = format!("player-{}", safe_serial);

    if let Some(existing) = app.get_webview_window(&label) {
        crate::player_windows::restore(&app, &label)?;
        let _ = existing.show();
        let _ = existing.unminimize();
        let _ = existing.set_focus();
        return Ok(());
    }

    let mut dest = tauri::Url::parse(&format!("{}/player", state.url))
        .map_err(|e| e.to_string())?;
    dest.query_pairs_mut()
        .append_pair("serial", &serial)
        .append_pair("embedded", "1");

    let win_title = title.unwrap_or_else(|| format!("投屏 - {}", serial));
    let w = width.unwrap_or(380.0);
    let h = height.unwrap_or(836.0);
    let borderless = borderless.unwrap_or(false);

    let token_script = if let Ok(guard) = state.backend.lock() {
        if let Some(b) = guard.as_ref() {
            let t = serde_json::to_string(&b.ready.token).unwrap_or_default();
            format!("Object.defineProperty(window,'__MYWEBSCRCPY_DESKTOP_TOKEN__',{{value:{}}});", t)
        } else {
            String::new()
        }
    } else {
        String::new()
    };

    let builder = tauri::WebviewWindowBuilder::new(&app, &label, tauri::WebviewUrl::External(dest))
        .title(win_title)
        .inner_size(w, h)
        .min_inner_size(240.0, 360.0)
        .transparent(true)
        .decorations(!borderless)
        .on_page_load(|window, payload| {
            if matches!(payload.event(), tauri::webview::PageLoadEvent::Started) {
                if let Err(error) = crate::player_windows::restore_after_navigation(window.app_handle(), window.label()) {
                    eprintln!("[Desktop] 页面加载后恢复小窗失败: {error}");
                }
            }
        })
        .initialization_script(&token_script)
        .initialization_script(CLICK_SCRIPT);

    // Track before loading so the first frontend snapshot already has this window.
    crate::player_windows::track(&app, &label, &serial, borderless)?;
    if let Err(error) = builder.build() {
        crate::player_windows::forget(&app, &label);
        return Err(error.to_string());
    }
    Ok(())
}

#[tauri::command]
pub fn set_window_always_on_top(window: tauri::WebviewWindow, always_on_top: bool) -> Result<(), String> {
    window.set_always_on_top(always_on_top).map_err(|e| e.to_string())
}

#[tauri::command]
pub fn is_window_always_on_top(window: tauri::WebviewWindow) -> Result<bool, String> {
    window.is_always_on_top().map_err(|e| e.to_string())
}

pub const CLICK_SCRIPT: &str = include_str!("links.js");
