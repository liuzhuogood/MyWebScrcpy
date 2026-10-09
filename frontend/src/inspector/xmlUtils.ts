// XML 快照解析、坐标换算、XPath 生成（纯函数）。

export interface Box {
  x: number
  y: number
  w: number
  h: number
  node: Element
}

const BOUNDS_RE = /^\[(-?\d+),(-?\d+)\]\[(-?\d+),(-?\d+)\]$/

export function parseXml(text: string): Document {
  const doc = new DOMParser().parseFromString(text, 'application/xml')
  if (doc.querySelector('parsererror'))
    throw new Error('XML 格式无效')
  return doc
}

function rawBounds(node: Element): { left: number, top: number, right: number, bottom: number } | null {
  const m = BOUNDS_RE.exec(node.getAttribute('bounds') || '')
  return m ? { left: +m[1], top: +m[2], right: +m[3], bottom: +m[4] } : null
}

/** 快照坐标系尺寸：display_size 优先，否则取全部 bounds 最大值 */
export function measureBasis(doc: Document, displaySize?: { width: number, height: number }): { width: number, height: number } | null {
  if (displaySize && displaySize.width > 0 && displaySize.height > 0)
    return displaySize
  let right = 0
  let bottom = 0
  doc.querySelectorAll('[bounds]').forEach((n) => {
    const b = rawBounds(n)
    if (b) {
      right = Math.max(right, b.right)
      bottom = Math.max(bottom, b.bottom)
    }
  })
  return right > 0 && bottom > 0 ? { width: right, height: bottom } : null
}

/** 快照 bounds → canvas 像素坐标 */
export function toBox(node: Element, basis: { width: number, height: number } | null, canvasW: number, canvasH: number): Box | null {
  const b = rawBounds(node)
  if (!b)
    return null
  const sx = basis && canvasW ? canvasW / basis.width : 1
  const sy = basis && canvasH ? canvasH / basis.height : 1
  return {
    x: b.left * sx,
    y: b.top * sy,
    w: (b.right - b.left) * sx,
    h: (b.bottom - b.top) * sy,
    node,
  }
}

export function xpathLiteral(value: string): string {
  if (!value.includes(`'`))
    return `'${value}'`
  if (!value.includes(`"`))
    return `"${value}"`
  return `concat(${value.split(`'`).map(part => `'${part}'`).join(`, "'", `)})`
}

/** 按选中属性生成 XPath；无属性时退化为绝对路径 */
export function makeXPath(node: Element, attributes: Set<string>): string {
  const predicates = [...attributes]
    .filter(name => node.getAttribute(name))
    .map(name => `@${name}=${xpathLiteral(node.getAttribute(name) || '')}`)
  if (predicates.length)
    return `//*[${predicates.join(' and ')}]`
  const parts: string[] = []
  for (let cur: Element | null = node; cur && cur.nodeType === 1; cur = cur.parentElement) {
    let index = 1
    for (let sib = cur.previousElementSibling; sib; sib = sib.previousElementSibling) {
      if (sib.tagName === cur.tagName)
        index++
    }
    parts.unshift(`${cur.tagName.toLowerCase()}[${index}]`)
  }
  return `/${parts.join('/')}`
}

/** XPath 求值，返回带有效 bounds 的命中节点 */
export function evalXPath(doc: Document, expr: string): { total: number, nodes: Element[] } {
  const result = doc.evaluate(expr, doc, null, XPathResult.ORDERED_NODE_SNAPSHOT_TYPE, null)
  const nodes: Element[] = []
  for (let i = 0; i < result.snapshotLength; i++) {
    const n = result.snapshotItem(i)
    if (n && n.nodeType === 1 && rawBounds(n as Element))
      nodes.push(n as Element)
  }
  return { total: result.snapshotLength, nodes }
}
