// Feature-gated native smoke verification; absent from production binaries.
use std::{thread, time::Duration};
use tauri::{AppHandle, Manager};

pub fn start(app: &AppHandle) {
    if std::env::var_os("MYWEBSCRCPY_NATIVE_REPORT").is_none() {
        return;
    }
    let app = app.clone();
    thread::spawn(move || {
        thread::sleep(Duration::from_secs(3));
        let ui = app.clone();
        let _ = app.run_on_main_thread(move || {
            // Exercise the same handler used by native menu items.
            crate::menu::action(&ui, "dependencies");
        });
        thread::sleep(Duration::from_secs(2));
        let ui = app.clone();
        let _ = app.run_on_main_thread(move || {
            if let Some(window) = ui.get_webview_window("main") {
                let _ = window.close();
            }
        });
        thread::sleep(Duration::from_millis(500));
        let ui = app.clone();
        let _ = app.run_on_main_thread(move || {
            if let Some(window) = ui.get_webview_window("main") {
                let hidden = match window.is_visible() { Ok(value)=>!value,Err(_)=>return };
                crate::menu::action(&ui,"show");
                let restored = match window.is_visible() { Ok(value)=>value,Err(_)=>return };
                let script=r#"
                    fetch('/api/desktop/native-verification', {
                      method:'POST',headers:{'Content-Type':'application/json',Authorization:'Bearer '+window.__MYWEBSCRCPY_DESKTOP_TOKEN__},
                      body:JSON.stringify({title:document.title,path:location.pathname,
                        mounted:!!document.querySelector('#app')?.childElementCount,
                        dependency_heading:document.querySelector('h1')?.textContent==='依赖管理',
                        close_hides:$HIDDEN,restore_visible:$RESTORED,
                        video_decoder:typeof VideoDecoder!=='undefined',audio_decoder:typeof AudioDecoder!=='undefined'})
                    });
                "#.replace("$HIDDEN",&hidden.to_string()).replace("$RESTORED",&restored.to_string());
                let _ = window.eval(script);
            }
        });
    });
}
