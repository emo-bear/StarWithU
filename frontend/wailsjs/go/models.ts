export namespace main {
	
	export class Settings {
	    dataDir: string;
	    eventCount: number;
	    imageCount: number;
	    loveStart: string;
	    spanDays: number;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dataDir = source["dataDir"];
	        this.eventCount = source["eventCount"];
	        this.imageCount = source["imageCount"];
	        this.loveStart = source["loveStart"];
	        this.spanDays = source["spanDays"];
	    }
	}

}

export namespace store {
	
	export class Event {
	    id: number;
	    title: string;
	    date: string;
	    description: string;
	    images: string[];
	    x: number;
	    y: number;
	    z: number;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Event(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.date = source["date"];
	        this.description = source["description"];
	        this.images = source["images"];
	        this.x = source["x"];
	        this.y = source["y"];
	        this.z = source["z"];
	        this.createdAt = source["createdAt"];
	    }
	}

}

