export interface DependencyInfo {
  id: string
  name: string
  purpose: string
  installed: boolean
  path: string
  version: string
  error: string
  download_url: string
}
export interface DependencyPaths { adb: string; python: string }
export interface DependencyStatus { tools: DependencyInfo[]; paths: DependencyPaths; configurable: boolean }
