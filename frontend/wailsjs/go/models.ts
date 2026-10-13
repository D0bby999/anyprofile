export namespace engine {
	
	export class Status {
	    engine: string;
	    version: string;
	    chromium: string;
	    installed: boolean;
	    problem: string;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.engine = source["engine"];
	        this.version = source["version"];
	        this.chromium = source["chromium"];
	        this.installed = source["installed"];
	        this.problem = source["problem"];
	    }
	}

}

export namespace fury {
	
	export class Persona {
	    id: string;
	    screen: string;
	    gpu: string;
	    weight: number;
	
	    static createFrom(source: any = {}) {
	        return new Persona(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.screen = source["screen"];
	        this.gpu = source["gpu"];
	        this.weight = source["weight"];
	    }
	}

}

export namespace netproxy {
	
	export class ParsedLine {
	    line: number;
	    display: string;
	    err: string;
	
	    static createFrom(source: any = {}) {
	        return new ParsedLine(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.line = source["line"];
	        this.display = source["display"];
	        this.err = source["err"];
	    }
	}
	export class Upstream {
	    Scheme: string;
	    Host: string;
	    Port: number;
	    Username: string;
	    Password: string;
	
	    static createFrom(source: any = {}) {
	        return new Upstream(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Scheme = source["Scheme"];
	        this.Host = source["Host"];
	        this.Port = source["Port"];
	        this.Username = source["Username"];
	        this.Password = source["Password"];
	    }
	}

}

export namespace service {
	
	export class ImportResult {
	    added: number;
	    failed: netproxy.ParsedLine[];
	
