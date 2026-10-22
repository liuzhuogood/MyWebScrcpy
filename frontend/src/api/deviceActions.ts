/**
 * 设备动作统一接口封装
 * 契约核对自:
 * - internal/automation/routes.go (writeError, handleEvent)
 * - internal/automation/types.go (EventRequest, ActionResult)
 * - internal/automation/service.go (Event)
 */

export interface EventRequest {
  serial: string
  event: 'back' | 'home' | 'app_switch'
  expires_ms?: number
}

export interface ActionResult {
  request_id: string
  status: 'executed' | 'failed'
  executed: boolean
  mode: string
  elapsed_ms: number
  error_code?: string
}

export interface ApiErrorResponse {
  code: string
  message: string
}

export async function sendDeviceEvent(
  serial: string,
  event: 'back' | 'home' | 'app_switch',
  signal?: AbortSignal,
  timeoutMs = 15000,
): Promise<ActionResult> {
  const reqBody: EventRequest = {
    serial,
    event,
    expires_ms: timeoutMs,
  }

  const timeoutController = new AbortController()
  const timeoutId = setTimeout(() => {
    timeoutController.abort(new Error(`请求超时 (${timeoutMs}ms)`))
  }, timeoutMs)

  function onParentAbort() {
    timeoutController.abort(signal?.reason || new Error('操作已取消'))
  }

  if (signal) {
    if (signal.aborted) {
      clearTimeout(timeoutId)
      throw signal.reason || new Error('操作已取消')
    }
    signal.addEventListener('abort', onParentAbort)
  }

  try {
    const res = await fetch('/api/v1/actions/events', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(reqBody),
      signal: timeoutController.signal,
    })

    if (!res.ok) {
      let code = `http_${res.status}`
      let message = `HTTP ${res.status}`
      try {
        const errJson = (await res.json()) as ApiErrorResponse
        if (errJson && typeof errJson === 'object') {
          if (typeof errJson.code === 'string')
            code = errJson.code
          if (typeof errJson.message === 'string')
            message = errJson.message
        }
      }
      catch {
        // 非 JSON 响应保持状态码
      }
      const err = new Error(`${code}: ${message}`)
      console.warn(`[deviceActions] 设备 ${serial} 事件 ${event} 失败:`, err.message)
      throw err
    }

    const data = (await res.json()) as ActionResult
    // 严格按后端 ActionResult 契约验证：executed 为 true 且 status 为 executed 才算成功
    if (data.status !== 'executed' || data.executed !== true) {
      const errCode = data.error_code || 'execution_failed'
      const err = new Error(`动作执行失败 (${errCode})`)
      console.warn(`[deviceActions] 设备 ${serial} 响应报告未完成:`, data)
      throw err
    }

    return data
  }
  catch (error) {
    if (signal?.aborted) {
      throw signal.reason || new Error('操作已取消')
    }
    throw error
  }
  finally {
    clearTimeout(timeoutId)
    if (signal) {
      signal.removeEventListener('abort', onParentAbort)
    }
  }
}
