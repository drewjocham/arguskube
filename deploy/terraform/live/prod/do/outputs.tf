output "cluster_name"    { value = module.platform.cluster_name }
output "cluster_endpoint"{ value = module.platform.cluster_endpoint }
output "namespace"       { value = module.apps.namespace }

output "kubeconfig" {
  description = "Authenticate to the cluster."
  value       = "doctl kubernetes cluster kubeconfig save ${module.platform.cluster_name}"
}
