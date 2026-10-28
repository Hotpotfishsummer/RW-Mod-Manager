export interface ModItem {
  id: string
  fileName: string
  filePath: string
  isDir: boolean
  enabled: boolean
  title: string
  titleZh: string
  description: string
  minVersion: string
  tags: string[]
  thumbnail: string
  unitCount: number
  mapCount: number
  fileSize: number
  steamId: string
  modTime: number
}

export interface MapItem {
  id: string
  fileName: string
  filePath: string
  isPack: boolean
  packName: string
  title: string
  width: number
  height: number
  playerCount: number
  hasPreview: boolean
  previewData: string
  fileSize: number
}

export interface GameStatus {
  gameDir: string
  isValid: boolean
  version: string
  totalMods: number
  enabledMods: number
  disabledMods: number
  totalMaps: number
}

export interface LaunchConfig {
  executableType: 'exe64' | 'exe32' | 'java'
  maxMemoryMb: number
  fullscreen: boolean
  noMods: boolean
  safeMode: boolean
  extraArgs: string
}

export interface AppConfig {
  gameDir: string
  launchConfig: LaunchConfig
}
