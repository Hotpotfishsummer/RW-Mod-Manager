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
  lanConfig: LanConfig
}

export type LanRole = 'client' | 'server'

export interface NetworkInterfaceInfo {
  name: string
  ip: string
  mask: string
  subnet: string
  broadcastIp: string
  isDefault: boolean
}

export interface LanPeer {
  instanceId: string
  deviceName: string
  ip: string
  httpPort: number
  role: LanRole
  presetVersion: string
  appVersion?: string
  lastSeen: number
  isConflict: boolean
}

export interface LanConfig {
  instanceId: string
  deviceName: string
  role: LanRole
  selectedSubnet: string
  broadcastPort: number
  httpPort: number
  autoAcceptClientPushes: boolean
}

export interface PresetModItem {
  fileName: string
  title: string
  hash: string
  fileSize: number
  enabled: boolean
  isDir: boolean
}

export interface PresetMapItem {
  fileName: string
  title: string
  filePath: string
  locationType: 'standalone' | 'map_pack' | 'unit_mod'
  active: boolean
  fileSize: number
  parentModFile: string
}

export interface LaunchPreset {
  id: string
  name: string
  updatedAt: number
  versionHash: string
  mods: PresetModItem[]
  maps: PresetMapItem[]
}

export interface SyncDiff {
  serverPresetHash: string
  clientPresetHash: string
  isInSync: boolean
  missingMods: PresetModItem[]
  missingMaps: PresetMapItem[]
  stateMismatches: PresetModItem[]
  extraModsCount: number
  summary: string
}

export interface PushRequest {
  id: string
  peerId: string
  peerName: string
  peerIp: string
  itemType: 'mod' | 'map'
  fileName: string
  fileSize: number
  status: 'pending' | 'accepted' | 'rejected' | 'completed'
  createdAt: number
}

export interface LanStatus {
  role: LanRole
  instanceId: string
  deviceName: string
  selectedSubnet: string
  localIp: string
  httpPort: number
  hasServerConflict: boolean
  conflictingServers: LanPeer[]
  activeServer: LanPeer | null
  peerCount: number
  presetVersionHash: string
  appVersion?: string
  autoAcceptPushes: boolean
}

export type UpdateSource = 'lan' | 'github'

export interface UpdateInfo {
  hasUpdate: boolean
  currentVersion: string
  latestVersion: string
  source: UpdateSource
  providerName: string
  downloadUrl: string
  releaseNotes: string
  fileSize: number
  sha256?: string
  publishedAt: string
  lanAvailable: boolean
  lanVersion: string
  githubAvailable: boolean
  githubVersion: string
}

export interface UpdateProgress {
  status: 'checking' | 'downloading' | 'installing' | 'ready' | 'error'
  percentage: number
  downloaded: number
  total: number
  speedBytes: number
  message: string
  error?: string
}