	    static createFrom(source: any = {}) {
	        return new ImportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.added = source["added"];
	        this.failed = this.convertValues(source["failed"], netproxy.ParsedLine);
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
	export class NewProfile {
	    name: string;
	    proxyId: string;
	    site: string;
	    startUrl: string;
	    notes: string;
	    tags: string[];
	
	    static createFrom(source: any = {}) {
	        return new NewProfile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.proxyId = source["proxyId"];
	        this.site = source["site"];
	        this.startUrl = source["startUrl"];
	        this.notes = source["notes"];
	        this.tags = source["tags"];
	    }
	}
	export class Orphan {
	    id: string;
	    bytes: number;
	
	    static createFrom(source: any = {}) {
	        return new Orphan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.bytes = source["bytes"];
	    }
	}
	export class ProfileRow {
	    id: string;
	    name: string;
	    engine: string;
	    engineVersion: string;
	    personaId: string;
	    seedHex: string;
	    platform: string;
	    proxyId: string;
	    timezone: string;
	    languages: string;
	    uiLocale: string;
	    lat?: number;
	    lon?: number;
	    site: string;
	    serial: number;
	    login: string;
	    hasPassword: boolean;
	    hasTotp: boolean;
	    startUrl: string;
	    notes: string;
	    tags: string[];
	    cdpNextLaunch: boolean;
	    pinned: boolean;
	    createdAt: string;
	    updatedAt: string;
	    lastOpenAt: string;
	    deletedAt: string;
	    state: string;
	    stateError: string;
	    cdpPort: number;
	    proxyLabel: string;
	    proxyCountry: string;
	    proxyExitIp: string;
	
	    static createFrom(source: any = {}) {
	        return new ProfileRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.engine = source["engine"];
	        this.engineVersion = source["engineVersion"];
	        this.personaId = source["personaId"];
	        this.seedHex = source["seedHex"];
	        this.platform = source["platform"];
	        this.proxyId = source["proxyId"];
	        this.timezone = source["timezone"];
	        this.languages = source["languages"];
	        this.uiLocale = source["uiLocale"];
	        this.lat = source["lat"];
	        this.lon = source["lon"];
	        this.site = source["site"];
	        this.serial = source["serial"];
	        this.login = source["login"];
	        this.hasPassword = source["hasPassword"];
	        this.hasTotp = source["hasTotp"];
	        this.startUrl = source["startUrl"];
	        this.notes = source["notes"];
	        this.tags = source["tags"];
	        this.cdpNextLaunch = source["cdpNextLaunch"];
	        this.pinned = source["pinned"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.lastOpenAt = source["lastOpenAt"];
	        this.deletedAt = source["deletedAt"];
	        this.state = source["state"];
	        this.stateError = source["stateError"];
	        this.cdpPort = source["cdpPort"];
	        this.proxyLabel = source["proxyLabel"];
	        this.proxyCountry = source["proxyCountry"];
	        this.proxyExitIp = source["proxyExitIp"];
	    }
	}
	export class ProxyRow {
	    id: string;
	    label: string;
	    scheme: string;
	    host: string;
	    port: number;
	    username: string;
	    exitIp: string;
	    country: string;
	    timezone: string;
	    lat?: number;
	    lon?: number;
	    rotating: boolean;
	    checkedAt: string;
	    createdAt: string;
	    usedBy: string[];
	
	    static createFrom(source: any = {}) {
	        return new ProxyRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.scheme = source["scheme"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.username = source["username"];
	        this.exitIp = source["exitIp"];
	        this.country = source["country"];
	        this.timezone = source["timezone"];
	        this.lat = source["lat"];
	        this.lon = source["lon"];
	        this.rotating = source["rotating"];
	        this.checkedAt = source["checkedAt"];
	        this.createdAt = source["createdAt"];
	        this.usedBy = source["usedBy"];
	    }
	}
	export class SiteInput {
	    name: string;
	    domains: string[];
	    startUrl: string;
	    color: string;
	
	    static createFrom(source: any = {}) {
	        return new SiteInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.domains = source["domains"];
	        this.startUrl = source["startUrl"];
	        this.color = source["color"];
	    }
	}
	export class StorageInfo {
	    profilesBytes: number;
	    orphans: Orphan[];
	
	    static createFrom(source: any = {}) {
	        return new StorageInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profilesBytes = source["profilesBytes"];
	        this.orphans = this.convertValues(source["orphans"], Orphan);
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
	export class TOTPCode {
	    code: string;
	    secondsLeft: number;
	
	    static createFrom(source: any = {}) {
	        return new TOTPCode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.code = source["code"];
	        this.secondsLeft = source["secondsLeft"];
	    }
	}

}

export namespace site {
	
	export class Site {
	    id: string;
	    name: string;
	    group: string;
	    domains: string[];
	    startUrl: string;
	    icon: string;
	    color: string;
	    custom: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Site(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.group = source["group"];
	        this.domains = source["domains"];
	        this.startUrl = source["startUrl"];
	        this.icon = source["icon"];
	        this.color = source["color"];
	        this.custom = source["custom"];
	    }
	}

}

export namespace store {
	
	export class Profile {
	    id: string;
	    name: string;
	    engine: string;
	    engineVersion: string;
	    personaId: string;
	    seedHex: string;
	    platform: string;
	    proxyId: string;
	    timezone: string;
	    languages: string;
	    uiLocale: string;
	    lat?: number;
	    lon?: number;
	    site: string;
	    serial: number;
	    login: string;
	    hasPassword: boolean;
	    hasTotp: boolean;
	    startUrl: string;
	    notes: string;
	    tags: string[];
	    cdpNextLaunch: boolean;
	    pinned: boolean;
	    createdAt: string;
	    updatedAt: string;
	    lastOpenAt: string;
	    deletedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Profile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.engine = source["engine"];
	        this.engineVersion = source["engineVersion"];
	        this.personaId = source["personaId"];
	        this.seedHex = source["seedHex"];
	        this.platform = source["platform"];
	        this.proxyId = source["proxyId"];
	        this.timezone = source["timezone"];
	        this.languages = source["languages"];
	        this.uiLocale = source["uiLocale"];
	        this.lat = source["lat"];
	        this.lon = source["lon"];
	        this.site = source["site"];
	        this.serial = source["serial"];
	        this.login = source["login"];
	        this.hasPassword = source["hasPassword"];
	        this.hasTotp = source["hasTotp"];
	        this.startUrl = source["startUrl"];
	        this.notes = source["notes"];
	        this.tags = source["tags"];
	        this.cdpNextLaunch = source["cdpNextLaunch"];
	        this.pinned = source["pinned"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.lastOpenAt = source["lastOpenAt"];
	        this.deletedAt = source["deletedAt"];
	    }
	}
	export class ProfileFilter {
	    query: string;
	    tag: string;
	    trashed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProfileFilter(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.query = source["query"];
	        this.tag = source["tag"];
	        this.trashed = source["trashed"];
	    }
	}
	export class Proxy {
	    id: string;
	    label: string;
	    scheme: string;
	    host: string;
	    port: number;
	    username: string;
	    exitIp: string;
	    country: string;
	    timezone: string;
	    lat?: number;
	    lon?: number;
	    rotating: boolean;
	    checkedAt: string;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Proxy(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.scheme = source["scheme"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.username = source["username"];
	        this.exitIp = source["exitIp"];
	        this.country = source["country"];
	        this.timezone = source["timezone"];
	        this.lat = source["lat"];
	        this.lon = source["lon"];
	        this.rotating = source["rotating"];
	        this.checkedAt = source["checkedAt"];
	        this.createdAt = source["createdAt"];
	    }
	}

}

