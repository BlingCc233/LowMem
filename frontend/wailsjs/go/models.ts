export namespace gorm {
	
	export class DeletedAt {
	    // Go type: time
	    Time: any;
	    Valid: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DeletedAt(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Time = this.convertValues(source["Time"], null);
	        this.Valid = source["Valid"];
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

export namespace models {
	
	export class Group {
	    group_id: number;
	    group_name: string;
	    avatar_url: string;
	    member_count: number;
	
	    static createFrom(source: any = {}) {
	        return new Group(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.group_id = source["group_id"];
	        this.group_name = source["group_name"];
	        this.avatar_url = source["avatar_url"];
	        this.member_count = source["member_count"];
	    }
	}
	export class MessageElement {
	    type: string;
	    text?: string;
	    url?: string;
	    file?: string;
	    qq?: string;
	    name?: string;
	    reply_id?: number;
	
	    static createFrom(source: any = {}) {
	        return new MessageElement(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.text = source["text"];
	        this.url = source["url"];
	        this.file = source["file"];
	        this.qq = source["qq"];
	        this.name = source["name"];
	        this.reply_id = source["reply_id"];
	    }
	}
	export class User {
	    user_id: number;
	    nickname: string;
	    avatar_url: string;
	    is_friend: boolean;
	
	    static createFrom(source: any = {}) {
	        return new User(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.user_id = source["user_id"];
	        this.nickname = source["nickname"];
	        this.avatar_url = source["avatar_url"];
	        this.is_friend = source["is_friend"];
	    }
	}
	export class Message {
	    database_id: number;
	    message_id: number;
	    message_type: string;
	    sub_type: string;
	    sender_id: number;
	    sender: User;
	    target_id: number;
	    group_id: number;
	    group: Group;
	    content: string;
	    raw_message: string;
	    elements?: MessageElement[];
	    reply_to?: number;
	    time: number;
	    is_send: boolean;
	    is_read: boolean;
	    recall: boolean;
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    updated_at: any;
	
	    static createFrom(source: any = {}) {
	        return new Message(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.database_id = source["database_id"];
	        this.message_id = source["message_id"];
	        this.message_type = source["message_type"];
	        this.sub_type = source["sub_type"];
	        this.sender_id = source["sender_id"];
	        this.sender = this.convertValues(source["sender"], User);
	        this.target_id = source["target_id"];
	        this.group_id = source["group_id"];
	        this.group = this.convertValues(source["group"], Group);
	        this.content = source["content"];
	        this.raw_message = source["raw_message"];
	        this.elements = this.convertValues(source["elements"], MessageElement);
	        this.reply_to = source["reply_to"];
	        this.time = source["time"];
	        this.is_send = source["is_send"];
	        this.is_read = source["is_read"];
	        this.recall = source["recall"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.updated_at = this.convertValues(source["updated_at"], null);
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
	
export class UnreadSummary {
    chat_id: number;
    is_group: boolean;
    count: number;
	
	    static createFrom(source: any = {}) {
	        return new UnreadSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
        this.chat_id = source["chat_id"];
        this.is_group = source["is_group"];
        this.count = source["count"];
    }
}

export class ChatSession {
    chat_id: number;
    is_group: boolean;
    name: string;
    avatar: string;
    last_message: string;
    time: number;
    unread: number;

    static createFrom(source: any = {}) {
        return new ChatSession(source);
    }

    constructor(source: any = {}) {
        if ('string' === typeof source) source = JSON.parse(source);
        this.chat_id = source["chat_id"];
        this.is_group = source["is_group"];
        this.name = source["name"];
        this.avatar = source["avatar"];
        this.last_message = source["last_message"];
        this.time = source["time"];
        this.unread = source["unread"];
    }
}

}
