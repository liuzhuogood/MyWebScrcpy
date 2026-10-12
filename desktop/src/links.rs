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

pub const CLICK_SCRIPT: &str = include_str!("links.js");
