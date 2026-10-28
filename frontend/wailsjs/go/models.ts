export namespace models {
	
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
	
	    static createFrom(source: any = {}) {
	        return new AppConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gameDir = source["gameDir"];
	        this.launchConfig = this.convertValues(source["launchConfig"], LaunchConfig);
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
	    }
	}
	
	export class MapItem {
	    id: string;
	    fileName: string;
	    filePath: string;
	    isPack: boolean;
	    packName: string;
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

}

