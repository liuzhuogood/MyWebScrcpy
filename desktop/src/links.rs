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
) -> Result<(), String> {
    let state = app.state::<State>();
    let safe_serial = serial.replace(|c: char| !c.is_alphanumeric(), "_");
    let label = format!("player-{}", safe_serial);

    if let Some(existing) = app.get_webview_window(&label) {
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
    let h = height.unwrap_or(780.0);

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
        .initialization_script(&token_script)
        .initialization_script(CLICK_SCRIPT);

    builder.build().map_err(|e| e.to_string())?;
    Ok(())
}

pub const CLICK_SCRIPT: &str = include_str!("links.js");
