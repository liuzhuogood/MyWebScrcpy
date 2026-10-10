import { ref } from 'vue'

export function useTemplateMatches(_serial: string, _getCanvas: () => HTMLCanvasElement | null) {
  const overlayRef = ref<HTMLCanvasElement | null>(null)
  const showBoxes = ref(false)
  const matches = ref<unknown[]>([])

  function setShow(on: boolean) {
    showBoxes.value = on
    clear()
  }

  function clear() {
    const overlay = overlayRef.value
    overlay?.getContext('2d')?.clearRect(0, 0, overlay.width, overlay.height)
  }

  function start() {
    clear()
  }

  function stop() {
    clear()
  }

  return { overlayRef, showBoxes, matches, setShow, start, stop, refresh: () => {} }
}
