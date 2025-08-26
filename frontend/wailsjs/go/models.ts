export namespace main {
	
	export class Game {
	    appid: number;
	    name: string;
	    image: string;
	
	    static createFrom(source: any = {}) {
	        return new Game(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.appid = source["appid"];
	        this.name = source["name"];
	        this.image = source["image"];
	    }
	}

}

