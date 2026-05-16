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

export namespace alertproc {
	
	export class AgentProfile {
	    autoInvestigate: boolean;
	    autoDocument: boolean;
	    canAck: boolean;
	    canSilence: boolean;
	    canAdjustParams: boolean;
	    silenceWindow: number;
	    fatigueThreshold: number;
	
	    static createFrom(source: any = {}) {
	        return new AgentProfile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.autoInvestigate = source["autoInvestigate"];
	        this.autoDocument = source["autoDocument"];
	        this.canAck = source["canAck"];
	        this.canSilence = source["canSilence"];
	        this.canAdjustParams = source["canAdjustParams"];
	        this.silenceWindow = source["silenceWindow"];
	        this.fatigueThreshold = source["fatigueThreshold"];
	    }
	}
	export class Investigation {
	    alertID: string;
	    signature: string;
	    hypothesis: string;
	    error: string;
	    // Go type: time
	    recordedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Investigation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.alertID = source["alertID"];
	        this.signature = source["signature"];
	        this.hypothesis = source["hypothesis"];
	        this.error = source["error"];
	        this.recordedAt = this.convertValues(source["recordedAt"], null);
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
	export class Rule {
	    id: string;
	    name: string;
	    enabled: boolean;
	    severity: string;
	
	    static createFrom(source: any = {}) {
	        return new Rule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.enabled = source["enabled"];
	        this.severity = source["severity"];
	    }
	}
	export class Settings {
	    sensitivity: number;
	    baselineWindow: number;
	    metricType: string;
	    algorithm: string;
	    threshold: number;
	    targetScope: string;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sensitivity = source["sensitivity"];
	        this.baselineWindow = source["baselineWindow"];
	        this.metricType = source["metricType"];
	        this.algorithm = source["algorithm"];
	        this.threshold = source["threshold"];
	        this.targetScope = source["targetScope"];
	    }
	}

}

export namespace argocd {
	
	export class AppHistoryEntry {
	    id: number;
	    revision: string;
	    deployedAt: string;
	    source?: string;
	
	    static createFrom(source: any = {}) {
	        return new AppHistoryEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.revision = source["revision"];
	        this.deployedAt = source["deployedAt"];
	        this.source = source["source"];
	    }
	}
	export class AppResource {
	    kind: string;
	    name: string;
	    namespace: string;
	    status: string;
	    health: string;
	    group: string;
	    version: string;
	
	    static createFrom(source: any = {}) {
	        return new AppResource(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.status = source["status"];
	        this.health = source["health"];
	        this.group = source["group"];
	        this.version = source["version"];
	    }
	}
	export class App {
	    name: string;
	    namespace: string;
	    project: string;
	    syncStatus: string;
	    healthStatus: string;
	    replicas: number;
	    readyReplicas: number;
	    image: string;
	    lastSync: string;
	    repoUrl: string;
	    path: string;
	    targetRevision: string;
	    destServer: string;
	    destNamespace: string;
	    createdAt: string;
	    resources?: AppResource[];
	    history?: AppHistoryEntry[];
	
