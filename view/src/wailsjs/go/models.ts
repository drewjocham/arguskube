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

export namespace k8s {
	
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

