export namespace app {
	
	export class CellEditInput {
	    database: string;
	    schema: string;
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
	        this.schema = source["schema"];
	        this.table = source["table"];
	        this.column = source["column"];
	        this.value = source["value"];
	        this.key = source["key"];
	    }
	}
	export class ColumnInput {
	    name: string;
	    type: string;
	    nullable: boolean;
	    default?: string;
	    primaryKey: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ColumnInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.nullable = source["nullable"];
	        this.default = source["default"];
	        this.primaryKey = source["primaryKey"];
	    }
	}
	export class ColumnValuesInput {
	    database: string;
	    schema: string;
	    table: string;
	    column: string;
	
	    static createFrom(source: any = {}) {
	        return new ColumnValuesInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.database = source["database"];
	        this.schema = source["schema"];
	        this.table = source["table"];
	        this.column = source["column"];
	    }
	}
	export class DatabaseOptions {
	    owners: string[];
	    templates: string[];
	    encodings: string[];
	    collations: string[];
	    tablespaces: string[];
	    extensions: string[];
	
	    static createFrom(source: any = {}) {
	        return new DatabaseOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.owners = source["owners"];
	        this.templates = source["templates"];
	        this.encodings = source["encodings"];
	        this.collations = source["collations"];
	        this.tablespaces = source["tablespaces"];
	        this.extensions = source["extensions"];
	    }
	}
	export class DatabaseSpecInput {
	    name: string;
	    owner: string;
	    template: string;
	    encoding: string;
	    collation: string;
	    ctype: string;
	    tablespace: string;
	    connectionLimit: number;
	    allowConnections: boolean;
	    isTemplate: boolean;
	    comment: string;
	    extensions: string[];
	
	    static createFrom(source: any = {}) {
	        return new DatabaseSpecInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.owner = source["owner"];
	        this.template = source["template"];
	        this.encoding = source["encoding"];
	        this.collation = source["collation"];
	        this.ctype = source["ctype"];
	        this.tablespace = source["tablespace"];
	        this.connectionLimit = source["connectionLimit"];
	        this.allowConnections = source["allowConnections"];
	        this.isTemplate = source["isTemplate"];
	        this.comment = source["comment"];
	        this.extensions = source["extensions"];
	    }
	}
	export class ExportInput {
	    database: string;
	    schema: string;
	    format: string;
	    table: string;
	    columns: string[];
	    rows: string[][];
	
	    static createFrom(source: any = {}) {
	        return new ExportInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.database = source["database"];
	        this.schema = source["schema"];
	        this.format = source["format"];
	        this.table = source["table"];
	        this.columns = source["columns"];
	        this.rows = source["rows"];
	    }
	}
	export class ForeignKeyInfo {
	    name: string;
	    columns: string[];
	    referencedSchema: string;
	    referencedTable: string;
	    referencedColumns: string[];
	    onUpdate: string;
	    onDelete: string;
	
	    static createFrom(source: any = {}) {
	        return new ForeignKeyInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.columns = source["columns"];
	        this.referencedSchema = source["referencedSchema"];
	        this.referencedTable = source["referencedTable"];
	        this.referencedColumns = source["referencedColumns"];
	        this.onUpdate = source["onUpdate"];
	        this.onDelete = source["onDelete"];
	    }
	}
	export class ForeignKeyInput {
	    name: string;
	    columns: string[];
	    referencedSchema: string;
	    referencedTable: string;
	    referencedColumns: string[];
	    onUpdate: string;
	    onDelete: string;
	
	    static createFrom(source: any = {}) {
	        return new ForeignKeyInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.columns = source["columns"];
	        this.referencedSchema = source["referencedSchema"];
	        this.referencedTable = source["referencedTable"];
	        this.referencedColumns = source["referencedColumns"];
	        this.onUpdate = source["onUpdate"];
	        this.onDelete = source["onDelete"];
	    }
	}
	export class HistoryEntry {
	    database: string;
	    schema: string;
	    profileId: string;
	    sql: string;
	    rows: number;
	    rowsAffected: number;
	    elapsedMs: number;
	    errorCode: string;
	    error: string;
	    runs: number;
	    ranAt: number;
	
	    static createFrom(source: any = {}) {
	        return new HistoryEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.database = source["database"];
	        this.schema = source["schema"];
	        this.profileId = source["profileId"];
	        this.sql = source["sql"];
	        this.rows = source["rows"];
	        this.rowsAffected = source["rowsAffected"];
	        this.elapsedMs = source["elapsedMs"];
	        this.errorCode = source["errorCode"];
	        this.error = source["error"];
	        this.runs = source["runs"];
	        this.ranAt = source["ranAt"];
	    }
	}
	export class IndexInfo {
	    name: string;
	    columns: string[];
	    unique: boolean;
	    primary: boolean;
	    method: string;
	
	    static createFrom(source: any = {}) {
	        return new IndexInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.columns = source["columns"];
	        this.unique = source["unique"];
	        this.primary = source["primary"];
	        this.method = source["method"];
	    }
	}
	export class IndexInput {
	    name: string;
	    columns: string[];
	    unique: boolean;
	    method: string;
	
	    static createFrom(source: any = {}) {
	        return new IndexInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.columns = source["columns"];
	        this.unique = source["unique"];
	        this.method = source["method"];
	    }
	}
	export class Namespace {
	    database: string;
	    schema: string;
	
	    static createFrom(source: any = {}) {
	        return new Namespace(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.database = source["database"];
	        this.schema = source["schema"];
	    }
	}
	export class RoutineInfo {
	    name: string;
	    procedure: boolean;
	    arguments: string;
	
	    static createFrom(source: any = {}) {
	        return new RoutineInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.procedure = source["procedure"];
	        this.arguments = source["arguments"];
	    }
	}
	export class Objects {
	    tables: string[];
	    views: string[];
	    routines: RoutineInfo[];
	
	    static createFrom(source: any = {}) {
	        return new Objects(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tables = source["tables"];
	        this.views = source["views"];
	        this.routines = this.convertValues(source["routines"], RoutineInfo);
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
	    engine: string;
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
	        this.engine = source["engine"];
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
	    database: string;
	    schema: string;
	    runId: string;
	    sql: string;
	    maxRows: number;
	    explain: boolean;
	
	    static createFrom(source: any = {}) {
	        return new QueryInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.database = source["database"];
	        this.schema = source["schema"];
	        this.runId = source["runId"];
	        this.sql = source["sql"];
	        this.maxRows = source["maxRows"];
	        this.explain = source["explain"];
	    }
	}
	
	export class RowDeleteInput {
	    database: string;
	    schema: string;
	    table: string;
	    key: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new RowDeleteInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.database = source["database"];
	        this.schema = source["schema"];
	        this.table = source["table"];
	        this.key = source["key"];
	    }
	}
	export class RowValueInput {
	    column: string;
	    value?: string;
	
	    static createFrom(source: any = {}) {
	        return new RowValueInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.column = source["column"];
	        this.value = source["value"];
	    }
	}
	export class RowInsertInput {
	    database: string;
	    schema: string;
	    table: string;
	    values: RowValueInput[];
	
	    static createFrom(source: any = {}) {
	        return new RowInsertInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.database = source["database"];
	        this.schema = source["schema"];
	        this.table = source["table"];
	        this.values = this.convertValues(source["values"], RowValueInput);
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
	
	export class StatementOutcome {
	    sql: string;
	    rows: number;
	    rowsAffected: number;
	    elapsedMs: number;
	    error: string;
	    errorCode: string;
	
	    static createFrom(source: any = {}) {
	        return new StatementOutcome(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sql = source["sql"];
	        this.rows = source["rows"];
	        this.rowsAffected = source["rowsAffected"];
	        this.elapsedMs = source["elapsedMs"];
	        this.error = source["error"];
	        this.errorCode = source["errorCode"];
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
	export class ScriptResult {
	    database: string;
	    schema: string;
	    columns: TableColumn[];
	    rows: string[][];
	    rowsAffected: number;
	    truncated: boolean;
	    elapsedMs: number;
	    table: string;
	    statements: StatementOutcome[];
	
	    static createFrom(source: any = {}) {
	        return new ScriptResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.database = source["database"];
	        this.schema = source["schema"];
	        this.columns = this.convertValues(source["columns"], TableColumn);
	        this.rows = source["rows"];
	        this.rowsAffected = source["rowsAffected"];
	        this.truncated = source["truncated"];
	        this.elapsedMs = source["elapsedMs"];
	        this.table = source["table"];
	        this.statements = this.convertValues(source["statements"], StatementOutcome);
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
	
	
	export class TableCriterionInput {
	    kind: string;
	    column: string;
	    operator: string;
	    values: string[];
	    conjunction: string;
	    children: TableCriterionInput[];
	
	    static createFrom(source: any = {}) {
	        return new TableCriterionInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.column = source["column"];
	        this.operator = source["operator"];
	        this.values = source["values"];
	        this.conjunction = source["conjunction"];
	        this.children = this.convertValues(source["children"], TableCriterionInput);
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
	export class TableList {
	    columns: TableColumn[];
	    rows: string[][];
	
	    static createFrom(source: any = {}) {
	        return new TableList(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.columns = this.convertValues(source["columns"], TableColumn);
	        this.rows = source["rows"];
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
	export class TableSortInput {
	    column: string;
	    descending: boolean;
	
	    static createFrom(source: any = {}) {
	        return new TableSortInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.column = source["column"];
	        this.descending = source["descending"];
	    }
	}
	export class TablePageInput {
	    database: string;
	    schema: string;
	    table: string;
	    runId: string;
	    limit: number;
	    offset: number;
	    sort: TableSortInput[];
	    filter: TableCriterionInput;
	
	    static createFrom(source: any = {}) {
	        return new TablePageInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.database = source["database"];
	        this.schema = source["schema"];
	        this.table = source["table"];
	        this.runId = source["runId"];
	        this.limit = source["limit"];
	        this.offset = source["offset"];
	        this.sort = this.convertValues(source["sort"], TableSortInput);
	        this.filter = this.convertValues(source["filter"], TableCriterionInput);
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
	export class TableRows {
	    columns: TableColumn[];
	    rows: string[][];
	    total: number;
	    limit: number;
	    offset: number;
	    statement: string;
	
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
	        this.statement = source["statement"];
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
	export class TableSchema {
	    name: string;
	    columns: string[];
	
	    static createFrom(source: any = {}) {
	        return new TableSchema(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.columns = source["columns"];
	    }
	}
	
	export class TableSpecInput {
	    database: string;
	    schema: string;
	    table: string;
	    columns: ColumnInput[];
	
	    static createFrom(source: any = {}) {
	        return new TableSpecInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.database = source["database"];
	        this.schema = source["schema"];
	        this.table = source["table"];
	        this.columns = this.convertValues(source["columns"], ColumnInput);
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
	export class TableStructure {
	    columns: TableColumn[];
	    indexes: IndexInfo[];
	    foreignKeys: ForeignKeyInfo[];
	
	    static createFrom(source: any = {}) {
	        return new TableStructure(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.columns = this.convertValues(source["columns"], TableColumn);
	        this.indexes = this.convertValues(source["indexes"], IndexInfo);
	        this.foreignKeys = this.convertValues(source["foreignKeys"], ForeignKeyInfo);
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

