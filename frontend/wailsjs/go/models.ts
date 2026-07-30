export namespace app {
	
	export class CellEditInput {
	    database: string;
	    table: string;
	    column: string;
	    value?: string;
	    key: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new CellEditInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.database = source["database"];
	        this.table = source["table"];
	        this.column = source["column"];
	        this.value = source["value"];
	        this.key = source["key"];
	    }
	}
	export class Profile {
	    id: string;
	    name: string;
	    engine: string;
	    host: string;
	    port: number;
	    user: string;
	    database: string;
	    tlsMode: string;
	    savePassword: boolean;
	    hasPassword: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Profile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.engine = source["engine"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.user = source["user"];
	        this.database = source["database"];
	        this.tlsMode = source["tlsMode"];
	        this.savePassword = source["savePassword"];
	        this.hasPassword = source["hasPassword"];
	    }
	}
	export class ProfileInput {
	    id: string;
	    name: string;
	    host: string;
	    port: number;
	    user: string;
	    password: string;
	    database: string;
	    tlsMode: string;
	    savePassword: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProfileInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.user = source["user"];
	        this.password = source["password"];
	        this.database = source["database"];
	        this.tlsMode = source["tlsMode"];
	        this.savePassword = source["savePassword"];
	    }
	}
	export class QueryInput {
	    runId: string;
	    database: string;
	    sql: string;
	    maxRows: number;
	    explain: boolean;
	
	    static createFrom(source: any = {}) {
	        return new QueryInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.runId = source["runId"];
	        this.database = source["database"];
	        this.sql = source["sql"];
	        this.maxRows = source["maxRows"];
	        this.explain = source["explain"];
	    }
	}
	export class TableColumn {
	    name: string;
	    type: string;
	    fullType: string;
	    nullable: boolean;
	    primaryKey: boolean;
	    default?: string;
	
	    static createFrom(source: any = {}) {
	        return new TableColumn(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.fullType = source["fullType"];
	        this.nullable = source["nullable"];
	        this.primaryKey = source["primaryKey"];
	        this.default = source["default"];
	    }
	}
	export class QueryResult {
	    columns: TableColumn[];
	    rows: string[][];
	    rowsAffected: number;
	    truncated: boolean;
	    elapsedMs: number;
	
	    static createFrom(source: any = {}) {
	        return new QueryResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.columns = this.convertValues(source["columns"], TableColumn);
	        this.rows = source["rows"];
	        this.rowsAffected = source["rowsAffected"];
	        this.truncated = source["truncated"];
	        this.elapsedMs = source["elapsedMs"];
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
	export class Session {
	    id: string;
	    profileId: string;
	    target: string;
	    serverVersion: string;
	
	    static createFrom(source: any = {}) {
	        return new Session(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.profileId = source["profileId"];
	        this.target = source["target"];
	        this.serverVersion = source["serverVersion"];
	    }
	}
	
	export class TablePageInput {
	    database: string;
	    table: string;
	    limit: number;
	    offset: number;
	
	    static createFrom(source: any = {}) {
	        return new TablePageInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.database = source["database"];
	        this.table = source["table"];
	        this.limit = source["limit"];
	        this.offset = source["offset"];
	    }
	}
	export class TableRows {
	    columns: TableColumn[];
	    rows: string[][];
	    total: number;
	    limit: number;
	    offset: number;
	
	    static createFrom(source: any = {}) {
	        return new TableRows(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.columns = this.convertValues(source["columns"], TableColumn);
	        this.rows = source["rows"];
	        this.total = source["total"];
	        this.limit = source["limit"];
	        this.offset = source["offset"];
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
	export class TestResult {
	    serverVersion: string;
	    latencyMs: number;
	
	    static createFrom(source: any = {}) {
	        return new TestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.serverVersion = source["serverVersion"];
	        this.latencyMs = source["latencyMs"];
	    }
	}

}

