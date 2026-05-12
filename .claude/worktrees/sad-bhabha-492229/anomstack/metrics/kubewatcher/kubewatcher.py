def ingest():
    """
    Ingest Kubernetes cluster metrics from the K8s API.

    Uses in-cluster config when running inside a pod, falls back to
    kubeconfig for local development.
    """
    from dagster import get_dagster_logger
    import pandas as pd
    from kubernetes import client, config

    logger = get_dagster_logger()
    logger.info("ingesting kubewatcher metrics from K8s API")

    try:
        config.load_incluster_config()
        logger.info("using in-cluster config")
    except config.ConfigException:
        config.load_kube_config()
        logger.info("using kubeconfig")

    v1 = client.CoreV1Api()
    ts = pd.Timestamp.utcnow().floor("S")
    metrics = {}

    pods = v1.list_pod_for_all_namespaces(watch=False)
    total_pods = len(pods.items)
    running_pods = 0
    pending_pods = 0
    failed_pods = 0
    crashloop_pods = 0
    total_restarts = 0
    image_pull_backoff = 0
    oom_pods = 0

    for pod in pods.items:
        phase = pod.status.phase
        if phase == "Running":
            running_pods += 1
        elif phase == "Pending":
            pending_pods += 1
        elif phase == "Failed":
            failed_pods += 1

        for cs in pod.status.container_statuses or []:
            total_restarts += cs.restart_count

            state = cs.state
            if state.waiting and state.waiting.reason:
                if state.waiting.reason == "CrashLoopBackOff":
                    crashloop_pods += 1
                elif state.waiting.reason == "ImagePullBackOff":
                    image_pull_backoff += 1
            if state.terminated and state.terminated.reason == "OOMKilled":
                oom_pods += 1

    metrics["pod_total"] = total_pods
    metrics["pod_running"] = running_pods
    metrics["pod_pending"] = pending_pods
    metrics["pod_failed"] = failed_pods
    metrics["pod_crashloop"] = crashloop_pods
    metrics["pod_image_pull_backoff"] = image_pull_backoff
    metrics["pod_oom_killed"] = oom_pods
    metrics["pod_restarts_total"] = total_restarts
    metrics["pod_health_pct"] = round((running_pods / max(total_pods, 1)) * 100, 2)

    nodes = v1.list_node(watch=False)
    total_nodes = len(nodes.items)
    ready_nodes = 0
    disk_pressure = 0
    memory_pressure = 0
    pid_pressure = 0

    for node in nodes.items:
        for condition in node.status.conditions or []:
            if condition.type == "Ready" and condition.status == "True":
                ready_nodes += 1
            if condition.type == "DiskPressure" and condition.status == "True":
                disk_pressure += 1
            if condition.type == "MemoryPressure" and condition.status == "True":
                memory_pressure += 1
            if condition.type == "PIDPressure" and condition.status == "True":
                pid_pressure += 1

    metrics["node_total"] = total_nodes
    metrics["node_ready"] = ready_nodes
    metrics["node_disk_pressure"] = disk_pressure
    metrics["node_memory_pressure"] = memory_pressure
    metrics["node_pid_pressure"] = pid_pressure

    field_selector = "type=Warning"
    events = v1.list_event_for_all_namespaces(
        field_selector=field_selector, limit=500
    )
    recent_warnings = 0
    for event in events.items:
        if event.last_timestamp:
            age = pd.Timestamp.utcnow() - pd.Timestamp(event.last_timestamp).tz_localize(None)
            if age.total_seconds() < 3600:
                recent_warnings += 1
        elif event.metadata.creation_timestamp:
            age = pd.Timestamp.utcnow() - pd.Timestamp(event.metadata.creation_timestamp).tz_localize(None)
            if age.total_seconds() < 3600:
                recent_warnings += 1

    metrics["event_warnings_1h"] = recent_warnings

    logger.info(
        "cluster snapshot",
        extra={
            "pods": f"{running_pods}/{total_pods}",
            "nodes": f"{ready_nodes}/{total_nodes}",
            "restarts": total_restarts,
            "warnings_1h": recent_warnings,
            "crashloops": crashloop_pods,
        },
    )

    all_rows = []
    for metric_name, value in metrics.items():
        all_rows.append({
            "metric_timestamp": ts,
            "metric_name": metric_name,
            "metric_value": value,
        })

    df = pd.DataFrame(all_rows)
    df = df[["metric_timestamp", "metric_name", "metric_value"]]
    return df
