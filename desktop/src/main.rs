#![cfg_attr(not(debug_assertions), windows_subsystem = "windows")]

mod backend;
#[cfg(any(test, all(windows, not(debug_assertions))))]
mod embedded_service;
mod links;
mod main_window;
mod menu;
mod player_windows;
mod update_backup;
mod updater;
#[cfg(feature = "native-verify")]
mod verify;
#[cfg(feature = "native-verify")]
mod verify_players;

use std::{
    path::PathBuf,
    sync::{
        atomic::{AtomicBool, Ordering},
        Mutex,
    },
};
use tauri::{Manager, RunEvent, WebviewUrl, WebviewWindowBuilder, WindowEvent};

pub struct State {
    backend: Mutex<Option<backend::Backend>>,
    url: String,
    tray_available: AtomicBool,
}

fn main() {
    #[cfg(target_os = "macos")]
    backend::fix_macos_env();

    let args: Vec<String> = std::env::args().skip(1).collect();
    #[allow(unused_mut)]
    let mut service_args = args.clone();
    #[cfg(target_os = "linux")]
    if args.is_empty() {
        if let Ok(path) = backend::executable() {
            if let Ok(output) = backend::command(&path).arg("--print-mode").output() {
                if output.status.success() && output.stdout == b"service\n" {
                    service_args.push("--no-gui".into());
                }
            }
        }
    }
    if service_args.iter().any(|arg| arg == "--no-gui") {
        // This works before creating a window, but Linux callers without GTK
        // must invoke the companion Go launcher directly.
        let code = match backend::executable().and_then(|path| {
            backend::command(&path)
                .args(&service_args)
                .status()
                .map_err(|e| e.to_string())
        }) {
            Ok(status) => status.code().unwrap_or(1),
            Err(error) => {
                eprintln!("[Desktop] {error}");
                1
            }
        };
        std::process::exit(code);
    }
    let launcher_ready = match args.as_slice() {
        [] => None,
        [flag] if flag == "--gui" => None,
        [flag, value] if flag == "--launcher-ready" => Some(PathBuf::from(value)),
        _ => {
            eprintln!("[Desktop] 不支持的启动参数: {args:?}");
            std::process::exit(2);
        }
    };
    let builder = tauri::Builder::default()
        .manage(player_windows::PlayerWindows::default())
        .manage(main_window::MainWindowManager::default())
        .plugin(tauri_plugin_single_instance::init(|app, args, _| {
            menu::show(app);
            if let Some(index) = args.iter().position(|value| value == "--launcher-ready") {
                if let Some(path) = args.get(index + 1) { let _ = backend::mark_launcher_ready(Some(std::path::Path::new(path))); }
            }
        }))
        .plugin(tauri_plugin_opener::init())
 .plugin(tauri_plugin_updater::Builder::new().build())
        .invoke_handler(tauri::generate_handler![
            links::open_browser_link,
            links::open_device_window,
            links::set_window_always_on_top,
            links::is_window_always_on_top,
            player_windows::list_player_windows,
            player_windows::set_player_opacity,
            player_windows::set_player_click_through,
            player_windows::set_player_borderless,
            player_windows::set_player_toolbar_visible,
            player_windows::restore_player_window,
            main_window::get_main_window_state,
            main_window::set_main_window_opacity,
            main_window::set_main_window_click_through,
            main_window::set_main_window_borderless,
            main_window::set_main_window_always_on_top,
            main_window::restore_main_window,
            updater::desktop_check_update,
            updater::desktop_upgrade
        ])
        .setup(move |app| {
            let data = app.path().app_data_dir()?;
 updater::prepare_startup(&data).map_err(std::io::Error::other)?;
            let backend = backend::Backend::start(&data).map_err(std::io::Error::other)?;
            let url = backend.ready.url.clone();
            app.add_capability(
                tauri::ipc::CapabilityBuilder::new("browser-links")
                    .window("main")
                    .window("player-*")
                    .local(false)
                    .remote(format!("{url}/*"))
                    .permission("allow-open-browser-link")
                    .permission("allow-open-device-window")
                    .permission("allow-set-window-always-on-top")
                    .permission("allow-is-window-always-on-top")
                    .permission("allow-list-player-windows")
                    .permission("allow-set-player-opacity")
                    .permission("allow-set-player-click-through")
                    .permission("allow-set-player-borderless")
                    .permission("allow-set-player-toolbar-visible")
                    .permission("core:window:allow-start-dragging")
                    .permission("allow-restore-player-window")
                    .permission("allow-get-main-window-state")
                    .permission("allow-set-main-window-opacity")
                    .permission("allow-set-main-window-click-through")
                    .permission("allow-set-main-window-borderless")
                    .permission("allow-set-main-window-always-on-top")
                    .permission("allow-restore-main-window")
 .permission("allow-desktop-check-update")
 .permission("allow-desktop-upgrade"),
            )?;
 let token=serde_json::to_string(&backend.ready.token)?;
            app.manage(State { backend: Mutex::new(Some(backend)), url: url.clone(), tray_available: AtomicBool::new(false) });
            player_windows::install(app.handle());
            let local_origin = url.parse().map_err(std::io::Error::other)?;
            let navigation_app=app.handle().clone();
 let popup_app=app.handle().clone();
            let popup_origin = url.clone();
            WebviewWindowBuilder::new(app, "main", WebviewUrl::External(local_origin))
                .title("MyWebScrcpy").inner_size(1280.0, 820.0)
                .transparent(true)
                .on_new_window(move |destination,_| {
                    use tauri_plugin_opener::OpenerExt;
                    if destination.as_str().starts_with(&(popup_origin.clone()+"/")) || destination.scheme()=="https" {
                        if let Err(error)=popup_app.opener().open_url(destination.as_str(),None::<&str>) {eprintln!("[Desktop] 打开链接失败: {error}");}
                    }
                    tauri::webview::NewWindowResponse::Deny
                })
                .initialization_script(format!("Object.defineProperty(window,'__MYWEBSCRCPY_DESKTOP_TOKEN__',{{value:{token}}});"))
                .initialization_script(links::CLICK_SCRIPT)
                .on_navigation(move |destination| {
                    if destination.as_str().starts_with(&(url.clone()+"/")) {return true;}
                    let allowed = destination.scheme()=="https" && matches!(destination.host_str(),Some("developer.android.com"|"www.python.org"|"github.com"));
                    if allowed {
                        use tauri_plugin_opener::OpenerExt;
                        if let Err(error)=navigation_app.opener().open_url(destination.as_str(),None::<&str>) {eprintln!("[Desktop] 打开外部链接失败: {error}");}
                    }
                    false
                })
                .build()?;
            #[cfg(feature="native-verify")]
 verify::start(app.handle());
 let available = menu::install(app)?;
 menu::show(app.handle());
            app.state::<State>().tray_available.store(available, Ordering::Relaxed);
            backend::mark_launcher_ready(launcher_ready.as_deref()).map_err(std::io::Error::other)?;
            Ok(())
        })
        .on_window_event(|window, event| {
            if matches!(event, WindowEvent::Destroyed) {
                player_windows::forget(window.app_handle(), window.label());
            }
            if let WindowEvent::CloseRequested { api, .. } = event {
                if window.label() == "main" {
                    let state = window.state::<State>();
                    if state.tray_available.load(Ordering::Relaxed) && window.hide().is_ok() { api.prevent_close(); }
                }
            }
        });
    let app = match builder.build(tauri::generate_context!()) {
        Ok(app) => app,
        Err(error) => {
            eprintln!("[Desktop] 初始化失败: {error}");
            std::process::exit(1);
        }
    };
    app.run(|app, event| {
        if let RunEvent::Exit = event {
            if let Some(state) = app.try_state::<State>() {
                if let Ok(mut backend) = state.backend.lock() {
                    backend.take();
                }
            }
        }
        #[cfg(target_os = "macos")]
        if let RunEvent::Reopen { .. } = event {
            menu::show(app);
        }
    });
}
