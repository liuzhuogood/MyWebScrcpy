fn main() {
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
        ]),
    ))
    .expect("desktop build configuration failed")
}
