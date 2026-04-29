/**
 * @typedef {'critical' | 'warning' | 'info'} Severity
 * @typedef {'free' | 'pro'} Tier
 *
 * @typedef {Object} Tag
 * @property {string} label
 * @property {string} color
 *
 * @typedef {Object} Alert
 * @property {string} id
 * @property {string} name
 * @property {Severity} severity
 * @property {string} namespace
 * @property {string} timestamp
 * @property {string} [podName]
 * @property {string} [podPhase]
 * @property {number} restartCount
 * @property {string} [memoryLimit]
 * @property {string} [memoryRequest]
 * @property {string} [cpuLimit]
 * @property {string} [cpuRequest]
 * @property {number} [cpuThrottle]
 * @property {string} [nodeName]
 * @property {number} [diskUsage]
 * @property {string} [imageTag]
 * @property {string} [previousImage]
 * @property {string} [deployTime]
 * @property {string} description
 * @property {Tag[]} tags
 * @property {string[]} [relatedAlerts]
 *
 * @typedef {Object} RunbookStep
 * @property {number} number
 * @property {string} text
 * @property {string} [command]
 *
 * @typedef {Object} Diagnosis
 * @property {string} alertId
 * @property {string} hypothesis
 * @property {number} confidence
 * @property {RunbookStep[]} steps
 * @property {string} [decisionLogEntry]
 * @property {string} [cascadeNote]
 *
 * @typedef {Object} ClusterMetrics
 * @property {number} podHealthPct
 * @property {number} podsRunning
 * @property {number} podsTotal
 * @property {number} errorRate
 * @property {number} errorRatePrev
 * @property {number} restartCount
 * @property {string} restartTop
 * @property {string} p99Latency
 * @property {string} sloStatus
 *
 * @typedef {Object} ClusterInfo
 * @property {string} name
 * @property {number} nodeCount
 * @property {string} k8sVersion
 *
 * @typedef {Object} LogLine
 * @property {string} timestamp
 * @property {string} source
 * @property {string} level
 * @property {string} message
 *
 * @typedef {Object} Bundle
 * @property {Alert} alert
 * @property {Object[]} [decisionLog]
 * @property {Alert[]} [cascadeAlerts]
 * @property {Object[]} [anomalyResults]
 * @property {Diagnosis} [diagnosis]
 */

export {}
