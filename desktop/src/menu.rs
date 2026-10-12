use crate::State;
use tauri::{
    menu::{Menu, MenuBuilder, PredefinedMenuItem, SubmenuBuilder},
    tray::{MouseButton, MouseButtonState, TrayIconBuilder, TrayIconEvent},
    App, AppHandle, Manager,
};
use tauri_plugin_opener::OpenerExt;

pub fn show(app: &AppHandle) {
    if let Some(window) = app.get_webview_window("main") {
        if let Err(error) = window
            .show()
            .and_then(|_| window.unminimize())
            .and_then(|_| window.set_focus())
        {
            eprintln!("[Desktop] 显示窗口失败: {error}");
        }
    }
}

pub fn action(app: &AppHandle, id: &str) {
    match id {
        "show" => show(app),
        "hide" => {
            if let Some(window) = app.get_webview_window("main") {
                if let Err(error) = window.hide() {
                    eprintln!("[Desktop] 隐藏窗口失败: {error}");
                }
            }
        }
        "quit" => app.exit(0),
        "browser" => {
            let state = app.state::<State>();
            if let Err(error) = app.opener().open_url(&state.url, None::<&str>) {
                eprintln!("[Desktop] 打开浏览器失败: {error}");
            }
        }
        "home" | "dashboard" | "scripts" | "dependencies" => {
            let path = match id {
                "home" => "/",
                "dashboard" => "/dashboard",
                "scripts" => "/scripts",
                _ => "/dependencies",
            };
            let state = app.state::<State>();
            if let Some(window) = app.get_webview_window("main") {
                let url = format!("{}{path}", state.url)
                    .parse()
                    .expect("validated local URL");
                if let Err(error) = window.navigate(url) {
                    eprintln!("[Desktop] 页面导航失败: {error}");
                }
                show(app);
            }
        }
        _ => {}
    }
}

pub fn install(app: &mut App) -> tauri::Result<bool> {
    let application = SubmenuBuilder::new(app, "MyWebScrcpy")
        .item(&PredefinedMenuItem::about(
            app,
            Some("关于 MyWebScrcpy"),
            None,
        )?)
        .separator()
        .text("dependencies", "依赖管理")
        .text("browser", "在浏览器中打开")
        .separator()
        .text("quit", "退出 MyWebScrcpy")
        .build()?;
    let edit = SubmenuBuilder::new(app, "编辑")
        .item(&PredefinedMenuItem::undo(app, Some("撤销"))?)
        .item(&PredefinedMenuItem::redo(app, Some("重做"))?)
        .separator()
        .item(&PredefinedMenuItem::cut(app, Some("剪切"))?)
        .item(&PredefinedMenuItem::copy(app, Some("复制"))?)
        .item(&PredefinedMenuItem::paste(app, Some("粘贴"))?)
        .item(&PredefinedMenuItem::select_all(app, Some("全选"))?)
        .build()?;
    let view = SubmenuBuilder::new(app, "视图")
        .text("home", "设备列表")
        .text("dashboard", "大屏监控")
        .text("scripts", "脚本与任务")
        .separator()
        .text("show", "打开主窗口")
        .build()?;
    let menu = Menu::with_items(app, &[&application, &edit, &view])?;
    app.set_menu(menu)?;
    app.on_menu_event(|app, event| action(app, event.id.as_ref()));
    let tray_menu = MenuBuilder::new(app)
        .text("show", "打开主窗口")
        .text("home", "设备列表")
        .text("dashboard", "大屏监控")
        .text("scripts", "脚本与任务")
        .text("dependencies", "依赖管理")
        .separator()
        .text("browser", "在浏览器中打开")
        .separator()
        .text("quit", "退出 MyWebScrcpy")
        .build()?;
    let mut tray = TrayIconBuilder::with_id("main-tray")
        .tooltip("MyWebScrcpy")
        .menu(&tray_menu)
        .show_menu_on_left_click(true)
        .on_tray_icon_event(|tray, event| {
            if matches!(
                event,
                TrayIconEvent::Click {
                    button: MouseButton::Left,
                    button_state: MouseButtonState::Up,
                    ..
                }
            ) {
                show(tray.app_handle());
            }
        });
    if let Some(icon) = app.default_window_icon() {
        tray = tray.icon(icon.clone());
    }
    match tray.build(app) {
        Ok(_) => Ok(tray_host_available()),
        Err(error) => {
            eprintln!("[Desktop] 系统托盘不可用，关闭窗口将退出: {error}");
            Ok(false)
        }
    }
}

// Linux tray creation can succeed even when the desktop has no tray host.
// Only hide on close when the standard watcher explicitly reports a host.
fn tray_host_available() -> bool {
    #[cfg(not(target_os = "linux"))]
    {
        true
    }
    #[cfg(target_os = "linux")]
    {
        use std::{
            process::{Command, Stdio},
            thread,
            time::{Duration, Instant},
        };
        let mut child = match Command::new("gdbus")
            .args([
                "call",
                "--session",
                "--dest",
                "org.kde.StatusNotifierWatcher",
                "--object-path",
                "/StatusNotifierWatcher",
                "--method",
                "org.freedesktop.DBus.Properties.Get",
                "org.kde.StatusNotifierWatcher",
                "IsStatusNotifierHostRegistered",
            ])
            .stdout(Stdio::piped())
            .stderr(Stdio::null())
            .spawn()
        {
            Ok(child) => child,
            Err(_) => return false,
        };
        let deadline = Instant::now() + Duration::from_millis(800);
        loop {
            match child.try_wait() {
                Ok(Some(status)) => {
                    return status.success()
                        && child
                            .wait_with_output()
                            .map(|output| output.stdout == b"(<true>,)\n")
                            .unwrap_or(false)
                }
                Err(_) => break,
                Ok(None) => {}
            }
            if Instant::now() >= deadline {
                break;
            }
            thread::sleep(Duration::from_millis(20));
        }
        let _ = child.kill();
        let _ = child.wait();
        false
    }
}
