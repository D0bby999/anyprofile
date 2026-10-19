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
	    running: boolean;
	    error: string;
	
	    static createFrom(source: any = {}) {
	        return new APIStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.port = source["port"];
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
	
	export class APISettings {
	    enabled: boolean;
	    port: number;
	
	    static createFrom(source: any = {}) {
	        return new APISettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.port = source["port"];
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
	export class ProStatus {
	    active: boolean;
	    key: string;
	    state: string;
	    paidThrough: string;
	    version: string;
	    notes: string;
	    checkedAt: string;
	    error: string;
	    waiting: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.active = source["active"];
	        this.key = source["key"];
	        this.state = source["state"];
	        this.paidThrough = source["paidThrough"];
	        this.version = source["version"];
	        this.notes = source["notes"];
	        this.checkedAt = source["checkedAt"];
	        this.error = source["error"];
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
	    createdAt: string;
	    note: string;
	    expiresOn: string;
	    usedBy: string[];
	    expiry: string;
	
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
	        this.createdAt = source["createdAt"];
	        this.note = source["note"];
	        this.expiresOn = source["expiresOn"];
	        this.usedBy = source["usedBy"];
	        this.expiry = source["expiry"];
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
	    orphans: Orphan[];
	
	    static createFrom(source: any = {}) {
	        return new StorageInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profilesBytes = source["profilesBytes"];
	        this.snapshotsBytes = source["snapshotsBytes"];
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
	
	    static createFrom(source: any = {}) {
	        return new EngineMeta(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hwc = source["hwc"];
	        this.memGiB = source["memGiB"];
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
	    exitIp6: string;
	    timezone6: string;
	    lat?: number;
	    lon?: number;
	    rotating: boolean;
	    checkedAt: string;
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

}

