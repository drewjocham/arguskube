export namespace agentconn {
	
	export class Anomaly {
	    timestamp: string;
	    score: number;
	    target: string;
	    rule: string;
	
	    static createFrom(source: any = {}) {
	        return new Anomaly(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = source["timestamp"];
	        this.score = source["score"];
	        this.target = source["target"];
	        this.rule = source["rule"];
	    }
	}
	export class TopologyEdge {
	    source: string;
	    target: string;
	
	    static createFrom(source: any = {}) {
	        return new TopologyEdge(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.target = source["target"];
	    }
	}
	export class TopologyNode {
	    id: string;
	    kind: string;
	    name: string;
	    namespace: string;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new TopologyNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.kind = source["kind"];
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.status = source["status"];
	    }
	}
	export class TopologyGraph {
	    nodes: TopologyNode[];
	    edges: TopologyEdge[];
	
	    static createFrom(source: any = {}) {
	        return new TopologyGraph(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nodes = this.convertValues(source["nodes"], TopologyNode);
	        this.edges = this.convertValues(source["edges"], TopologyEdge);
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

export namespace ai {
	
	export class AgentEvent {
	    // Go type: time
	    timestamp: any;
	    type: string;
	    summary: string;
	    alertId?: string;
	    namespace?: string;
	    severity?: string;
	
	    static createFrom(source: any = {}) {
	        return new AgentEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = this.convertValues(source["timestamp"], null);
	        this.type = source["type"];
	        this.summary = source["summary"];
	        this.alertId = source["alertId"];
	        this.namespace = source["namespace"];
	        this.severity = source["severity"];
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
	export class AutoSummary {
	    alertId: string;
	    summary: string;
	    severity: string;
	    // Go type: time
	    timestamp: any;
	    confidence: number;
	
	    static createFrom(source: any = {}) {
	        return new AutoSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.alertId = source["alertId"];
	        this.summary = source["summary"];
	        this.severity = source["severity"];
	        this.timestamp = this.convertValues(source["timestamp"], null);
	        this.confidence = source["confidence"];
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
	export class ChatEntry {
	    role: string;
	    content: string;
	    // Go type: time
	    timestamp: any;
	
	    static createFrom(source: any = {}) {
	        return new ChatEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.role = source["role"];
	        this.content = source["content"];
	        this.timestamp = this.convertValues(source["timestamp"], null);
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

export namespace alerts {
	
	export class Tag {
	    label: string;
	    color: string;
	
	    static createFrom(source: any = {}) {
	        return new Tag(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.label = source["label"];
	        this.color = source["color"];
	    }
	}
	export class Alert {
	    id: string;
	    name: string;
	    severity: string;
	    namespace: string;
	    // Go type: time
	    timestamp: any;
	    podName?: string;
	    podPhase?: string;
	    restartCount: number;
	    containerID?: string;
	    memoryLimit?: string;
	    memoryRequest?: string;
	    cpuLimit?: string;
	    cpuRequest?: string;
	    cpuThrottle?: number;
	    nodeName?: string;
	    diskUsage?: number;
	    diskCapacity?: string;
	    evictedPods?: string[];
	    imageTag?: string;
	    previousImage?: string;
	    // Go type: time
	    deployTime?: any;
	    description: string;
	    tags: Tag[];
	    relatedAlerts?: string[];
	
	    static createFrom(source: any = {}) {
	        return new Alert(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.severity = source["severity"];
	        this.namespace = source["namespace"];
	        this.timestamp = this.convertValues(source["timestamp"], null);
	        this.podName = source["podName"];
	        this.podPhase = source["podPhase"];
	        this.restartCount = source["restartCount"];
	        this.containerID = source["containerID"];
	        this.memoryLimit = source["memoryLimit"];
	        this.memoryRequest = source["memoryRequest"];
	        this.cpuLimit = source["cpuLimit"];
	        this.cpuRequest = source["cpuRequest"];
	        this.cpuThrottle = source["cpuThrottle"];
	        this.nodeName = source["nodeName"];
	        this.diskUsage = source["diskUsage"];
	        this.diskCapacity = source["diskCapacity"];
	        this.evictedPods = source["evictedPods"];
	        this.imageTag = source["imageTag"];
	        this.previousImage = source["previousImage"];
	        this.deployTime = this.convertValues(source["deployTime"], null);
	        this.description = source["description"];
	        this.tags = this.convertValues(source["tags"], Tag);
	        this.relatedAlerts = source["relatedAlerts"];
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
	export class ClusterMetrics {
	    podHealthPct: number;
	    podsRunning: number;
	    podsTotal: number;
	    podsPending: number;
	    podsFailed: number;
	    errorRate: number;
	    errorRatePrev: number;
	    restartCount: number;
	    restartTop: string;
	    warningEvents: number;
	    totalCpuMillis: number;
	    totalMemoryBytes: number;
	    p99Latency: string;
	    sloStatus: string;
	
	    static createFrom(source: any = {}) {
	        return new ClusterMetrics(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.podHealthPct = source["podHealthPct"];
	        this.podsRunning = source["podsRunning"];
	        this.podsTotal = source["podsTotal"];
	        this.podsPending = source["podsPending"];
	        this.podsFailed = source["podsFailed"];
	        this.errorRate = source["errorRate"];
	        this.errorRatePrev = source["errorRatePrev"];
	        this.restartCount = source["restartCount"];
	        this.restartTop = source["restartTop"];
	        this.warningEvents = source["warningEvents"];
	        this.totalCpuMillis = source["totalCpuMillis"];
	        this.totalMemoryBytes = source["totalMemoryBytes"];
	        this.p99Latency = source["p99Latency"];
	        this.sloStatus = source["sloStatus"];
	    }
	}
	export class RunbookStep {
	    number: number;
	    text: string;
	    command?: string;
	
	    static createFrom(source: any = {}) {
	        return new RunbookStep(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.number = source["number"];
	        this.text = source["text"];
	        this.command = source["command"];
	    }
	}
	export class Diagnosis {
	    alertId: string;
	    hypothesis: string;
	    confidence: number;
	    steps: RunbookStep[];
	    decisionLogEntry?: string;
	    cascadeNote?: string;
	
	    static createFrom(source: any = {}) {
	        return new Diagnosis(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.alertId = source["alertId"];
	        this.hypothesis = source["hypothesis"];
	        this.confidence = source["confidence"];
	        this.steps = this.convertValues(source["steps"], RunbookStep);
	        this.decisionLogEntry = source["decisionLogEntry"];
	        this.cascadeNote = source["cascadeNote"];
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
	export class LogLine {
	    // Go type: time
	    timestamp: any;
	    source: string;
	    level: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new LogLine(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = this.convertValues(source["timestamp"], null);
	        this.source = source["source"];
	        this.level = source["level"];
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
	

}

export namespace anomaly {
	
	export class DetectResult {
	    metric_name: string;
	    is_anomaly: boolean;
	    score: number;
	    description: string;
	    // Go type: time
	    detected_at: any;
	    model_used: string;
	
	    static createFrom(source: any = {}) {
	        return new DetectResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.metric_name = source["metric_name"];
	        this.is_anomaly = source["is_anomaly"];
	        this.score = source["score"];
	        this.description = source["description"];
	        this.detected_at = this.convertValues(source["detected_at"], null);
	        this.model_used = source["model_used"];
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
	export class Job {
	    name: string;
	    metric: string;
	    schedule: string;
	    last_run: string;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new Job(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.metric = source["metric"];
	        this.schedule = source["schedule"];
	        this.last_run = source["last_run"];
	        this.status = source["status"];
	    }
	}

}

export namespace context {
	
	export class DecisionEntry {
	    date: string;
	    content: string;
	
	    static createFrom(source: any = {}) {
	        return new DecisionEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.date = source["date"];
	        this.content = source["content"];
	    }
	}
	export class Bundle {
	    alert: alerts.Alert;
	    decisionLog?: DecisionEntry[];
	    cascadeAlerts?: alerts.Alert[];
	    anomalyResults?: anomaly.DetectResult[];
	    diagnosis?: alerts.Diagnosis;
	
	    static createFrom(source: any = {}) {
	        return new Bundle(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.alert = this.convertValues(source["alert"], alerts.Alert);
	        this.decisionLog = this.convertValues(source["decisionLog"], DecisionEntry);
	        this.cascadeAlerts = this.convertValues(source["cascadeAlerts"], alerts.Alert);
	        this.anomalyResults = this.convertValues(source["anomalyResults"], anomaly.DetectResult);
	        this.diagnosis = this.convertValues(source["diagnosis"], alerts.Diagnosis);
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

export namespace http {
	
	export class Response {
	    Status: string;
	    StatusCode: number;
	    Proto: string;
	    ProtoMajor: number;
	    ProtoMinor: number;
	    Header: Record<string, Array<string>>;
	    Body: any;
	    ContentLength: number;
	    TransferEncoding: string[];
	    Close: boolean;
	    Uncompressed: boolean;
	    Trailer: Record<string, Array<string>>;
	    Request?: Request;
	    TLS?: tls.ConnectionState;
	
	    static createFrom(source: any = {}) {
	        return new Response(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Status = source["Status"];
	        this.StatusCode = source["StatusCode"];
	        this.Proto = source["Proto"];
	        this.ProtoMajor = source["ProtoMajor"];
	        this.ProtoMinor = source["ProtoMinor"];
	        this.Header = source["Header"];
	        this.Body = source["Body"];
	        this.ContentLength = source["ContentLength"];
	        this.TransferEncoding = source["TransferEncoding"];
	        this.Close = source["Close"];
	        this.Uncompressed = source["Uncompressed"];
	        this.Trailer = source["Trailer"];
	        this.Request = this.convertValues(source["Request"], Request);
	        this.TLS = this.convertValues(source["TLS"], tls.ConnectionState);
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
	export class Request {
	    Method: string;
	    URL?: url.URL;
	    Proto: string;
	    ProtoMajor: number;
	    ProtoMinor: number;
	    Header: Record<string, Array<string>>;
	    Body: any;
	    ContentLength: number;
	    TransferEncoding: string[];
	    Close: boolean;
	    Host: string;
	    Form: Record<string, Array<string>>;
	    PostForm: Record<string, Array<string>>;
	    MultipartForm?: multipart.Form;
	    Trailer: Record<string, Array<string>>;
	    RemoteAddr: string;
	    RequestURI: string;
	    TLS?: tls.ConnectionState;
	    Response?: Response;
	    Pattern: string;
	
	    static createFrom(source: any = {}) {
	        return new Request(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Method = source["Method"];
	        this.URL = this.convertValues(source["URL"], url.URL);
	        this.Proto = source["Proto"];
	        this.ProtoMajor = source["ProtoMajor"];
	        this.ProtoMinor = source["ProtoMinor"];
	        this.Header = source["Header"];
	        this.Body = source["Body"];
	        this.ContentLength = source["ContentLength"];
	        this.TransferEncoding = source["TransferEncoding"];
	        this.Close = source["Close"];
	        this.Host = source["Host"];
	        this.Form = source["Form"];
	        this.PostForm = source["PostForm"];
	        this.MultipartForm = this.convertValues(source["MultipartForm"], multipart.Form);
	        this.Trailer = source["Trailer"];
	        this.RemoteAddr = source["RemoteAddr"];
	        this.RequestURI = source["RequestURI"];
	        this.TLS = this.convertValues(source["TLS"], tls.ConnectionState);
	        this.Response = this.convertValues(source["Response"], Response);
	        this.Pattern = source["Pattern"];
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

export namespace incidents {
	
	export class Incident {
	    id: string;
	    title: string;
	    severity: string;
	    status: string;
	    type: string;
	    description: string;
	    namespace?: string;
	    alertId?: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	    // Go type: time
	    resolvedAt?: any;
	    tags?: string[];
	
	    static createFrom(source: any = {}) {
	        return new Incident(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.severity = source["severity"];
	        this.status = source["status"];
	        this.type = source["type"];
	        this.description = source["description"];
	        this.namespace = source["namespace"];
	        this.alertId = source["alertId"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
	        this.resolvedAt = this.convertValues(source["resolvedAt"], null);
	        this.tags = source["tags"];
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

export namespace k8s {
	
	export class Application {
	    name: string;
	    namespace: string;
	    syncStatus: string;
	    healthStatus: string;
	    replicas: number;
	    readyReplicas: number;
	    image: string;
	    lastSync: string;
	
	    static createFrom(source: any = {}) {
	        return new Application(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.syncStatus = source["syncStatus"];
	        this.healthStatus = source["healthStatus"];
	        this.replicas = source["replicas"];
	        this.readyReplicas = source["readyReplicas"];
	        this.image = source["image"];
	        this.lastSync = source["lastSync"];
	    }
	}
	export class ClusterInfo {
	    name: string;
	    nodeCount: number;
	    k8sVersion: string;
	
	    static createFrom(source: any = {}) {
	        return new ClusterInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.nodeCount = source["nodeCount"];
	        this.k8sVersion = source["k8sVersion"];
	    }
	}
	export class ContextInfo {
	    name: string;
	    cluster: string;
	    active: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ContextInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.cluster = source["cluster"];
	        this.active = source["active"];
	    }
	}
	export class KeyValue {
	    key: string;
	    value: string;
	
	    static createFrom(source: any = {}) {
	        return new KeyValue(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.value = source["value"];
	    }
	}
	export class LogEntry {
	    time: string;
	    message: string;
	    pod: string;
	    namespace: string;
	    container: string;
	    node: string;
	
	    static createFrom(source: any = {}) {
	        return new LogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = source["time"];
	        this.message = source["message"];
	        this.pod = source["pod"];
	        this.namespace = source["namespace"];
	        this.container = source["container"];
	        this.node = source["node"];
	    }
	}
	export class LogQueryResult {
	    entries: LogEntry[];
	    total: number;
	    fields: string[];
	    histogram: number[];
	
	    static createFrom(source: any = {}) {
	        return new LogQueryResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.entries = this.convertValues(source["entries"], LogEntry);
	        this.total = source["total"];
	        this.fields = source["fields"];
	        this.histogram = source["histogram"];
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
	export class ResourceColumn {
	    key: string;
	    header: string;
	
	    static createFrom(source: any = {}) {
	        return new ResourceColumn(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.header = source["header"];
	    }
	}
	export class ResourceCondition {
	    type: string;
	    status: string;
	    reason: string;
	    message: string;
	    age: string;
	
	    static createFrom(source: any = {}) {
	        return new ResourceCondition(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.status = source["status"];
	        this.reason = source["reason"];
	        this.message = source["message"];
	        this.age = source["age"];
	    }
	}
	export class ResourceEvent {
	    type: string;
	    reason: string;
	    message: string;
	    count: number;
	    age: string;
	
	    static createFrom(source: any = {}) {
	        return new ResourceEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.reason = source["reason"];
	        this.message = source["message"];
	        this.count = source["count"];
	        this.age = source["age"];
	    }
	}
	export class ResourceDetailResult {
	    kind: string;
	    name: string;
	    namespace: string;
	    created: string;
	    labels: Record<string, string>;
	    annotations: Record<string, string>;
	    properties: KeyValue[];
	    data: Record<string, string>;
	    conditions: ResourceCondition[];
	    events: ResourceEvent[];
	
	    static createFrom(source: any = {}) {
	        return new ResourceDetailResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.created = source["created"];
	        this.labels = source["labels"];
	        this.annotations = source["annotations"];
	        this.properties = this.convertValues(source["properties"], KeyValue);
	        this.data = source["data"];
	        this.conditions = this.convertValues(source["conditions"], ResourceCondition);
	        this.events = this.convertValues(source["events"], ResourceEvent);
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
	
	export class ResourceItem {
	    name: string;
	    namespace: string;
	    status: string;
	    statusColor: string;
	    age: string;
	    fields: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new ResourceItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.status = source["status"];
	        this.statusColor = source["statusColor"];
	        this.age = source["age"];
	        this.fields = source["fields"];
	    }
	}
	export class ResourceSchema {
	    kind: string;
	    columns: ResourceColumn[];
	
	    static createFrom(source: any = {}) {
	        return new ResourceSchema(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.columns = this.convertValues(source["columns"], ResourceColumn);
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
	export class ResourceListResult {
	    schema: ResourceSchema;
	    items: ResourceItem[];
	    total: number;
	
	    static createFrom(source: any = {}) {
	        return new ResourceListResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.schema = this.convertValues(source["schema"], ResourceSchema);
	        this.items = this.convertValues(source["items"], ResourceItem);
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
	
	export class TopologyEdge {
	    source: string;
	    target: string;
	    label?: string;
	
	    static createFrom(source: any = {}) {
	        return new TopologyEdge(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.target = source["target"];
	        this.label = source["label"];
	    }
	}
	export class TopologyNode {
	    id: string;
	    kind: string;
	    name: string;
	    namespace: string;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new TopologyNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.kind = source["kind"];
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.status = source["status"];
	    }
	}
	export class TopologyResult {
	    nodes: TopologyNode[];
	    edges: TopologyEdge[];
	
	    static createFrom(source: any = {}) {
	        return new TopologyResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nodes = this.convertValues(source["nodes"], TopologyNode);
	        this.edges = this.convertValues(source["edges"], TopologyEdge);
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

export namespace multipart {
	
	export class FileHeader {
	    Filename: string;
	    Header: Record<string, Array<string>>;
	    Size: number;
	
	    static createFrom(source: any = {}) {
	        return new FileHeader(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Filename = source["Filename"];
	        this.Header = source["Header"];
	        this.Size = source["Size"];
	    }
	}
	export class Form {
	    Value: Record<string, Array<string>>;
	    File: Record<string, Array<FileHeader>>;
	
	    static createFrom(source: any = {}) {
	        return new Form(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Value = source["Value"];
	        this.File = this.convertValues(source["File"], Array<FileHeader>, true);
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

export namespace net {
	
	export class IPNet {
	    IP: number[];
	    Mask: number[];
	
	    static createFrom(source: any = {}) {
	        return new IPNet(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.IP = source["IP"];
	        this.Mask = source["Mask"];
	    }
	}

}

export namespace notebooks {
	
	export class FileEntry {
	    id: string;
	    name: string;
	    path: string;
	    type: string;
	    children?: FileEntry[];
	    // Go type: time
	    modified: any;
	
	    static createFrom(source: any = {}) {
	        return new FileEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.path = source["path"];
	        this.type = source["type"];
	        this.children = this.convertValues(source["children"], FileEntry);
	        this.modified = this.convertValues(source["modified"], null);
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

export namespace pkg {
	
	export class AIOptimization {
	    issue: string;
	    fix: string;
	
	    static createFrom(source: any = {}) {
	        return new AIOptimization(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.issue = source["issue"];
	        this.fix = source["fix"];
	    }
	}
	export class Vulnerability {
	    id: string;
	    pkg: string;
	    severity: string;
	    desc: string;
	    fix: string;
	
	    static createFrom(source: any = {}) {
	        return new Vulnerability(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.pkg = source["pkg"];
	        this.severity = source["severity"];
	        this.desc = source["desc"];
	        this.fix = source["fix"];
	    }
	}
	export class ScannedImage {
	    id: string;
	    name: string;
	    namespace: string;
	    lastScan: string;
	    critical: number;
	    high: number;
	    medium: number;
	    low: number;
	    status: string;
	    cves: Vulnerability[];
	    aiOpt: AIOptimization;
	
	    static createFrom(source: any = {}) {
	        return new ScannedImage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.lastScan = source["lastScan"];
	        this.critical = source["critical"];
	        this.high = source["high"];
	        this.medium = source["medium"];
	        this.low = source["low"];
	        this.status = source["status"];
	        this.cves = this.convertValues(source["cves"], Vulnerability);
	        this.aiOpt = this.convertValues(source["aiOpt"], AIOptimization);
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

export namespace pkix {
	
	export class AttributeTypeAndValue {
	    Type: number[];
	    Value: any;
	
	    static createFrom(source: any = {}) {
	        return new AttributeTypeAndValue(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Type = source["Type"];
	        this.Value = source["Value"];
	    }
	}
	export class Extension {
	    Id: number[];
	    Critical: boolean;
	    Value: number[];
	
	    static createFrom(source: any = {}) {
	        return new Extension(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Id = source["Id"];
	        this.Critical = source["Critical"];
	        this.Value = source["Value"];
	    }
	}
	export class Name {
	    Country: string[];
	    Organization: string[];
	    OrganizationalUnit: string[];
	    Locality: string[];
	    Province: string[];
	    StreetAddress: string[];
	    PostalCode: string[];
	    SerialNumber: string;
	    CommonName: string;
	    Names: AttributeTypeAndValue[];
	    ExtraNames: AttributeTypeAndValue[];
	
	    static createFrom(source: any = {}) {
	        return new Name(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Country = source["Country"];
	        this.Organization = source["Organization"];
	        this.OrganizationalUnit = source["OrganizationalUnit"];
	        this.Locality = source["Locality"];
	        this.Province = source["Province"];
	        this.StreetAddress = source["StreetAddress"];
	        this.PostalCode = source["PostalCode"];
	        this.SerialNumber = source["SerialNumber"];
	        this.CommonName = source["CommonName"];
	        this.Names = this.convertValues(source["Names"], AttributeTypeAndValue);
	        this.ExtraNames = this.convertValues(source["ExtraNames"], AttributeTypeAndValue);
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

export namespace popeye {
	
	export class Finding {
	    id: string;
	    resource: string;
	    name: string;
	    namespace: string;
	    severity: string;
	    sevLevel: number;
	    message: string;
	    explanation: string;
	    fix: string;
	    command: string;
	
	    static createFrom(source: any = {}) {
	        return new Finding(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.resource = source["resource"];
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.severity = source["severity"];
	        this.sevLevel = source["sevLevel"];
	        this.message = source["message"];
	        this.explanation = source["explanation"];
	        this.fix = source["fix"];
	        this.command = source["command"];
	    }
	}
	export class Report {
	    // Go type: time
	    timestamp: any;
	    score: number;
	    grade: string;
	    findings: Finding[];
	    totalOk: number;
	    totalInfo: number;
	    totalWarn: number;
	    totalError: number;
	    scanTimeMs: number;
	    clusterName: string;
	
	    static createFrom(source: any = {}) {
	        return new Report(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = this.convertValues(source["timestamp"], null);
	        this.score = source["score"];
	        this.grade = source["grade"];
	        this.findings = this.convertValues(source["findings"], Finding);
	        this.totalOk = source["totalOk"];
	        this.totalInfo = source["totalInfo"];
	        this.totalWarn = source["totalWarn"];
	        this.totalError = source["totalError"];
	        this.scanTimeMs = source["scanTimeMs"];
	        this.clusterName = source["clusterName"];
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

export namespace runbooks {
	
	export class Runbook {
	    id: string;
	    name: string;
	    trigger: string;
	    status: string;
	    steps: number;
	    lastRun: string;
	    // Go type: time
	    modified: any;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new Runbook(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.trigger = source["trigger"];
	        this.status = source["status"];
	        this.steps = source["steps"];
	        this.lastRun = source["lastRun"];
	        this.modified = this.convertValues(source["modified"], null);
	        this.path = source["path"];
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

export namespace setup {
	
	export class SetupResult {
	    success: boolean;
	    message: string;
	    output: string;
	
	    static createFrom(source: any = {}) {
	        return new SetupResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.message = source["message"];
	        this.output = source["output"];
	    }
	}
	export class ToolStatus {
	    name: string;
	    installed: boolean;
	    version: string;
	    via: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new ToolStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.installed = source["installed"];
	        this.version = source["version"];
	        this.via = source["via"];
	        this.message = source["message"];
	    }
	}

}

export namespace tls {
	
	export class ConnectionState {
	    Version: number;
	    HandshakeComplete: boolean;
	    DidResume: boolean;
	    CipherSuite: number;
	    CurveID: number;
	    NegotiatedProtocol: string;
	    NegotiatedProtocolIsMutual: boolean;
	    ServerName: string;
	    PeerCertificates: x509.Certificate[];
	    VerifiedChains: x509.Certificate[][];
	    SignedCertificateTimestamps: number[][];
	    OCSPResponse: number[];
	    TLSUnique: number[];
	    ECHAccepted: boolean;
	    HelloRetryRequest: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Version = source["Version"];
	        this.HandshakeComplete = source["HandshakeComplete"];
	        this.DidResume = source["DidResume"];
	        this.CipherSuite = source["CipherSuite"];
	        this.CurveID = source["CurveID"];
	        this.NegotiatedProtocol = source["NegotiatedProtocol"];
	        this.NegotiatedProtocolIsMutual = source["NegotiatedProtocolIsMutual"];
	        this.ServerName = source["ServerName"];
	        this.PeerCertificates = this.convertValues(source["PeerCertificates"], x509.Certificate);
	        this.VerifiedChains = this.convertValues(source["VerifiedChains"], x509.Certificate);
	        this.SignedCertificateTimestamps = source["SignedCertificateTimestamps"];
	        this.OCSPResponse = source["OCSPResponse"];
	        this.TLSUnique = source["TLSUnique"];
	        this.ECHAccepted = source["ECHAccepted"];
	        this.HelloRetryRequest = source["HelloRetryRequest"];
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

export namespace url {
	
	export class Userinfo {
	
	
	    static createFrom(source: any = {}) {
	        return new Userinfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}
	export class URL {
	    Scheme: string;
	    Opaque: string;
	    // Go type: Userinfo
	    User?: any;
	    Host: string;
	    Path: string;
	    Fragment: string;
	    RawQuery: string;
	    RawPath: string;
	    RawFragment: string;
	    ForceQuery: boolean;
	    OmitHost: boolean;
	
	    static createFrom(source: any = {}) {
	        return new URL(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Scheme = source["Scheme"];
	        this.Opaque = source["Opaque"];
	        this.User = this.convertValues(source["User"], null);
	        this.Host = source["Host"];
	        this.Path = source["Path"];
	        this.Fragment = source["Fragment"];
	        this.RawQuery = source["RawQuery"];
	        this.RawPath = source["RawPath"];
	        this.RawFragment = source["RawFragment"];
	        this.ForceQuery = source["ForceQuery"];
	        this.OmitHost = source["OmitHost"];
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

export namespace x509 {
	
	export class PolicyMapping {
	    // Go type: OID
	    IssuerDomainPolicy: any;
	    // Go type: OID
	    SubjectDomainPolicy: any;
	
	    static createFrom(source: any = {}) {
	        return new PolicyMapping(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.IssuerDomainPolicy = this.convertValues(source["IssuerDomainPolicy"], null);
	        this.SubjectDomainPolicy = this.convertValues(source["SubjectDomainPolicy"], null);
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
	export class OID {
	
	
	    static createFrom(source: any = {}) {
	        return new OID(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}
	export class Certificate {
	    Raw: number[];
	    RawTBSCertificate: number[];
	    RawSubjectPublicKeyInfo: number[];
	    RawSubject: number[];
	    RawIssuer: number[];
	    Signature: number[];
	    SignatureAlgorithm: number;
	    PublicKeyAlgorithm: number;
	    PublicKey: any;
	    Version: number;
	    // Go type: big
	    SerialNumber?: any;
	    Issuer: pkix.Name;
	    Subject: pkix.Name;
	    // Go type: time
	    NotBefore: any;
	    // Go type: time
	    NotAfter: any;
	    KeyUsage: number;
	    Extensions: pkix.Extension[];
	    ExtraExtensions: pkix.Extension[];
	    UnhandledCriticalExtensions: number[][];
	    ExtKeyUsage: number[];
	    UnknownExtKeyUsage: number[][];
	    BasicConstraintsValid: boolean;
	    IsCA: boolean;
	    MaxPathLen: number;
	    MaxPathLenZero: boolean;
	    SubjectKeyId: number[];
	    AuthorityKeyId: number[];
	    OCSPServer: string[];
	    IssuingCertificateURL: string[];
	    DNSNames: string[];
	    EmailAddresses: string[];
	    IPAddresses: number[][];
	    URIs: url.URL[];
	    PermittedDNSDomainsCritical: boolean;
	    PermittedDNSDomains: string[];
	    ExcludedDNSDomains: string[];
	    PermittedIPRanges: net.IPNet[];
	    ExcludedIPRanges: net.IPNet[];
	    PermittedEmailAddresses: string[];
	    ExcludedEmailAddresses: string[];
	    PermittedURIDomains: string[];
	    ExcludedURIDomains: string[];
	    CRLDistributionPoints: string[];
	    PolicyIdentifiers: number[][];
	    Policies: OID[];
	    InhibitAnyPolicy: number;
	    InhibitAnyPolicyZero: boolean;
	    InhibitPolicyMapping: number;
	    InhibitPolicyMappingZero: boolean;
	    RequireExplicitPolicy: number;
	    RequireExplicitPolicyZero: boolean;
	    PolicyMappings: PolicyMapping[];
	
	    static createFrom(source: any = {}) {
	        return new Certificate(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Raw = source["Raw"];
	        this.RawTBSCertificate = source["RawTBSCertificate"];
	        this.RawSubjectPublicKeyInfo = source["RawSubjectPublicKeyInfo"];
	        this.RawSubject = source["RawSubject"];
	        this.RawIssuer = source["RawIssuer"];
	        this.Signature = source["Signature"];
	        this.SignatureAlgorithm = source["SignatureAlgorithm"];
	        this.PublicKeyAlgorithm = source["PublicKeyAlgorithm"];
	        this.PublicKey = source["PublicKey"];
	        this.Version = source["Version"];
	        this.SerialNumber = this.convertValues(source["SerialNumber"], null);
	        this.Issuer = this.convertValues(source["Issuer"], pkix.Name);
	        this.Subject = this.convertValues(source["Subject"], pkix.Name);
	        this.NotBefore = this.convertValues(source["NotBefore"], null);
	        this.NotAfter = this.convertValues(source["NotAfter"], null);
	        this.KeyUsage = source["KeyUsage"];
	        this.Extensions = this.convertValues(source["Extensions"], pkix.Extension);
	        this.ExtraExtensions = this.convertValues(source["ExtraExtensions"], pkix.Extension);
	        this.UnhandledCriticalExtensions = source["UnhandledCriticalExtensions"];
	        this.ExtKeyUsage = source["ExtKeyUsage"];
	        this.UnknownExtKeyUsage = source["UnknownExtKeyUsage"];
	        this.BasicConstraintsValid = source["BasicConstraintsValid"];
	        this.IsCA = source["IsCA"];
	        this.MaxPathLen = source["MaxPathLen"];
	        this.MaxPathLenZero = source["MaxPathLenZero"];
	        this.SubjectKeyId = source["SubjectKeyId"];
	        this.AuthorityKeyId = source["AuthorityKeyId"];
	        this.OCSPServer = source["OCSPServer"];
	        this.IssuingCertificateURL = source["IssuingCertificateURL"];
	        this.DNSNames = source["DNSNames"];
	        this.EmailAddresses = source["EmailAddresses"];
	        this.IPAddresses = source["IPAddresses"];
	        this.URIs = this.convertValues(source["URIs"], url.URL);
	        this.PermittedDNSDomainsCritical = source["PermittedDNSDomainsCritical"];
	        this.PermittedDNSDomains = source["PermittedDNSDomains"];
	        this.ExcludedDNSDomains = source["ExcludedDNSDomains"];
	        this.PermittedIPRanges = this.convertValues(source["PermittedIPRanges"], net.IPNet);
	        this.ExcludedIPRanges = this.convertValues(source["ExcludedIPRanges"], net.IPNet);
	        this.PermittedEmailAddresses = source["PermittedEmailAddresses"];
	        this.ExcludedEmailAddresses = source["ExcludedEmailAddresses"];
	        this.PermittedURIDomains = source["PermittedURIDomains"];
	        this.ExcludedURIDomains = source["ExcludedURIDomains"];
	        this.CRLDistributionPoints = source["CRLDistributionPoints"];
	        this.PolicyIdentifiers = source["PolicyIdentifiers"];
	        this.Policies = this.convertValues(source["Policies"], OID);
	        this.InhibitAnyPolicy = source["InhibitAnyPolicy"];
	        this.InhibitAnyPolicyZero = source["InhibitAnyPolicyZero"];
	        this.InhibitPolicyMapping = source["InhibitPolicyMapping"];
	        this.InhibitPolicyMappingZero = source["InhibitPolicyMappingZero"];
	        this.RequireExplicitPolicy = source["RequireExplicitPolicy"];
	        this.RequireExplicitPolicyZero = source["RequireExplicitPolicyZero"];
	        this.PolicyMappings = this.convertValues(source["PolicyMappings"], PolicyMapping);
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

