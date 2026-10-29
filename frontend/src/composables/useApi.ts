import {
  ModItem,
  MapItem,
  GameStatus,
  AppConfig,
  GamePreferences,
  LanConfig,
  LanStatus,
  LanPeer,
  NetworkInterfaceInfo,
  LaunchPreset,
  SyncDiff,
  PushRequest,
  UpdateInfo,
  UpdateProgress
} from '../types/api'

declare global {
  interface Window {
    go?: {
      main: {
        App: {
          GetConfig(): Promise<AppConfig>
          SaveConfig(cfg: AppConfig): Promise<void>
          SelectGameDir(): Promise<string>
          SetGameDir(dir: string): Promise<GameStatus>
          GetGameStatus(): Promise<GameStatus>
          ScanMods(): Promise<ModItem[]>
          ToggleMod(fileName: string, enabled: boolean): Promise<void>
          BatchToggleMods(enableAll: boolean): Promise<void>
          ScanMaps(): Promise<MapItem[]>
          ActivateMapParentMod(parentModFile: string): Promise<void>
          GenerateMapThumbnail(tmxPath: string): Promise<string>
          BatchGenerateThumbnails(force: boolean): Promise<number>
          GetGamePreferences(): Promise<GamePreferences>
          SaveGamePreferences(prefs: GamePreferences): Promise<void>
          LaunchGame(): Promise<void>
          LaunchGameOnline(): Promise<void>
          LaunchGameSingle(): Promise<void>
          OpenFolder(folderType: string): Promise<void>
          GetLanConfig(): Promise<LanConfig>
          SaveLanConfig(cfg: LanConfig): Promise<void>
          GetNetworkInterfaces(): Promise<NetworkInterfaceInfo[]>
          SwitchLanRole(role: string): Promise<void>
          GetLanStatus(): Promise<LanStatus>
          GetDiscoveredPeers(): Promise<LanPeer[]>
          GetDynamicPreset(): Promise<LaunchPreset>
          RefreshDynamicPreset(): Promise<LaunchPreset>
          ComparePresetWithServer(serverIP: string, serverPort: number): Promise<SyncDiff>
          SyncFromHost(serverIP: string, serverPort: number): Promise<void>
          RequestPushToHost(serverIP: string, serverPort: number, itemType: string, fileName: string): Promise<void>
          HandlePushDecision(requestID: string, accept: boolean): Promise<void>
          GetPendingPushRequests(): Promise<PushRequest[]>
          CheckForUpdates(): Promise<UpdateInfo>
          DownloadAndApplyUpdate(source: string): Promise<void>
          GetCurrentVersion(): Promise<string>
        }
      }
    }
  }
}

const isWails = () => typeof window !== 'undefined' && !!window.go?.main?.App

