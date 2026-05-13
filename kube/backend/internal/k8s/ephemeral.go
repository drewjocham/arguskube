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

func (c *Client) InjectDebugContainer(ctx context.Context, namespace, podName, debugImage string) (*DebugSession, error) {
	if debugImage == "" {
		debugImage = "nicolaka/netshoot:latest"
	}

	pod, err := c.cs.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get pod: %w", err)
	}

	containerName := "argus-debug-" + podName
	if len(containerName) > 63 {
		containerName = containerName[:63]
	}

	ephemeral := &corev1.EphemeralContainer{
		EphemeralContainerCommon: corev1.EphemeralContainerCommon{
			Name:            containerName,
			Image:           debugImage,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Stdin:           true,
			TTY:             true,
			Env: []corev1.EnvVar{
				{Name: "DEBUG_POD", Value: podName},
				{Name: "DEBUG_NAMESPACE", Value: namespace},
			},
		},
		TargetContainerName: "",
	}

	if len(pod.Spec.Containers) > 0 {
		ephemeral.TargetContainerName = pod.Spec.Containers[0].Name
	}

	pod.Spec.EphemeralContainers = append(pod.Spec.EphemeralContainers, *ephemeral)
	updated, err := c.cs.CoreV1().Pods(namespace).Update(ctx, pod, metav1.UpdateOptions{})
	if err != nil {
		return nil, fmt.Errorf("inject debug container: %w", err)
	}

	_ = updated
	return &DebugSession{
		PodName:       podName,
		Namespace:     namespace,
		ContainerName: containerName,
		DebugImage:    debugImage,
		Started:       true,
	}, nil
}
