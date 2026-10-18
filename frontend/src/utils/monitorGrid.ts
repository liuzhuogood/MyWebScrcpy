export interface MonitorGridResult {
  rows: number
  cols: number
}

/**
 * 计算监控室/铺满模式下的最佳行列划分
 * @param count 在线设备数量
 * @param containerWidth 容器实际宽度（像素）
 * @param containerHeight 容器实际高度（像素）
 * @param phoneAspectRatio 手机典型宽高比（默认 9/16 ≈ 0.5625）
 */
export function computeMonitorGrid(
  count: number,
  containerWidth = 1920,
  containerHeight = 1080,
  phoneAspectRatio = 9 / 16,
): MonitorGridResult {
  if (count <= 0)
    return { rows: 0, cols: 0 }

  // 当设备数 <= 4 时：单行流式排布，每行 count（最多 4）个
  if (count <= 4)
    return { rows: 1, cols: count }

  // 当设备数 > 4 时：根据视口可用宽高与手机比例，在 R >= 2 的范围内搜索让设备画面面积最大的最佳 (rows, cols)
  const W = Math.max(containerWidth, 100)
  const H = Math.max(containerHeight, 100)
  const isTallerScreen = H > W

  let bestR = 2
  let bestC = Math.ceil(count / 2)
  let maxArea = 0

  for (let R = 2; R <= count; R++) {
    const C = Math.ceil(count / R)
    if (C > 16)
      continue

    const cellW = W / C
    const cellH = H / R
    const cellRatio = cellW / cellH

    let wDev: number
    let hDev: number
    if (cellRatio > phoneAspectRatio) {
      // 单元格更宽，高度受限，设备填满高度
      hDev = cellH
      wDev = cellH * phoneAspectRatio
    }
    else {
      // 单元格更高，宽度受限，设备填满宽度
      wDev = cellW
      hDev = cellW / phoneAspectRatio
    }

    const area = wDev * hDev
    const isPreferred = isTallerScreen ? R >= C : C >= R

    if (
      area > maxArea + 1e-4
      || (Math.abs(area - maxArea) <= 1e-4 && isPreferred && (isTallerScreen ? R > bestR : C > bestC))
    ) {
      maxArea = area
      bestR = R
      bestC = C
    }
  }

  return { rows: bestR, cols: bestC }
}
