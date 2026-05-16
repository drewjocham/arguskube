# ── VPC ────────────────────────────────────────────────────────────
resource "google_compute_network" "runner" {
  name                    = "argus-runner-${var.run_id}-vpc"
  auto_create_subnetworks = false
}

resource "google_compute_subnetwork" "runner" {
  name          = "argus-runner-${var.run_id}-subnet"
  network       = google_compute_network.runner.id
  region        = var.region
  ip_cidr_range = "10.0.0.0/17"

  private_ip_google_access = true

  secondary_ip_range {
    range_name    = "pods"
    ip_cidr_range = "10.4.0.0/14"
  }
  secondary_ip_range {
    range_name    = "services"
    ip_cidr_range = "10.8.0.0/20"
  }
}

# ── Cloud NAT (outbound for private nodes)
resource "google_compute_router" "runner" {
  name    = "argus-runner-${var.run_id}-router"
  network = google_compute_network.runner.id
  region  = var.region
}

resource "google_compute_router_nat" "runner" {
  name                               = "argus-runner-${var.run_id}-nat"
  router                             = google_compute_router.runner.name
  region                             = var.region
  nat_ip_allocate_option             = "AUTO_ONLY"
  source_subnetwork_ip_ranges_to_nat = "ALL_SUBNETWORKS_ALL_IP_RANGES"
}

# ── GKE Cluster — spot nodes, ephemeral
resource "google_container_cluster" "runner" {
  name       = "argus-runner-${var.run_id}"
  location   = var.region
  network    = google_compute_network.runner.name
  subnetwork = google_compute_subnetwork.runner.name

  initial_node_count       = 1
  remove_default_node_pool = true
  deletion_protection      = false

  ip_allocation_policy {
    cluster_secondary_range_name  = "pods"
    services_secondary_range_name = "services"
  }

  workload_identity_config {
    workload_pool = "${var.project_id}.svc.id.goog"
  }

  release_channel {
    channel = "RAPID"
  }

  master_auth {
    client_certificate_config {
      issue_client_certificate = false
    }
  }

  addons_config {
    http_load_balancing {
      disabled = true
    }
    horizontal_pod_autoscaling {
      disabled = true
    }
  }

  resource_labels = local.labels
}

# ── Spot Node Pool ────────────────────────────────────────────────
resource "google_container_node_pool" "spot" {
  name       = "spot"
  location   = var.region
  cluster    = google_container_cluster.runner.name

  initial_node_count = var.node_count

  autoscaling {
    min_node_count = 0
    max_node_count = var.node_count
  }

  management {
    auto_repair  = false
    auto_upgrade = false
  }

  node_config {
    machine_type = var.machine_type
    disk_size_gb = 20
    disk_type    = "pd-standard"
    image_type   = "COS_CONTAINERD"

    # Spot = preemptible, ~70% cheaper
    preemptible = true

    oauth_scopes = ["https://www.googleapis.com/auth/cloud-platform"]

    labels = local.labels
  }

  lifecycle {
    ignore_changes = [initial_node_count]
  }
}

# ── Broker Namespace ──────────────────────────────────────────────
resource "kubernetes_namespace" "broker" {
  metadata {
    name   = "argus-broker-${var.run_id}"
    labels = local.labels
  }

  depends_on = [google_container_cluster.runner]
}

# ── Broker Service ────────────────────────────────────────────────
# In-cluster service the runner's load test engine connects to.
resource "kubernetes_service" "broker" {
  metadata {
    name      = "broker-${var.run_id}"
    namespace = kubernetes_namespace.broker.metadata[0].name
    labels    = local.labels
  }
  spec {
    selector = {
      app = "broker-${var.run_id}"
    }
    port {
      port        = local.broker_port
      target_port = local.broker_port
      protocol    = "TCP"
    }
  }
}

# ── Helm: Install the selected broker ─────────────────────────────
# Uses community charts. The runner publishes to the in-cluster
# service address so no external ingress is needed.
resource "helm_release" "broker" {
  count      = var.broker_kind != "pubsub" && var.broker_kind != "rest" ? 1 : 0
  name       = "broker-${var.run_id}"
  namespace  = kubernetes_namespace.broker.metadata[0].name
  repository = local.helm_repo
  chart      = local.helm_chart
  version    = local.helm_version

  set {
    name  = "fullnameOverride"
    value = "broker-${var.run_id}"
  }
  set {
    name  = "service.nameOverride"
    value = "broker-${var.run_id}"
  }

  dynamic "set" {
    for_each = local.helm_values
    content {
      name  = set.key
      value = set.value
    }
  }

  timeout = 180

  depends_on = [google_container_cluster.runner, kubernetes_namespace.broker]
}

# ── PubSub: no in-cluster broker; uses GCP service directly ───────
# For REST mode, deploy an echo server so the runner has a target.
resource "kubernetes_deployment" "rest_echo" {
  count      = var.broker_kind == "rest" ? 1 : 0
  metadata {
    name      = "broker-${var.run_id}"
    namespace = kubernetes_namespace.broker.metadata[0].name
    labels    = merge(local.labels, { app = "broker-${var.run_id}" })
  }
  spec {
    replicas = 1
    selector {
      match_labels = { app = "broker-${var.run_id}" }
    }
    template {
      metadata {
        labels = { app = "broker-${var.run_id}" }
      }
      spec {
        container {
          image = "nginx:alpine"
          name  = "echo"
          port {
            container_port = 80
          }
          # Simple nginx config that echoes request bodies
          command = ["/bin/sh", "-c"]
          args = [
            "echo 'server { listen 80; location / { return 200 \"ok\n\"; add_header Content-Type text/plain; } }' > /etc/nginx/conf.d/default.conf && nginx -g 'daemon off;'"
          ]
        }
        node_selector = {
          "cloud.google.com/gke-preemptible" = "true"
        }
        toleration {
          key      = "cloud.google.com/gke-preemptible"
          operator = "Exists"
          effect   = "NoSchedule"
        }
      }
    }
  }
}
