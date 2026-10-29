export namespace models {
	
	export class LanConfig {
	    instanceId: string;
	    deviceName: string;
	    role: string;
	    selectedSubnet: string;
	    broadcastPort: number;
	    httpPort: number;
	    autoAcceptClientPushes: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LanConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.instanceId = source["instanceId"];
	        this.deviceName = source["deviceName"];
	        this.role = source["role"];
	        this.selectedSubnet = source["selectedSubnet"];
	        this.broadcastPort = source["broadcastPort"];
	        this.httpPort = source["httpPort"];
	        this.autoAcceptClientPushes = source["autoAcceptClientPushes"];
	    }
	}
	export class GamePreferences {
	    highRefreshRate: boolean;
	    renderVsync: boolean;
	    showFps: boolean;
	    renderSmoothDelta: boolean;
	    renderAntiAlias: boolean;
	    renderFancyWater: boolean;
	    renderClouds: boolean;
	    renderExtraShadows: boolean;
	    teamShaders: boolean;
	    newRender: boolean;
	    teamUnitCapSinglePlayer: number;
	    teamUnitCapHostedGame: number;
	    uiRenderScale: number;
	
	    static createFrom(source: any = {}) {
	        return new GamePreferences(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.highRefreshRate = source["highRefreshRate"];
	        this.renderVsync = source["renderVsync"];
	        this.showFps = source["showFps"];
	        this.renderSmoothDelta = source["renderSmoothDelta"];
	        this.renderAntiAlias = source["renderAntiAlias"];
	        this.renderFancyWater = source["renderFancyWater"];
	        this.renderClouds = source["renderClouds"];
	        this.renderExtraShadows = source["renderExtraShadows"];
	        this.teamShaders = source["teamShaders"];
	        this.newRender = source["newRender"];
	        this.teamUnitCapSinglePlayer = source["teamUnitCapSinglePlayer"];
	        this.teamUnitCapHostedGame = source["teamUnitCapHostedGame"];
	        this.uiRenderScale = source["uiRenderScale"];
	    }
	}
	export class LaunchConfig {
	    executableType: string;
	    maxMemoryMb: number;
	    fullscreen: boolean;
	    noMods: boolean;
	    safeMode: boolean;
	    extraArgs: string;
	
	    static createFrom(source: any = {}) {
	        return new LaunchConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.executableType = source["executableType"];
	        this.maxMemoryMb = source["maxMemoryMb"];
	        this.fullscreen = source["fullscreen"];
	        this.noMods = source["noMods"];
	        this.safeMode = source["safeMode"];
	        this.extraArgs = source["extraArgs"];
	    }
	}
	export class AppConfig {
	    gameDir: string;
	    launchConfig: LaunchConfig;
	    gamePreferences: GamePreferences;
	    lanConfig: LanConfig;
	
	    static createFrom(source: any = {}) {
	        return new AppConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gameDir = source["gameDir"];
	        this.launchConfig = this.convertValues(source["launchConfig"], LaunchConfig);
	        this.gamePreferences = this.convertValues(source["gamePreferences"], GamePreferences);
	        this.lanConfig = this.convertValues(source["lanConfig"], LanConfig);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class GameStatus {
	    gameDir: string;
	    isValid: boolean;
	    version: string;
	    totalMods: number;
	    enabledMods: number;
	    disabledMods: number;
	    totalMaps: number;
	    activeMaps: number;
	
	    static createFrom(source: any = {}) {
	        return new GameStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gameDir = source["gameDir"];
	        this.isValid = source["isValid"];
	        this.version = source["version"];
	        this.totalMods = source["totalMods"];
	        this.enabledMods = source["enabledMods"];
	        this.disabledMods = source["disabledMods"];
	        this.totalMaps = source["totalMaps"];
	        this.activeMaps = source["activeMaps"];
	    }
	}
	
	export class LanPeer {
	    instanceId: string;
	    deviceName: string;
	    ip: string;
	    httpPort: number;
	    role: string;
	    presetVersion: string;
	    appVersion: string;
	    lastSeen: number;
	    isConflict: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LanPeer(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.instanceId = source["instanceId"];
	        this.deviceName = source["deviceName"];
	        this.ip = source["ip"];
	        this.httpPort = source["httpPort"];
	        this.role = source["role"];
	        this.presetVersion = source["presetVersion"];
	        this.appVersion = source["appVersion"];
	        this.lastSeen = source["lastSeen"];
	        this.isConflict = source["isConflict"];
	    }
	}
	export class LanStatus {
	    role: string;
	    instanceId: string;
	    deviceName: string;
	    selectedSubnet: string;
	    localIp: string;
	    httpPort: number;
	    hasServerConflict: boolean;
	    conflictingServers: LanPeer[];
	    activeServer?: LanPeer;
	    peerCount: number;
	    presetVersionHash: string;
	    appVersion: string;
	    autoAcceptPushes: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LanStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.role = source["role"];
	        this.instanceId = source["instanceId"];
	        this.deviceName = source["deviceName"];
	        this.selectedSubnet = source["selectedSubnet"];
	        this.localIp = source["localIp"];
	        this.httpPort = source["httpPort"];
	        this.hasServerConflict = source["hasServerConflict"];
	        this.conflictingServers = this.convertValues(source["conflictingServers"], LanPeer);
	        this.activeServer = this.convertValues(source["activeServer"], LanPeer);
	        this.peerCount = source["peerCount"];
	        this.presetVersionHash = source["presetVersionHash"];
	        this.appVersion = source["appVersion"];
	        this.autoAcceptPushes = source["autoAcceptPushes"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class PresetMapItem {
	    fileName: string;
	    title: string;
	    filePath: string;
	    locationType: string;
	    active: boolean;
	    fileSize: number;
	    parentModFile: string;
	
	    static createFrom(source: any = {}) {
	        return new PresetMapItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fileName = source["fileName"];
	        this.title = source["title"];
	        this.filePath = source["filePath"];
	        this.locationType = source["locationType"];
	        this.active = source["active"];
	        this.fileSize = source["fileSize"];
	        this.parentModFile = source["parentModFile"];
	    }
	}
	export class PresetModItem {
	    fileName: string;
	    title: string;
	    hash: string;
	    fileSize: number;
	    enabled: boolean;
	    isDir: boolean;
	
	    static createFrom(source: any = {}) {
	        return new PresetModItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fileName = source["fileName"];
	        this.title = source["title"];
	        this.hash = source["hash"];
	        this.fileSize = source["fileSize"];
	        this.enabled = source["enabled"];
	        this.isDir = source["isDir"];
	    }
	}
	export class LaunchPreset {
	    id: string;
	    name: string;
	    updatedAt: number;
	    versionHash: string;
	    mods: PresetModItem[];
	    maps: PresetMapItem[];
	
	    static createFrom(source: any = {}) {
	        return new LaunchPreset(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.updatedAt = source["updatedAt"];
	        this.versionHash = source["versionHash"];
	        this.mods = this.convertValues(source["mods"], PresetModItem);
	        this.maps = this.convertValues(source["maps"], PresetMapItem);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class MapItem {
	    id: string;
	    fileName: string;
	    filePath: string;
	    isPack: boolean;
	    packName: string;
	    parentModFile: string;
	    locationType: string;
	    active: boolean;
	    title: string;
	    width: number;
	    height: number;
	    playerCount: number;
	    hasPreview: boolean;
	    previewData: string;
	    fileSize: number;
	
	    static createFrom(source: any = {}) {
	        return new MapItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.fileName = source["fileName"];
	        this.filePath = source["filePath"];
	        this.isPack = source["isPack"];
	        this.packName = source["packName"];
	        this.parentModFile = source["parentModFile"];
	        this.locationType = source["locationType"];
	        this.active = source["active"];
	        this.title = source["title"];
	        this.width = source["width"];
	        this.height = source["height"];
	        this.playerCount = source["playerCount"];
	        this.hasPreview = source["hasPreview"];
	        this.previewData = source["previewData"];
	        this.fileSize = source["fileSize"];
	    }
	}
	export class ModItem {
	    id: string;
	    fileName: string;
	    filePath: string;
	    isDir: boolean;
	    enabled: boolean;
	    title: string;
	    titleZh: string;
	    description: string;
	    minVersion: string;
	    tags: string[];
	    thumbnail: string;
	    unitCount: number;
	    mapCount: number;
	    fileSize: number;
	    steamId: string;
	    modTime: number;
	
	    static createFrom(source: any = {}) {
	        return new ModItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.fileName = source["fileName"];
	        this.filePath = source["filePath"];
	        this.isDir = source["isDir"];
	        this.enabled = source["enabled"];
	        this.title = source["title"];
	        this.titleZh = source["titleZh"];
	        this.description = source["description"];
	        this.minVersion = source["minVersion"];
	        this.tags = source["tags"];
	        this.thumbnail = source["thumbnail"];
	        this.unitCount = source["unitCount"];
	        this.mapCount = source["mapCount"];
	        this.fileSize = source["fileSize"];
	        this.steamId = source["steamId"];
	        this.modTime = source["modTime"];
	    }
	}
	export class NetworkInterfaceInfo {
	    name: string;
	    ip: string;
	    mask: string;
	    subnet: string;
	    broadcastIp: string;
	    isDefault: boolean;
	
	    static createFrom(source: any = {}) {
	        return new NetworkInterfaceInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.ip = source["ip"];
	        this.mask = source["mask"];
	        this.subnet = source["subnet"];
	        this.broadcastIp = source["broadcastIp"];
	        this.isDefault = source["isDefault"];
	    }
	}
	
	
	export class PushRequest {
	    id: string;
	    peerId: string;
	    peerName: string;
	    peerIp: string;
	    itemType: string;
	    fileName: string;
	    fileSize: number;
	    status: string;
	    createdAt: number;
	
	    static createFrom(source: any = {}) {
	        return new PushRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.peerId = source["peerId"];
	        this.peerName = source["peerName"];
	        this.peerIp = source["peerIp"];
	        this.itemType = source["itemType"];
	        this.fileName = source["fileName"];
	        this.fileSize = source["fileSize"];
	        this.status = source["status"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class SyncDiff {
	    serverPresetHash: string;
	    clientPresetHash: string;
	    isInSync: boolean;
	    missingMods: PresetModItem[];
	    missingMaps: PresetMapItem[];
	    stateMismatches: PresetModItem[];
	    extraModsCount: number;
	    summary: string;
	
	    static createFrom(source: any = {}) {
	        return new SyncDiff(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.serverPresetHash = source["serverPresetHash"];
	        this.clientPresetHash = source["clientPresetHash"];
	        this.isInSync = source["isInSync"];
	        this.missingMods = this.convertValues(source["missingMods"], PresetModItem);
	        this.missingMaps = this.convertValues(source["missingMaps"], PresetMapItem);
	        this.stateMismatches = this.convertValues(source["stateMismatches"], PresetModItem);
	        this.extraModsCount = source["extraModsCount"];
	        this.summary = source["summary"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class UpdateInfo {
	    hasUpdate: boolean;
	    currentVersion: string;
	    latestVersion: string;
	    source: string;
	    providerName: string;
	    downloadUrl: string;
	    releaseNotes: string;
	    fileSize: number;
	    sha256: string;
	    publishedAt: string;
	    lanAvailable: boolean;
	    lanVersion: string;
	    githubAvailable: boolean;
	    githubVersion: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hasUpdate = source["hasUpdate"];
	        this.currentVersion = source["currentVersion"];
	        this.latestVersion = source["latestVersion"];
	        this.source = source["source"];
	        this.providerName = source["providerName"];
	        this.downloadUrl = source["downloadUrl"];
	        this.releaseNotes = source["releaseNotes"];
	        this.fileSize = source["fileSize"];
	        this.sha256 = source["sha256"];
	        this.publishedAt = source["publishedAt"];
	        this.lanAvailable = source["lanAvailable"];
	        this.lanVersion = source["lanVersion"];
	        this.githubAvailable = source["githubAvailable"];
	        this.githubVersion = source["githubVersion"];
	    }
	}

}

