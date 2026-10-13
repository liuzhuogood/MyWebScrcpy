import { useColorMode } from '@vueuse/core'

export const theme = useColorMode({ storageKey: 'mywebscrcpy-theme', initialValue: 'auto', emitAuto: true, disableTransition: true })
