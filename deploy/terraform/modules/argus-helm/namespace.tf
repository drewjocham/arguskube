resource "kubernetes_namespace" "argus" {
  metadata {
    name = var.namespace
    labels = {
      "pod-security.kubernetes.io/enforce" = "restricted"
    }
  }
}

# Single secret holding everything the backend pulls from env. The
# `count` guards against creating an empty secret when no secret data
# is supplied — that would still mount but with no keys, which is
# noisy in the chart.
resource "kubernetes_secret" "backend" {
  count = (var.deepseek_api_key != "" || var.argocd_token != "") ? 1 : 0

  metadata {
    name      = "argus-backend-secret"
    namespace = kubernetes_namespace.argus.metadata[0].name
  }

  data = {
    "deepseek-api-key" = var.deepseek_api_key
    "argocd-token"     = var.argocd_token
  }

  type = "Opaque"
}
