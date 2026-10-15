import { ref } from 'vue'

// 检查器输入拦截：面板打开时投屏点击只做取色固定 / 节点选中，不注入设备。
// 用显式状态而非监听器注册顺序，可测、可靠。

export const inspectorOpen = ref(false)
/** 当前 Tab：color 会固定十字线并吞掉点击；xml 点中绿框时选中节点；ocr 展示文字识别 */
export const inspectorTab = ref<'color' | 'xml' | 'ocr' | 'page' | 'log'>('color')

export function setInspector(open: boolean, tab: typeof inspectorTab.value = 'color') {
  inspectorOpen.value = open
  inspectorTab.value = tab
}

/** XML Tab 命中测试（client 坐标 → 是否点中绿框），由检查器核心注册；操控层据此决定是否放行 */
export const xmlHitTest: { fn: ((clientX: number, clientY: number) => boolean) | null } = { fn: null }
