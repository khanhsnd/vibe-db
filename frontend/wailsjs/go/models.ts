export namespace app {
	
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

