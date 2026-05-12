// This file has been split into domain-focused files for maintainability.
// See the following files:
//
// - resources_types.go      : Common types (ResourceColumn, ResourceSchema, etc.) and dispatcher functions
// - resources_workloads.go  : Pod, Deployment, StatefulSet, DaemonSet, ReplicaSet, Job, CronJob listers
// - resources_network.go    : Service, Endpoints, Ingress, NetworkPolicy listers
// - resources_config.go     : ConfigMap, Secret, HPA listers
// - resources_storage.go    : PVC, PV, StorageClass listers
// - resources_cluster.go    : Node, Namespace, Event listers
// - resources_detail.go     : All detail view methods (getPodDetail, getDeploymentDetail, etc.)
// - resources_helpers.go    : Formatting and status functions (fmtAge, podStatus, etc.)
package k8s
