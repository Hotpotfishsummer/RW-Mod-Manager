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
  parentModFile: string
  locationType: 'standalone' | 'map_pack' | 'unit_mod'
  active: boolean
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
  activeMaps: number
}

export interface LaunchConfig {
  executableType: 'exe64' | 'exe32' | 'java'
  maxMemoryMb: number
  fullscreen: boolean
  noMods: boolean
  safeMode: boolean
  extraArgs: string
}

export interface GamePreferences {
  highRefreshRate: boolean
  renderVsync: boolean
  showFps: boolean
  renderSmoothDelta: boolean
  renderAntiAlias: boolean
  renderFancyWater: boolean
  renderClouds: boolean
  renderExtraShadows: boolean
  teamShaders: boolean
  newRender: boolean
  teamUnitCapSinglePlayer: number
  teamUnitCapHostedGame: number
  uiRenderScale: number
}

export interface AppConfig {
  gameDir: string
  launchConfig: LaunchConfig
  gamePreferences: GamePreferences
}
