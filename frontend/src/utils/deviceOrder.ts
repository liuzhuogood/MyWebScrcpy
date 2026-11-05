export const DEVICE_ORDER_KEY = 'home_device_order'
export function readDeviceOrder(): string[] {
 try {
  const value: unknown = JSON.parse(localStorage.getItem(DEVICE_ORDER_KEY) ?? '[]')
  return Array.isArray(value) ? [...new Set(value.filter((v): v is string => typeof v === 'string'))] : []
 }
 catch (error) { console.warn('[device-order] Read preference failed', error); return [] }
}
export function orderDevices<T extends {serial: string}>(devices: T[], order: string[]): T[] {
 const positions = new Map(order.map((serial, index) => [serial, index]))
 return [...devices].sort((a,b) => {
  const ai = positions.get(a.serial), bi = positions.get(b.serial)
  if (ai !== undefined || bi !== undefined) return (ai ?? Infinity) - (bi ?? Infinity)
  return a.serial.localeCompare(b.serial)
 })
}
// Replace only visible slots so hidden/offline devices retain their positions.
export function reorderVisibleDevices(all: string[], visible: string[]): string[] {
 const set = new Set(visible)
 let index = 0
 return all.map(serial => set.has(serial) ? visible[index++]! : serial)
}
export function buildNumberMap(devices: {serial: string}[]): Map<string,string> {
 return new Map(devices.map((device,index)=>[device.serial,String(index+1).padStart(2,'0')]))
}
