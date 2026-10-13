use serde::Deserialize;
use std::{
    fs::{self, File, OpenOptions},
    io::{Read, Write},
    net::TcpStream,
    path::{Path, PathBuf},
    process::{Child, Command, Stdio},
    thread,
    time::{Duration, Instant},
};

#[derive(Clone, Deserialize)]
pub struct Ready {
    pub url: String,
    pub token: String,
    pid: u32,
}

pub struct Backend {
    child: Child,
    pub ready: Ready,
    ready_file: PathBuf,
}

pub fn executable() -> Result<PathBuf, String> {
    #[cfg(all(windows, not(debug_assertions)))]
    {
        return crate::embedded_service::executable();
    }
    #[cfg(not(all(windows, not(debug_assertions))))]
    {
        let exe = std::env::current_exe().map_err(|e| e.to_string())?;
        let name = if cfg!(windows) {
            "mywebscrcpy.exe"
        } else {
            "mywebscrcpy"
        };
        let bundled = exe.parent().ok_or("无法定位 App 目录")?.join(name);
        if bundled.is_file() {
            return Ok(bundled);
        }
        // cargo run/check uses the target-specific, explicitly configured sidecar.
        let suffix = if cfg!(windows) { ".exe" } else { "" };
        let development = Path::new(env!("CARGO_MANIFEST_DIR")).join(format!(
            "binaries/mywebscrcpy-{}{suffix}",
            env!("DESKTOP_TARGET")
        ));
        if cfg!(debug_assertions) && development.is_file() {
            return Ok(development);
        }
        Err(format!("缺少配套服务程序: {}", bundled.display()))
    }
}

pub fn command(path: &Path) -> Command {
    let mut command = Command::new(path);
    #[cfg(windows)]
    {
        use std::os::windows::process::CommandExt;
        command.creation_flags(0x08000000); // CREATE_NO_WINDOW
    }
    // Rust mutability is used only on Windows.
    #[cfg(not(windows))]
    let _ = &mut command;
    command
}

impl Backend {
    pub fn start(data: &Path) -> Result<Self, String> {
        fs::create_dir_all(data).map_err(|e| format!("创建数据目录失败: {e}"))?;
        let ready_file = data.join(format!("desktop-ready-{}.json", std::process::id()));
        if ready_file.exists() {
            fs::remove_file(&ready_file).map_err(|e| e.to_string())?;
        }
        let log = OpenOptions::new()
            .create(true)
            .append(true)
            .open(data.join("desktop.log"))
            .map_err(|e| format!("打开日志失败: {e}"))?;
        let error_log = log.try_clone().map_err(|e| e.to_string())?;
        let mut service_command=command(&executable()?);
 service_command.env_remove("MYWEBSCRCPY_UPDATE_RECEIPT");
 if let Some(receipt)=crate::update_backup::pending_receipt(data)? {service_command.env("MYWEBSCRCPY_UPDATE_RECEIPT",receipt);}
 let mut child = service_command
            .args(["--no-gui", "--listen", "127.0.0.1:0", "--data-dir"])
            .arg(data)
            .arg("--desktop-ready")
            .arg(&ready_file)
            .env_remove("TLS_CERT")
            .env_remove("TLS_KEY")
            .env_remove("PORT")
            .env_remove("SCRIPTS_DIR")
            .env_remove("TEMPLATES_DIR")
            .env("MYWEBSCRCPY_DESKTOP_VERSION", env!("CARGO_PKG_VERSION"))
            .stdin(Stdio::piped())
            .stdout(Stdio::from(log))
            .stderr(Stdio::from(error_log))
            .spawn()
            .map_err(|e| format!("启动服务失败: {e}"))?;
        crate::updater::record_backend_pid(data, child.id())?;
        let deadline = Instant::now() + Duration::from_secs(20);
        loop {
            if let Ok(contents) = fs::read(&ready_file) {
                let ready: Ready = match serde_json::from_slice(&contents) {
                    Ok(value) => value,
                    Err(error) => {
                        Self::abort(&mut child);
                        return Err(format!("就绪信息错误: {error}"));
                    }
                };
                if ready.pid != child.id() || !ready.url.starts_with("http://127.0.0.1:") {
                    Self::abort(&mut child);
                    return Err("就绪信息与当前服务不匹配".into());
                }
                if request(&ready, "GET", "/api/desktop/health").is_ok() {
                    return Ok(Self {
                        child,
                        ready,
                        ready_file,
                    });
                }
            }
            match child.try_wait() {
                Ok(Some(status)) => {
                    return Err(format!(
                        "服务提前退出 ({status})，请查看 {}",
                        data.join("desktop.log").display()
                    ))
                }
                Err(error) => {
                    Self::abort(&mut child);
                    return Err(error.to_string());
                }
                Ok(None) => {}
            }
            if Instant::now() >= deadline {
                Self::abort(&mut child);
                return Err(format!(
                    "服务启动超时，请查看 {}",
                    data.join("desktop.log").display()
                ));
            }
            thread::sleep(Duration::from_millis(80));
        }
    }

