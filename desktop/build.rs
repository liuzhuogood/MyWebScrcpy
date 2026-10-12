fn main() {
    println!(
        "cargo:rustc-env=DESKTOP_TARGET={}",
        std::env::var("TARGET").unwrap()
    );
    tauri_build::try_build(
        tauri_build::Attributes::new().app_manifest(
            tauri_build::AppManifest::new().commands(&["open_browser_link"]),
        ),
    ).expect("desktop build configuration failed")
}
