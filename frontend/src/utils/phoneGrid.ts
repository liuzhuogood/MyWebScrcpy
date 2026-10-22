import type { DashSize } from '../composables/useHomePhoneView'

export interface PhoneGridTierSpec {
  width: number | null
  height: number | null
  cardClass: string
  containerClass: string
}

/**
 * 手机视图同宽同高规格与静态 Tailwind 类映射（统一生产事实源）：
 * - small:  200 × 380 px（顶栏 28px + 视口 352px）
 * - medium: 270 × 508 px（顶栏 28px + 视口 480px）
 * - large:  348 × 648 px（顶栏 28px + 视口 620px，保证在 375~390px 窄屏视口内无横向溢出）
 * - fit:    铺满模式（卡片满格，外部网格 2px 间隙）
 *
 * 注：类名必须保留静态字面量声明，确保 Vite / Tailwind 生产构建 AOT 静态扫描能够准确提取，
 * 严禁使用模板字符串动态拼接。
 */
export const PHONE_TIER_SPECS: Record<DashSize, PhoneGridTierSpec> = {
  fit: {
    width: null,
    height: null,
    cardClass: 'w-full h-full min-h-0 min-w-0',
    containerClass: 'grid gap-[2px] h-full w-full',
  },
  small: {
    width: 200,
    height: 380,
    cardClass: 'w-[200px] h-[380px] max-w-full shrink-0',
    containerClass: 'flex flex-wrap items-start content-start gap-[2px] overflow-y-auto',
  },
  medium: {
    width: 270,
    height: 508,
    cardClass: 'w-[270px] h-[508px] max-w-full shrink-0',
    containerClass: 'flex flex-wrap items-start content-start gap-[2px] overflow-y-auto',
  },
  large: {
    width: 348,
    height: 648,
    cardClass: 'w-[348px] h-[648px] max-w-full shrink-0',
    containerClass: 'flex flex-wrap items-start content-start gap-[2px] overflow-y-auto',
  },
}

/**
 * 获取指定档位格子 Tailwind 尺寸类名
 */
export function getPhoneTileDimensionClasses(size: DashSize): {
  cardClass: string
  containerClass: string
} {
  const spec = PHONE_TIER_SPECS[size]
  return {
    cardClass: spec.cardClass,
    containerClass: spec.containerClass,
  }
}

/**
 * 等比计算在容器中 contain 居中显示的画面尺寸及黑边偏移
 * @param deviceW 设备画面原始宽度
 * @param deviceH 设备画面原始高度
 * @param containerW 可用视口宽度
 * @param containerH 可用视口高度
 * @returns 缩放后的画面宽高与水平/垂直黑边
 */
export function computeContainDimensions(
  deviceW: number,
  deviceH: number,
  containerW: number,
  containerH: number,
): { width: number, height: number, offsetX: number, offsetY: number } {
  if (deviceW <= 0 || deviceH <= 0 || containerW <= 0 || containerH <= 0) {
    return { width: 0, height: 0, offsetX: 0, offsetY: 0 }
  }

  const ratio = deviceW / deviceH
  let dw = containerW
  let dh = dw / ratio

  if (dh > containerH) {
    dh = containerH
    dw = dh * ratio
  }

  const roundedW = Math.round(dw)
  const roundedH = Math.round(dh)
  const offsetX = Math.max(0, Math.round((containerW - roundedW) / 2))
  const offsetY = Math.max(0, Math.round((containerH - roundedH) / 2))

  return {
    width: roundedW,
    height: roundedH,
    offsetX,
    offsetY,
  }
}

/**
 * 将视口中指针点击坐标转换为设备真实像素坐标
 * @param clientX 视口 X 坐标
 * @param clientY 视口 Y 坐标
 * @param canvasRect canvas 画面包围盒
 * @param deviceW 设备原始宽度
 * @param deviceH 设备原始高度
 * @param strict 是否严格要求在 canvas 画面内部；为 true 时若点击落在黑边区域返回 null（用于初次按下/滚轮），为 false 时超出边缘截断 clamp（用于持续拖拽移动/松开）
 */
export function mapViewportToDeviceCoords(
  clientX: number,
  clientY: number,
  canvasRect: { left: number, top: number, width: number, height: number } | null | undefined,
  deviceW: number,
  deviceH: number,
  strict = true,
): { x: number, y: number } | null {
  if (!canvasRect || deviceW <= 0 || deviceH <= 0 || canvasRect.width <= 0 || canvasRect.height <= 0) {
    return null
  }

  // 严格模式（Down / TouchStart / Wheel）：落在真实 canvas 画面外的黑边区域时直接拦截返回 null
  if (strict) {
    if (
      clientX < canvasRect.left
      || clientX > canvasRect.left + canvasRect.width
      || clientY < canvasRect.top
      || clientY > canvasRect.top + canvasRect.height
    ) {
      return null
    }
  }

  const x = Math.round(((clientX - canvasRect.left) * deviceW) / canvasRect.width)
  const y = Math.round(((clientY - canvasRect.top) * deviceH) / canvasRect.height)

  return {
    x: Math.max(0, Math.min(deviceW - 1, x)),
    y: Math.max(0, Math.min(deviceH - 1, y)),
  }
}
