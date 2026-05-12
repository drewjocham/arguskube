package k8s

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argues/kube-watcher/internal/config"
	"log/slog"
	"os"
)

func TestCostConfigForProvider(t *testing.T) {
	tests := []struct {
		name     string
		provider CloudProvider
		wantCPU  float64
		wantMem  float64
		wantName string
	}{
		{"aws", ProviderAWS, 0.0425, 0.0053, "AWS"},
		{"gcp", ProviderGCP, 0.0335, 0.0045, "Google Cloud"},
		{"azure", ProviderAzure, 0.0440, 0.0054, "Azure"},
		{"digitalocean", ProviderDigitalOcean, 0.0300, 0.0038, "DigitalOcean"},
		{"unknown falls back to aws", CloudProvider("unknown"), 0.0425, 0.0053, "AWS"},
		{"empty falls back to aws", CloudProvider(""), 0.0425, 0.0053, "AWS"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := CostConfigForProvider(tt.provider)
			if cfg.CPUPerCoreHour != tt.wantCPU {
				t.Errorf("CPUPerCoreHour = %f, want %f", cfg.CPUPerCoreHour, tt.wantCPU)
			}
			if cfg.MemPerGBHour != tt.wantMem {
				t.Errorf("MemPerGBHour = %f, want %f", cfg.MemPerGBHour, tt.wantMem)
			}
			if cfg.ProviderLabel != tt.wantName {
				t.Errorf("ProviderLabel = %q, want %q", cfg.ProviderLabel, tt.wantName)
			}
		})
	}
}

func TestApplyCostRates(t *testing.T) {
	tests := []struct {
		name      string
		cpuCores  float64
		memGB     float64
		provider  CloudProvider
		wantDayGt float64 // TotalCostDay must be > this
	}{
		{"aws 1 core 1gb", 1.0, 1.0, ProviderAWS, 1.0},
		{"gcp 2 cores 4gb", 2.0, 4.0, ProviderGCP, 1.5},
		{"zero resources", 0.0, 0.0, ProviderAWS, -1},
		{"digitalocean 1 core 1gb", 1.0, 1.0, ProviderDigitalOcean, 0.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &CostBreakdown{CPUCores: tt.cpuCores, MemoryGB: tt.memGB}
			cfg := CostConfigForProvider(tt.provider)
			applyCostRates(b, cfg)
			if b.TotalCostDay <= tt.wantDayGt {
				t.Errorf("TotalCostDay = %f, want > %f", b.TotalCostDay, tt.wantDayGt)
			}
			if b.TotalCostMo <= 0 && tt.cpuCores > 0 {
				t.Errorf("TotalCostMo = %f, want > 0", b.TotalCostMo)
			}
			// Monthly should be ~30x daily
			if b.TotalCostDay > 0 {
				ratio := b.TotalCostMo / b.TotalCostDay
				if ratio < 29.5 || ratio > 30.5 {
					t.Errorf("Monthly/Daily ratio = %f, want ~30", ratio)
				}
			}
		})
	}
}

func TestPodResourceRequests(t *testing.T) {
	tests := []struct {
		name     string
		pod      corev1.Pod
		wantCPU  float64
		wantMem  float64
	}{
		{
			name: "no resources",
			pod: corev1.Pod{
				Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "main"}}},
			},
			wantCPU: 0,
			wantMem: 0,
		},
		{
			name: "250m cpu 128Mi mem",
			pod: corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name: "main",
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("250m"),
								corev1.ResourceMemory: resource.MustParse("128Mi"),
							},
						},
					}},
				},
			},
			wantCPU: 0.25,
			wantMem: 0.125,
		},
		{
			name: "multi-container",
			pod: corev1.Pod{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name: "app",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("500m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
							},
						},
						{
							Name: "sidecar",
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("100m"),
									corev1.ResourceMemory: resource.MustParse("64Mi"),
								},
							},
						},
					},
				},
			},
			wantCPU: 0.6,
			wantMem: 0.313, // ~320Mi / 1024
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cpu, mem := podResourceRequests(tt.pod)
			if cpu != tt.wantCPU {
				t.Errorf("cpuCores = %f, want %f", cpu, tt.wantCPU)
			}
			// Memory is rounded to 3 decimals, allow small delta
			if diff := mem - tt.wantMem; diff > 0.002 || diff < -0.002 {
				t.Errorf("memGB = %f, want ~%f", mem, tt.wantMem)
			}
		})
	}
}

func TestOwnerDeployment(t *testing.T) {
	tests := []struct {
		name string
		pod  corev1.Pod
		want string
	}{
		{
			name: "replicaset owner",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{Kind: "ReplicaSet", Name: "web-app-7b9f4c6d5f"},
					},
				},
			},
			want: "web-app",
		},
		{
			name: "statefulset owner",
			pod: corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					OwnerReferences: []metav1.OwnerReference{
						{Kind: "StatefulSet", Name: "redis"},
					},
				},
			},
			want: "redis",
		},
		{
			name: "no owner",
			pod:  corev1.Pod{},
			want: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ownerDeployment(tt.pod); got != tt.want {
				t.Errorf("ownerDeployment() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBuildDailyHistory(t *testing.T) {
	tests := []struct {
		name     string
		baseRate float64
		days     int
		wantLen  int
	}{
		{"30 days", 10.0, 30, 30},
		{"7 days", 5.0, 7, 7},
		{"zero rate", 0.0, 30, 30},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			history := buildDailyHistory(tt.baseRate, tt.days)
			if len(history) != tt.wantLen {
				t.Fatalf("len = %d, want %d", len(history), tt.wantLen)
			}
			for _, d := range history {
				if d.Date == "" {
					t.Error("empty date in history entry")
				}
				if tt.baseRate > 0 && d.CostDay <= 0 {
					t.Errorf("CostDay = %f for date %s, want > 0", d.CostDay, d.Date)
				}
			}
			// Dates should be ascending
			for i := 1; i < len(history); i++ {
				if history[i].Date <= history[i-1].Date {
					t.Errorf("dates not ascending: %s <= %s", history[i].Date, history[i-1].Date)
				}
			}
		})
	}
}

