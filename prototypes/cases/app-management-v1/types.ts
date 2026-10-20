export type AppType = 'user' | 'system'
export type AppStatus = 'enabled' | 'disabled'

export interface AppItem {
  packageName: string
  name: string
  iconColor: string
  type: AppType
  status: AppStatus
  versionName: string
  versionCode: number
  sizeBytes: number
  apkPath: string
  installTime: string
  updateTime: string
  minSdk: number
  targetSdk: number
  permissions: string[]
}

export interface DragFileCandidate {
  name: string
  size: number
}
