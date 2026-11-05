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

export namespace fingerprint {
	
	export class Check {
	    id: string;
	    ok: boolean;
	    level: string;
	    got?: string;
	    want?: string;
	
	    static createFrom(source: any = {}) {
	        return new Check(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.ok = source["ok"];
	        this.level = source["level"];
	        this.got = source["got"];
	        this.want = source["want"];
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

export namespace main {
	
	export class APIStatus {
	    enabled: boolean;
	    port: number;
	    agentManage: boolean;
	    running: boolean;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new APIStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.port = source["port"];
	        this.agentManage = source["agentManage"];
	        this.running = source["running"];
	        this.error = source["error"];
	    }
	}
	export class EngineInfo {
	    engine: string;
	    version: string;
	    chromium: string;
	    installed: boolean;
	    problem: string;
	    default: boolean;
	
	    static createFrom(source: any = {}) {
	        return new EngineInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.engine = source["engine"];
	        this.version = source["version"];
	        this.chromium = source["chromium"];
	        this.installed = source["installed"];
	        this.problem = source["problem"];
	        this.default = source["default"];
	    }
	}
	export class FolderList {
	    folders: service.FolderRow[];
	    unfiled: number;
	
	    static createFrom(source: any = {}) {
	        return new FolderList(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.folders = this.convertValues(source["folders"], service.FolderRow);
	        this.unfiled = source["unfiled"];
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

export namespace mobile {
	
	export class Component {
	    engine: string;
	    installed: string;
	    latest: string;
	
	    static createFrom(source: any = {}) {
	        return new Component(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.engine = source["engine"];
	        this.installed = source["installed"];
	        this.latest = source["latest"];
	    }
	}
	export class InstallStatus {
	    runtime: Component;
	    image: Component;
	
	    static createFrom(source: any = {}) {
	        return new InstallStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.runtime = this.convertValues(source["runtime"], Component);
	        this.image = this.convertValues(source["image"], Component);
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
	export class Metrics {
	    bootMs: number;
	    startMs: number;
	    resetMs: number;
	    adbPort: number;
	    headless: boolean;
	    actions: number;
	    lastActionMs: number;
	    snapshotMs: number;
	
	    static createFrom(source: any = {}) {
	        return new Metrics(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.bootMs = source["bootMs"];
	        this.startMs = source["startMs"];
	        this.resetMs = source["resetMs"];
	        this.adbPort = source["adbPort"];
	        this.headless = source["headless"];
	        this.actions = source["actions"];
	        this.lastActionMs = source["lastActionMs"];
	        this.snapshotMs = source["snapshotMs"];
	    }
	}

}

export namespace netproxy {
	
	export class ParsedLine {
	    line: number;
	    display: string;
	    err: string;
	    code: string;
	
	    static createFrom(source: any = {}) {
	        return new ParsedLine(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.line = source["line"];
	        this.display = source["display"];
	        this.err = source["err"];
	        this.code = source["code"];
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

export namespace pro {
	
	export class AccountDevice {
	    install_id: string;
	    name: string;
	    platform: string;
	    last_seen: number;
	    current: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AccountDevice(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.install_id = source["install_id"];
	        this.name = source["name"];
	        this.platform = source["platform"];
	        this.last_seen = source["last_seen"];
	        this.current = source["current"];
	    }
	}
	export class AccountLicense {
	    id: string;
	    kind: string;
	    source: string;
	    status: string;
	    features: string[];
	    max_devices: number;
	    paid_through: number;
	    license_expires_at: number;
	    grace_until: number;
	    updates_through: number;
	    devices: number;
	    email: string;
	
	    static createFrom(source: any = {}) {
	        return new AccountLicense(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.kind = source["kind"];
	        this.source = source["source"];
	        this.status = source["status"];
	        this.features = source["features"];
	        this.max_devices = source["max_devices"];
	        this.paid_through = source["paid_through"];
	        this.license_expires_at = source["license_expires_at"];
	        this.grace_until = source["grace_until"];
	        this.updates_through = source["updates_through"];
	        this.devices = source["devices"];
	        this.email = source["email"];
	    }
	}
	export class DeviceInfo {
	    install_id: string;
	    name: string;
	    platform: string;
	    last_seen: number;
	    current: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DeviceInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.install_id = source["install_id"];
	        this.name = source["name"];
	        this.platform = source["platform"];
	        this.last_seen = source["last_seen"];
	        this.current = source["current"];
	    }
	}

}

export namespace service {
	
	export class APISettings {
	    enabled: boolean;
	    port: number;
	    agentManage: boolean;
	
	    static createFrom(source: any = {}) {
	        return new APISettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.port = source["port"];
	        this.agentManage = source["agentManage"];
	    }
	}
	export class BackupResult {
	    profiles: number;
	    proxies: number;
	    accounts: number;
	    extensions: number;
	    bytes: number;
	    profilesHere: number;
	    proxiesHere: number;
	
	    static createFrom(source: any = {}) {
	        return new BackupResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profiles = source["profiles"];
	        this.proxies = source["proxies"];
	        this.accounts = source["accounts"];
	        this.extensions = source["extensions"];
	        this.bytes = source["bytes"];
	        this.profilesHere = source["profilesHere"];
	        this.proxiesHere = source["proxiesHere"];
	    }
	}
	export class BulkCreate {
	    engine: string;
	    baseName: string;
	    count: number;
	    proxy: string;
	    sites: string[];
	    startUrl: string;
	    notes: string;
	    tags: string[];
	
	    static createFrom(source: any = {}) {
	        return new BulkCreate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.engine = source["engine"];
	        this.baseName = source["baseName"];
	        this.count = source["count"];
	        this.proxy = source["proxy"];
	        this.sites = source["sites"];
	        this.startUrl = source["startUrl"];
	        this.notes = source["notes"];
	        this.tags = source["tags"];
	    }
	}
	export class CookieImport {
	    imported: number;
	    skipped: number;
	    domains: string[];
	    siteMismatch: string;
	
	    static createFrom(source: any = {}) {
	        return new CookieImport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.imported = source["imported"];
	        this.skipped = source["skipped"];
	        this.domains = source["domains"];
	        this.siteMismatch = source["siteMismatch"];
	    }
	}
	export class Defaults {
	    engine: string;
	    site: string;
	    proxy: string;
	    onOpen: string;
	
	    static createFrom(source: any = {}) {
	        return new Defaults(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.engine = source["engine"];
	        this.site = source["site"];
	        this.proxy = source["proxy"];
	        this.onOpen = source["onOpen"];
	    }
	}
	export class DuplicateInput {
	    engine: string;
	    sourceId: string;
	    baseName: string;
	    count: number;
	    proxy: string;
	
	    static createFrom(source: any = {}) {
	        return new DuplicateInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.engine = source["engine"];
	        this.sourceId = source["sourceId"];
	        this.baseName = source["baseName"];
	        this.count = source["count"];
	        this.proxy = source["proxy"];
	    }
	}
	export class EngineRelease {
	    engine: string;
	    version: string;
	    chromium: string;
	    installed: boolean;
	    problem: string;
	    date: string;
	    notes: string;
	    profiles: number;
	
	    static createFrom(source: any = {}) {
	        return new EngineRelease(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.engine = source["engine"];
	        this.version = source["version"];
	        this.chromium = source["chromium"];
	        this.installed = source["installed"];
	        this.problem = source["problem"];
	        this.date = source["date"];
	        this.notes = source["notes"];
	        this.profiles = source["profiles"];
	    }
	}
	export class ExitSummary {
	    ip: string;
	    country: string;
	    at: string;
	
	    static createFrom(source: any = {}) {
	        return new ExitSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ip = source["ip"];
	        this.country = source["country"];
	        this.at = source["at"];
	    }
	}
	export class Flag {
	    id: string;
	    level: string;
	    params: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new Flag(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.level = source["level"];
	        this.params = source["params"];
	    }
	}
	export class FolderRow {
	    id: string;
	    name: string;
	    color: string;
	    sortOrder: number;
	    createdAt: string;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new FolderRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.color = source["color"];
	        this.sortOrder = source["sortOrder"];
	        this.createdAt = source["createdAt"];
	        this.count = source["count"];
	    }
	}
	export class ImportResult {
	    added: number;
	    failed: netproxy.ParsedLine[];
	    unreachable: string[];
	
	    static createFrom(source: any = {}) {
	        return new ImportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.added = source["added"];
	        this.failed = this.convertValues(source["failed"], netproxy.ParsedLine);
	        this.unreachable = source["unreachable"];
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
	export class MobileAvailability {
	    install?: mobile.InstallStatus;
	    supported: boolean;
	    driver: string;
	    pro: boolean;
	    ready: boolean;
	    reason: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new MobileAvailability(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.install = this.convertValues(source["install"], mobile.InstallStatus);
	        this.supported = source["supported"];
	        this.driver = source["driver"];
	        this.pro = source["pro"];
	        this.ready = source["ready"];
	        this.reason = source["reason"];
	        this.message = source["message"];
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
	export class MobileInput {
	    name: string;
	    proxyId: string;
	    ramMb: number;
	    cpus: number;
	    preset: string;
	    screenW: number;
	    screenH: number;
	    density: number;
	    notes: string;
	    tags: string[];
	    pinned: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MobileInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.proxyId = source["proxyId"];
	        this.ramMb = source["ramMb"];
	        this.cpus = source["cpus"];
	        this.preset = source["preset"];
	        this.screenW = source["screenW"];
	        this.screenH = source["screenH"];
	        this.density = source["density"];
	        this.notes = source["notes"];
	        this.tags = source["tags"];
	        this.pinned = source["pinned"];
	    }
	}
	export class MobileRow {
	    id: string;
	    name: string;
	    driver: string;
	    platform: string;
	    driverRef: string;
	    proxyId: string;
	    ramMb: number;
	    cpus: number;
	    preset: string;
	    screenW: number;
	    screenH: number;
	    density: number;
	    imageVersion: string;
	    notes: string;
	    tags: string[];
	    pinned: boolean;
	    serial: number;
	    createdAt: string;
	    updatedAt: string;
	    lastOpenAt: string;
	    backedUpAt: string;
	    deletedAt: string;
	    state: string;
	    error: string;
	    adbPort?: number;
	
	    static createFrom(source: any = {}) {
	        return new MobileRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.driver = source["driver"];
	        this.platform = source["platform"];
	        this.driverRef = source["driverRef"];
	        this.proxyId = source["proxyId"];
	        this.ramMb = source["ramMb"];
	        this.cpus = source["cpus"];
	        this.preset = source["preset"];
	        this.screenW = source["screenW"];
	        this.screenH = source["screenH"];
	        this.density = source["density"];
	        this.imageVersion = source["imageVersion"];
	        this.notes = source["notes"];
	        this.tags = source["tags"];
	        this.pinned = source["pinned"];
	        this.serial = source["serial"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.lastOpenAt = source["lastOpenAt"];
	        this.backedUpAt = source["backedUpAt"];
	        this.deletedAt = source["deletedAt"];
	        this.state = source["state"];
	        this.error = source["error"];
	        this.adbPort = source["adbPort"];
	    }
	}
	export class MobileStart {
	    warning: string;
	
	    static createFrom(source: any = {}) {
	        return new MobileStart(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.warning = source["warning"];
	    }
	}
	export class NewProfile {
	    engine: string;
	    name: string;
	    proxyId: string;
	    sites: string[];
	    startUrl: string;
	    notes: string;
	    tags: string[];
	
	    static createFrom(source: any = {}) {
	        return new NewProfile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.engine = source["engine"];
	        this.name = source["name"];
	        this.proxyId = source["proxyId"];
	        this.sites = source["sites"];
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
	export class ProAccountStatus {
	    signedIn: boolean;
	    email: string;
	    name: string;
	    license?: pro.AccountLicense;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ProAccountStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.signedIn = source["signedIn"];
	        this.email = source["email"];
	        this.name = source["name"];
	        this.license = this.convertValues(source["license"], pro.AccountLicense);
	        this.error = source["error"];
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
	export class ProStatus {
	    active: boolean;
	    key: string;
	    state: string;
	    kind: string;
	    source: string;
	    features: string[];
	    paidThrough: string;
	    licenseExpiresAt: string;
	    graceUntil: string;
	    updatesThrough: string;
	    version: string;
	    notes: string;
	    checkedAt: string;
	    error: string;
	    offline: boolean;
	    test: boolean;
	    waiting: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.active = source["active"];
	        this.key = source["key"];
	        this.state = source["state"];
	        this.kind = source["kind"];
	        this.source = source["source"];
	        this.features = source["features"];
	        this.paidThrough = source["paidThrough"];
	        this.licenseExpiresAt = source["licenseExpiresAt"];
	        this.graceUntil = source["graceUntil"];
	        this.updatesThrough = source["updatesThrough"];
	        this.version = source["version"];
	        this.notes = source["notes"];
	        this.checkedAt = source["checkedAt"];
	        this.error = source["error"];
	        this.offline = source["offline"];
	        this.test = source["test"];
	        this.waiting = source["waiting"];
	    }
	}
	export class ProfileRow {
	    id: string;
	    name: string;
	    engine: string;
	    engineVersion: string;
	    engineMeta: store.EngineMeta;
	    personaId: string;
	    seedHex: string;
	    platform: string;
	    proxyId: string;
	    timezone: string;
	    languages: string;
	    uiLocale: string;
	    lat?: number;
	    lon?: number;
	    sites: string[];
	    serial: number;
	    accounts: store.Account[];
	    extensions: string[];
	    fpVerdict: string;
	    fpDiff: string[];
	    fpCheckedAt: string;
	    machine: string;
	    startUrl: string;
	    notes: string;
	    tags: string[];
	    cdpNextLaunch: boolean;
	    pinned: boolean;
	    createdAt: string;
	    updatedAt: string;
	    lastOpenAt: string;
	    deletedAt: string;
	    folderId: string;
	    state: string;
	    stateError: string;
	    cdpPort: number;
	    proxyLabel: string;
	    proxyCountry: string;
	    proxyExitIp: string;
	    proxyExpiry: string;
	    lastExit?: ExitSummary;
	    flags: Flag[];
	    checks: fingerprint.Check[];
	    position: number;
	
	    static createFrom(source: any = {}) {
	        return new ProfileRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.engine = source["engine"];
	        this.engineVersion = source["engineVersion"];
	        this.engineMeta = this.convertValues(source["engineMeta"], store.EngineMeta);
	        this.personaId = source["personaId"];
	        this.seedHex = source["seedHex"];
	        this.platform = source["platform"];
	        this.proxyId = source["proxyId"];
	        this.timezone = source["timezone"];
	        this.languages = source["languages"];
	        this.uiLocale = source["uiLocale"];
	        this.lat = source["lat"];
	        this.lon = source["lon"];
	        this.sites = source["sites"];
	        this.serial = source["serial"];
	        this.accounts = this.convertValues(source["accounts"], store.Account);
	        this.extensions = source["extensions"];
	        this.fpVerdict = source["fpVerdict"];
	        this.fpDiff = source["fpDiff"];
	        this.fpCheckedAt = source["fpCheckedAt"];
	        this.machine = source["machine"];
	        this.startUrl = source["startUrl"];
	        this.notes = source["notes"];
	        this.tags = source["tags"];
	        this.cdpNextLaunch = source["cdpNextLaunch"];
	        this.pinned = source["pinned"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.lastOpenAt = source["lastOpenAt"];
	        this.deletedAt = source["deletedAt"];
	        this.folderId = source["folderId"];
	        this.state = source["state"];
	        this.stateError = source["stateError"];
	        this.cdpPort = source["cdpPort"];
	        this.proxyLabel = source["proxyLabel"];
	        this.proxyCountry = source["proxyCountry"];
	        this.proxyExitIp = source["proxyExitIp"];
	        this.proxyExpiry = source["proxyExpiry"];
	        this.lastExit = this.convertValues(source["lastExit"], ExitSummary);
	        this.flags = this.convertValues(source["flags"], Flag);
	        this.checks = this.convertValues(source["checks"], fingerprint.Check);
	        this.position = source["position"];
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
	export class ProfilePage {
	    rows: ProfileRow[];
	    total: number;
	
	    static createFrom(source: any = {}) {
	        return new ProfilePage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rows = this.convertValues(source["rows"], ProfileRow);
	        this.total = source["total"];
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
	
	export class ProxyMeta {
	    label: string;
	    note: string;
	    expiresOn: string;
	
	    static createFrom(source: any = {}) {
	        return new ProxyMeta(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.note = source["note"];
	        this.expiresOn = source["expiresOn"];
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
	    exitIp6: string;
	    timezone6: string;
	    lat?: number;
	    lon?: number;
	    rotating: boolean;
	    checkedAt: string;
	    failedAt: string;
	    createdAt: string;
	    note: string;
	    expiresOn: string;
	    usedBy: string[];
	    expiry: string;
	    assignable: boolean;
	
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
	        this.exitIp6 = source["exitIp6"];
	        this.timezone6 = source["timezone6"];
	        this.lat = source["lat"];
	        this.lon = source["lon"];
	        this.rotating = source["rotating"];
	        this.checkedAt = source["checkedAt"];
	        this.failedAt = source["failedAt"];
	        this.createdAt = source["createdAt"];
	        this.note = source["note"];
	        this.expiresOn = source["expiresOn"];
	        this.usedBy = source["usedBy"];
	        this.expiry = source["expiry"];
	        this.assignable = source["assignable"];
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
	export class SnapshotSettings {
	    auto: boolean;
	    keep: number;
	
	    static createFrom(source: any = {}) {
	        return new SnapshotSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.auto = source["auto"];
	        this.keep = source["keep"];
	    }
	}
	export class StorageInfo {
	    profilesBytes: number;
	    snapshotsBytes: number;
	    mobileBytes: number;
	    orphans: Orphan[];
	
	    static createFrom(source: any = {}) {
	        return new StorageInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profilesBytes = source["profilesBytes"];
	        this.snapshotsBytes = source["snapshotsBytes"];
	        this.mobileBytes = source["mobileBytes"];
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
	export class UpdateStatus {
	    current: string;
	    latest: string;
	    date: string;
	    notes: string;
	    available: boolean;
	    checkedAt: string;
	    auto: boolean;
	    ready: boolean;
	
	    static createFrom(source: any = {}) {
	        return new UpdateStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.current = source["current"];
	        this.latest = source["latest"];
	        this.date = source["date"];
	        this.notes = source["notes"];
	        this.available = source["available"];
	        this.checkedAt = source["checkedAt"];
	        this.auto = source["auto"];
	        this.ready = source["ready"];
	    }
	}
	export class WhatsNew {
	    version: string;
	    notes: string;
	
	    static createFrom(source: any = {}) {
	        return new WhatsNew(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.notes = source["notes"];
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
	    startUrls?: Record<string, string>;
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
	        this.startUrls = source["startUrls"];
	        this.icon = source["icon"];
	        this.color = source["color"];
	        this.custom = source["custom"];
	    }
	}

}

export namespace store {
	
	export class Account {
	    site: string;
	    login: string;
	    hasPassword: boolean;
	    hasTotp: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Account(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.site = source["site"];
	        this.login = source["login"];
	        this.hasPassword = source["hasPassword"];
	        this.hasTotp = source["hasTotp"];
	    }
	}
	export class EngineMeta {
	    hwc?: number;
	    memGiB?: number;
	    canvasNoise?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new EngineMeta(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hwc = source["hwc"];
	        this.memGiB = source["memGiB"];
	        this.canvasNoise = source["canvasNoise"];
	    }
	}
	export class Extension {
	    id: string;
	    name: string;
	    version: string;
	    createdAt: string;
	    profiles: number;
	
	    static createFrom(source: any = {}) {
	        return new Extension(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.version = source["version"];
	        this.createdAt = source["createdAt"];
	        this.profiles = source["profiles"];
	    }
	}
	export class Folder {
	    id: string;
	    name: string;
	    color: string;
	    sortOrder: number;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Folder(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.color = source["color"];
	        this.sortOrder = source["sortOrder"];
	        this.createdAt = source["createdAt"];
	    }
	}
	export class MobileDevice {
	    id: string;
	    name: string;
	    driver: string;
	    platform: string;
	    driverRef: string;
	    proxyId: string;
	    ramMb: number;
	    cpus: number;
	    preset: string;
	    screenW: number;
	    screenH: number;
	    density: number;
	    imageVersion: string;
	    notes: string;
	    tags: string[];
	    pinned: boolean;
	    serial: number;
	    createdAt: string;
	    updatedAt: string;
	    lastOpenAt: string;
	    backedUpAt: string;
	    deletedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new MobileDevice(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.driver = source["driver"];
	        this.platform = source["platform"];
	        this.driverRef = source["driverRef"];
	        this.proxyId = source["proxyId"];
	        this.ramMb = source["ramMb"];
	        this.cpus = source["cpus"];
	        this.preset = source["preset"];
	        this.screenW = source["screenW"];
	        this.screenH = source["screenH"];
	        this.density = source["density"];
	        this.imageVersion = source["imageVersion"];
	        this.notes = source["notes"];
	        this.tags = source["tags"];
	        this.pinned = source["pinned"];
	        this.serial = source["serial"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.lastOpenAt = source["lastOpenAt"];
	        this.backedUpAt = source["backedUpAt"];
	        this.deletedAt = source["deletedAt"];
	    }
	}
	export class Profile {
	    id: string;
	    name: string;
	    engine: string;
	    engineVersion: string;
	    engineMeta: EngineMeta;
	    personaId: string;
	    seedHex: string;
	    platform: string;
	    proxyId: string;
	    timezone: string;
	    languages: string;
	    uiLocale: string;
	    lat?: number;
	    lon?: number;
	    sites: string[];
	    serial: number;
	    accounts: Account[];
	    extensions: string[];
	    fpVerdict: string;
	    fpDiff: string[];
	    fpCheckedAt: string;
	    machine: string;
	    startUrl: string;
	    notes: string;
	    tags: string[];
	    cdpNextLaunch: boolean;
	    pinned: boolean;
	    createdAt: string;
	    updatedAt: string;
	    lastOpenAt: string;
	    deletedAt: string;
	    folderId: string;
	
	    static createFrom(source: any = {}) {
	        return new Profile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.engine = source["engine"];
	        this.engineVersion = source["engineVersion"];
	        this.engineMeta = this.convertValues(source["engineMeta"], EngineMeta);
	        this.personaId = source["personaId"];
	        this.seedHex = source["seedHex"];
	        this.platform = source["platform"];
	        this.proxyId = source["proxyId"];
	        this.timezone = source["timezone"];
	        this.languages = source["languages"];
	        this.uiLocale = source["uiLocale"];
	        this.lat = source["lat"];
	        this.lon = source["lon"];
	        this.sites = source["sites"];
	        this.serial = source["serial"];
	        this.accounts = this.convertValues(source["accounts"], Account);
	        this.extensions = source["extensions"];
	        this.fpVerdict = source["fpVerdict"];
	        this.fpDiff = source["fpDiff"];
	        this.fpCheckedAt = source["fpCheckedAt"];
	        this.machine = source["machine"];
	        this.startUrl = source["startUrl"];
	        this.notes = source["notes"];
	        this.tags = source["tags"];
	        this.cdpNextLaunch = source["cdpNextLaunch"];
	        this.pinned = source["pinned"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	        this.lastOpenAt = source["lastOpenAt"];
	        this.deletedAt = source["deletedAt"];
	        this.folderId = source["folderId"];
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
	export class ProfileFilter {
	    query: string;
	    tag: string;
	    trashed: boolean;
	    limit: number;
	    offset: number;
	    folderId: string;
	    unfiled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProfileFilter(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.query = source["query"];
	        this.tag = source["tag"];
	        this.trashed = source["trashed"];
	        this.limit = source["limit"];
	        this.offset = source["offset"];
	        this.folderId = source["folderId"];
	        this.unfiled = source["unfiled"];
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
	    exitIp6: string;
	    timezone6: string;
	    lat?: number;
	    lon?: number;
	    rotating: boolean;
	    checkedAt: string;
	    failedAt: string;
	    createdAt: string;
	    note: string;
	    expiresOn: string;
	
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
	        this.exitIp6 = source["exitIp6"];
	        this.timezone6 = source["timezone6"];
	        this.lat = source["lat"];
	        this.lon = source["lon"];
	        this.rotating = source["rotating"];
	        this.checkedAt = source["checkedAt"];
	        this.failedAt = source["failedAt"];
	        this.createdAt = source["createdAt"];
	        this.note = source["note"];
	        this.expiresOn = source["expiresOn"];
	    }
	}
	export class Session {
	    id: number;
	    profileId: string;
	    engine: string;
	    engineVersion: string;
	    proxyId: string;
	    startedAt: string;
	    stoppedAt: string;
	    exitIp: string;
	    exitCountry: string;
	    exitTz: string;
	    exitError: string;
	    exitIp6: string;
	    exitTz6: string;
	    checks: number[];
	
	    static createFrom(source: any = {}) {
	        return new Session(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.profileId = source["profileId"];
	        this.engine = source["engine"];
	        this.engineVersion = source["engineVersion"];
	        this.proxyId = source["proxyId"];
	        this.startedAt = source["startedAt"];
	        this.stoppedAt = source["stoppedAt"];
	        this.exitIp = source["exitIp"];
	        this.exitCountry = source["exitCountry"];
	        this.exitTz = source["exitTz"];
	        this.exitError = source["exitError"];
	        this.exitIp6 = source["exitIp6"];
	        this.exitTz6 = source["exitTz6"];
	        this.checks = source["checks"];
	    }
	}
	export class Snapshot {
	    id: number;
	    profileId: string;
	    createdAt: string;
	    reason: string;
	    bytes: number;
	    file: string;
	
	    static createFrom(source: any = {}) {
	        return new Snapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.profileId = source["profileId"];
	        this.createdAt = source["createdAt"];
	        this.reason = source["reason"];
	        this.bytes = source["bytes"];
	        this.file = source["file"];
	    }
	}
	export class Template {
	    id: string;
	    name: string;
	    site: string;
	    sites: string[];
	    proxyNote: string;
	    languages: string;
	    uiLocale: string;
	    engine: string;
	    startUrl: string;
	    notes: string;
	    tags: string[];
	    folderId: string;
	    builtin: boolean;
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Template(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.site = source["site"];
	        this.sites = source["sites"];
	        this.proxyNote = source["proxyNote"];
	        this.languages = source["languages"];
	        this.uiLocale = source["uiLocale"];
	        this.engine = source["engine"];
	        this.startUrl = source["startUrl"];
	        this.notes = source["notes"];
	        this.tags = source["tags"];
	        this.folderId = source["folderId"];
	        this.builtin = source["builtin"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}

}

