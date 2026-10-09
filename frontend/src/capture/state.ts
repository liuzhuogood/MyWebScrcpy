import { ref } from 'vue'

/** 选区层打开时，投屏键盘/鼠标操控需让路（层覆盖画布天然拦截指针；键盘靠 closest 判定） */
export const captureActive = ref(false)
