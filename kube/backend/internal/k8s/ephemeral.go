package k8s

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DebugSession struct {
	PodName       string `json:"podName"`
	Namespace     string `json:"namespace"`
	ContainerName string `json:"containerName"`
	DebugImage    string `json:"debugImage"`
	Started       bool   `json:"started"`
}

// DebugOptions describes a single ephemeral-debug injection. Every
// field is optional — empty falls back to the prior hardcoded
// behavior (netshoot image, no command override, first container as
// target). The frontend's "Debug" popup lets users pick a preset
// strategy and override any of these per pod.
type DebugOptions struct {
	// Image to run as the ephemeral container. Empty → netshoot.
	Image string `json:"image"`
	// Command overrides the image's ENTRYPOINT. Empty → image default.
	Command []string `json:"command"`
	// Args overrides the image's CMD. Only meaningful with Command.
	Args []string `json:"args"`
	// TargetContainer is the existing container whose namespaces the
	// debug container shares (PID namespace, filesystem visibility via
	// /proc/<pid>/root). Empty → first container in pod.Spec.Containers.
	TargetContainer string `json:"targetContainer"`
	// Env adds container env vars beyond the two we always set
	// (DEBUG_POD, DEBUG_NAMESPACE).
	Env map[string]string `json:"env"`
}

// InjectDebugContainer is the legacy single-image entry point —
// kept for callers that haven't migrated to InjectDebugContainerWithOptions.
// Delegates to the options variant with just the image set.
func (c *Client) InjectDebugContainer(ctx context.Context, namespace, podName, debugImage string) (*DebugSession, error) {
	return c.InjectDebugContainerWithOptions(ctx, namespace, podName, DebugOptions{Image: debugImage})
}

// InjectDebugContainerWithOptions runs the same ephemeral-container
// dance as InjectDebugContainer but accepts a full options bag so
// the frontend can offer multiple debug strategies (netshoot /
// busybox / alpine / custom image) and override the entrypoint,
// target container, and extra env vars per pod.
func (c *Client) InjectDebugContainerWithOptions(ctx context.Context, namespace, podName string, opts DebugOptions) (*DebugSession, error) {
	if opts.Image == "" {
		opts.Image = "nicolaka/netshoot:latest"
	}

	pod, err := c.cs.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get pod: %w", err)
	}

	containerName := "argus-debug-" + podName
	if len(containerName) > 63 {
		containerName = containerName[:63]
	}

	// Always include the pod/namespace beacons. User-supplied env
	// adds on top — caller can't shadow them because we append last.
	env := []corev1.EnvVar{
		{Name: "DEBUG_POD", Value: podName},
		{Name: "DEBUG_NAMESPACE", Value: namespace},
	}
	for k, v := range opts.Env {
		// Skip the reserved keys silently so a sloppy caller can't
		// override DEBUG_POD / DEBUG_NAMESPACE.
		if k == "DEBUG_POD" || k == "DEBUG_NAMESPACE" {
			continue
		}
		env = append(env, corev1.EnvVar{Name: k, Value: v})
	}

	ephemeral := &corev1.EphemeralContainer{
		EphemeralContainerCommon: corev1.EphemeralContainerCommon{
			Name:            containerName,
			Image:           opts.Image,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Stdin:           true,
			TTY:             true,
			Env:             env,
			Command:         opts.Command,
			Args:            opts.Args,
		},
	}

	// Pin the target container. Caller-provided wins if it matches
	// a real container; otherwise fall back to the first container.
	if opts.TargetContainer != "" && podHasContainer(pod, opts.TargetContainer) {
		ephemeral.TargetContainerName = opts.TargetContainer
	} else if len(pod.Spec.Containers) > 0 {
		ephemeral.TargetContainerName = pod.Spec.Containers[0].Name
	}

	pod.Spec.EphemeralContainers = append(pod.Spec.EphemeralContainers, *ephemeral)
	if _, err := c.cs.CoreV1().Pods(namespace).Update(ctx, pod, metav1.UpdateOptions{}); err != nil {
		return nil, fmt.Errorf("inject debug container: %w", err)
	}

	return &DebugSession{
		PodName:       podName,
		Namespace:     namespace,
		ContainerName: containerName,
		DebugImage:    opts.Image,
		Started:       true,
	}, nil
}

func podHasContainer(pod *corev1.Pod, name string) bool {
	for _, c := range pod.Spec.Containers {
		if c.Name == name {
			return true
		}
	}
	for _, c := range pod.Spec.InitContainers {
		if c.Name == name {
			return true
		}
	}
	return false
}
