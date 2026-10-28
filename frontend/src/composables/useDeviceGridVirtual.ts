import { computed, onMounted, ref, shallowRef, watch } from 'vue'
import { elementScroll, observeElementOffset, observeElementRect, useVirtualizer, windowScroll, observeWindowOffset, observeWindowRect } from '@tanstack/vue-virtual'
import { useElementSize, useWindowSize } from '@vueuse/core'
import type { Device } from '@/api/devices'

interface DeviceGridVirtualOptions {
  devices: () => Device[]
  /** 当前视图模式，列数与卡片高度不同 */
  viewMode: () => 'deck' | 'compact'
  /** 网格容器元素 */
  gridEl: () => HTMLElement | null
}

const GAP = 20
const OVERSCAN_PX = 600

/**
 * 首页设备网格的窗口化渲染：
 * 主滚动容器是 window，设备按响应式列数切成行，
 * 只渲染可视区附近的行，用等高占位撑住滚动条，保留原生整页滚动。
 */
export function useDeviceGridVirtual(options: DeviceGridVirtualOptions) {
  const scrollEl = ref<HTMLElement | Window | null>(null)

  function resolveScrollEl() {
    let el = options.gridEl()
    while (el) {
      const oy = getComputedStyle(el).overflowY
      if ((oy === 'auto' || oy === 'scroll') && el.scrollHeight > el.clientHeight)
        return el
      el = el.parentElement
    }
    return window
  }

  onMounted(() => {
    scrollEl.value = resolveScrollEl()
  })

  const windowSize = useWindowSize()
  const gridSize = useElementSize(options.gridEl)

  /** 列数：按现有 grid 响应式断点推导，与 Tailwind 前缀一致 */
  const cols = computed(() => {
    void windowSize.width.value
    void gridSize.width.value
    const el = options.gridEl()
    if (!el)
      return 1
    const w = el.clientWidth
    if (options.viewMode() === 'compact')
      return w >= 1280 ? 3 : w >= 768 ? 2 : 1
    return w >= 1536 ? 4 : w >= 1280 ? 3 : w >= 768 ? 2 : 1
  })

  /** 行高：列宽 / 卡片宽高比 + 行间距（卡片为竖向矩形，高 > 宽） */
  const rowHeight = computed(() => {
    void gridSize.width.value
    const el = options.gridEl()
    if (!el)
      return 320
    const w = el.clientWidth
    const c = cols.value
    const colW = (w - GAP * (c - 1)) / c
    const ratio = options.viewMode() === 'compact' ? 1.08 : 1.12
    return Math.round(colW * ratio) + GAP
  })

  /** 行数据：每行是一个设备切片 */
  const rows = computed(() => {
    const list = options.devices()
    const c = cols.value
    const out: Device[][] = []
    for (let i = 0; i < list.length; i += c)
      out.push(list.slice(i, i + c))
    return out
  })

  const rowList = shallowRef<Device[][]>([])

  function syncRows() {
    const r = rows.value
    if (r.length !== rowList.value.length || r[0] !== rowList.value[0])
      rowList.value = r
  }

  const virtualizer = useVirtualizer(
    computed(() => {
      const isWindowScroll = scrollEl.value === window
      return {
        getScrollElement: () => scrollEl.value as any,
        count: (syncRows(), rowList.value.length),
        estimateSize: () => rowHeight.value,
        overscan: Math.ceil(OVERSCAN_PX / 200),
        observeElementRect: (isWindowScroll ? observeWindowRect : observeElementRect) as any,
        observeElementOffset: (isWindowScroll ? observeWindowOffset : observeElementOffset) as any,
        scrollToFn: (isWindowScroll ? windowScroll : elementScroll) as any,
      }
    }),
  )

  const virtualItems = computed(() => virtualizer.value.getVirtualItems())
  const totalSize = computed(() => virtualizer.value.getTotalSize())

  /** 重新测量，列数/视图模式/数据变化时校正行高 */
  function remeasure() {
    virtualizer.value.measure()
  }

  onMounted(() => {
    watch([cols, rowHeight, () => options.devices().length], () => {
      remeasure()
    })
  })

  return {
    cols,
    rows,
    virtualItems,
    totalSize,
    remeasure,
  }
}
