// Native verification build only: use real WebViews and the production IPC ACL.
use std::{thread, time::Duration};
use tauri::{AppHandle, Manager};

pub fn start(app: &AppHandle) {
    let app = app.clone();
    thread::spawn(move || {
        thread::sleep(Duration::from_secs(3));
        let main = app.get_webview_window("main").unwrap();
        let script = format!(
            "window.__verifyHotkey = {};\n{}",
            std::env::var_os("MYWEBSCRCPY_VERIFY_HOTKEY").is_some(),
            include_str!("verify_players.js")
        );
        if let Err(error) = main.eval(script) {
            eprintln!("[Desktop Verify] {error}");
            app.exit(1);
            return;
        }
        let report = std::env::var("MYWEBSCRCPY_NATIVE_REPORT").unwrap();
        let mut closed_player = false;
        let mut restored_menu = false;
        for _ in 0..600 {
            for label in ["player-verify_a", "player-verify_b"] {
                if let Some(window) = app.get_webview_window(label) {
                    let _ = window.eval(include_str!("verify_player_worker.js"));
                }
            }
            let progress = std::fs::read(&report)
                .ok()
                .and_then(|bytes| serde_json::from_slice::<serde_json::Value>(&bytes).ok());
            if let Some(value) = &progress {
                if value["player_windows"]["stage"] == "close_a" && !closed_player {
                    closed_player = true;
                    if let Some(window) = app.get_webview_window("player-verify_a") {
                        let _ = window.close();
                    }
                }
                if value["player_windows"]["stage"] == "restore_menu" && !restored_menu {
                    restored_menu = true;
                    crate::menu::action(&app, "restore-players");
                }
            }
            let finished = progress.is_some_and(|value| value["player_windows"]["ok"].is_boolean());
            if finished {
                crate::player_windows::restore_all(&app);
                app.exit(0);
                return;
            }
            thread::sleep(Duration::from_millis(100));
        }
        crate::player_windows::restore_all(&app);
        app.exit(1);
    });
}
