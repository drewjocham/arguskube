package k8s

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestDuplicateDeployment(t *testing.T) {
	cs := fake.NewSimpleClientset()
	c := &Client{cs: cs, logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))}

	rep := int32(3)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-app",
			Namespace: "prod",
			Labels:    map[string]string{"app": "my-app"},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &rep,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{Name: "app", Image: "nginx:latest"},
					},
				},
			},
		},
	}
	_, err := cs.AppsV1().Deployments("prod").Create(context.Background(), dep, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	tests := []struct {
		name           string
		sourceNs       string
		sourceName     string
		targetNs       string
		wantErr        string
		wantTargetName string
	}{
		{
			name:           "duplicate to staging",
			sourceNs:       "prod",
			sourceName:     "my-app",
			targetNs:       "staging",
			wantTargetName: "my-app",
		},
		{
			name:       "source not found",
			sourceNs:   "prod",
			sourceName: "nonexistent",
			targetNs:   "staging",
			wantErr:    "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := c.DuplicateDeployment(context.Background(), tt.sourceNs, tt.sourceName, tt.targetNs)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Deployment != tt.wantTargetName {
				t.Errorf("deployment name = %q, want %q", result.Deployment, tt.wantTargetName)
			}
			if result.Namespace != tt.targetNs {
				t.Errorf("namespace = %q, want %q", result.Namespace, tt.targetNs)
			}
			created, err := cs.AppsV1().Deployments(tt.targetNs).Get(context.Background(), tt.wantTargetName, metav1.GetOptions{})
			if err != nil {
				t.Fatalf("target deployment not created: %v", err)
			}
			if created.Spec.Replicas == nil || *created.Spec.Replicas != 3 {
				t.Error("target deployment spec was not preserved")
			}
		})
	}
}

func TestCreateJobFromCronJob(t *testing.T) {
	cs := fake.NewSimpleClientset()
	c := &Client{cs: cs, logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))}

	cj := &batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "db-backup",
			Namespace: "default",
			UID:       "abc-123",
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "0 * * * *",
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{Name: "backup", Image: "postgres:15"},
							},
							RestartPolicy: corev1.RestartPolicyNever,
						},
					},
				},
			},
		},
	}
	_, err := cs.BatchV1().CronJobs("default").Create(context.Background(), cj, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Fake client doesn't process GenerateName, so we verify by listing jobs.
	ctx := context.Background()
	job, err := c.CreateJobFromCronJob(ctx, "default", "db-backup")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if job == nil {
		t.Fatal("job is nil")
	}
	if job.Namespace != "default" {
		t.Errorf("job namespace = %q, want %q", job.Namespace, "default")
	}
	if len(job.Spec.Template.Spec.Containers) == 0 {
		t.Error("job has no containers")
	}
	// Verify container image is preserved from cronjob.
	if job.Spec.Template.Spec.Containers[0].Image != "postgres:15" {
		t.Errorf("container image = %q, want %q", job.Spec.Template.Spec.Containers[0].Image, "postgres:15")
	}

	// Error case: nonexistent cronjob.
	_, err = c.CreateJobFromCronJob(ctx, "default", "nonexistent")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected 'not found' error, got %v", err)
	}
}

func TestSuggestResources(t *testing.T) {
	cs := fake.NewSimpleClientset()
	c := &Client{cs: cs, logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))}

	node := &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
		Status: corev1.NodeStatus{
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:    resource.MustParse("4"),
				corev1.ResourceMemory: resource.MustParse("16Gi"),
			},
		},
	}
	_, err := cs.CoreV1().Nodes().Create(context.Background(), node, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	tests := []struct {
		name    string
		profile string
		wantErr string
		check   string
	}{
		{
			name:    "valid profile returns suggestion",
			profile: "medium-api",
			check:   "250m",
		},
		{
			name:    "returns node capacity",
			profile: "small-web",
			check:   "100m",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sug, err := c.GetResourceSuggestion(context.Background(), tt.profile)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if sug.Profile.CPURequest != tt.check {
				t.Errorf("CPURequest = %q, want %q", sug.Profile.CPURequest, tt.check)
			}
			if len(sug.NodeCapacity) == 0 {
				t.Error("expected node capacity list, got empty")
			}
		})
	}
}