	    static createFrom(source: any = {}) {
	        return new App(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.project = source["project"];
	        this.syncStatus = source["syncStatus"];
	        this.healthStatus = source["healthStatus"];
	        this.replicas = source["replicas"];
	        this.readyReplicas = source["readyReplicas"];
	        this.image = source["image"];
	        this.lastSync = source["lastSync"];
	        this.repoUrl = source["repoUrl"];
	        this.path = source["path"];
	        this.targetRevision = source["targetRevision"];
	        this.destServer = source["destServer"];
	        this.destNamespace = source["destNamespace"];
	        this.createdAt = source["createdAt"];
	        this.resources = this.convertValues(source["resources"], AppResource);
	        this.history = this.convertValues(source["history"], AppHistoryEntry);
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
	export class AppDiff {
	    resource: string;
	    live: string;
	    target: string;
	    diff: string;
	
	    static createFrom(source: any = {}) {
	        return new AppDiff(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.resource = source["resource"];
	        this.live = source["live"];
	        this.target = source["target"];
	        this.diff = source["diff"];
	    }
	}
	
	
	export class SyncResult {
	    phase: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new SyncResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.phase = source["phase"];
	        this.message = source["message"];
	    }
	}

}

export namespace auth {
	
	export class Store {
	
	
	    static createFrom(source: any = {}) {
	        return new Store(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}

}

export namespace broker {
	
	export class AMQP1Config {
	    url: string;
	    authMode: string;
	    username?: string;
	    password?: string;
	    bearerToken?: string;
	    senderTarget: string;
	    tlsCaCert?: string;
	    tlsClientCert?: string;
	    tlsClientKey?: string;
	    insecureSkipVerify?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AMQP1Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.authMode = source["authMode"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.bearerToken = source["bearerToken"];
	        this.senderTarget = source["senderTarget"];
	        this.tlsCaCert = source["tlsCaCert"];
	        this.tlsClientCert = source["tlsClientCert"];
	        this.tlsClientKey = source["tlsClientKey"];
	        this.insecureSkipVerify = source["insecureSkipVerify"];
	    }
	}
	export class RESTConfig {
	    baseURL: string;
	    method: string;
	    path?: string;
	    headers?: Record<string, string>;
	    contentType?: string;
	    timeoutSeconds?: number;
	    insecureSkipTLS?: boolean;
	    basicAuthUser?: string;
	    basicAuthPassword?: string;
	    bearerToken?: string;
	    successCodes?: number[];
	
	    static createFrom(source: any = {}) {
	        return new RESTConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.baseURL = source["baseURL"];
	        this.method = source["method"];
	        this.path = source["path"];
	        this.headers = source["headers"];
	        this.contentType = source["contentType"];
	        this.timeoutSeconds = source["timeoutSeconds"];
	        this.insecureSkipTLS = source["insecureSkipTLS"];
	        this.basicAuthUser = source["basicAuthUser"];
	        this.basicAuthPassword = source["basicAuthPassword"];
	        this.bearerToken = source["bearerToken"];
	        this.successCodes = source["successCodes"];
	    }
	}
	export class RabbitMQConfig {
	    url: string;
	    exchange: string;
	    exchangeType?: string;
	    publisherConfirms: boolean;
	    tlsCaCert?: string;
	    tlsClientCert?: string;
	    tlsClientKey?: string;
	    insecureSkipVerify?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new RabbitMQConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.exchange = source["exchange"];
	        this.exchangeType = source["exchangeType"];
	        this.publisherConfirms = source["publisherConfirms"];
	        this.tlsCaCert = source["tlsCaCert"];
	        this.tlsClientCert = source["tlsClientCert"];
	        this.tlsClientKey = source["tlsClientKey"];
	        this.insecureSkipVerify = source["insecureSkipVerify"];
	    }
	}
	export class KafkaConfig {
	    bootstrapServers: string;
	    clientId?: string;
	    authMode: string;
	    username?: string;
	    password?: string;
	    oauthBearerToken?: string;
	    tlsCaCert?: string;
	    tlsClientCert?: string;
	    tlsClientKey?: string;
	    acks?: string;
	    insecureSkipVerify?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new KafkaConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.bootstrapServers = source["bootstrapServers"];
	        this.clientId = source["clientId"];
	        this.authMode = source["authMode"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.oauthBearerToken = source["oauthBearerToken"];
	        this.tlsCaCert = source["tlsCaCert"];
	        this.tlsClientCert = source["tlsClientCert"];
	        this.tlsClientKey = source["tlsClientKey"];
	        this.acks = source["acks"];
	        this.insecureSkipVerify = source["insecureSkipVerify"];
	    }
	}
	export class NATSConfig {
	    servers: string;
	    useJetStream: boolean;
	    authMode: string;
	    username?: string;
	    password?: string;
	    token?: string;
	    nkeySeed?: string;
	    credsFile?: string;
	    insecureSkipVerify?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new NATSConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.servers = source["servers"];
	        this.useJetStream = source["useJetStream"];
	        this.authMode = source["authMode"];
	        this.username = source["username"];
	        this.password = source["password"];
	        this.token = source["token"];
	        this.nkeySeed = source["nkeySeed"];
	        this.credsFile = source["credsFile"];
	        this.insecureSkipVerify = source["insecureSkipVerify"];
	    }
	}
	export class PubSubConfig {
	    projectId: string;
	    authMode: string;
	    serviceAccountJson?: string;
	    endpoint?: string;
	
	    static createFrom(source: any = {}) {
	        return new PubSubConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.projectId = source["projectId"];
	        this.authMode = source["authMode"];
	        this.serviceAccountJson = source["serviceAccountJson"];
	        this.endpoint = source["endpoint"];
	    }
	}
	export class Config {
	    kind: string;
	    pubsub?: PubSubConfig;
	    nats?: NATSConfig;
	    kafka?: KafkaConfig;
	    rabbitmq?: RabbitMQConfig;
	    amqp1?: AMQP1Config;
	    rest?: RESTConfig;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.pubsub = this.convertValues(source["pubsub"], PubSubConfig);
	        this.nats = this.convertValues(source["nats"], NATSConfig);
	        this.kafka = this.convertValues(source["kafka"], KafkaConfig);
	        this.rabbitmq = this.convertValues(source["rabbitmq"], RabbitMQConfig);
	        this.amqp1 = this.convertValues(source["amqp1"], AMQP1Config);
	        this.rest = this.convertValues(source["rest"], RESTConfig);
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

export namespace config {
	
	export class AuthConfig {
	    PublicBaseURL: string;
	    GoogleClientID: string;
	    GoogleClientSecret: string;
	    OIDCIssuer: string;
	    OIDCClientID: string;
	    OIDCClientSecret: string;
	    OIDCDisplayName: string;
	    AllowLocalSignup: boolean;
	    DevMode: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AuthConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.PublicBaseURL = source["PublicBaseURL"];
	        this.GoogleClientID = source["GoogleClientID"];
	        this.GoogleClientSecret = source["GoogleClientSecret"];
	        this.OIDCIssuer = source["OIDCIssuer"];
	        this.OIDCClientID = source["OIDCClientID"];
	        this.OIDCClientSecret = source["OIDCClientSecret"];
	        this.OIDCDisplayName = source["OIDCDisplayName"];
	        this.AllowLocalSignup = source["AllowLocalSignup"];
	        this.DevMode = source["DevMode"];
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

export namespace envprobe {
	
	export class Result {
	    id: string;
	    title: string;
	    status: string;
	    detail?: string;
	    actionLabel?: string;
	    actionId?: string;
	    // Go type: time
	    ran: any;
	    latencyMs: number;
	
	    static createFrom(source: any = {}) {
	        return new Result(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.status = source["status"];
	        this.detail = source["detail"];
	        this.actionLabel = source["actionLabel"];
	        this.actionId = source["actionId"];
	        this.ran = this.convertValues(source["ran"], null);
	        this.latencyMs = source["latencyMs"];
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
	
	export class ServeMux {
	
	
	    static createFrom(source: any = {}) {
	        return new ServeMux(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
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

export namespace intstr {
	
	export class IntOrString {
	    Type: number;
	    IntVal: number;
	    StrVal: string;
	
	    static createFrom(source: any = {}) {
	        return new IntOrString(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Type = source["Type"];
	        this.IntVal = source["IntVal"];
	        this.StrVal = source["StrVal"];
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
	export class TokenStatus {
	    provider: string;
	    expired: boolean;
	    expiresAt?: string;
	    issuer?: string;
	
	    static createFrom(source: any = {}) {
	        return new TokenStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.expired = source["expired"];
	        this.expiresAt = source["expiresAt"];
	        this.issuer = source["issuer"];
	    }
	}
	export class AuthCheckResult {
	    tokens: TokenStatus[];
	    allValid: boolean;
	    clusterOK: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AuthCheckResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tokens = this.convertValues(source["tokens"], TokenStatus);
	        this.allValid = source["allValid"];
	        this.clusterOK = source["clusterOK"];
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
	export class BackendRefSummary {
	    name: string;
	    port: number;
	    weight: number;
	
	    static createFrom(source: any = {}) {
	        return new BackendRefSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.port = source["port"];
	        this.weight = source["weight"];
	    }
	}
	export class BackendRefWeight {
	    name: string;
	    namespace?: string;
	    port: number;
	    weight: number;
	
	    static createFrom(source: any = {}) {
	        return new BackendRefWeight(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.port = source["port"];
	        this.weight = source["weight"];
	    }
	}
	export class BlastRadiusInfo {
	    clusterName: string;
	    environment: string;
	    isProd: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BlastRadiusInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.clusterName = source["clusterName"];
	        this.environment = source["environment"];
	        this.isProd = source["isProd"];
	    }
	}
	export class BridgePort {
	    name: string;
	    port: number;
	    targetPort?: number;
	    protocol: string;
	
	    static createFrom(source: any = {}) {
	        return new BridgePort(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.port = source["port"];
	        this.targetPort = source["targetPort"];
	        this.protocol = source["protocol"];
	    }
	}
	export class BridgeResult {
	    serviceName: string;
	    namespace: string;
	    serviceYAML: string;
	    endpointsYAML?: string;
	    reachable?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BridgeResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.serviceName = source["serviceName"];
	        this.namespace = source["namespace"];
	        this.serviceYAML = source["serviceYAML"];
	        this.endpointsYAML = source["endpointsYAML"];
	        this.reachable = source["reachable"];
	    }
	}
	export class CanIResult {
	    verb: string;
	    resource: string;
	    namespace: string;
	    allowed: boolean;
	    reason?: string;
	
	    static createFrom(source: any = {}) {
	        return new CanIResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.verb = source["verb"];
	        this.resource = source["resource"];
	        this.namespace = source["namespace"];
	        this.allowed = source["allowed"];
	        this.reason = source["reason"];
	    }
	}
	export class DailyCost {
	    date: string;
	    costDay: number;
	
	    static createFrom(source: any = {}) {
	        return new DailyCost(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.date = source["date"];
	        this.costDay = source["costDay"];
	    }
	}
	export class CostCategory {
	    name: string;
	    costMo: number;
	    percentage: number;
	
	    static createFrom(source: any = {}) {
	        return new CostCategory(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.costMo = source["costMo"];
	        this.percentage = source["percentage"];
	    }
	}
	export class CostBreakdown {
	    name: string;
	    namespace?: string;
	    kind: string;
	    cpuCores: number;
	    memoryGB: number;
	    cpuCostHr: number;
	    memCostHr: number;
	    totalCostHr: number;
	    totalCostDay: number;
	    totalCostMo: number;
	    podCount: number;
	
	    static createFrom(source: any = {}) {
	        return new CostBreakdown(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.kind = source["kind"];
	        this.cpuCores = source["cpuCores"];
	        this.memoryGB = source["memoryGB"];
	        this.cpuCostHr = source["cpuCostHr"];
	        this.memCostHr = source["memCostHr"];
	        this.totalCostHr = source["totalCostHr"];
	        this.totalCostDay = source["totalCostDay"];
	        this.totalCostMo = source["totalCostMo"];
	        this.podCount = source["podCount"];
	    }
	}
	export class ClusterCostReport {
	    provider: string;
	    providerLabel: string;
	    namespaces: CostBreakdown[];
	    topDeployments: CostBreakdown[];
	    costCategories: CostCategory[];
	    dailyHistory: DailyCost[];
	    totalCostHr: number;
	    totalCostDay: number;
	    totalCostMo: number;
	    totalCostYear: number;
	    totalCpu: number;
	    totalMemGb: number;
	    podCount: number;
	
	    static createFrom(source: any = {}) {
	        return new ClusterCostReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.providerLabel = source["providerLabel"];
	        this.namespaces = this.convertValues(source["namespaces"], CostBreakdown);
	        this.topDeployments = this.convertValues(source["topDeployments"], CostBreakdown);
	        this.costCategories = this.convertValues(source["costCategories"], CostCategory);
	        this.dailyHistory = this.convertValues(source["dailyHistory"], DailyCost);
	        this.totalCostHr = source["totalCostHr"];
	        this.totalCostDay = source["totalCostDay"];
	        this.totalCostMo = source["totalCostMo"];
	        this.totalCostYear = source["totalCostYear"];
	        this.totalCpu = source["totalCpu"];
	        this.totalMemGb = source["totalMemGb"];
	        this.podCount = source["podCount"];
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
	export class ContextProbeResult {
	    name: string;
	    cluster: string;
	    active: boolean;
	    reachable: boolean;
	    latencyMs: number;
	    serverVersion?: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new ContextProbeResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.cluster = source["cluster"];
	        this.active = source["active"];
	        this.reachable = source["reachable"];
	        this.latencyMs = source["latencyMs"];
	        this.serverVersion = source["serverVersion"];
	        this.error = source["error"];
	    }
	}
	export class ContextResolution {
	    chosen: string;
	    confidence: string;
	    probes: ContextProbeResult[];
	    reachableCount: number;
	
	    static createFrom(source: any = {}) {
	        return new ContextResolution(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.chosen = source["chosen"];
	        this.confidence = source["confidence"];
	        this.probes = this.convertValues(source["probes"], ContextProbeResult);
	        this.reachableCount = source["reachableCount"];
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
	export class CorrelatedEvent {
	    timestamp: string;
	    type: string;
	    source: string;
	    message: string;
	    level: string;
	
	    static createFrom(source: any = {}) {
	        return new CorrelatedEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = source["timestamp"];
	        this.type = source["type"];
	        this.source = source["source"];
	        this.message = source["message"];
	        this.level = source["level"];
	    }
	}
	export class CorrelationResult {
	    podName: string;
	    namespace: string;
	    timeline: CorrelatedEvent[];
	    totalLogs: number;
	    totalEvents: number;
	
	    static createFrom(source: any = {}) {
	        return new CorrelationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.podName = source["podName"];
	        this.namespace = source["namespace"];
	        this.timeline = this.convertValues(source["timeline"], CorrelatedEvent);
	        this.totalLogs = source["totalLogs"];
	        this.totalEvents = source["totalEvents"];
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
	
	
	export class CrashInfo {
	    exitCode: number;
	    reason: string;
	    logs: string;
	    pod: string;
	    container: string;
	
	    static createFrom(source: any = {}) {
	        return new CrashInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.exitCode = source["exitCode"];
	        this.reason = source["reason"];
	        this.logs = source["logs"];
	        this.pod = source["pod"];
	        this.container = source["container"];
	    }
	}
	
	export class DebugSession {
	    podName: string;
	    namespace: string;
	    containerName: string;
	    debugImage: string;
	    started: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DebugSession(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.podName = source["podName"];
	        this.namespace = source["namespace"];
	        this.containerName = source["containerName"];
	        this.debugImage = source["debugImage"];
	        this.started = source["started"];
	    }
	}
	export class DeploymentRevision {
	    revision: string;
	    replicaSet: string;
	    image: string;
	    replicas: number;
	    readyReplicas: number;
	    active: boolean;
	    changeCause?: string;
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new DeploymentRevision(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.revision = source["revision"];
	        this.replicaSet = source["replicaSet"];
	        this.image = source["image"];
	        this.replicas = source["replicas"];
	        this.readyReplicas = source["readyReplicas"];
	        this.active = source["active"];
	        this.changeCause = source["changeCause"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
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
	export class DuplicateResult {
	    deployment: string;
	    namespace: string;
	
	    static createFrom(source: any = {}) {
	        return new DuplicateResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.deployment = source["deployment"];
	        this.namespace = source["namespace"];
	    }
	}
	export class EndpointEvent {
	    timestamp: string;
	    type: string;
	    ip: string;
	    podName?: string;
	    reason?: string;
	
	    static createFrom(source: any = {}) {
	        return new EndpointEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = source["timestamp"];
	        this.type = source["type"];
	        this.ip = source["ip"];
	        this.podName = source["podName"];
	        this.reason = source["reason"];
	    }
	}
	export class FailingPod {
	    name: string;
	    namespace: string;
	    status: string;
	    reason: string;
	    exitCode?: number;
	    logs?: string;
	
	    static createFrom(source: any = {}) {
	        return new FailingPod(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.status = source["status"];
	        this.reason = source["reason"];
	        this.exitCode = source["exitCode"];
	        this.logs = source["logs"];
	    }
	}
	export class EndpointReadiness {
	    serviceName: string;
	    namespace: string;
	    expectedEndpoints: number;
	    actualEndpoints: number;
	    missingCount: number;
	    healthy: boolean;
	    failingPods?: FailingPod[];
	    timeline?: EndpointEvent[];
	
	    static createFrom(source: any = {}) {
	        return new EndpointReadiness(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.serviceName = source["serviceName"];
	        this.namespace = source["namespace"];
	        this.expectedEndpoints = source["expectedEndpoints"];
	        this.actualEndpoints = source["actualEndpoints"];
	        this.missingCount = source["missingCount"];
	        this.healthy = source["healthy"];
	        this.failingPods = this.convertValues(source["failingPods"], FailingPod);
	        this.timeline = this.convertValues(source["timeline"], EndpointEvent);
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
	export class EndpointSliceInfo {
	    name: string;
	    namespace: string;
	    addressType: string;
	    endpoints: number;
	    readyCount: number;
	    zoneCount: Record<string, number>;
	    byService?: string;
	
	    static createFrom(source: any = {}) {
	        return new EndpointSliceInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.addressType = source["addressType"];
	        this.endpoints = source["endpoints"];
	        this.readyCount = source["readyCount"];
	        this.zoneCount = source["zoneCount"];
	        this.byService = source["byService"];
	    }
	}
	export class EnvVarSpec {
	    name: string;
	    value: string;
	
	    static createFrom(source: any = {}) {
	        return new EnvVarSpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.value = source["value"];
	    }
	}
	export class ExternalBridge {
	    name: string;
	    namespace: string;
	    reachable: boolean;
	    lastPing?: string;
	
	    static createFrom(source: any = {}) {
	        return new ExternalBridge(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.reachable = source["reachable"];
	        this.lastPing = source["lastPing"];
	    }
	}
	export class ExternalBridgeSpec {
	    name: string;
	    namespace: string;
	    type: string;
	    externalName?: string;
	    externalIPs?: string[];
	    ports: BridgePort[];
	
	    static createFrom(source: any = {}) {
	        return new ExternalBridgeSpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.type = source["type"];
	        this.externalName = source["externalName"];
	        this.externalIPs = source["externalIPs"];
	        this.ports = this.convertValues(source["ports"], BridgePort);
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
	
	export class GatewayCondition {
	    type: string;
	    status: string;
	    reason: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new GatewayCondition(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.status = source["status"];
	        this.reason = source["reason"];
	        this.message = source["message"];
	    }
	}
	export class RouteConflict {
	    hostname: string;
	    namespaceA: string;
	    routeNameA: string;
	    namespaceB: string;
	    routeNameB: string;
	
	    static createFrom(source: any = {}) {
	        return new RouteConflict(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hostname = source["hostname"];
	        this.namespaceA = source["namespaceA"];
	        this.routeNameA = source["routeNameA"];
	        this.namespaceB = source["namespaceB"];
	        this.routeNameB = source["routeNameB"];
	    }
	}
	export class RouteParentRef {
	    name: string;
	    namespace?: string;
	    group: string;
	    kind: string;
	
	    static createFrom(source: any = {}) {
	        return new RouteParentRef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.group = source["group"];
	        this.kind = source["kind"];
	    }
	}
	export class HTTPRouteSummary {
	    name: string;
	    namespace: string;
	    hostnames: string[];
	    parentRefs: RouteParentRef[];
	    conditions: GatewayCondition[];
	    matches: number;
	    backendRefs: BackendRefSummary[];
	
	    static createFrom(source: any = {}) {
	        return new HTTPRouteSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.hostnames = source["hostnames"];
	        this.parentRefs = this.convertValues(source["parentRefs"], RouteParentRef);
	        this.conditions = this.convertValues(source["conditions"], GatewayCondition);
	        this.matches = source["matches"];
	        this.backendRefs = this.convertValues(source["backendRefs"], BackendRefSummary);
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
	export class GatewaySummary {
	    name: string;
	    namespace: string;
	    className: string;
	    listeners: number;
	    addresses: string[];
	    conditions: GatewayCondition[];
	    attachedRoutes: number;
	
	    static createFrom(source: any = {}) {
	        return new GatewaySummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.className = source["className"];
	        this.listeners = source["listeners"];
	        this.addresses = source["addresses"];
	        this.conditions = this.convertValues(source["conditions"], GatewayCondition);
	        this.attachedRoutes = source["attachedRoutes"];
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
	export class GatewayRouteGraph {
	    gateways: GatewaySummary[];
	    httpRoutes: HTTPRouteSummary[];
	    conflicts?: RouteConflict[];
	
	    static createFrom(source: any = {}) {
	        return new GatewayRouteGraph(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.gateways = this.convertValues(source["gateways"], GatewaySummary);
	        this.httpRoutes = this.convertValues(source["httpRoutes"], HTTPRouteSummary);
	        this.conflicts = this.convertValues(source["conflicts"], RouteConflict);
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
	
	
	export class ImpersonationView {
	    user: string;
	    group?: string;
	    capabilities: CanIResult[];
	
	    static createFrom(source: any = {}) {
	        return new ImpersonationView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.user = source["user"];
	        this.group = source["group"];
	        this.capabilities = this.convertValues(source["capabilities"], CanIResult);
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
	export class OrphanedPod {
	    podName: string;
	    namespace: string;
	    hasMatchingSvc: boolean;
	
	    static createFrom(source: any = {}) {
	        return new OrphanedPod(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.podName = source["podName"];
	        this.namespace = source["namespace"];
	        this.hasMatchingSvc = source["hasMatchingSvc"];
	    }
	}
	export class LabelMismatch {
	    podName: string;
	    selectorKey: string;
	    selectorValue: string;
	    actualValue: string;
	    missing: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LabelMismatch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.podName = source["podName"];
	        this.selectorKey = source["selectorKey"];
	        this.selectorValue = source["selectorValue"];
	        this.actualValue = source["actualValue"];
	        this.missing = source["missing"];
	    }
	}
	export class LabelMatch {
	    podName: string;
	    complete: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LabelMatch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.podName = source["podName"];
	        this.complete = source["complete"];
	    }
	}
	export class LabelDiffResult {
	    serviceName: string;
	    namespace: string;
	    selector: Record<string, string>;
	    matches: LabelMatch[];
	    mismatches: LabelMismatch[];
	    orphanedPods?: OrphanedPod[];
	    hasIssues: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LabelDiffResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.serviceName = source["serviceName"];
	        this.namespace = source["namespace"];
	        this.selector = source["selector"];
	        this.matches = this.convertValues(source["matches"], LabelMatch);
	        this.mismatches = this.convertValues(source["mismatches"], LabelMismatch);
	        this.orphanedPods = this.convertValues(source["orphanedPods"], OrphanedPod);
	        this.hasIssues = source["hasIssues"];
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
	export class MigrationResult {
	    originalIngress: string;
	    gatewayYAML: string;
	    httpRouteYAML: string;
	    warnings?: string[];
	
	    static createFrom(source: any = {}) {
	        return new MigrationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.originalIngress = source["originalIngress"];
	        this.gatewayYAML = source["gatewayYAML"];
	        this.httpRouteYAML = source["httpRouteYAML"];
	        this.warnings = source["warnings"];
	    }
	}
	export class NodeCapacity {
	    cpu: string;
	    memory: string;
	
	    static createFrom(source: any = {}) {
	        return new NodeCapacity(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cpu = source["cpu"];
	        this.memory = source["memory"];
	    }
	}
	export class NodeLogEntry {
	    time: string;
	    level: string;
	    service: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new NodeLogEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.time = source["time"];
	        this.level = source["level"];
	        this.service = source["service"];
	        this.message = source["message"];
	    }
	}
	export class OrphanedEndpoint {
	    endpointName: string;
	    ip: string;
	    podName?: string;
	    namespace: string;
	
	    static createFrom(source: any = {}) {
	        return new OrphanedEndpoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.endpointName = source["endpointName"];
	        this.ip = source["ip"];
	        this.podName = source["podName"];
	        this.namespace = source["namespace"];
	    }
	}
	
	export class PortSpec {
	    name: string;
	    port: number;
	    targetPort?: number;
	    protocol: string;
	
	    static createFrom(source: any = {}) {
	        return new PortSpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.port = source["port"];
	        this.targetPort = source["targetPort"];
	        this.protocol = source["protocol"];
	    }
	}
	export class ProbeSpec {
	    type: string;
	    httpPath?: string;
	    httpPort?: number;
	    tcpSocketPort?: number;
	    command?: string;
	    initialDelaySeconds: number;
	    periodSeconds: number;
	    timeoutSeconds: number;
	    failureThreshold: number;
	
	    static createFrom(source: any = {}) {
	        return new ProbeSpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.httpPath = source["httpPath"];
	        this.httpPort = source["httpPort"];
	        this.tcpSocketPort = source["tcpSocketPort"];
	        this.command = source["command"];
	        this.initialDelaySeconds = source["initialDelaySeconds"];
	        this.periodSeconds = source["periodSeconds"];
	        this.timeoutSeconds = source["timeoutSeconds"];
	        this.failureThreshold = source["failureThreshold"];
	    }
	}
	export class RegistryTag {
	    tag: string;
	    size?: number;
	    pushedAt?: string;
	
	    static createFrom(source: any = {}) {
	        return new RegistryTag(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tag = source["tag"];
	        this.size = source["size"];
	        this.pushedAt = source["pushedAt"];
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
	    extra?: Record<string, any>;
	
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
	        this.extra = source["extra"];
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
	export class ResourceProfile {
	    cpuRequest?: string;
	    memoryRequest?: string;
	    cpuLimit?: string;
	    memoryLimit?: string;
	
	    static createFrom(source: any = {}) {
	        return new ResourceProfile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cpuRequest = source["cpuRequest"];
	        this.memoryRequest = source["memoryRequest"];
	        this.cpuLimit = source["cpuLimit"];
	        this.memoryLimit = source["memoryLimit"];
	    }
	}
	
	export class ResourceSuggestion {
	    profile: ResourceProfile;
	    nodeCapacity?: NodeCapacity[];
	
	    static createFrom(source: any = {}) {
	        return new ResourceSuggestion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profile = this.convertValues(source["profile"], ResourceProfile);
	        this.nodeCapacity = this.convertValues(source["nodeCapacity"], NodeCapacity);
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
	
	
	export class ServicePod {
	    name: string;
	    namespace: string;
	    status: string;
	    container: string;
	
	    static createFrom(source: any = {}) {
	        return new ServicePod(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.status = source["status"];
	        this.container = source["container"];
	    }
	}
	export class ServiceSelectorInfo {
	    serviceName: string;
	    namespace: string;
	    selector: Record<string, string>;
	    matchingPods: number;
	
	    static createFrom(source: any = {}) {
	        return new ServiceSelectorInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.serviceName = source["serviceName"];
	        this.namespace = source["namespace"];
	        this.selector = source["selector"];
	        this.matchingPods = source["matchingPods"];
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
	export class TopologyWarning {
	    serviceName: string;
	    namespace: string;
	    maxZonePct: number;
	    totalInZone: number;
	    totalEndpoints: number;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new TopologyWarning(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.serviceName = source["serviceName"];
	        this.namespace = source["namespace"];
	        this.maxZonePct = source["maxZonePct"];
	        this.totalInZone = source["totalInZone"];
	        this.totalEndpoints = source["totalEndpoints"];
	        this.message = source["message"];
	    }
	}
	export class VPAContainerRecommend {
	    containerName: string;
	    lowerCpu: string;
	    lowerMemory: string;
	    targetCpu: string;
	    targetMemory: string;
	    upperCpu: string;
	    upperMemory: string;
	    uncappedCpu?: string;
	    uncappedMemory?: string;
	
	    static createFrom(source: any = {}) {
	        return new VPAContainerRecommend(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.containerName = source["containerName"];
	        this.lowerCpu = source["lowerCpu"];
	        this.lowerMemory = source["lowerMemory"];
	        this.targetCpu = source["targetCpu"];
	        this.targetMemory = source["targetMemory"];
	        this.upperCpu = source["upperCpu"];
	        this.upperMemory = source["upperMemory"];
	        this.uncappedCpu = source["uncappedCpu"];
	        this.uncappedMemory = source["uncappedMemory"];
	    }
	}
	export class VPARecommendation {
	    name: string;
	    namespace: string;
	    targetRef: string;
	    updateMode: string;
	    containers: VPAContainerRecommend[];
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new VPARecommendation(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.targetRef = source["targetRef"];
	        this.updateMode = source["updateMode"];
	        this.containers = this.convertValues(source["containers"], VPAContainerRecommend);
	        this.createdAt = this.convertValues(source["createdAt"], null);
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
	export class WasteItem {
	    name: string;
	    cpuRequest: string;
	    memoryRequest: string;
	    cpuUsage?: string;
	    memoryUsage?: string;
	    wasteCPU?: string;
	    wasteMem?: string;
	    ratio?: string;
	
	    static createFrom(source: any = {}) {
	        return new WasteItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.cpuRequest = source["cpuRequest"];
	        this.memoryRequest = source["memoryRequest"];
	        this.cpuUsage = source["cpuUsage"];
	        this.memoryUsage = source["memoryUsage"];
	        this.wasteCPU = source["wasteCPU"];
	        this.wasteMem = source["wasteMem"];
	        this.ratio = source["ratio"];
	    }
	}
	export class WasteProfile {
	    namespace: string;
	    deployments: WasteItem[];
	    totalWasteCPU: string;
	    totalWasteMem: string;
	    score: string;
	
	    static createFrom(source: any = {}) {
	        return new WasteProfile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.namespace = source["namespace"];
	        this.deployments = this.convertValues(source["deployments"], WasteItem);
	        this.totalWasteCPU = source["totalWasteCPU"];
	        this.totalWasteMem = source["totalWasteMem"];
	        this.score = source["score"];
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
	export class WorkloadSpec {
	    name: string;
	    namespace: string;
	    image: string;
	    replicas: number;
	    labels?: Record<string, string>;
	    ports?: PortSpec[];
	    envVars?: EnvVarSpec[];
	    resources?: ResourceProfile;
	    liveness?: ProbeSpec;
	    readiness?: ProbeSpec;
	    startup?: ProbeSpec;
	    generateSvc: boolean;
	
	    static createFrom(source: any = {}) {
	        return new WorkloadSpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.image = source["image"];
	        this.replicas = source["replicas"];
	        this.labels = source["labels"];
	        this.ports = this.convertValues(source["ports"], PortSpec);
	        this.envVars = this.convertValues(source["envVars"], EnvVarSpec);
	        this.resources = this.convertValues(source["resources"], ResourceProfile);
	        this.liveness = this.convertValues(source["liveness"], ProbeSpec);
	        this.readiness = this.convertValues(source["readiness"], ProbeSpec);
	        this.startup = this.convertValues(source["startup"], ProbeSpec);
	        this.generateSvc = source["generateSvc"];
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
	export class WorkloadYAML {
	    deployment: string;
	    service?: string;
	
	    static createFrom(source: any = {}) {
	        return new WorkloadYAML(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.deployment = source["deployment"];
	        this.service = source["service"];
	    }
	}
	export class ZoneDistribution {
	    totalEndpoints: number;
	    zoneCounts: Record<string, number>;
	    imbalanced: boolean;
	    maxZonePct: number;
	
	    static createFrom(source: any = {}) {
	        return new ZoneDistribution(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalEndpoints = source["totalEndpoints"];
	        this.zoneCounts = source["zoneCounts"];
	        this.imbalanced = source["imbalanced"];
	        this.maxZonePct = source["maxZonePct"];
	    }
	}

}

export namespace loadtest {
	
	export class Payload {
	    kind: string;
	    filename?: string;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new Payload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.filename = source["filename"];
	        this.size = source["size"];
	    }
	}
	export class ScalePlan {
	    namespace?: string;
	    deployment?: string;
	    preScaleToZero?: boolean;
	    minReplicas?: number;
	    preScaleTimeoutNs?: number;
	    postScaleTimeoutNs?: number;
	    drainObserveDurationNs?: number;
	
	    static createFrom(source: any = {}) {
	        return new ScalePlan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.namespace = source["namespace"];
	        this.deployment = source["deployment"];
	        this.preScaleToZero = source["preScaleToZero"];
	        this.minReplicas = source["minReplicas"];
	        this.preScaleTimeoutNs = source["preScaleTimeoutNs"];
	        this.postScaleTimeoutNs = source["postScaleTimeoutNs"];
	        this.drainObserveDurationNs = source["drainObserveDurationNs"];
	    }
	}
	export class Ramp {
	    kind: string;
	    durationNs?: number;
	    rate?: number;
	    rampTo?: number;
	    stepEveryNs?: number;
	    stepBy?: number;
	    spikeCount?: number;
	    spikeSize?: number;
	    spikeIdleNs?: number;
	
	    static createFrom(source: any = {}) {
	        return new Ramp(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.durationNs = source["durationNs"];
	        this.rate = source["rate"];
	        this.rampTo = source["rampTo"];
	        this.stepEveryNs = source["stepEveryNs"];
	        this.stepBy = source["stepBy"];
	        this.spikeCount = source["spikeCount"];
	        this.spikeSize = source["spikeSize"];
	        this.spikeIdleNs = source["spikeIdleNs"];
	    }
	}
	export class RunSpec {
	    name?: string;
	    broker: broker.Config;
	    destination: string;
	    payload: Payload;
	    count: number;
	    workers?: number;
	    ramp: Ramp;
	    scale: ScalePlan;
	
	    static createFrom(source: any = {}) {
	        return new RunSpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.broker = this.convertValues(source["broker"], broker.Config);
	        this.destination = source["destination"];
	        this.payload = this.convertValues(source["payload"], Payload);
	        this.count = source["count"];
	        this.workers = source["workers"];
	        this.ramp = this.convertValues(source["ramp"], Ramp);
	        this.scale = this.convertValues(source["scale"], ScalePlan);
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
	export class Preset {
	    id: string;
	    name: string;
	    description: string;
	    whenToUse: string;
	    spec: RunSpec;
	
	    static createFrom(source: any = {}) {
	        return new Preset(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.whenToUse = source["whenToUse"];
	        this.spec = this.convertValues(source["spec"], RunSpec);
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
	
	export class Summary {
	    sent: number;
	    acked: number;
	    errors: number;
	    durationNs: number;
	    throughputPerSec: number;
	    p50AckLatencyNs: number;
	    p95AckLatencyNs: number;
	    p99AckLatencyNs: number;
	    maxAckLatencyNs: number;
	    errorBreakdown?: Record<string, number>;
	
	    static createFrom(source: any = {}) {
	        return new Summary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sent = source["sent"];
	        this.acked = source["acked"];
	        this.errors = source["errors"];
	        this.durationNs = source["durationNs"];
	        this.throughputPerSec = source["throughputPerSec"];
	        this.p50AckLatencyNs = source["p50AckLatencyNs"];
	        this.p95AckLatencyNs = source["p95AckLatencyNs"];
	        this.p99AckLatencyNs = source["p99AckLatencyNs"];
	        this.maxAckLatencyNs = source["maxAckLatencyNs"];
	        this.errorBreakdown = source["errorBreakdown"];
	    }
	}
	export class ScaleEvent {
	    // Go type: time
	    at: any;
	    phase: string;
	    replicas: number;
	    ready: number;
	
	    static createFrom(source: any = {}) {
	        return new ScaleEvent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.at = this.convertValues(source["at"], null);
	        this.phase = source["phase"];
	        this.replicas = source["replicas"];
	        this.ready = source["ready"];
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
	export class Sample {
	    // Go type: time
	    at: any;
	    ackLatencyNs: number;
	    ok: boolean;
	    err?: string;
	
	    static createFrom(source: any = {}) {
	        return new Sample(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.at = this.convertValues(source["at"], null);
	        this.ackLatencyNs = source["ackLatencyNs"];
	        this.ok = source["ok"];
	        this.err = source["err"];
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
	export class RunRecord {
	    spec: RunSpec;
	    brokerKind: string;
	    // Go type: time
	    started: any;
	    // Go type: time
	    finished: any;
	    samples?: Sample[];
	    scaleLog?: ScaleEvent[];
	    summary: Summary;
	    finalError?: string;
	
	    static createFrom(source: any = {}) {
	        return new RunRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.spec = this.convertValues(source["spec"], RunSpec);
	        this.brokerKind = source["brokerKind"];
	        this.started = this.convertValues(source["started"], null);
	        this.finished = this.convertValues(source["finished"], null);
	        this.samples = this.convertValues(source["samples"], Sample);
	        this.scaleLog = this.convertValues(source["scaleLog"], ScaleEvent);
	        this.summary = this.convertValues(source["summary"], Summary);
	        this.finalError = source["finalError"];
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

export namespace oauthproviders {
	
	export class ProviderInfo {
	    name: string;
	    displayName: string;
	
	    static createFrom(source: any = {}) {
	        return new ProviderInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.displayName = source["displayName"];
	    }
	}
	export class UserInfo {
	    provider: string;
	    id: string;
	    email: string;
	    name: string;
	    raw?: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new UserInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.id = source["id"];
	        this.email = source["email"];
	        this.name = source["name"];
	        this.raw = source["raw"];
	    }
	}

}

export namespace pkg {
	
	export class ArgusCDStatus {
	    connected: boolean;
	    url: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new ArgusCDStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.connected = source["connected"];
	        this.url = source["url"];
	        this.message = source["message"];
	    }
	}
	export class CodeReviewReport {
	    id: string;
	    title: string;
	    provider: string;
	    prRef: string;
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new CodeReviewReport(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.provider = source["provider"];
	        this.prRef = source["prRef"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
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
	export class DBConnectionInput {
	    id?: string;
	    name: string;
	    db_type: string;
	    host: string;
	    port: number;
	    user: string;
	    password?: string;
	    db_name: string;
	    ssl_mode: string;
	    pool_size: number;
	    tags: string[];
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DBConnectionInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.db_type = source["db_type"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.user = source["user"];
	        this.password = source["password"];
	        this.db_name = source["db_name"];
	        this.ssl_mode = source["ssl_mode"];
	        this.pool_size = source["pool_size"];
	        this.tags = source["tags"];
	        this.enabled = source["enabled"];
	    }
	}
	export class DBConnectionTestResult {
	    ok: boolean;
	    message: string;
	    latency_ms: number;
	
	    static createFrom(source: any = {}) {
	        return new DBConnectionTestResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ok = source["ok"];
	        this.message = source["message"];
	        this.latency_ms = source["latency_ms"];
	    }
	}
	export class DBConnectionView {
	    id: string;
	    name: string;
	    db_type: string;
	    host: string;
	    port: number;
	    user: string;
	    db_name: string;
	    ssl_mode: string;
	    pool_size: number;
	    tags: string[];
	    enabled: boolean;
	    created_at: number;
	    updated_at: number;
	    has_password: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DBConnectionView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.db_type = source["db_type"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.user = source["user"];
	        this.db_name = source["db_name"];
	        this.ssl_mode = source["ssl_mode"];
	        this.pool_size = source["pool_size"];
	        this.tags = source["tags"];
	        this.enabled = source["enabled"];
	        this.created_at = source["created_at"];
	        this.updated_at = source["updated_at"];
	        this.has_password = source["has_password"];
	    }
	}
	export class EnvVarSpec {
	    name: string;
	    required: boolean;
	    hint: string;
	    default?: string;
	
	    static createFrom(source: any = {}) {
	        return new EnvVarSpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.required = source["required"];
	        this.hint = source["hint"];
	        this.default = source["default"];
	    }
	}
	export class DeployArtifact {
	    tool: string;
	    flavor: string;
	    description: string;
	    commandText: string;
	    fileText: string;
	    fileName: string;
	    envVars: EnvVarSpec[];
	
	    static createFrom(source: any = {}) {
	        return new DeployArtifact(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tool = source["tool"];
	        this.flavor = source["flavor"];
	        this.description = source["description"];
	        this.commandText = source["commandText"];
	        this.fileText = source["fileText"];
	        this.fileName = source["fileName"];
	        this.envVars = this.convertValues(source["envVars"], EnvVarSpec);
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
	export class EnvValidationResult {
	    tool: string;
	    flavor: string;
	    missing: string[];
	    present: string[];
	    vars: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new EnvValidationResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tool = source["tool"];
	        this.flavor = source["flavor"];
	        this.missing = source["missing"];
	        this.present = source["present"];
	        this.vars = source["vars"];
	    }
	}
	
	export class GChatSpaceView {
	    id: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new GChatSpaceView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	    }
	}
	export class GitHubBranch {
	    name: string;
	    sha: string;
	    protected: boolean;
	
	    static createFrom(source: any = {}) {
	        return new GitHubBranch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.sha = source["sha"];
	        this.protected = source["protected"];
	    }
	}
	export class GitHubPullRequest {
	    number: number;
	    title: string;
	    state: string;
	    author: string;
	    branch: string;
	    base: string;
	    url: string;
	    draft: boolean;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new GitHubPullRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.number = source["number"];
	        this.title = source["title"];
	        this.state = source["state"];
	        this.author = source["author"];
	        this.branch = source["branch"];
	        this.base = source["base"];
	        this.url = source["url"];
	        this.draft = source["draft"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class LocalQuotaStatus {
	    used: number;
	    limit: number;
	    resetAt: number;
	    isPro: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LocalQuotaStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.used = source["used"];
	        this.limit = source["limit"];
	        this.resetAt = source["resetAt"];
	        this.isPro = source["isPro"];
	    }
	}
	export class NextSuggestionResult {
	    suggestion?: userprofile.Suggestion;
	    suppressed: boolean;
	    reason?: string;
	
	    static createFrom(source: any = {}) {
	        return new NextSuggestionResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.suggestion = this.convertValues(source["suggestion"], userprofile.Suggestion);
	        this.suppressed = source["suppressed"];
	        this.reason = source["reason"];
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
	export class OAuthPollResult {
	    status: string;
	    user?: oauthproviders.UserInfo;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new OAuthPollResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.user = this.convertValues(source["user"], oauthproviders.UserInfo);
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
	export class PayloadFileInfo {
	    name: string;
	    size: number;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new PayloadFileInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.size = source["size"];
	        this.path = source["path"];
	    }
	}
	export class PayloadPathResolution {
	    kind: string;
	    files: PayloadFileInfo[];
	    sample?: string;
	
	    static createFrom(source: any = {}) {
	        return new PayloadPathResolution(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.files = this.convertValues(source["files"], PayloadFileInfo);
	        this.sample = source["sample"];
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
	export class SecretRefInfo {
	    kind: string;
	    value: string;
	    key: string;
	    resolvable: boolean;
	    supported: boolean;
	    description: string;
	
	    static createFrom(source: any = {}) {
	        return new SecretRefInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.value = source["value"];
	        this.key = source["key"];
	        this.resolvable = source["resolvable"];
	        this.supported = source["supported"];
	        this.description = source["description"];
	    }
	}
	export class SecretSource {
	    name: string;
	    namespace: string;
	    kind: string;
	    type?: string;
	    encrypted: boolean;
	    hint?: string;
	
	    static createFrom(source: any = {}) {
	        return new SecretSource(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.namespace = source["namespace"];
	        this.kind = source["kind"];
	        this.type = source["type"];
	        this.encrypted = source["encrypted"];
	        this.hint = source["hint"];
	    }
	}
	export class SecretStoreInfo {
	    backend: string;
	    available: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SecretStoreInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.backend = source["backend"];
	        this.available = source["available"];
	    }
	}
	export class SecretsToolStatus {
	    tool: string;
	    found: boolean;
	    version?: string;
	    path?: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new SecretsToolStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tool = source["tool"];
	        this.found = source["found"];
	        this.version = source["version"];
	        this.path = source["path"];
	        this.error = source["error"];
	    }
	}
	export class SettingsPayload {
	    kubeconfigPath: string;
	    currentContext: string;
	    namespace: string;
	    deepseekApiKey: string;
	    llmBaseUrl: string;
	    llmModel: string;
	    mcpServersConfig: string;
	    agentInstructions: string;
	    anomstackUrl: string;
	    prometheusUrl: string;
	    argocdUrl: string;
	    argocdToken: string;
	    argocdInsecure: boolean;
	    snykToken: string;
	    trivyBinary: string;
	    falcoUrl: string;
	    pipelinesEnabled: boolean;
	    pipelinesProvider: string;
	    githubToken: string;
	    githubOwner: string;
	    githubRepo: string;
	    githubWorkflow: string;
	    gitlabUrl: string;
	    gitlabToken: string;
	    gitlabProjectId: string;
	    gitlabRef: string;
	    awsRegion: string;
	    awsAccessKey: string;
	    awsSecretKey: string;
	    awsProject: string;
	    gcpProject: string;
	    gcpRegion: string;
	    gcpCredentials: string;
	    circleciToken: string;
	    circleciProjectSlug: string;
	    azureOrganization: string;
	    azureProject: string;
	    azurePipelineId: string;
	    azureToken: string;
	    azureBranch: string;
	    notifyOnPrOpened: boolean;
	    notifyOnPrUpdated: boolean;
	    notifyOnPrCommented: boolean;
	    notifyOnPrMerged: boolean;
	    autoCodeReview: boolean;
	    codeReviewDestination: string;
	    gdriveFolderId: string;
	    codeReviewS3Prefix: string;
	    codeReviewEmailTo: string;
	    confluenceUrl: string;
	    confluenceEmail: string;
	    confluenceToken: string;
	    confluenceSpaceKey: string;
	    confluenceParentPageId: string;
	    notionToken: string;
	    notionDatabaseId: string;
	    evernoteToken: string;
	    evernoteNotebookGuid: string;
	    onenoteToken: string;
	    onenoteSectionId: string;
	    amplenoteApiKey: string;
	    standardNotesUrl: string;
	    standardNotesToken: string;
	    obsidianVaultPath: string;
	    joplinUrl: string;
	    joplinToken: string;
	    logseqGraphPath: string;
	    bearToken: string;
	    tier: string;
	    logLevel: string;
	
	    static createFrom(source: any = {}) {
	        return new SettingsPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kubeconfigPath = source["kubeconfigPath"];
	        this.currentContext = source["currentContext"];
	        this.namespace = source["namespace"];
	        this.deepseekApiKey = source["deepseekApiKey"];
	        this.llmBaseUrl = source["llmBaseUrl"];
	        this.llmModel = source["llmModel"];
	        this.mcpServersConfig = source["mcpServersConfig"];
	        this.agentInstructions = source["agentInstructions"];
	        this.anomstackUrl = source["anomstackUrl"];
	        this.prometheusUrl = source["prometheusUrl"];
	        this.argocdUrl = source["argocdUrl"];
	        this.argocdToken = source["argocdToken"];
	        this.argocdInsecure = source["argocdInsecure"];
	        this.snykToken = source["snykToken"];
	        this.trivyBinary = source["trivyBinary"];
	        this.falcoUrl = source["falcoUrl"];
	        this.pipelinesEnabled = source["pipelinesEnabled"];
	        this.pipelinesProvider = source["pipelinesProvider"];
	        this.githubToken = source["githubToken"];
	        this.githubOwner = source["githubOwner"];
	        this.githubRepo = source["githubRepo"];
	        this.githubWorkflow = source["githubWorkflow"];
	        this.gitlabUrl = source["gitlabUrl"];
	        this.gitlabToken = source["gitlabToken"];
	        this.gitlabProjectId = source["gitlabProjectId"];
	        this.gitlabRef = source["gitlabRef"];
	        this.awsRegion = source["awsRegion"];
	        this.awsAccessKey = source["awsAccessKey"];
	        this.awsSecretKey = source["awsSecretKey"];
	        this.awsProject = source["awsProject"];
	        this.gcpProject = source["gcpProject"];
	        this.gcpRegion = source["gcpRegion"];
	        this.gcpCredentials = source["gcpCredentials"];
	        this.circleciToken = source["circleciToken"];
	        this.circleciProjectSlug = source["circleciProjectSlug"];
	        this.azureOrganization = source["azureOrganization"];
	        this.azureProject = source["azureProject"];
	        this.azurePipelineId = source["azurePipelineId"];
	        this.azureToken = source["azureToken"];
	        this.azureBranch = source["azureBranch"];
	        this.notifyOnPrOpened = source["notifyOnPrOpened"];
	        this.notifyOnPrUpdated = source["notifyOnPrUpdated"];
	        this.notifyOnPrCommented = source["notifyOnPrCommented"];
	        this.notifyOnPrMerged = source["notifyOnPrMerged"];
	        this.autoCodeReview = source["autoCodeReview"];
	        this.codeReviewDestination = source["codeReviewDestination"];
	        this.gdriveFolderId = source["gdriveFolderId"];
	        this.codeReviewS3Prefix = source["codeReviewS3Prefix"];
	        this.codeReviewEmailTo = source["codeReviewEmailTo"];
	        this.confluenceUrl = source["confluenceUrl"];
	        this.confluenceEmail = source["confluenceEmail"];
	        this.confluenceToken = source["confluenceToken"];
	        this.confluenceSpaceKey = source["confluenceSpaceKey"];
	        this.confluenceParentPageId = source["confluenceParentPageId"];
	        this.notionToken = source["notionToken"];
	        this.notionDatabaseId = source["notionDatabaseId"];
	        this.evernoteToken = source["evernoteToken"];
	        this.evernoteNotebookGuid = source["evernoteNotebookGuid"];
	        this.onenoteToken = source["onenoteToken"];
	        this.onenoteSectionId = source["onenoteSectionId"];
	        this.amplenoteApiKey = source["amplenoteApiKey"];
	        this.standardNotesUrl = source["standardNotesUrl"];
	        this.standardNotesToken = source["standardNotesToken"];
	        this.obsidianVaultPath = source["obsidianVaultPath"];
	        this.joplinUrl = source["joplinUrl"];
	        this.joplinToken = source["joplinToken"];
	        this.logseqGraphPath = source["logseqGraphPath"];
	        this.bearToken = source["bearToken"];
	        this.tier = source["tier"];
	        this.logLevel = source["logLevel"];
	    }
	}
	export class SlackChannelView {
	    id: string;
	    name: string;
	
	    static createFrom(source: any = {}) {
	        return new SlackChannelView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	    }
	}
	export class UsagePayload {
	    // Go type: usage
	    today: any;
	    // Go type: usage
	    month: any;
	    // Go type: usage
	    lifetime: any;
	    byModel: usage.ModelTotals[];
	    // Go type: usage
	    rates: any;
	    // Go type: time
	    firstRecordedAt?: any;
	    monthlyBudget: number;
	
	    static createFrom(source: any = {}) {
	        return new UsagePayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.today = this.convertValues(source["today"], null);
	        this.month = this.convertValues(source["month"], null);
	        this.lifetime = this.convertValues(source["lifetime"], null);
	        this.byModel = this.convertValues(source["byModel"], usage.ModelTotals);
	        this.rates = this.convertValues(source["rates"], null);
	        this.firstRecordedAt = this.convertValues(source["firstRecordedAt"], null);
	        this.monthlyBudget = source["monthlyBudget"];
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
	export class VaultEntry {
	    id: string;
	    label: string;
	    kind: string;
	    status: string;
	    message?: string;
	    configured: boolean;
	    probable: boolean;
	    configureAnchor: string;
	    lastCheckedAt?: string;
	
	    static createFrom(source: any = {}) {
	        return new VaultEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	        this.kind = source["kind"];
	        this.status = source["status"];
	        this.message = source["message"];
	        this.configured = source["configured"];
	        this.probable = source["probable"];
	        this.configureAnchor = source["configureAnchor"];
	        this.lastCheckedAt = source["lastCheckedAt"];
	    }
	}
	export class VaultSecret {
	    key: string;
	    valueMask: string;
	    notes?: string;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new VaultSecret(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.valueMask = source["valueMask"];
	        this.notes = source["notes"];
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
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
	export class WorkspaceConnectionView {
	    id: string;
	    service: string;
	    external_workspace_id?: string;
	    display_name: string;
	    email?: string;
	    avatar_url?: string;
	    connected_at: number;
	    updated_at: number;
	
	    static createFrom(source: any = {}) {
	        return new WorkspaceConnectionView(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.service = source["service"];
	        this.external_workspace_id = source["external_workspace_id"];
	        this.display_name = source["display_name"];
	        this.email = source["email"];
	        this.avatar_url = source["avatar_url"];
	        this.connected_at = source["connected_at"];
	        this.updated_at = source["updated_at"];
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

export namespace saasapi {
	
	export class CreditTransaction {
	    id: string;
	    amount: number;
	    type: string;
	    runId?: string;
	    // Go type: time
	    createdAt: any;
	    note?: string;
	
	    static createFrom(source: any = {}) {
	        return new CreditTransaction(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.amount = source["amount"];
	        this.type = source["type"];
	        this.runId = source["runId"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.note = source["note"];
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
	export class DistLoadPayload {
	    source: string;
	    bytes?: string;
	    filename?: string;
	    filePath?: string;
	    fileMode?: string;
	    aiPrompt?: string;
	
	    static createFrom(source: any = {}) {
	        return new DistLoadPayload(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.bytes = source["bytes"];
	        this.filename = source["filename"];
	        this.filePath = source["filePath"];
	        this.fileMode = source["fileMode"];
	        this.aiPrompt = source["aiPrompt"];
	    }
	}
	export class DistLoadRamp {
	    profile: string;
	    rate?: number;
	    rampTo?: number;
	    stepBy?: number;
	    stepEverySec?: number;
	    spikeCount?: number;
	    spikeSize?: number;
	    spikeIdleSec?: number;
	    durationSec?: number;
	
	    static createFrom(source: any = {}) {
	        return new DistLoadRamp(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.profile = source["profile"];
	        this.rate = source["rate"];
	        this.rampTo = source["rampTo"];
	        this.stepBy = source["stepBy"];
	        this.stepEverySec = source["stepEverySec"];
	        this.spikeCount = source["spikeCount"];
	        this.spikeSize = source["spikeSize"];
	        this.spikeIdleSec = source["spikeIdleSec"];
	        this.durationSec = source["durationSec"];
	    }
	}
	export class FieldCheck {
	    path: string;
	    kind: string;
	    value?: any;
	
	    static createFrom(source: any = {}) {
	        return new FieldCheck(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.kind = source["kind"];
	        this.value = source["value"];
	    }
	}
	export class RESTExpect {
	    status?: number;
	    bodyMatches?: string;
	    fieldChecks?: FieldCheck[];
	
	    static createFrom(source: any = {}) {
	        return new RESTExpect(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.status = source["status"];
	        this.bodyMatches = source["bodyMatches"];
	        this.fieldChecks = this.convertValues(source["fieldChecks"], FieldCheck);
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
	export class RESTEndpoint {
	    name?: string;
	    method: string;
	    url: string;
	    headers?: Record<string, string>;
	    body?: string;
	    expect?: RESTExpect;
	    chain?: RESTEndpoint[];
	
	    static createFrom(source: any = {}) {
	        return new RESTEndpoint(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.method = source["method"];
	        this.url = source["url"];
	        this.headers = source["headers"];
	        this.body = source["body"];
	        this.expect = this.convertValues(source["expect"], RESTExpect);
	        this.chain = this.convertValues(source["chain"], RESTEndpoint);
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
	export class RESTAuth {
	    mode: string;
	    bearerAuthUrl?: string;
	    bearerMethod?: string;
	    bearerBody?: string;
	    bearerHeaders?: Record<string, string>;
	    bearerTokenPath?: string;
	    apiKeyHeader?: string;
	    apiKeyValue?: string;
	
	    static createFrom(source: any = {}) {
	        return new RESTAuth(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.bearerAuthUrl = source["bearerAuthUrl"];
	        this.bearerMethod = source["bearerMethod"];
	        this.bearerBody = source["bearerBody"];
	        this.bearerHeaders = source["bearerHeaders"];
	        this.bearerTokenPath = source["bearerTokenPath"];
	        this.apiKeyHeader = source["apiKeyHeader"];
	        this.apiKeyValue = source["apiKeyValue"];
	    }
	}
	export class DistLoadScenario {
	    auth?: RESTAuth;
	    endpoints: RESTEndpoint[];
	
	    static createFrom(source: any = {}) {
	        return new DistLoadScenario(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.auth = this.convertValues(source["auth"], RESTAuth);
	        this.endpoints = this.convertValues(source["endpoints"], RESTEndpoint);
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
	export class RegionSpec {
	    provider: string;
	    region: string;
	    instanceType: string;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new RegionSpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.region = source["region"];
	        this.instanceType = source["instanceType"];
	        this.count = source["count"];
	    }
	}
	export class DistLoadSpec {
	    name: string;
	    regions: RegionSpec[];
	    broker: number[];
	    destination: string;
	    payloadSize: number;
	    count: number;
	    workers: number;
	    rampProfile: string;
	    rampRate: number;
	    timeoutMins: number;
	    ramp?: DistLoadRamp;
	    runner?: string;
	    presetId?: string;
	    payload?: DistLoadPayload;
	    scenario?: DistLoadScenario;
	
	    static createFrom(source: any = {}) {
	        return new DistLoadSpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.regions = this.convertValues(source["regions"], RegionSpec);
	        this.broker = source["broker"];
	        this.destination = source["destination"];
	        this.payloadSize = source["payloadSize"];
	        this.count = source["count"];
	        this.workers = source["workers"];
	        this.rampProfile = source["rampProfile"];
	        this.rampRate = source["rampRate"];
	        this.timeoutMins = source["timeoutMins"];
	        this.ramp = this.convertValues(source["ramp"], DistLoadRamp);
	        this.runner = source["runner"];
	        this.presetId = source["presetId"];
	        this.payload = this.convertValues(source["payload"], DistLoadPayload);
	        this.scenario = this.convertValues(source["scenario"], DistLoadScenario);
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
	export class LoadSummary {
	    totalSent: number;
	    totalAcked: number;
	    totalErrors: number;
	    throughput: number;
	    p50LatencyMs: number;
	    p95LatencyMs: number;
	    p99LatencyMs: number;
	    durationSec: number;
	
	    static createFrom(source: any = {}) {
	        return new LoadSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalSent = source["totalSent"];
	        this.totalAcked = source["totalAcked"];
	        this.totalErrors = source["totalErrors"];
	        this.throughput = source["throughput"];
	        this.p50LatencyMs = source["p50LatencyMs"];
	        this.p95LatencyMs = source["p95LatencyMs"];
	        this.p99LatencyMs = source["p99LatencyMs"];
	        this.durationSec = source["durationSec"];
	    }
	}
	export class WorkerStatus {
	    region: string;
	    sent: number;
	    acked: number;
	    errors: number;
	    throughput: number;
	    p50Ms: number;
	    p95Ms: number;
	    p99Ms: number;
	    state: string;
	
	    static createFrom(source: any = {}) {
	        return new WorkerStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.region = source["region"];
	        this.sent = source["sent"];
	        this.acked = source["acked"];
	        this.errors = source["errors"];
	        this.throughput = source["throughput"];
	        this.p50Ms = source["p50Ms"];
	        this.p95Ms = source["p95Ms"];
	        this.p99Ms = source["p99Ms"];
	        this.state = source["state"];
	    }
	}
	export class ProvisionInfo {
	    region: string;
	    vmsSpec: number;
	    vmsReady: number;
	    state: string;
	    errorMessage?: string;
	
	    static createFrom(source: any = {}) {
	        return new ProvisionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.region = source["region"];
	        this.vmsSpec = source["vmsSpec"];
	        this.vmsReady = source["vmsReady"];
	        this.state = source["state"];
	        this.errorMessage = source["errorMessage"];
	    }
	}
	export class DistLoadStatus {
	    runId: string;
	    state: string;
	    name: string;
	    provisionProgress?: ProvisionInfo[];
	    workers?: WorkerStatus[];
	    summary?: LoadSummary;
	    creditsUsed?: number;
	    creditsEstimated?: number;
	    error?: string;
	    // Go type: time
	    startedAt: any;
	    // Go type: time
	    finishedAt?: any;
	
	    static createFrom(source: any = {}) {
	        return new DistLoadStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.runId = source["runId"];
	        this.state = source["state"];
	        this.name = source["name"];
	        this.provisionProgress = this.convertValues(source["provisionProgress"], ProvisionInfo);
	        this.workers = this.convertValues(source["workers"], WorkerStatus);
	        this.summary = this.convertValues(source["summary"], LoadSummary);
	        this.creditsUsed = source["creditsUsed"];
	        this.creditsEstimated = source["creditsEstimated"];
	        this.error = source["error"];
	        this.startedAt = this.convertValues(source["startedAt"], null);
	        this.finishedAt = this.convertValues(source["finishedAt"], null);
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
	
	
	export class LoadUsageSummary {
	    totalCreditsUsed: number;
	    thisMonth: number;
	    runsThisMonth: number;
	    avgCostPerRun: number;
	
	    static createFrom(source: any = {}) {
	        return new LoadUsageSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalCreditsUsed = source["totalCreditsUsed"];
	        this.thisMonth = source["thisMonth"];
	        this.runsThisMonth = source["runsThisMonth"];
	        this.avgCostPerRun = source["avgCostPerRun"];
	    }
	}
	
	
	
	
	export class RegionOption {
	    provider: string;
	    region: string;
	    label: string;
	    instanceTypes: string[];
	    defaultType: string;
	
	    static createFrom(source: any = {}) {
	        return new RegionOption(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.region = source["region"];
	        this.label = source["label"];
	        this.instanceTypes = source["instanceTypes"];
	        this.defaultType = source["defaultType"];
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

export namespace userprofile {
	
	export class Suggestion {
	    kind: string;
	    title: string;
	    body: string;
	    actionLabel?: string;
	    actionId?: string;
	    muteKey: string;
	    expiresInS: number;
	
	    static createFrom(source: any = {}) {
	        return new Suggestion(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.title = source["title"];
	        this.body = source["body"];
	        this.actionLabel = source["actionLabel"];
	        this.actionId = source["actionId"];
	        this.muteKey = source["muteKey"];
	        this.expiresInS = source["expiresInS"];
	    }
	}

}

export namespace v1 {
	
	export class ClientIPConfig {
	    timeoutSeconds?: number;
	
	    static createFrom(source: any = {}) {
	        return new ClientIPConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timeoutSeconds = source["timeoutSeconds"];
	    }
	}
	export class Time {
	
	
	    static createFrom(source: any = {}) {
	        return new Time(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}
	export class Condition {
	    type: string;
	    status: string;
	    observedGeneration?: number;
	    lastTransitionTime: Time;
	    reason: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new Condition(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.type = source["type"];
	        this.status = source["status"];
	        this.observedGeneration = source["observedGeneration"];
	        this.lastTransitionTime = this.convertValues(source["lastTransitionTime"], Time);
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
	export class ObjectReference {
	    kind?: string;
	    namespace?: string;
	    name?: string;
	    uid?: string;
	    apiVersion?: string;
	    resourceVersion?: string;
	    fieldPath?: string;
	
	    static createFrom(source: any = {}) {
	        return new ObjectReference(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.namespace = source["namespace"];
	        this.name = source["name"];
	        this.uid = source["uid"];
	        this.apiVersion = source["apiVersion"];
	        this.resourceVersion = source["resourceVersion"];
	        this.fieldPath = source["fieldPath"];
	    }
	}
	export class EndpointAddress {
	    ip: string;
	    hostname?: string;
	    nodeName?: string;
	    targetRef?: ObjectReference;
	
	    static createFrom(source: any = {}) {
	        return new EndpointAddress(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ip = source["ip"];
	        this.hostname = source["hostname"];
	        this.nodeName = source["nodeName"];
	        this.targetRef = this.convertValues(source["targetRef"], ObjectReference);
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
	export class EndpointPort {
	    name?: string;
	    port: number;
	    protocol?: string;
	    appProtocol?: string;
	
	    static createFrom(source: any = {}) {
	        return new EndpointPort(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.port = source["port"];
	        this.protocol = source["protocol"];
	        this.appProtocol = source["appProtocol"];
	    }
	}
	export class EndpointSubset {
	    addresses?: EndpointAddress[];
	    notReadyAddresses?: EndpointAddress[];
	    ports?: EndpointPort[];
	
	    static createFrom(source: any = {}) {
	        return new EndpointSubset(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.addresses = this.convertValues(source["addresses"], EndpointAddress);
	        this.notReadyAddresses = this.convertValues(source["notReadyAddresses"], EndpointAddress);
	        this.ports = this.convertValues(source["ports"], EndpointPort);
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
	export class FieldsV1 {
	
	
	    static createFrom(source: any = {}) {
	        return new FieldsV1(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	
	    }
	}
	export class ManagedFieldsEntry {
	    manager?: string;
	    operation?: string;
	    apiVersion?: string;
	    time?: Time;
	    fieldsType?: string;
	    // Go type: FieldsV1
	    fieldsV1?: any;
	    subresource?: string;
	
	    static createFrom(source: any = {}) {
	        return new ManagedFieldsEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.manager = source["manager"];
	        this.operation = source["operation"];
	        this.apiVersion = source["apiVersion"];
	        this.time = this.convertValues(source["time"], Time);
	        this.fieldsType = source["fieldsType"];
	        this.fieldsV1 = this.convertValues(source["fieldsV1"], null);
	        this.subresource = source["subresource"];
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
	export class OwnerReference {
	    apiVersion: string;
	    kind: string;
	    name: string;
	    uid: string;
	    controller?: boolean;
	    blockOwnerDeletion?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new OwnerReference(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.apiVersion = source["apiVersion"];
	        this.kind = source["kind"];
	        this.name = source["name"];
	        this.uid = source["uid"];
	        this.controller = source["controller"];
	        this.blockOwnerDeletion = source["blockOwnerDeletion"];
	    }
	}
	export class Endpoints {
	    kind?: string;
	    apiVersion?: string;
	    name?: string;
	    generateName?: string;
	    namespace?: string;
	    selfLink?: string;
	    uid?: string;
	    resourceVersion?: string;
	    generation?: number;
	    creationTimestamp?: Time;
	    deletionTimestamp?: Time;
	    deletionGracePeriodSeconds?: number;
	    labels?: Record<string, string>;
	    annotations?: Record<string, string>;
	    ownerReferences?: OwnerReference[];
	    finalizers?: string[];
	    managedFields?: ManagedFieldsEntry[];
	    subsets?: EndpointSubset[];
	
	    static createFrom(source: any = {}) {
	        return new Endpoints(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.apiVersion = source["apiVersion"];
	        this.name = source["name"];
	        this.generateName = source["generateName"];
	        this.namespace = source["namespace"];
	        this.selfLink = source["selfLink"];
	        this.uid = source["uid"];
	        this.resourceVersion = source["resourceVersion"];
	        this.generation = source["generation"];
	        this.creationTimestamp = this.convertValues(source["creationTimestamp"], Time);
	        this.deletionTimestamp = this.convertValues(source["deletionTimestamp"], Time);
	        this.deletionGracePeriodSeconds = source["deletionGracePeriodSeconds"];
	        this.labels = source["labels"];
	        this.annotations = source["annotations"];
	        this.ownerReferences = this.convertValues(source["ownerReferences"], OwnerReference);
	        this.finalizers = source["finalizers"];
	        this.managedFields = this.convertValues(source["managedFields"], ManagedFieldsEntry);
	        this.subsets = this.convertValues(source["subsets"], EndpointSubset);
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
	export class PortStatus {
	    port: number;
	    protocol: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new PortStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.port = source["port"];
	        this.protocol = source["protocol"];
	        this.error = source["error"];
	    }
	}
	export class LoadBalancerIngress {
	    ip?: string;
	    hostname?: string;
	    ipMode?: string;
	    ports?: PortStatus[];
	
	    static createFrom(source: any = {}) {
	        return new LoadBalancerIngress(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ip = source["ip"];
	        this.hostname = source["hostname"];
	        this.ipMode = source["ipMode"];
	        this.ports = this.convertValues(source["ports"], PortStatus);
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
	export class LoadBalancerStatus {
	    ingress?: LoadBalancerIngress[];
	
	    static createFrom(source: any = {}) {
	        return new LoadBalancerStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ingress = this.convertValues(source["ingress"], LoadBalancerIngress);
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
	
	
	export class ServiceStatus {
	    loadBalancer?: LoadBalancerStatus;
	    conditions?: Condition[];
	
	    static createFrom(source: any = {}) {
	        return new ServiceStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.loadBalancer = this.convertValues(source["loadBalancer"], LoadBalancerStatus);
	        this.conditions = this.convertValues(source["conditions"], Condition);
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
	export class SessionAffinityConfig {
	    clientIP?: ClientIPConfig;
	
	    static createFrom(source: any = {}) {
	        return new SessionAffinityConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.clientIP = this.convertValues(source["clientIP"], ClientIPConfig);
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
	export class ServicePort {
	    name?: string;
	    protocol?: string;
	    appProtocol?: string;
	    port: number;
	    targetPort?: intstr.IntOrString;
	    nodePort?: number;
	
	    static createFrom(source: any = {}) {
	        return new ServicePort(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.protocol = source["protocol"];
	        this.appProtocol = source["appProtocol"];
	        this.port = source["port"];
	        this.targetPort = this.convertValues(source["targetPort"], intstr.IntOrString);
	        this.nodePort = source["nodePort"];
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
	export class ServiceSpec {
	    ports?: ServicePort[];
	    selector?: Record<string, string>;
	    clusterIP?: string;
	    clusterIPs?: string[];
	    type?: string;
	    externalIPs?: string[];
	    sessionAffinity?: string;
	    loadBalancerIP?: string;
	    loadBalancerSourceRanges?: string[];
	    externalName?: string;
	    externalTrafficPolicy?: string;
	    healthCheckNodePort?: number;
	    publishNotReadyAddresses?: boolean;
	    sessionAffinityConfig?: SessionAffinityConfig;
	    ipFamilies?: string[];
	    ipFamilyPolicy?: string;
	    allocateLoadBalancerNodePorts?: boolean;
	    loadBalancerClass?: string;
	    internalTrafficPolicy?: string;
	    trafficDistribution?: string;
	
	    static createFrom(source: any = {}) {
	        return new ServiceSpec(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ports = this.convertValues(source["ports"], ServicePort);
	        this.selector = source["selector"];
	        this.clusterIP = source["clusterIP"];
	        this.clusterIPs = source["clusterIPs"];
	        this.type = source["type"];
	        this.externalIPs = source["externalIPs"];
	        this.sessionAffinity = source["sessionAffinity"];
	        this.loadBalancerIP = source["loadBalancerIP"];
	        this.loadBalancerSourceRanges = source["loadBalancerSourceRanges"];
	        this.externalName = source["externalName"];
	        this.externalTrafficPolicy = source["externalTrafficPolicy"];
	        this.healthCheckNodePort = source["healthCheckNodePort"];
	        this.publishNotReadyAddresses = source["publishNotReadyAddresses"];
	        this.sessionAffinityConfig = this.convertValues(source["sessionAffinityConfig"], SessionAffinityConfig);
	        this.ipFamilies = source["ipFamilies"];
	        this.ipFamilyPolicy = source["ipFamilyPolicy"];
	        this.allocateLoadBalancerNodePorts = source["allocateLoadBalancerNodePorts"];
	        this.loadBalancerClass = source["loadBalancerClass"];
	        this.internalTrafficPolicy = source["internalTrafficPolicy"];
	        this.trafficDistribution = source["trafficDistribution"];
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
	export class Service {
	    kind?: string;
	    apiVersion?: string;
	    name?: string;
	    generateName?: string;
	    namespace?: string;
	    selfLink?: string;
	    uid?: string;
	    resourceVersion?: string;
	    generation?: number;
	    creationTimestamp?: Time;
	    deletionTimestamp?: Time;
	    deletionGracePeriodSeconds?: number;
	    labels?: Record<string, string>;
	    annotations?: Record<string, string>;
	    ownerReferences?: OwnerReference[];
	    finalizers?: string[];
	    managedFields?: ManagedFieldsEntry[];
	    spec?: ServiceSpec;
	    status?: ServiceStatus;
	
	    static createFrom(source: any = {}) {
	        return new Service(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.apiVersion = source["apiVersion"];
	        this.name = source["name"];
	        this.generateName = source["generateName"];
	        this.namespace = source["namespace"];
	        this.selfLink = source["selfLink"];
	        this.uid = source["uid"];
	        this.resourceVersion = source["resourceVersion"];
	        this.generation = source["generation"];
	        this.creationTimestamp = this.convertValues(source["creationTimestamp"], Time);
	        this.deletionTimestamp = this.convertValues(source["deletionTimestamp"], Time);
	        this.deletionGracePeriodSeconds = source["deletionGracePeriodSeconds"];
	        this.labels = source["labels"];
	        this.annotations = source["annotations"];
	        this.ownerReferences = this.convertValues(source["ownerReferences"], OwnerReference);
	        this.finalizers = source["finalizers"];
	        this.managedFields = this.convertValues(source["managedFields"], ManagedFieldsEntry);
	        this.spec = this.convertValues(source["spec"], ServiceSpec);
	        this.status = this.convertValues(source["status"], ServiceStatus);
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

export namespace vulnscan {
	
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

export namespace workflows {
	
	export class Step {
	    id: number;
	    type: string;
	    name: string;
	    icon: string;
	    actionType: string;
	    config?: any;
	
	    static createFrom(source: any = {}) {
	        return new Step(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.type = source["type"];
	        this.name = source["name"];
	        this.icon = source["icon"];
	        this.actionType = source["actionType"];
	        this.config = source["config"];
	    }
	}
	export class Workflow {
	    id: string;
	    title: string;
	    steps: Step[];
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Workflow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.steps = this.convertValues(source["steps"], Step);
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
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
	export class WorkflowSummary {
	    id: string;
	    title: string;
	    stepCount: number;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new WorkflowSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.stepCount = source["stepCount"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
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

export namespace workspace {
	
	export class AuthURL {
	    url: string;
	    state: string;
	
	    static createFrom(source: any = {}) {
	        return new AuthURL(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	        this.state = source["state"];
	    }
	}
	export class Doc {
	    id: string;
	    title: string;
	    url: string;
	    modified: number;
	
	    static createFrom(source: any = {}) {
	        return new Doc(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.url = source["url"];
	        this.modified = source["modified"];
	    }
	}
	export class DocBody {
	    id: string;
	    title: string;
	    body: string;
	
	    static createFrom(source: any = {}) {
	        return new DocBody(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.body = source["body"];
	    }
	}
	export class Sheet {
	    id: string;
	    title: string;
	    url: string;
	    tabs: string[];
	
	    static createFrom(source: any = {}) {
	        return new Sheet(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.url = source["url"];
	        this.tabs = source["tabs"];
	    }
	}
	export class Task {
	    id: string;
	    title: string;
	    notes?: string;
	    status: string;
	    due?: string;
	    updated?: string;
	
	    static createFrom(source: any = {}) {
	        return new Task(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.notes = source["notes"];
	        this.status = source["status"];
	        this.due = source["due"];
	        this.updated = source["updated"];
	    }
	}
	export class TaskList {
	    id: string;
	    title: string;
	
	    static createFrom(source: any = {}) {
	        return new TaskList(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
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

