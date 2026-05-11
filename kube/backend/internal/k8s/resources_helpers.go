package k8s

import (
	"fmt"
	"sort"
	"strings"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
)

// --- Formatting helpers ---

func fmtAge(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		if m > 0 {
			return fmt.Sprintf("%dh%dm", h, m)
		}
		return fmt.Sprintf("%dh", h)
	default:
		days := int(d.Hours()) / 24
		hours := int(d.Hours()) % 24
		if hours > 0 {
			return fmt.Sprintf("%dd%dh", days, hours)
		}
		return fmt.Sprintf("%dd", days)
	}
}

func fmtDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
}

func fmtTimestamp(t time.Time) string {
	if t.IsZero() {
		return "—"
	}
	return fmt.Sprintf("%s (%s)", fmtAge(t), t.Format("Jan 02, 2006 3:04:05 PM MST"))
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func fmtMapSlice(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	parts := make([]string, 0, len(m))
	for k, v := range m {
		parts = append(parts, k+"="+v)
	}
	sort.Strings(parts)
	return strings.Join(parts, ", ")
}

func fmtServicePorts(ports []corev1.ServicePort) string {
	if len(ports) == 0 {
		return ""
	}
	parts := make([]string, 0, len(ports))
	for _, p := range ports {
		s := fmt.Sprintf("%d/%s", p.Port, p.Protocol)
		if p.Name != "" {
			s = p.Name + ":" + s
		}
		parts = append(parts, s)
	}
	return strings.Join(parts, ", ")
}

func fmtAccessModes(modes []corev1.PersistentVolumeAccessMode) string {
	parts := make([]string, 0, len(modes))
	for _, m := range modes {
		switch m {
		case corev1.ReadWriteOnce:
			parts = append(parts, "RWO")
		case corev1.ReadOnlyMany:
			parts = append(parts, "ROX")
		case corev1.ReadWriteMany:
			parts = append(parts, "RWX")
		default:
			parts = append(parts, string(m))
		}
	}
	return strings.Join(parts, ", ")
}

func formatBytes(b int64) string {
	const (
		gi = 1 << 30
		mi = 1 << 20
	)
	switch {
	case b >= gi:
		return fmt.Sprintf("%.1fGi", float64(b)/float64(gi))
	case b >= mi:
		return fmt.Sprintf("%.0fMi", float64(b)/float64(mi))
	default:
		return fmt.Sprintf("%dB", b)
	}
}

func extractImages(containers []corev1.Container) string {
	parts := make([]string, 0, len(containers))
	for _, c := range containers {
		img := c.Image
		// Show just the image name + tag, not the full registry path.
		if idx := strings.LastIndex(img, "/"); idx >= 0 {
			img = img[idx+1:]
		}
		parts = append(parts, img)
	}
	return strings.Join(parts, ", ")
}

func ptrInt32(p *int32) int32 {
	if p == nil {
		return 0
	}
	return *p
}

// --- Status derivation ---

func podStatus(p *corev1.Pod) (string, string) {
	// Check container states first.
	for _, cs := range p.Status.ContainerStatuses {
		if cs.State.Waiting != nil {
			reason := cs.State.Waiting.Reason
			switch reason {
			case "CrashLoopBackOff":
				return "CrashLoopBackOff", "red"
			case "ImagePullBackOff", "ErrImagePull":
				return reason, "red"
			case "ContainerCreating":
				return reason, "amber"
			default:
				return reason, "amber"
			}
		}
		if cs.State.Terminated != nil {
			reason := cs.State.Terminated.Reason
			if reason == "OOMKilled" {
				return "OOMKilled", "red"
			}
			if reason == "Completed" {
				return "Completed", "blue"
			}
			return reason, "red"
		}
	}

	switch p.Status.Phase {
	case corev1.PodRunning:
		return "Running", "green"
	case corev1.PodSucceeded:
		return "Completed", "blue"
	case corev1.PodFailed:
		return "Failed", "red"
	case corev1.PodPending:
		return "Pending", "amber"
	default:
		return string(p.Status.Phase), "gray"
	}
}

func deploymentStatus(d *appsv1.Deployment) string {
	for _, cond := range d.Status.Conditions {
		if cond.Type == appsv1.DeploymentProgressing && cond.Status == corev1.ConditionFalse {
			return "Progressing"
		}
	}
	if d.Status.ReadyReplicas == ptrInt32(d.Spec.Replicas) {
		return "Running"
	}
	return "Updating"
}

func jobStatus(j *batchv1.Job) (string, string) {
	for _, cond := range j.Status.Conditions {
		if cond.Type == batchv1.JobComplete && cond.Status == corev1.ConditionTrue {
			return "Complete", "green"
		}
		if cond.Type == batchv1.JobFailed && cond.Status == corev1.ConditionTrue {
			return "Failed", "red"
		}
	}
	if j.Status.Active > 0 {
		return "Running", "blue"
	}
	return "Pending", "amber"
}

func nodeStatus(n *corev1.Node) (string, string) {
	for _, cond := range n.Status.Conditions {
		if cond.Type == corev1.NodeReady {
			if cond.Status == corev1.ConditionTrue {
				return "Ready", "green"
			}
			return "NotReady", "red"
		}
	}
	return "Unknown", "gray"
}

func nodeRoles(n *corev1.Node) string {
	roles := make([]string, 0)
	for k := range n.Labels {
		if strings.HasPrefix(k, "node-role.kubernetes.io/") {
			role := strings.TrimPrefix(k, "node-role.kubernetes.io/")
			if role != "" {
				roles = append(roles, role)
			}
		}
	}
	return strings.Join(roles, ", ")
}