func TestEstimateCostsMultiProvider(t *testing.T) {
	tests := []struct {
		name           string
		provider       CloudProvider
		pods           []corev1.Pod
		wantPodCount   int
		wantCostGt0    bool
		wantProvider   string
		wantCategories int
		wantHistory    int
	}{
		{
			name:     "aws with pods",
			provider: ProviderAWS,
			pods: []corev1.Pod{
				makeFinOpsPod("web", "default", "500m", "256Mi"),
				makeFinOpsPod("api", "default", "1000m", "512Mi"),
			},
			wantPodCount:   2,
			wantCostGt0:    true,
			wantProvider:   "aws",
			wantCategories: 2,
			wantHistory:    30,
		},
		{
			name:     "gcp with pods",
			provider: ProviderGCP,
			pods: []corev1.Pod{
				makeFinOpsPod("web", "prod", "500m", "256Mi"),
			},
			wantPodCount:   1,
			wantCostGt0:    true,
			wantProvider:   "gcp",
			wantCategories: 2,
			wantHistory:    30,
		},
		{
			name:        "empty cluster",
			provider:    ProviderAzure,
			pods:        nil,
			wantPodCount: 0,
			wantCostGt0: false,
			wantProvider: "azure",
		},
		{
			name:     "digitalocean cheaper than aws",
			provider: ProviderDigitalOcean,
			pods: []corev1.Pod{
				makeFinOpsPod("app", "default", "1000m", "1Gi"),
			},
			wantPodCount:   1,
			wantCostGt0:    true,
			wantProvider:   "digitalocean",
			wantCategories: 2,
			wantHistory:    30,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cs := fake.NewSimpleClientset()
			ctx := context.Background()
			for i := range tt.pods {
				_, _ = cs.CoreV1().Pods(tt.pods[i].Namespace).Create(ctx, &tt.pods[i], metav1.CreateOptions{})
			}
			c := &Client{
				cs:     cs,
				cfg:    &config.OnlineDataConfig{},
				logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
			}

			cfg := CostConfigForProvider(tt.provider)
			report, err := c.EstimateCosts(ctx, cfg)
			if err != nil {
				t.Fatalf("EstimateCosts() error = %v", err)
			}
			if report.PodCount != tt.wantPodCount {
				t.Errorf("PodCount = %d, want %d", report.PodCount, tt.wantPodCount)
			}
			if report.Provider != tt.wantProvider {
				t.Errorf("Provider = %q, want %q", report.Provider, tt.wantProvider)
			}
			if tt.wantCostGt0 && report.TotalCostMo <= 0 {
				t.Errorf("TotalCostMo = %f, want > 0", report.TotalCostMo)
			}
			if report.TotalCostYear <= 0 && tt.wantCostGt0 {
				t.Errorf("TotalCostYear = %f, want > 0", report.TotalCostYear)
			}
			// Yearly should be ~365/30 * monthly
			if report.TotalCostMo > 0 {
				ratio := report.TotalCostYear / report.TotalCostMo
				if ratio < 11.5 || ratio > 12.5 {
					t.Errorf("Year/Month ratio = %f, want ~12.17", ratio)
				}
			}
			if len(report.CostCategories) != tt.wantCategories {
				t.Errorf("CostCategories len = %d, want %d", len(report.CostCategories), tt.wantCategories)
			}
			if len(report.DailyHistory) != tt.wantHistory {
				t.Errorf("DailyHistory len = %d, want %d", len(report.DailyHistory), tt.wantHistory)
			}
		})
	}
}

func TestProviderCostComparison(t *testing.T) {
	// DigitalOcean should be cheaper than AWS for identical workload.
	cs := fake.NewSimpleClientset()
	ctx := context.Background()
	pod := makeFinOpsPod("app", "default", "2000m", "4Gi")
	_, _ = cs.CoreV1().Pods("default").Create(ctx, &pod, metav1.CreateOptions{})

	c := &Client{
		cs:     cs,
		cfg:    &config.OnlineDataConfig{},
		logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	}

	awsReport, err := c.EstimateCosts(ctx, CostConfigForProvider(ProviderAWS))
	if err != nil {
		t.Fatal(err)
	}
	doReport, err := c.EstimateCosts(ctx, CostConfigForProvider(ProviderDigitalOcean))
	if err != nil {
		t.Fatal(err)
	}

	if doReport.TotalCostMo >= awsReport.TotalCostMo {
		t.Errorf("DigitalOcean ($%.2f/mo) should be cheaper than AWS ($%.2f/mo)",
			doReport.TotalCostMo, awsReport.TotalCostMo)
	}
}

// --- finops test fixture ---

func makeFinOpsPod(name, ns, cpu, mem string) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "main",
				Image: "app:v1",
				Resources: corev1.ResourceRequirements{
					Requests: corev1.ResourceList{
						corev1.ResourceCPU:    resource.MustParse(cpu),
						corev1.ResourceMemory: resource.MustParse(mem),
					},
				},
			}},
		},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
}
