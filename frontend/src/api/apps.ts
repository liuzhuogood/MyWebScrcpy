function withSerial(endpoint: string, serial: string): string {
  const sep = endpoint.includes('?') ? '&' : '?'
  return `${endpoint}${sep}serial=${encodeURIComponent(serial)}`
}

export interface InstallOptions {
  replaceExisting: boolean
  grantPermissions: boolean
  allowDowngrade: boolean
}

export interface InstallResponse {
  ok: boolean
  message: string
  error?: string
  output?: string
}

/**
 * XHR 方式向设备上传并执行 APK 安装，支持实时上传进度回调与取消。
 */
export function installApk(
  serial: string,
  file: File,
  opts: InstallOptions,
  onProgress: (percent: number) => void,
  onDone: (res: InstallResponse) => void,
): () => void {
  const form = new FormData()
  form.append('file', file, file.name)
  form.append('replace', String(opts.replaceExisting))
  form.append('grant', String(opts.grantPermissions))
  form.append('downgrade', String(opts.allowDowngrade))

  const xhr = new XMLHttpRequest()
  xhr.open('POST', withSerial('/api/apps/install', serial))

  xhr.upload.onprogress = (e) => {
    if (e.lengthComputable && e.total > 0) {
      onProgress(Math.round((e.loaded / e.total) * 100))
    }
  }

  xhr.onload = () => {
    let data: InstallResponse
    try {
      data = JSON.parse(xhr.responseText)
    }
    catch {
      data = {
        ok: false,
        message: xhr.statusText || '解析服务器响应失败',
        error: 'INVALID_RESPONSE',
      }
    }

    if (xhr.status >= 200 && xhr.status < 300 && data.ok) {
      onDone(data)
    }
    else {
      onDone({
        ok: false,
        message: data.message || '安装失败',
        error: data.error || 'INSTALL_ERROR',
        output: data.output,
      })
    }
  }

  xhr.onerror = () => {
    onDone({
      ok: false,
      message: '网络连接异常，安装请求失败',
      error: 'NETWORK_ERROR',
    })
  }

  xhr.onabort = () => {
    onDone({
      ok: false,
      message: '用户已取消安装',
      error: 'ABORTED',
    })
  }

  xhr.send(form)
  return () => xhr.abort()
}
