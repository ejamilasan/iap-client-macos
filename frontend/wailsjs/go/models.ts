export namespace config {
	
	export class AppSettings {
	    defaultProject?: string;
	    autoConnect: boolean;
	    showWindowsOnly: boolean;
	    rememberPasswords: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AppSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.defaultProject = source["defaultProject"];
	        this.autoConnect = source["autoConnect"];
	        this.showWindowsOnly = source["showWindowsOnly"];
	        this.rememberPasswords = source["rememberPasswords"];
	    }
	}
	export class RDPSettings {
	    fullScreen: boolean;
	    screenWidth: number;
	    screenHeight: number;
	    colorDepth: number;
	    audioMode: number;
	    driveRedirect: boolean;
	    clipboardShare: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RDPSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fullScreen = source["fullScreen"];
	        this.screenWidth = source["screenWidth"];
	        this.screenHeight = source["screenHeight"];
	        this.colorDepth = source["colorDepth"];
	        this.audioMode = source["audioMode"];
	        this.driveRedirect = source["driveRedirect"];
	        this.clipboardShare = source["clipboardShare"];
	    }
	}
	export class Connection {
	    id: string;
	    name: string;
	    project: string;
	    zone: string;
	    instance: string;
	    username: string;
	    domain?: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	    // Go type: time
	    lastUsedAt?: any;
	    rdpSettings?: RDPSettings;
	
	    static createFrom(source: any = {}) {
	        return new Connection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.project = source["project"];
	        this.zone = source["zone"];
	        this.instance = source["instance"];
	        this.username = source["username"];
	        this.domain = source["domain"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.lastUsedAt = this.convertValues(source["lastUsedAt"], null);
	        this.rdpSettings = this.convertValues(source["rdpSettings"], RDPSettings);
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

}

export namespace gcp {
	
	export class Instance {
	    name: string;
	    zone: string;
	    status: string;
	    machineType: string;
	    internalIP: string;
	    externalIP: string;
	    project: string;
	    isWindows: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Instance(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.zone = source["zone"];
	        this.status = source["status"];
	        this.machineType = source["machineType"];
	        this.internalIP = source["internalIP"];
	        this.externalIP = source["externalIP"];
	        this.project = source["project"];
	        this.isWindows = source["isWindows"];
	    }
	}
	export class Project {
	    id: string;
	    name: string;
	    number: number;
	
	    static createFrom(source: any = {}) {
	        return new Project(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.number = source["number"];
	    }
	}

}

