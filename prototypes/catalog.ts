import type { Component } from 'vue'

export interface PrototypeMeta {
  title: string
}

const metadata = import.meta.glob<PrototypeMeta>('./cases/*/meta.ts', {
  eager: true,
  import: 'default',
})
const pages = import.meta.glob<{ default: Component }>('./cases/*/Prototype.vue')

export const prototypes = Object.entries(metadata).map(([file, meta]) => {
  const id = file.slice('./cases/'.length, -'/meta.ts'.length)
  const pagePath = `./cases/${id}/Prototype.vue`
  const loader = pages[pagePath]
  if (!meta.title.trim() || !loader)
    throw new Error(`[prototype] ${id} 缺少标题或 Prototype.vue`)
  return { id, title: meta.title, loader }
}).sort((a, b) => a.id.localeCompare(b.id))

for (const file of Object.keys(pages)) {
  const metaPath = file.replace('/Prototype.vue', '/meta.ts')
  if (!metadata[metaPath])
    throw new Error(`[prototype] ${file} 缺少 meta.ts`)
}
