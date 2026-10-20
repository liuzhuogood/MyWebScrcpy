fn main() {
    let target = std::env::var("TARGET").unwrap();
    if target.contains("windows") {
        println!("cargo:rerun-if-changed=binaries/mywebscrcpy-{target}.exe");
    }
    println!("cargo:rerun-if-env-changed=MYWEBSCRCPY_UPDATER_PUBLIC_KEY");
    println!("cargo:rerun-if-changed=updater.pub");
    let key=std::env::var("MYWEBSCRCPY_UPDATER_PUBLIC_KEY").unwrap_or_else(|_| std::fs::read_to_string("updater.pub").expect("missing updater public key"));
    println!("cargo:rustc-env=MYWEBSCRCPY_UPDATER_PUBLIC_KEY={}",key.trim());
    println!(
        "cargo:rustc-env=DESKTOP_TARGET={}",
        std::env::var("TARGET").unwrap()
    );
    tauri_build::try_build(tauri_build::Attributes::new().app_manifest(
        tauri_build::AppManifest::new().commands(&[
            "open_browser_link",
            "desktop_check_update",
            "desktop_upgrade",
            "open_device_window",
            "set_window_always_on_top",
            "is_window_always_on_top",
            "list_player_windows",
            "set_player_opacity",
            "set_player_click_through",
            "set_player_borderless",
            "set_player_toolbar_visible",
            "restore_player_window",
            "get_main_window_state",
            "set_main_window_opacity",
            "set_main_window_click_through",
            "set_main_window_borderless",
            "set_main_window_always_on_top",
            "restore_main_window",
            "toggle_window_maximize",
            "close_window",
        ]),
    ))
    .expect("desktop build configuration failed")
}