export function useApi() {
  const getConfig = async (): Promise<AppConfig> => {
    if (isWails()) return window.go!.main.App.GetConfig()
    return {
      gameDir: 'E:\\Rusted Warfare\\Rusted Warfare',
      launchConfig: {
        executableType: 'exe64',
        maxMemoryMb: 4096,
        fullscreen: false,
        noMods: false,
        safeMode: false,
        extraArgs: ''
      },
      gamePreferences: {
        highRefreshRate: true,
        renderVsync: true,
        showFps: true,
        renderSmoothDelta: false,
        renderAntiAlias: true,
        renderFancyWater: false,
        renderClouds: true,
        renderExtraShadows: true,
        teamShaders: true,
        newRender: false,
        teamUnitCapSinglePlayer: 5000,
        teamUnitCapHostedGame: 2000,
        uiRenderScale: 1.0
      },
      lanConfig: {
        instanceId: 'mock-local-client-1',
        deviceName: 'Local-Player-PC',
        role: 'client',
        selectedSubnet: '',
        broadcastPort: 38567,
        httpPort: 38568,
        autoAcceptClientPushes: false
      }
    }
  }

  const saveConfig = async (cfg: AppConfig): Promise<void> => {
    if (isWails()) return window.go!.main.App.SaveConfig(cfg)
    console.log('Mock saveConfig:', cfg)
  }

  const selectGameDir = async (): Promise<string> => {
    if (isWails()) return window.go!.main.App.SelectGameDir()
    return 'E:\\Rusted Warfare\\Rusted Warfare'
  }

  const setGameDir = async (dir: string): Promise<GameStatus> => {
    if (isWails()) return window.go!.main.App.SetGameDir(dir)
    return {
      gameDir: dir,
      isValid: true,
      version: '1.15',
      totalMods: 19,
      enabledMods: 3,
      disabledMods: 16,
      totalMaps: 306,
      activeMaps: 163
    }
  }

  const getGameStatus = async (): Promise<GameStatus> => {
    if (isWails()) return window.go!.main.App.GetGameStatus()
    return {
      gameDir: 'E:\\Rusted Warfare\\Rusted Warfare',
      isValid: true,
      version: '1.15',
      totalMods: 19,
      enabledMods: 3,
      disabledMods: 16,
      totalMaps: 306,
      activeMaps: 163
    }
  }

  const scanMods = async (): Promise<ModItem[]> => {
    if (isWails()) return window.go!.main.App.ScanMods()
    return []
  }

  const toggleMod = async (fileName: string, enabled: boolean): Promise<void> => {
    if (isWails()) return window.go!.main.App.ToggleMod(fileName, enabled)
    console.log(`Mock toggleMod: ${fileName} -> ${enabled}`)
  }

  const batchToggleMods = async (enableAll: boolean): Promise<void> => {
    if (isWails()) return window.go!.main.App.BatchToggleMods(enableAll)
    console.log(`Mock batchToggleMods: ${enableAll}`)
  }

  const scanMaps = async (): Promise<MapItem[]> => {
    if (isWails()) return window.go!.main.App.ScanMaps()
    return []
  }

  const activateMapParentMod = async (parentModFile: string): Promise<void> => {
    if (isWails()) return window.go!.main.App.ActivateMapParentMod(parentModFile)
    console.log(`Mock activateMapParentMod: ${parentModFile}`)
  }

  const generateMapThumbnail = async (tmxPath: string): Promise<string> => {
    if (isWails()) return window.go!.main.App.GenerateMapThumbnail(tmxPath)
    return ''
  }

  const batchGenerateThumbnails = async (force: boolean = false): Promise<number> => {
    if (isWails()) return window.go!.main.App.BatchGenerateThumbnails(force)
    return 0
  }

  const getGamePreferences = async (): Promise<GamePreferences> => {
    if (isWails()) return window.go!.main.App.GetGamePreferences()
    return {
      highRefreshRate: true,
      renderVsync: true,
      showFps: true,
      renderSmoothDelta: false,
      renderAntiAlias: true,
      renderFancyWater: false,
      renderClouds: true,
      renderExtraShadows: true,
      teamShaders: true,
      newRender: false,
      teamUnitCapSinglePlayer: 5000,
      teamUnitCapHostedGame: 2000,
      uiRenderScale: 1.0
    }
  }

  const saveGamePreferences = async (prefs: GamePreferences): Promise<void> => {
    if (isWails()) return window.go!.main.App.SaveGamePreferences(prefs)
    console.log('Mock saveGamePreferences:', prefs)
  }

  const launchGame = async (): Promise<void> => {
    if (isWails()) return window.go!.main.App.LaunchGame()
    console.log('Mock launchGame')
  }

  const launchGameOnline = async (): Promise<void> => {
    if (isWails()) return window.go!.main.App.LaunchGameOnline()
    console.log('Mock launchGameOnline')
  }

  const launchGameSingle = async (): Promise<void> => {
    if (isWails()) return window.go!.main.App.LaunchGameSingle()
    console.log('Mock launchGameSingle')
  }

  const openFolder = async (folderType: string): Promise<void> => {
    if (isWails()) return window.go!.main.App.OpenFolder(folderType)
    console.log(`Mock openFolder: ${folderType}`)
  }

  // ================= 局域网与同步相关方法 =================

  const getLanConfig = async (): Promise<LanConfig> => {
    if (isWails()) return window.go!.main.App.GetLanConfig()
    return {
      instanceId: 'mock-instance-1',
      deviceName: 'Local-Player',
      role: 'client',
      selectedSubnet: '',
      broadcastPort: 38567,
      httpPort: 38568,
      autoAcceptClientPushes: false
    }
  }

  const saveLanConfig = async (cfg: LanConfig): Promise<void> => {
    if (isWails()) return window.go!.main.App.SaveLanConfig(cfg)
    console.log('Mock saveLanConfig:', cfg)
  }

  const getNetworkInterfaces = async (): Promise<NetworkInterfaceInfo[]> => {
    if (isWails()) {
      const res = await window.go!.main.App.GetNetworkInterfaces()
      return res || []
    }
    return [
      {
        name: '以太网 (Ethernet)',
        ip: '192.168.1.100',
        mask: '255.255.255.0',
        subnet: '192.168.1.0/24',
        broadcastIp: '192.168.1.255',
        isDefault: true
      },
      {
        name: '虚拟专网 (Tailscale/Radmin)',
        ip: '26.12.34.56',
        mask: '255.0.0.0',
        subnet: '26.0.0.0/8',
        broadcastIp: '26.255.255.255',
        isDefault: false
      }
    ]
  }

  const switchLanRole = async (role: string): Promise<void> => {
    if (isWails()) return window.go!.main.App.SwitchLanRole(role)
    console.log('Mock switchLanRole:', role)
  }

  const getLanStatus = async (): Promise<LanStatus> => {
    if (isWails()) {
      const res = await window.go!.main.App.GetLanStatus()
      if (res) {
        res.conflictingServers = res.conflictingServers || []
      }
      return res
    }
    return {
      role: 'client',
      instanceId: 'mock-client-1',
      deviceName: 'Player-PC',
      selectedSubnet: '',
      localIp: '192.168.1.100',
      httpPort: 38568,
      hasServerConflict: false,
      conflictingServers: [],
      activeServer: {
        instanceId: 'mock-server-host',
        deviceName: 'Host-Server-PC',
        ip: '192.168.1.188',
        httpPort: 38568,
        role: 'server',
        presetVersion: 'BF0941BE3D9D3A6F',
        lastSeen: Date.now() / 1000,
        isConflict: false
      },
      peerCount: 2,
      presetVersionHash: 'BF0941BE3D9D3A6F',
      autoAcceptPushes: false
    }
  }

  const getDiscoveredPeers = async (): Promise<LanPeer[]> => {
    if (isWails()) {
      const res = await window.go!.main.App.GetDiscoveredPeers()
      return res || []
    }
    return [
      {
        instanceId: 'mock-server-host',
        deviceName: 'Host-Server-PC (房主)',
        ip: '192.168.1.188',
        httpPort: 38568,
        role: 'server',
        presetVersion: 'BF0941BE3D9D3A6F',
        lastSeen: Date.now() / 1000,
        isConflict: false
      },
      {
        instanceId: 'mock-client-friend',
        deviceName: 'Teammate-PC',
        ip: '192.168.1.105',
        httpPort: 38568,
        role: 'client',
        presetVersion: 'BF0941BE3D9D3A6F',
        lastSeen: Date.now() / 1000,
        isConflict: false
      }
    ]
  }

  const getDynamicPreset = async (): Promise<LaunchPreset> => {
    if (isWails()) {
      const res = await window.go!.main.App.GetDynamicPreset()
      if (res) {
        res.mods = res.mods || []
        res.maps = res.maps || []
      }
      return res
    }
    return {
      id: 'dynamic',
      name: '动态启动预设',
      updatedAt: Date.now() / 1000,
      versionHash: 'BF0941BE3D9D3A6F',
      mods: [],
      maps: []
    }
  }

  const refreshDynamicPreset = async (): Promise<LaunchPreset> => {
    if (isWails()) {
      const res = await window.go!.main.App.RefreshDynamicPreset()
      if (res) {
        res.mods = res.mods || []
        res.maps = res.maps || []
      }
      return res
    }
    return getDynamicPreset()
  }

  const comparePresetWithServer = async (serverIP: string, serverPort: number): Promise<SyncDiff> => {
    if (isWails()) {
      const res = await window.go!.main.App.ComparePresetWithServer(serverIP, serverPort)
      if (res) {
        res.missingMods = res.missingMods || []
        res.missingMaps = res.missingMaps || []
        res.stateMismatches = res.stateMismatches || []
      }
      return res
    }
    return {
      serverPresetHash: 'BF0941BE3D9D3A6F',
      clientPresetHash: 'BF0941BE3D9D3A6F',
      isInSync: true,
      missingMods: [],
      missingMaps: [],
      stateMismatches: [],
      extraModsCount: 0,
      summary: '启动预设与房主服务端完全一致，已就绪'
    }
  }

  const syncFromHost = async (serverIP: string, serverPort: number): Promise<void> => {
    if (isWails()) return window.go!.main.App.SyncFromHost(serverIP, serverPort)
    console.log(`Mock syncFromHost from ${serverIP}:${serverPort}`)
  }

  const requestPushToHost = async (
    serverIP: string,
    serverPort: number,
    itemType: string,
    fileName: string
  ): Promise<void> => {
    if (isWails()) return window.go!.main.App.RequestPushToHost(serverIP, serverPort, itemType, fileName)
    console.log(`Mock requestPushToHost: ${itemType} ${fileName} to ${serverIP}:${serverPort}`)
  }

  const handlePushDecision = async (requestID: string, accept: boolean): Promise<void> => {
    if (isWails()) return window.go!.main.App.HandlePushDecision(requestID, accept)
    console.log(`Mock handlePushDecision: ${requestID} -> ${accept}`)
  }

  const getPendingPushRequests = async (): Promise<PushRequest[]> => {
    if (isWails()) {
      const res = await window.go!.main.App.GetPendingPushRequests()
      return res || []
    }
    return []
  }

  const checkForUpdates = async (): Promise<UpdateInfo> => {
    if (isWails()) return window.go!.main.App.CheckForUpdates()
    return {
      hasUpdate: false,
      currentVersion: '1.0.0',
      latestVersion: '1.0.0',
      source: 'lan',
      providerName: '本地测试',
      downloadUrl: '',
      releaseNotes: '当前已是最新版本',
      fileSize: 0,
      publishedAt: '刚刚',
      lanAvailable: false,
      lanVersion: '1.0.0',
      githubAvailable: false,
      githubVersion: '1.0.0'
    }
  }

  const downloadAndApplyUpdate = async (source: string): Promise<void> => {
    if (isWails()) return window.go!.main.App.DownloadAndApplyUpdate(source)
    console.log('Mock downloadAndApplyUpdate:', source)
  }

  const getCurrentVersion = async (): Promise<string> => {
    if (isWails()) return window.go!.main.App.GetCurrentVersion()
    return '1.0.0'
  }

  return {
    getConfig,
    saveConfig,
    selectGameDir,
    setGameDir,
    getGameStatus,
    scanMods,
    toggleMod,
    batchToggleMods,
    scanMaps,
    activateMapParentMod,
    generateMapThumbnail,
    batchGenerateThumbnails,
    getGamePreferences,
    saveGamePreferences,
    launchGame,
    launchGameOnline,
    launchGameSingle,
    openFolder,
    // LAN & Sync
    getLanConfig,
    saveLanConfig,
    getNetworkInterfaces,
    switchLanRole,
    getLanStatus,
    getDiscoveredPeers,
    getDynamicPreset,
    refreshDynamicPreset,
    comparePresetWithServer,
    syncFromHost,
    requestPushToHost,
    handlePushDecision,
    getPendingPushRequests,
    // Updater
    checkForUpdates,
    downloadAndApplyUpdate,
    getCurrentVersion
  }
}
