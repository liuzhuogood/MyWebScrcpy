export interface DependencyInfo {
  id: string
  name: string
  purpose: string
  installed: boolean
  path: string
  version: string
  error: string
  download_url: string
  python?: { pip_available: boolean; sdk_version: string; sdk_error: string; required_version: string; ready: boolean }
}
export interface DependencyPaths { adb: string; python: string }
export interface DependencyStatus { tools: DependencyInfo[]; paths: DependencyPaths; configurable: boolean }
