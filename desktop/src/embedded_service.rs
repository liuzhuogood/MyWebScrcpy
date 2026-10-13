use sha2::{Digest, Sha256};
use std::{
    fs,
    io::Write,
    path::{Path, PathBuf},
};

#[cfg(all(windows, not(debug_assertions)))]
pub fn executable() -> Result<PathBuf, String> {
    const PAYLOAD: &[u8] = include_bytes!(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/binaries/mywebscrcpy-",
        env!("DESKTOP_TARGET"),
        ".exe"
    ));
    let root = std::env::var_os("LOCALAPPDATA").ok_or("无法定位本机应用缓存目录")?;
    materialize(
        &PathBuf::from(root)
            .join("com.mywebscrcpy.desktop")
            .join("runtime"),
        PAYLOAD,
    )
}

// Content-addressed paths let different GUI versions run without replacing a
// service executable that is still in use by another process.
fn materialize(root: &Path, payload: &[u8]) -> Result<PathBuf, String> {
    let digest = format!("{:x}", Sha256::digest(payload));
    let directory = root.join(digest);
    fs::create_dir_all(&directory).map_err(|e| format!("创建服务缓存失败: {e}"))?;
    let lock = fs::OpenOptions::new()
        .create(true)
        .truncate(false)
        .write(true)
        .open(directory.join("extract.lock"))
        .map_err(|e| format!("打开服务缓存锁失败: {e}"))?;
    lock.lock().map_err(|e| format!("锁定服务缓存失败: {e}"))?;
    let destination = directory.join("mywebscrcpy.exe");
    match fs::read(&destination) {
        Ok(bytes) if bytes == payload => return Ok(destination),
        Ok(_) => fs::remove_file(&destination).map_err(|e| format!("修复服务缓存失败: {e}"))?,
        Err(e) if e.kind() == std::io::ErrorKind::NotFound => {}
        Err(e) => return Err(format!("读取服务缓存失败: {e}")),
    }
    let mut temporary = tempfile::NamedTempFile::new_in(&directory)
        .map_err(|e| format!("创建服务临时文件失败: {e}"))?;
    temporary
        .write_all(payload)
        .map_err(|e| format!("写入服务缓存失败: {e}"))?;
    temporary
        .as_file()
        .sync_all()
        .map_err(|e| format!("同步服务缓存失败: {e}"))?;
    if let Err(error) = temporary.persist_noclobber(&destination) {
        // A concurrent GUI process may have completed the same extraction.
        if fs::read(&destination)
            .map(|bytes| bytes == payload)
            .unwrap_or(false)
        {
            return Ok(destination);
        }
        return Err(format!("发布服务缓存失败: {}", error.error));
    }
    eprintln!("[Desktop] 已释放内置服务: {}", destination.display());
    Ok(destination)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn repairs_corruption_and_preserves_other_versions() {
        let root = tempfile::tempdir().unwrap();
        let first = materialize(root.path(), b"first service").unwrap();
        fs::write(&first, b"corrupted").unwrap();
        assert_eq!(materialize(root.path(), b"first service").unwrap(), first);
        let second = materialize(root.path(), b"second service").unwrap();
        assert_ne!(first, second);
        assert_eq!(fs::read(first).unwrap(), b"first service");
        assert_eq!(fs::read(second).unwrap(), b"second service");
    }

    #[test]
    fn concurrent_extraction_publishes_complete_bytes() {
        let root = tempfile::tempdir().unwrap();
        let corrupted = materialize(root.path(), &[42; 65536]).unwrap();
        fs::write(corrupted, b"corrupted").unwrap();
        std::thread::scope(|scope| {
            let threads: Vec<_> = (0..8)
                .map(|_| scope.spawn(|| materialize(root.path(), &[42; 65536]).unwrap()))
                .collect();
            let paths: Vec<_> = threads
                .into_iter()
                .map(|thread| thread.join().unwrap())
                .collect();
            assert!(paths.iter().all(|path| path == &paths[0]));
            assert_eq!(fs::read(&paths[0]).unwrap(), [42; 65536]);
        });
    }
}