    fn abort(child: &mut Child) {
        // Closing stdin requests cleanup even before HTTP readiness is available.
        child.stdin.take();
        let deadline = Instant::now() + Duration::from_secs(3);
        while Instant::now() < deadline {
            if matches!(child.try_wait(), Ok(Some(_))) {
                return;
            }
            thread::sleep(Duration::from_millis(50));
        }
        let _ = child.kill();
        let _ = child.wait();
    }

    pub fn stop(&mut self) {
        let _ = request(&self.ready, "POST", "/api/desktop/shutdown");
        self.child.stdin.take();
        let deadline = Instant::now() + Duration::from_secs(8);
        while Instant::now() < deadline {
            if matches!(self.child.try_wait(), Ok(Some(_))) {
                let _ = fs::remove_file(&self.ready_file);
                return;
            }
            thread::sleep(Duration::from_millis(80));
        }
        eprintln!("[Desktop] 服务未按时退出，终止当前 App 的子进程");
        let _ = self.child.kill();
        let _ = self.child.wait();
        let _ = fs::remove_file(&self.ready_file);
    }
}
impl Drop for Backend {
    fn drop(&mut self) {
        self.stop();
    }
}

pub fn request(ready: &Ready, method: &str, path: &str) -> Result<(), String> {
    let address = ready
        .url
        .strip_prefix("http://")
        .ok_or("服务地址格式错误")?;
    let address = address.parse().map_err(|e| format!("服务地址错误: {e}"))?;
    let mut socket =
        TcpStream::connect_timeout(&address, Duration::from_secs(1)).map_err(|e| e.to_string())?;
    socket
        .set_read_timeout(Some(Duration::from_secs(2)))
        .map_err(|e| e.to_string())?;
    socket
        .set_write_timeout(Some(Duration::from_secs(2)))
        .map_err(|e| e.to_string())?;
    write!(socket, "{method} {path} HTTP/1.1\r\nHost: localhost\r\nAuthorization: Bearer {}\r\nConnection: close\r\nContent-Length: 0\r\n\r\n", ready.token).map_err(|e| e.to_string())?;
    let mut response = String::new();
    socket
        .take(4096)
        .read_to_string(&mut response)
        .map_err(|e| e.to_string())?;
    if response.starts_with("HTTP/1.1 200 ") || response.starts_with("HTTP/1.1 202 ") {
        Ok(())
    } else {
        Err(response
            .split("\r\n\r\n")
            .nth(1)
            .unwrap_or("服务请求失败")
            .trim()
            .to_string())
    }
}

pub fn mark_launcher_ready(path: Option<&Path>) -> Result<(), String> {
    if let Some(path) = path {
        File::create(path).map_err(|e| e.to_string())?;
    }
    Ok(())
}

#[cfg(target_os = "macos")]
pub fn fix_macos_env() {
    let current_path = std::env::var("PATH").unwrap_or_default();
    if current_path != "/usr/bin:/bin:/usr/sbin:/sbin" && !current_path.is_empty() {
        return;
    }
    let shell = std::env::var("SHELL").unwrap_or_else(|_| "/bin/zsh".into());
    let mut command = Command::new(&shell);
    command.args([
        "-l",
        "-i",
        "-c",
        "printf '__MYWEBSCRCPY_ENV_START__\\n'; env -0; printf '__MYWEBSCRCPY_ENV_END__\\n'",
    ]);
    if let Ok(output) = command.output() {
        let stdout = output.stdout;
        let start_tag = b"__MYWEBSCRCPY_ENV_START__\n";
        let end_tag = b"__MYWEBSCRCPY_ENV_END__\n";
        if let Some(start_pos) = stdout.windows(start_tag.len()).position(|w| w == start_tag) {
            let payload = &stdout[start_pos + start_tag.len()..];
            let end_pos = payload
                .windows(end_tag.len())
                .position(|w| w == end_tag)
                .unwrap_or(payload.len());
            let env_bytes = &payload[..end_pos];
            for entry in env_bytes.split(|&b| b == 0) {
                if entry.is_empty() {
                    continue;
                }
                if let Some(eq_pos) = entry.iter().position(|&b| b == b'=') {
                    if let (Ok(key), Ok(val)) = (
                        std::str::from_utf8(&entry[..eq_pos]),
                        std::str::from_utf8(&entry[eq_pos + 1..]),
                    ) {
                        match key {
                            "PATH" | "ANDROID_HOME" | "ANDROID_SDK_ROOT" | "NVM_DIR" => {
                                std::env::set_var(key, val);
                            }
                            _ => {
                                if std::env::var(key).is_err()
                                    && !key.starts_with("XPC_")
                                    && !key.starts_with("__CF")
                                {
                                    std::env::set_var(key, val);
                                }
                            }
                        }
                    }
                }
            }
        }
    }
}
