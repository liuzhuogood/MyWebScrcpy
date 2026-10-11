import { Marked } from 'marked'
import DOMPurify from 'dompurify'
import { templateImageUrl, type TemplateItem } from '@/api/templates'

export function initialDocument(name: string): string {
  return `---\nname: ${JSON.stringify(name)}\nversion: 1.0.0\ntimeout_sec: 120\nenabled: false\n---\n\n# 步骤流程\n- 唤醒设备\n- 等待 2 秒\n`
}

export function hasDocumentTargets(content: string): boolean {
  return /^target_devices?\s*:/mi.test(content)
}

export function removeDocumentTargets(content: string): string {
  return content.replace(/^target_devices?\s*:[^\r\n]*(?:\r?\n|$)/gmi, '')
}

const escapeHTML = (value: string) => value.replace(/[&<>"']/g, ch => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' })[ch]!)

// Only exact template IDs in the selected scope become images; sanitize all Markdown HTML.
export function renderDocument(content: string, assets: TemplateItem[]): string {
  const markdown = new Marked({ gfm: true, breaks: true })
  markdown.use({ renderer: {
    image(token) {
      const match = token.href.match(/^template:\/\/([^?\s]+)(?:\?threshold=[0-9.]+)?$/)
      if (!match) return false
      const asset = assets.find(item => item.id === match[1])
      if (!asset || !asset.enabled) return `<span class="template-unavailable">${escapeHTML(token.text)} · 模板不可用</span>`
      return `<img src="${escapeHTML(templateImageUrl(asset.id, asset.updated_at))}" alt="${escapeHTML(token.text)}" loading="lazy">`
    },
  } })
  const body = content.replace(/^---\r?\n[\s\S]*?\r?\n---(?:\r?\n|$)/, '')
  return DOMPurify.sanitize(markdown.parse(body, { async: false }), { USE_PROFILES: { html: true } })
}

export const taskStatus: Record<string, string> = {
  running: '运行中', stopping: '终止中', completed: '已完成', failed: '失败', killed: '已终止', skipped: '已跳过',
  started: '已启动', partial: '部分异常', success: '已完成',
}

export function dateTime(timestamp: number): string {
  return timestamp > 0 ? new Date(timestamp).toLocaleString('zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit', second: '2-digit', hour12: false }) : '—'
}
