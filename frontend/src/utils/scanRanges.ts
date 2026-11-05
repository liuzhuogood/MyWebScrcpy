export function formatScanRange(start: string[], end: string[]): string {
  for (const parts of [start, end]) {
    if (parts.length !== 4 || parts.some(v => !/^\d{1,3}$/.test(v) || Number(v) > 255)) throw new Error('IP 每段须为 0 至 255')
  }
  const number = (parts: string[]) => parts.reduce((n, v) => n * 256 + Number(v), 0)
  const first = number(start), last = number(end)
  if (last < first || last - first >= 256) throw new Error('IP 范围须递增且最多 256 个地址')
  for (const value of [start, end]) {
    const a = Number(value[0]), b = Number(value[1])
    if (!(a === 10 || a === 127 || a === 192 && b === 168 || a === 172 && b >= 16 && b <= 31 || a === 169 && b === 254)) throw new Error('仅支持局域网或回环地址扫描')
  }
  return `${start.map(Number).join('.')}-${end.map(Number).join('.')}`
}
