import type { FileItem } from '@/api/files'

function extOf(name: string): string {
  const idx = (name || '').lastIndexOf('.')
  return idx > 0 ? name.slice(idx + 1).toLowerCase() : ''
}

const IMAGE_EXT = ['png', 'jpg', 'jpeg', 'gif', 'webp', 'svg', 'bmp', 'ico']
const VIDEO_EXT = ['mp4', 'webm', 'mov', 'mkv', 'ogg']
const TEXT_EXT = ['txt', 'log', 'json', 'xml', 'md', 'yml', 'yaml', 'js', 'ts', 'html', 'css', 'sh', 'csv', 'ini', 'conf', 'properties']

export type PreviewType = 'image' | 'video' | 'text' | null

export function getPreviewType(item: FileItem): PreviewType {
  if (!item || item.kind === 'folder')
    return null
  const ext = extOf(item.name)
  if (IMAGE_EXT.includes(ext) || item.kind === 'image')
    return 'image'
  if (VIDEO_EXT.includes(ext) || item.kind === 'video')
    return 'video'
  if (TEXT_EXT.includes(ext) || item.kind === 'document' || item.mime?.startsWith('text/'))
    return 'text'
  return null
}

/** 文件类型徽标文字（最长 4 字符） */
export function badgeText(item: FileItem): string {
  const ext = extOf(item.name)
  const upper = ext ? ext.toUpperCase() : ''
  if (upper)
    return upper.length > 4 ? upper.slice(0, 4) : upper
  if (item.kind === 'image')
    return 'IMG'
  if (item.kind === 'video')
    return 'VID'
  if (item.kind === 'audio')
    return 'AUD'
  if (item.kind === 'document')
    return 'DOC'
  return 'FILE'
}

export function kindLabel(item: FileItem): string {
  if (item.kind === 'folder')
    return '文件夹'
  return item.mime || item.kind || '文件'
}
