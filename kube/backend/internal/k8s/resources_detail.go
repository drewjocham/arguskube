package k8s

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *Client) getPodDetail(ctx context.Context, ns, name string) (*ResourceDetailResult, error) {
	p, err := c.cs.CoreV1().Pods(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	status, _ := podStatus(p)

	props := []KeyValue{
		{Key: "Status", Value: status},
		{Key: "Node", Value: orDash(p.Spec.NodeName)},
		{Key: "Pod IP", Value: orDash(p.Status.PodIP)},
		{Key: "QoS Class", Value: string(p.Status.QOSClass)},
		{Key: "Service Account", Value: orDash(p.Spec.ServiceAccountName)},
		{Key: "Restart Policy", Value: string(p.Spec.RestartPolicy)},
	}

	if len(p.OwnerReferences) > 0 {
		props = append(props, KeyValue{Key: "Controlled By", Value: p.OwnerReferences[0].Kind + "/" + p.OwnerReferences[0].Name})
	}

	for _, c := range p.Spec.Containers {
		props = append(props, KeyValue{Key: "Container: " + c.Name, Value: c.Image})
	}

	// Container statuses — structured for the frontend.
	containers := make([]KeyValue, 0)
	for _, cs := range p.Status.ContainerStatuses {
		state := "Unknown"
		detail := ""
		if cs.State.Running != nil {
			state = "Running"
			detail = fmt.Sprintf("Started %s", fmtAge(cs.State.Running.StartedAt.Time))
		} else if cs.State.Waiting != nil {
			state = "Waiting"
			detail = cs.State.Waiting.Reason
		} else if cs.State.Terminated != nil {
			state = "Terminated"
			detail = cs.State.Terminated.Reason
		}
		containers = append(containers, KeyValue{
			Key:   cs.Name,
			Value: fmt.Sprintf("%s|%s|%t|%d|%s", state, cs.Image, cs.Ready, cs.RestartCount, detail),
		})
	}
	for _, cs := range p.Status.InitContainerStatuses {
		state := "Unknown"
		detail := ""
		if cs.State.Running != nil {
			state = "Running"
		} else if cs.State.Waiting != nil {
			state = "Waiting"
			detail = cs.State.Waiting.Reason
		} else if cs.State.Terminated != nil {
			state = "Terminated"
			detail = cs.State.Terminated.Reason
			if cs.State.Terminated.ExitCode == 0 {
				state = "Completed"
			}
		}
		containers = append(containers, KeyValue{
			Key:   "(init) " + cs.Name,
			Value: fmt.Sprintf("%s|%s|%t|%d|%s", state, cs.Image, cs.Ready, cs.RestartCount, detail),
		})
	}

	// Add volumes summary.
	for _, v := range p.Spec.Volumes {
		src := "Unknown"
		if v.ConfigMap != nil {
			src = "ConfigMap/" + v.ConfigMap.Name
		} else if v.Secret != nil {
			src = "Secret/" + v.Secret.SecretName
		} else if v.PersistentVolumeClaim != nil {
			src = "PVC/" + v.PersistentVolumeClaim.ClaimName
		} else if v.EmptyDir != nil {
			src = "EmptyDir"
		} else if v.HostPath != nil {
			src = "HostPath: " + v.HostPath.Path
		} else if v.Projected != nil {
			src = "Projected"
		} else if v.DownwardAPI != nil {
			src = "DownwardAPI"
		}
		props = append(props, KeyValue{Key: "Volume: " + v.Name, Value: src})
	}

	conditions := make([]ResourceCondition, 0)
	for _, cond := range p.Status.Conditions {
		conditions = append(conditions, ResourceCondition{
			Type:    string(cond.Type),
			Status:  string(cond.Status),
			Reason:  orDash(cond.Reason),
			Message: orDash(cond.Message),
			Age:     fmtAge(cond.LastTransitionTime.Time),
		})
	}

	events := c.getResourceEvents(ctx, ns, "Pod", name)

	result := &ResourceDetailResult{
		Kind:        "Pod",
		Name:        p.Name,
		Namespace:   p.Namespace,
		Created:     fmtTimestamp(p.CreationTimestamp.Time),
		Labels:      p.Labels,
		Annotations: p.Annotations,
		Properties:  props,
		Conditions:  conditions,
		Events:      events,
	}

	// Stash container statuses in Extra field.
	if len(containers) > 0 {
		result.Extra = map[string]interface{}{
			"containers": containers,
		}
	}

	return result, nil
}

func (c *Client) getDeploymentDetail(ctx context.Context, ns, name string) (*ResourceDetailResult, error) {
	d, err := c.cs.AppsV1().Deployments(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	props := []KeyValue{
		{Key: "Status", Value: deploymentStatus(d)},
		{Key: "Replicas", Value: fmt.Sprintf("%d desired / %d ready / %d available", ptrInt32(d.Spec.Replicas), d.Status.ReadyReplicas, d.Status.AvailableReplicas)},
		{Key: "Strategy", Value: string(d.Spec.Strategy.Type)},
		{Key: "Selector", Value: fmtMapSlice(d.Spec.Selector.MatchLabels)},
	}

	for _, c := range d.Spec.Template.Spec.Containers {
		props = append(props, KeyValue{Key: "Container: " + c.Name, Value: c.Image})
	}

	conditions := make([]ResourceCondition, 0)
	for _, cond := range d.Status.Conditions {
		conditions = append(conditions, ResourceCondition{
			Type:    string(cond.Type),
			Status:  string(cond.Status),
			Reason:  orDash(cond.Reason),
			Message: orDash(cond.Message),
			Age:     fmtAge(cond.LastTransitionTime.Time),
		})
	}

	events := c.getResourceEvents(ctx, ns, "Deployment", name)

	return &ResourceDetailResult{
		Kind:        "Deployment",
		Name:        d.Name,
		Namespace:   d.Namespace,
		Created:     fmtTimestamp(d.CreationTimestamp.Time),
		Labels:      d.Labels,
		Annotations: d.Annotations,
		Properties:  props,
		Conditions:  conditions,
		Events:      events,
	}, nil
}

func (c *Client) getServiceDetail(ctx context.Context, ns, name string) (*ResourceDetailResult, error) {
	s, err := c.cs.CoreV1().Services(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	props := []KeyValue{
		{Key: "Type", Value: string(s.Spec.Type)},
		{Key: "Cluster IP", Value: orDash(s.Spec.ClusterIP)},
		{Key: "Ports", Value: fmtServicePorts(s.Spec.Ports)},
		{Key: "Selector", Value: fmtMapSlice(s.Spec.Selector)},
		{Key: "Session Affinity", Value: string(s.Spec.SessionAffinity)},
	}

	if s.Spec.ExternalName != "" {
		props = append(props, KeyValue{Key: "External Name", Value: s.Spec.ExternalName})
	}

	events := c.getResourceEvents(ctx, ns, "Service", name)

	return &ResourceDetailResult{
		Kind:        "Service",
		Name:        s.Name,
		Namespace:   s.Namespace,
		Created:     fmtTimestamp(s.CreationTimestamp.Time),
		Labels:      s.Labels,
		Annotations: s.Annotations,
		Properties:  props,
		Events:      events,
	}, nil
}

func (c *Client) getConfigMapDetail(ctx context.Context, ns, name string) (*ResourceDetailResult, error) {
	cm, err := c.cs.CoreV1().ConfigMaps(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	props := []KeyValue{
		{Key: "Data Keys", Value: fmt.Sprintf("%d", len(cm.Data))},
		{Key: "Binary Data Keys", Value: fmt.Sprintf("%d", len(cm.BinaryData))},
	}

	return &ResourceDetailResult{
		Kind:        "ConfigMap",
		Name:        cm.Name,
		Namespace:   cm.Namespace,
		Created:     fmtTimestamp(cm.CreationTimestamp.Time),
		Labels:      cm.Labels,
		Annotations: cm.Annotations,
		Properties:  props,
		Data:        cm.Data,
	}, nil
}

func (c *Client) getSecretDetail(ctx context.Context, ns, name string) (*ResourceDetailResult, error) {
	s, err := c.cs.CoreV1().Secrets(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	props := []KeyValue{
		{Key: "Type", Value: string(s.Type)},
		{Key: "Data Keys", Value: fmt.Sprintf("%d", len(s.Data))},
	}

	// Return values base64-encoded — the same encoding the K8s API and
	// `kubectl get secret -o yaml` use. The frontend keeps the value
	// obfuscated until the user clicks Reveal, then decodes locally
	// (SecretsView.vue:decodeBase64). The previous implementation pre-
	// masked values to "(N bytes)" server-side, which left the
	// frontend's Reveal button with nothing to decode and made every
	// secret look like its byte count after a click. Wire-level access
	// already requires an authenticated session (authorizeAPIRequest),
	// and any caller with that session could read the secret via the
	// raw K8s API anyway, so sending the encoded value doesn't widen
	// the trust boundary.
	encoded := make(map[string]string, len(s.Data))
	for k, v := range s.Data {
		encoded[k] = base64.StdEncoding.EncodeToString(v)
	}

	return &ResourceDetailResult{
		Kind:        "Secret",
		Name:        s.Name,
		Namespace:   s.Namespace,
		Created:     fmtTimestamp(s.CreationTimestamp.Time),
		Labels:      s.Labels,
		Annotations: s.Annotations,
		Properties:  props,
		Data:        encoded,
	}, nil
}

func (c *Client) getNodeDetail(ctx context.Context, name string) (*ResourceDetailResult, error) {
	n, err := c.cs.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	status, _ := nodeStatus(n)
	props := []KeyValue{
		{Key: "Status", Value: status},
		{Key: "Roles", Value: orDash(nodeRoles(n))},
		{Key: "Version", Value: n.Status.NodeInfo.KubeletVersion},
		{Key: "OS", Value: n.Status.NodeInfo.OSImage},
		{Key: "Kernel", Value: n.Status.NodeInfo.KernelVersion},
		{Key: "Container Runtime", Value: n.Status.NodeInfo.ContainerRuntimeVersion},
		{Key: "CPU Capacity", Value: n.Status.Capacity.Cpu().String()},
		{Key: "Memory Capacity", Value: formatBytes(n.Status.Capacity.Memory().Value())},
		{Key: "Pods Capacity", Value: n.Status.Capacity.Pods().String()},
	}

	for _, addr := range n.Status.Addresses {
		props = append(props, KeyValue{Key: string(addr.Type), Value: addr.Address})
	}

	conditions := make([]ResourceCondition, 0)
	for _, cond := range n.Status.Conditions {
		conditions = append(conditions, ResourceCondition{
			Type:    string(cond.Type),
			Status:  string(cond.Status),
			Reason:  orDash(cond.Reason),
			Message: orDash(cond.Message),
			Age:     fmtAge(cond.LastTransitionTime.Time),
		})
	}

	events := c.getResourceEvents(ctx, "", "Node", name)

	return &ResourceDetailResult{
		Kind:        "Node",
		Name:        n.Name,
		Namespace:   "",
		Created:     fmtTimestamp(n.CreationTimestamp.Time),
		Labels:      n.Labels,
		Annotations: n.Annotations,
		Properties:  props,
		Conditions:  conditions,
		Events:      events,
	}, nil
}

func (c *Client) getNamespaceDetail(ctx context.Context, name string) (*ResourceDetailResult, error) {
	ns, err := c.cs.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	props := []KeyValue{
		{Key: "Status", Value: string(ns.Status.Phase)},
	}

	return &ResourceDetailResult{
		Kind:        "Namespace",
		Name:        ns.Name,
		Namespace:   "",
		Created:     fmtTimestamp(ns.CreationTimestamp.Time),
		Labels:      ns.Labels,
		Annotations: ns.Annotations,
		Properties:  props,
	}, nil
}

func (c *Client) getPVCDetail(ctx context.Context, ns, name string) (*ResourceDetailResult, error) {
	pvc, err := c.cs.CoreV1().PersistentVolumeClaims(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	capacity := "—"
	if pvc.Status.Capacity != nil {
		if q, ok := pvc.Status.Capacity[corev1.ResourceStorage]; ok {
			capacity = q.String()
		}
	}
	sc := "—"
	if pvc.Spec.StorageClassName != nil {
		sc = *pvc.Spec.StorageClassName
	}

	props := []KeyValue{
		{Key: "Status", Value: string(pvc.Status.Phase)},
		{Key: "Volume", Value: orDash(pvc.Spec.VolumeName)},
		{Key: "Capacity", Value: capacity},
		{Key: "Access Modes", Value: fmtAccessModes(pvc.Spec.AccessModes)},
		{Key: "Storage Class", Value: sc},
	}

	events := c.getResourceEvents(ctx, ns, "PersistentVolumeClaim", name)

	return &ResourceDetailResult{
		Kind:        "PersistentVolumeClaim",
		Name:        pvc.Name,
		Namespace:   pvc.Namespace,
		Created:     fmtTimestamp(pvc.CreationTimestamp.Time),
		Labels:      pvc.Labels,
		Annotations: pvc.Annotations,
		Properties:  props,
		Events:      events,
	}, nil
}

func (c *Client) getIngressDetail(ctx context.Context, ns, name string) (*ResourceDetailResult, error) {
	ing, err := c.cs.NetworkingV1().Ingresses(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	className := "—"
	if ing.Spec.IngressClassName != nil {
		className = *ing.Spec.IngressClassName
	}

	props := []KeyValue{
		{Key: "Ingress Class", Value: className},
	}

	for _, rule := range ing.Spec.Rules {
		if rule.HTTP != nil {
			for _, path := range rule.HTTP.Paths {
				props = append(props, KeyValue{
					Key:   fmt.Sprintf("Rule: %s%s", orDash(rule.Host), path.Path),
					Value: fmt.Sprintf("%s:%d", path.Backend.Service.Name, path.Backend.Service.Port.Number),
				})
			}
		}
	}

	for _, tls := range ing.Spec.TLS {
		props = append(props, KeyValue{
			Key:   "TLS",
			Value: fmt.Sprintf("hosts=%s secret=%s", strings.Join(tls.Hosts, ","), tls.SecretName),
		})
	}

	events := c.getResourceEvents(ctx, ns, "Ingress", name)

	return &ResourceDetailResult{
		Kind:        "Ingress",
		Name:        ing.Name,
		Namespace:   ing.Namespace,
		Created:     fmtTimestamp(ing.CreationTimestamp.Time),
		Labels:      ing.Labels,
		Annotations: ing.Annotations,
		Properties:  props,
		Events:      events,
	}, nil
}

func (c *Client) getStatefulSetDetail(ctx context.Context, ns, name string) (*ResourceDetailResult, error) {
	ss, err := c.cs.AppsV1().StatefulSets(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// Determine status.
	desired := ptrInt32(ss.Spec.Replicas)
	status := "Available"
	if ss.Status.ReadyReplicas < desired {
		status = "Progressing"
	}
	if ss.Status.ReadyReplicas == 0 && desired > 0 {
		status = "Unavailable"
	}

	updateStrategy := string(ss.Spec.UpdateStrategy.Type)
	if ss.Spec.UpdateStrategy.RollingUpdate != nil && ss.Spec.UpdateStrategy.RollingUpdate.Partition != nil {
		updateStrategy += fmt.Sprintf(" (partition=%d)", *ss.Spec.UpdateStrategy.RollingUpdate.Partition)
	}

	props := []KeyValue{
		{Key: "Status", Value: status},
		{Key: "Replicas", Value: fmt.Sprintf("%d desired / %d ready / %d current / %d updated",
			desired, ss.Status.ReadyReplicas, ss.Status.CurrentReplicas, ss.Status.UpdatedReplicas)},
		{Key: "Service Name", Value: orDash(ss.Spec.ServiceName)},
		{Key: "Pod Management Policy", Value: string(ss.Spec.PodManagementPolicy)},
		{Key: "Update Strategy", Value: updateStrategy},
		{Key: "Selector", Value: fmtMapSlice(ss.Spec.Selector.MatchLabels)},
		{Key: "Revision", Value: ss.Status.CurrentRevision},
	}

	if ss.Status.CurrentRevision != ss.Status.UpdateRevision {
		props = append(props, KeyValue{Key: "Update Revision", Value: ss.Status.UpdateRevision})
	}

	// Volume claim templates.
	for _, pvc := range ss.Spec.VolumeClaimTemplates {
		capacity := "—"
		if req, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]; ok {
			capacity = req.String()
		}
		sc := "default"
		if pvc.Spec.StorageClassName != nil {
			sc = *pvc.Spec.StorageClassName
		}
		props = append(props, KeyValue{
			Key:   "VolumeClaimTemplate: " + pvc.Name,
			Value: fmt.Sprintf("%s, StorageClass=%s, %s", capacity, sc, fmtAccessModes(pvc.Spec.AccessModes)),
		})
	}

	// Container images.
	for _, ctr := range ss.Spec.Template.Spec.Containers {
		props = append(props, KeyValue{Key: "Container: " + ctr.Name, Value: ctr.Image})
	}

	conditions := make([]ResourceCondition, 0)
	for _, cond := range ss.Status.Conditions {
		conditions = append(conditions, ResourceCondition{
			Type:    string(cond.Type),
			Status:  string(cond.Status),
			Reason:  orDash(cond.Reason),
			Message: orDash(cond.Message),
			Age:     fmtAge(cond.LastTransitionTime.Time),
		})
	}

	events := c.getResourceEvents(ctx, ns, "StatefulSet", name)

	return &ResourceDetailResult{
		Kind:        "StatefulSet",
		Name:        ss.Name,
		Namespace:   ss.Namespace,
		Created:     fmtTimestamp(ss.CreationTimestamp.Time),
		Labels:      ss.Labels,
		Annotations: ss.Annotations,
		Properties:  props,
		Conditions:  conditions,
		Events:      events,
	}, nil
}

func (c *Client) getDaemonSetDetail(ctx context.Context, ns, name string) (*ResourceDetailResult, error) {
	ds, err := c.cs.AppsV1().DaemonSets(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	status := "Available"
	if ds.Status.NumberReady < ds.Status.DesiredNumberScheduled {
		status = "Progressing"
	}
	if ds.Status.NumberReady == 0 && ds.Status.DesiredNumberScheduled > 0 {
		status = "Unavailable"
	}

	updateStrategy := string(ds.Spec.UpdateStrategy.Type)
	if ds.Spec.UpdateStrategy.RollingUpdate != nil && ds.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable != nil {
		updateStrategy += fmt.Sprintf(" (maxUnavailable=%s)", ds.Spec.UpdateStrategy.RollingUpdate.MaxUnavailable.String())
	}

	props := []KeyValue{
		{Key: "Status", Value: status},
		{Key: "Desired", Value: fmt.Sprintf("%d scheduled / %d ready / %d available / %d updated",
			ds.Status.DesiredNumberScheduled, ds.Status.NumberReady, ds.Status.NumberAvailable, ds.Status.UpdatedNumberScheduled)},
		{Key: "Update Strategy", Value: updateStrategy},
		{Key: "Selector", Value: fmtMapSlice(ds.Spec.Selector.MatchLabels)},
		{Key: "Node Selector", Value: fmtMapSlice(ds.Spec.Template.Spec.NodeSelector)},
	}

	for _, ctr := range ds.Spec.Template.Spec.Containers {
		props = append(props, KeyValue{Key: "Container: " + ctr.Name, Value: ctr.Image})
	}

	conditions := make([]ResourceCondition, 0)
	for _, cond := range ds.Status.Conditions {
		conditions = append(conditions, ResourceCondition{
			Type:    string(cond.Type),
			Status:  string(cond.Status),
			Reason:  orDash(cond.Reason),
			Message: orDash(cond.Message),
			Age:     fmtAge(cond.LastTransitionTime.Time),
		})
	}

	events := c.getResourceEvents(ctx, ns, "DaemonSet", name)

	return &ResourceDetailResult{
		Kind:        "DaemonSet",
		Name:        ds.Name,
		Namespace:   ds.Namespace,
		Created:     fmtTimestamp(ds.CreationTimestamp.Time),
		Labels:      ds.Labels,
		Annotations: ds.Annotations,
		Properties:  props,
		Conditions:  conditions,
		Events:      events,
	}, nil
}

func (c *Client) getReplicaSetDetail(ctx context.Context, ns, name string) (*ResourceDetailResult, error) {
	rs, err := c.cs.AppsV1().ReplicaSets(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	desired := ptrInt32(rs.Spec.Replicas)
	status := "Available"
	if rs.Status.ReadyReplicas < desired {
		status = "Progressing"
	}
	if rs.Status.ReadyReplicas == 0 && desired > 0 {
		status = "Unavailable"
	}

	props := []KeyValue{
		{Key: "Status", Value: status},
		{Key: "Replicas", Value: fmt.Sprintf("%d desired / %d ready / %d total",
			desired, rs.Status.ReadyReplicas, rs.Status.Replicas)},
		{Key: "Fully Labeled Replicas", Value: fmt.Sprintf("%d", rs.Status.FullyLabeledReplicas)},
		{Key: "Selector", Value: fmtMapSlice(rs.Spec.Selector.MatchLabels)},
	}

	if len(rs.OwnerReferences) > 0 {
		props = append(props, KeyValue{Key: "Controlled By", Value: rs.OwnerReferences[0].Kind + "/" + rs.OwnerReferences[0].Name})
	}

	for _, ctr := range rs.Spec.Template.Spec.Containers {
		props = append(props, KeyValue{Key: "Container: " + ctr.Name, Value: ctr.Image})
	}

	conditions := make([]ResourceCondition, 0)
	for _, cond := range rs.Status.Conditions {
		conditions = append(conditions, ResourceCondition{
			Type:    string(cond.Type),
			Status:  string(cond.Status),
			Reason:  orDash(cond.Reason),
			Message: orDash(cond.Message),
			Age:     fmtAge(cond.LastTransitionTime.Time),
		})
	}

	events := c.getResourceEvents(ctx, ns, "ReplicaSet", name)

	return &ResourceDetailResult{
		Kind:        "ReplicaSet",
		Name:        rs.Name,
		Namespace:   rs.Namespace,
		Created:     fmtTimestamp(rs.CreationTimestamp.Time),
		Labels:      rs.Labels,
		Annotations: rs.Annotations,
		Properties:  props,
		Conditions:  conditions,
		Events:      events,
	}, nil
}

func (c *Client) getCronJobDetail(ctx context.Context, ns, name string) (*ResourceDetailResult, error) {
	cj, err := c.cs.BatchV1().CronJobs(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	suspend := "False"
	if cj.Spec.Suspend != nil && *cj.Spec.Suspend {
		suspend = "True"
	}

	concurrencyPolicy := string(cj.Spec.ConcurrencyPolicy)
	if concurrencyPolicy == "" {
		concurrencyPolicy = "Allow"
	}

	lastSchedule := "—"
	if cj.Status.LastScheduleTime != nil {
		lastSchedule = fmtAge(cj.Status.LastScheduleTime.Time) + " ago"
	}

	lastSuccess := "—"
	if cj.Status.LastSuccessfulTime != nil {
		lastSuccess = fmtAge(cj.Status.LastSuccessfulTime.Time) + " ago"
	}

	props := []KeyValue{
		{Key: "Schedule", Value: cj.Spec.Schedule},
		{Key: "Suspend", Value: suspend},
		{Key: "Concurrency Policy", Value: concurrencyPolicy},
		{Key: "Active Jobs", Value: fmt.Sprintf("%d", len(cj.Status.Active))},
		{Key: "Last Scheduled", Value: lastSchedule},
		{Key: "Last Successful", Value: lastSuccess},
	}

	if cj.Spec.StartingDeadlineSeconds != nil {
		props = append(props, KeyValue{Key: "Starting Deadline", Value: fmt.Sprintf("%ds", *cj.Spec.StartingDeadlineSeconds)})
	}

	if cj.Spec.SuccessfulJobsHistoryLimit != nil {
		props = append(props, KeyValue{Key: "Success History Limit", Value: fmt.Sprintf("%d", *cj.Spec.SuccessfulJobsHistoryLimit)})
	}
	if cj.Spec.FailedJobsHistoryLimit != nil {
		props = append(props, KeyValue{Key: "Failed History Limit", Value: fmt.Sprintf("%d", *cj.Spec.FailedJobsHistoryLimit)})
	}

	// Container images from job template.
	for _, ctr := range cj.Spec.JobTemplate.Spec.Template.Spec.Containers {
		props = append(props, KeyValue{Key: "Container: " + ctr.Name, Value: ctr.Image})
	}

	// Restart policy from job template.
	if rp := cj.Spec.JobTemplate.Spec.Template.Spec.RestartPolicy; rp != "" {
		props = append(props, KeyValue{Key: "Restart Policy", Value: string(rp)})
	}

	// Active job references — stash in Extra for the frontend.
	activeJobs := make([]map[string]string, 0, len(cj.Status.Active))
	for _, ref := range cj.Status.Active {
		activeJobs = append(activeJobs, map[string]string{
			"name":      ref.Name,
			"namespace": ref.Namespace,
		})
	}

	events := c.getResourceEvents(ctx, ns, "CronJob", name)

	result := &ResourceDetailResult{
		Kind:        "CronJob",
		Name:        cj.Name,
		Namespace:   cj.Namespace,
		Created:     fmtTimestamp(cj.CreationTimestamp.Time),
		Labels:      cj.Labels,
		Annotations: cj.Annotations,
		Properties:  props,
		Events:      events,
	}

	if len(activeJobs) > 0 {
		result.Extra = map[string]interface{}{
			"activeJobs": activeJobs,
		}
	}

	return result, nil
}

func (c *Client) getJobDetail(ctx context.Context, ns, name string) (*ResourceDetailResult, error) {
	j, err := c.cs.BatchV1().Jobs(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// Determine status.
	status := "Active"
	for _, cond := range j.Status.Conditions {
		if cond.Type == "Complete" && cond.Status == "True" {
			status = "Complete"
			break
		}
		if cond.Type == "Failed" && cond.Status == "True" {
			status = "Failed"
			break
		}
	}

	completions := int32(1)
	if j.Spec.Completions != nil {
		completions = *j.Spec.Completions
	}
	parallelism := int32(1)
	if j.Spec.Parallelism != nil {
		parallelism = *j.Spec.Parallelism
	}

	duration := "—"
	if j.Status.StartTime != nil {
		if j.Status.CompletionTime != nil {
			d := j.Status.CompletionTime.Time.Sub(j.Status.StartTime.Time)
			duration = d.Round(1e9).String()
		} else {
			duration = "Running"
		}
	}

	props := []KeyValue{
		{Key: "Status", Value: status},
		{Key: "Completions", Value: fmt.Sprintf("%d/%d", j.Status.Succeeded, completions)},
		{Key: "Parallelism", Value: fmt.Sprintf("%d", parallelism)},
		{Key: "Duration", Value: duration},
		{Key: "Active Pods", Value: fmt.Sprintf("%d", j.Status.Active)},
		{Key: "Succeeded Pods", Value: fmt.Sprintf("%d", j.Status.Succeeded)},
		{Key: "Failed Pods", Value: fmt.Sprintf("%d", j.Status.Failed)},
	}

	if j.Spec.BackoffLimit != nil {
		props = append(props, KeyValue{Key: "Backoff Limit", Value: fmt.Sprintf("%d", *j.Spec.BackoffLimit)})
	}
	if j.Spec.ActiveDeadlineSeconds != nil {
		props = append(props, KeyValue{Key: "Active Deadline", Value: fmt.Sprintf("%ds", *j.Spec.ActiveDeadlineSeconds)})
	}
	if j.Spec.TTLSecondsAfterFinished != nil {
		props = append(props, KeyValue{Key: "TTL After Finished", Value: fmt.Sprintf("%ds", *j.Spec.TTLSecondsAfterFinished)})
	}

	if len(j.OwnerReferences) > 0 {
		props = append(props, KeyValue{Key: "Controlled By", Value: j.OwnerReferences[0].Kind + "/" + j.OwnerReferences[0].Name})
	}

	// Container images.
	for _, ctr := range j.Spec.Template.Spec.Containers {
		props = append(props, KeyValue{Key: "Container: " + ctr.Name, Value: ctr.Image})
	}

	if rp := j.Spec.Template.Spec.RestartPolicy; rp != "" {
		props = append(props, KeyValue{Key: "Restart Policy", Value: string(rp)})
	}

	conditions := make([]ResourceCondition, 0)
	for _, cond := range j.Status.Conditions {
		conditions = append(conditions, ResourceCondition{
			Type:    string(cond.Type),
			Status:  string(cond.Status),
			Reason:  orDash(cond.Reason),
			Message: orDash(cond.Message),
			Age:     fmtAge(cond.LastTransitionTime.Time),
		})
	}

	events := c.getResourceEvents(ctx, ns, "Job", name)

	return &ResourceDetailResult{
		Kind:        "Job",
		Name:        j.Name,
		Namespace:   j.Namespace,
		Created:     fmtTimestamp(j.CreationTimestamp.Time),
		Labels:      j.Labels,
		Annotations: j.Annotations,
		Properties:  props,
		Conditions:  conditions,
		Events:      events,
	}, nil
}

// getPVDetail builds the detail view for a PersistentVolume. PVs are
// cluster-scoped, so namespace is unused and the lookup goes through
// the cluster-wide PersistentVolumes() API.
func (c *Client) getPVDetail(ctx context.Context, name string) (*ResourceDetailResult, error) {
	pv, err := c.cs.CoreV1().PersistentVolumes().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	capacity := "—"
	if q, ok := pv.Spec.Capacity[corev1.ResourceStorage]; ok {
		capacity = q.String()
	}

	claimRef := "—"
	if pv.Spec.ClaimRef != nil {
		claimRef = fmt.Sprintf("%s/%s", pv.Spec.ClaimRef.Namespace, pv.Spec.ClaimRef.Name)
	}

	source := pvSourceSummary(pv)

	props := []KeyValue{
		{Key: "Status", Value: string(pv.Status.Phase)},
		{Key: "Capacity", Value: capacity},
		{Key: "Access Modes", Value: fmtAccessModes(pv.Spec.AccessModes)},
		{Key: "Reclaim Policy", Value: string(pv.Spec.PersistentVolumeReclaimPolicy)},
		{Key: "Storage Class", Value: orDash(pv.Spec.StorageClassName)},
		{Key: "Volume Mode", Value: pvVolumeMode(pv)},
		{Key: "Claim", Value: claimRef},
		{Key: "Source", Value: source},
	}

	if pv.Status.Reason != "" {
		props = append(props, KeyValue{Key: "Reason", Value: pv.Status.Reason})
	}
	if pv.Status.Message != "" {
		props = append(props, KeyValue{Key: "Message", Value: pv.Status.Message})
	}

	events := c.getResourceEvents(ctx, "", "PersistentVolume", name)

	return &ResourceDetailResult{
		Kind:        "PersistentVolume",
		Name:        pv.Name,
		Namespace:   "", // cluster-scoped
		Created:     fmtTimestamp(pv.CreationTimestamp.Time),
		Labels:      pv.Labels,
		Annotations: pv.Annotations,
		Properties:  props,
		Events:      events,
	}, nil
}

// pvSourceSummary returns a short string describing the underlying volume
// source (csi driver, hostPath, nfs, etc.) so the detail view shows where
// the data actually lives.
func pvSourceSummary(pv *corev1.PersistentVolume) string {
	src := pv.Spec.PersistentVolumeSource
	switch {
	case src.CSI != nil:
		s := fmt.Sprintf("CSI %s", src.CSI.Driver)
		if src.CSI.VolumeHandle != "" {
			s += " (" + src.CSI.VolumeHandle + ")"
		}
		return s
	case src.HostPath != nil:
		return "hostPath: " + src.HostPath.Path
	case src.NFS != nil:
		return fmt.Sprintf("NFS %s:%s", src.NFS.Server, src.NFS.Path)
	case src.Local != nil:
		return "local: " + src.Local.Path
	case src.AWSElasticBlockStore != nil:
		return "AWS EBS: " + src.AWSElasticBlockStore.VolumeID
	case src.GCEPersistentDisk != nil:
		return "GCE PD: " + src.GCEPersistentDisk.PDName
	case src.AzureDisk != nil:
		return "Azure Disk: " + src.AzureDisk.DiskName
	case src.AzureFile != nil:
		return "Azure File: " + src.AzureFile.ShareName
	case src.ISCSI != nil:
		return fmt.Sprintf("iSCSI %s/%s", src.ISCSI.TargetPortal, src.ISCSI.IQN)
	case src.Cinder != nil:
		return "Cinder: " + src.Cinder.VolumeID
	case src.FlexVolume != nil:
		return "FlexVolume: " + src.FlexVolume.Driver
	default:
		return "—"
	}
}

func pvVolumeMode(pv *corev1.PersistentVolume) string {
	if pv.Spec.VolumeMode == nil {
		return "Filesystem"
	}
	return string(*pv.Spec.VolumeMode)
}

// getStorageClassDetail builds the detail view for a StorageClass. Like PVs,
// StorageClasses are cluster-scoped.
func (c *Client) getStorageClassDetail(ctx context.Context, name string) (*ResourceDetailResult, error) {
	sc, err := c.cs.StorageV1().StorageClasses().Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	reclaim := "Delete"
	if sc.ReclaimPolicy != nil {
		reclaim = string(*sc.ReclaimPolicy)
	}
	bindingMode := "Immediate"
	if sc.VolumeBindingMode != nil {
		bindingMode = string(*sc.VolumeBindingMode)
	}
	allowExpand := "false"
	if sc.AllowVolumeExpansion != nil && *sc.AllowVolumeExpansion {
		allowExpand = "true"
	}

	isDefault := "false"
	if v, ok := sc.Annotations["storageclass.kubernetes.io/is-default-class"]; ok && v == "true" {
		isDefault = "true"
	}

	props := []KeyValue{
		{Key: "Provisioner", Value: sc.Provisioner},
		{Key: "Reclaim Policy", Value: reclaim},
		{Key: "Volume Binding Mode", Value: bindingMode},
		{Key: "Allow Volume Expansion", Value: allowExpand},
		{Key: "Default", Value: isDefault},
	}
	for k, v := range sc.Parameters {
		props = append(props, KeyValue{Key: "param: " + k, Value: v})
	}

	return &ResourceDetailResult{
		Kind:        "StorageClass",
		Name:        sc.Name,
		Namespace:   "",
		Created:     fmtTimestamp(sc.CreationTimestamp.Time),
		Labels:      sc.Labels,
		Annotations: sc.Annotations,
		Properties:  props,
	}, nil
}

func (c *Client) getGenericDetail(ctx context.Context, kind, ns, name string) (*ResourceDetailResult, error) {
	return &ResourceDetailResult{
		Kind:      kind,
		Name:      name,
		Namespace: ns,
		Created:   "—",
		Properties: []KeyValue{
			{Key: "Note", Value: "Detail view not yet implemented for this resource type."},
		},
	}, nil
}

// --- Event helper ---

func (c *Client) getResourceEvents(ctx context.Context, ns, kind, name string) []ResourceEvent {
	fieldSelector := fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=%s", name, kind)
	list, err := c.cs.CoreV1().Events(ns).List(ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
	})
	if err != nil {
		return nil
	}

	events := make([]ResourceEvent, 0, len(list.Items))
	for _, ev := range list.Items {
		events = append(events, ResourceEvent{
			Type:    ev.Type,
			Reason:  ev.Reason,
			Message: ev.Message,
			Count:   ev.Count,
			Age:     fmtAge(ev.LastTimestamp.Time),
		})
	}
	return events
}
