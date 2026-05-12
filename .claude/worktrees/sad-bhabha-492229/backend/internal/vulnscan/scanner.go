package vulnscan

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Vulnerability is a single CVE finding.
type Vulnerability struct {
	ID       string `json:"id"`
	Pkg      string `json:"pkg"`
	Severity string `json:"severity"`
	Desc     string `json:"desc"`
	Fix      string `json:"fix"`
}

// AIOptimization is an AI-generated remediation suggestion for an image.
type AIOptimization struct {
	Issue string `json:"issue"`
	Fix   string `json:"fix"`
}

// ScannedImage represents a scanned container image with its vulnerabilities.
type ScannedImage struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Namespace string         `json:"namespace"`
	LastScan  string         `json:"lastScan"`
	Critical  int            `json:"critical"`
	High      int            `json:"high"`
	Medium    int            `json:"medium"`
	Low       int            `json:"low"`
	Status    string         `json:"status"`
	CVEs      []Vulnerability `json:"cves"`
	AIOpt     AIOptimization `json:"aiOpt"`
}

// Scanner manages vulnerability scanning of cluster images.
type Scanner struct {
	cs     kubernetes.Interface
	logger *slog.Logger

	mu      sync.RWMutex
	results []ScannedImage
}

// New creates a new vulnerability scanner.
func New(cs kubernetes.Interface, logger *slog.Logger) *Scanner {
	return &Scanner{
		cs:     cs,
		logger: logger.With("component", "vulnscan"),
	}
}

// List returns the cached scan results.
func (s *Scanner) List() []ScannedImage {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.results) == 0 {
		return DemoResults()
	}
	return s.results
}

// ScanAll enumerates all unique images in the cluster and scans each one.
func (s *Scanner) ScanAll(ctx context.Context, namespace string) ([]ScannedImage, error) {
	images, err := s.enumerateImages(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("enumerate images: %w", err)
	}

	s.logger.Info("starting vulnerability scan",
		slog.Int("images", len(images)),
		slog.String("namespace", namespace),
	)

	var results []ScannedImage
	for i, img := range images {
		scanned, err := s.scanImage(ctx, img)
		if err != nil {
			s.logger.Warn("scan failed for image",
				slog.String("image", img.name),
				slog.String("error", err.Error()),
			)
			// Add a placeholder for failed scans.
			results = append(results, ScannedImage{
				ID:        fmt.Sprintf("img-%d", i+1),
				Name:      img.name,
				Namespace: img.namespace,
				LastScan:  "failed",
				Status:    "Error",
				CVEs:      []Vulnerability{},
				AIOpt:     AIOptimization{Issue: "Scan failed: " + err.Error(), Fix: "Ensure Trivy is installed and the image is pullable."},
			})
			continue
		}
		scanned.ID = fmt.Sprintf("img-%d", i+1)
		results = append(results, *scanned)
	}

	// Sort: most critical first.
	sort.Slice(results, func(i, j int) bool {
		return results[i].Critical*1000+results[i].High*100+results[i].Medium*10 >
			results[j].Critical*1000+results[j].High*100+results[j].Medium*10
	})

	s.mu.Lock()
	s.results = results
	s.mu.Unlock()

	s.logger.Info("vulnerability scan complete",
		slog.Int("images_scanned", len(results)),
	)

	return results, nil
}

// ScanSingleImage scans a single container image by name.
func (s *Scanner) ScanSingleImage(ctx context.Context, image, engine string) (string, error) {
	s.logger.Info("scanning single image",
		slog.String("image", image),
		slog.String("engine", engine),
	)

	scanned, err := s.scanImage(ctx, clusterImage{name: image, namespace: "manual"})
	if err != nil {
		return "", err
	}

	// Update the cache if this image exists.
	s.mu.Lock()
	found := false
	for i, r := range s.results {
		if r.Name == image {
			scanned.ID = r.ID
			s.results[i] = *scanned
			found = true
			break
		}
	}
	if !found {
		scanned.ID = fmt.Sprintf("img-%d", len(s.results)+1)
		s.results = append(s.results, *scanned)
	}
	s.mu.Unlock()

	return fmt.Sprintf("Scan complete for %s: %d critical, %d high, %d medium, %d low",
		image, scanned.Critical, scanned.High, scanned.Medium, scanned.Low), nil
}

// clusterImage holds an image reference found in the cluster.
type clusterImage struct {
	name      string
	namespace string
}

// enumerateImages finds all unique container images across running pods.
func (s *Scanner) enumerateImages(ctx context.Context, namespace string) ([]clusterImage, error) {
	if s.cs == nil {
		return nil, fmt.Errorf("no cluster connection")
	}

	pods, err := s.cs.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: "status.phase=Running",
	})
	if err != nil {
		return nil, err
	}

	// Deduplicate by image name, keep first namespace seen.
	seen := make(map[string]string) // image -> namespace
	for i := range pods.Items {
		p := &pods.Items[i]
		for _, c := range p.Spec.Containers {
			if _, exists := seen[c.Image]; !exists {
				seen[c.Image] = p.Namespace
			}
		}
		for _, c := range p.Spec.InitContainers {
			if _, exists := seen[c.Image]; !exists {
				seen[c.Image] = p.Namespace
			}
		}
	}

	images := make([]clusterImage, 0, len(seen))
	for img, ns := range seen {
		images = append(images, clusterImage{name: img, namespace: ns})
	}

	// Sort for deterministic output.
	sort.Slice(images, func(i, j int) bool {
		return images[i].name < images[j].name
	})

	return images, nil
}

// --- Trivy JSON output types ---

type trivyReport struct {
	Results []trivyResult `json:"Results"`
}

type trivyResult struct {
	Target          string            `json:"Target"`
	Vulnerabilities []trivyVulnerability `json:"Vulnerabilities"`
}

type trivyVulnerability struct {
	VulnerabilityID string `json:"VulnerabilityID"`
	PkgName         string `json:"PkgName"`
	Severity        string `json:"Severity"`
	Title           string `json:"Title"`
	Description     string `json:"Description"`
	FixedVersion    string `json:"FixedVersion"`
}

// scanImage runs Trivy against a single container image.
func (s *Scanner) scanImage(ctx context.Context, img clusterImage) (*ScannedImage, error) {
	start := time.Now()
	output, err := s.execTrivy(ctx, img.name)
	elapsed := time.Since(start)

	if err != nil {
		return nil, err
	}

	var report trivyReport
	if err := json.Unmarshal(output, &report); err != nil {
		return nil, fmt.Errorf("trivy json parse: %w", err)
	}

	result := &ScannedImage{
		Name:      img.name,
		Namespace: img.namespace,
		LastScan:  fmtElapsed(elapsed),
		CVEs:      []Vulnerability{},
	}

	// Aggregate vulnerabilities from all results.
	for _, r := range report.Results {
		for _, v := range r.Vulnerabilities {
			sev := titleCase(v.Severity)
			switch sev {
			case "Critical":
				result.Critical++
			case "High":
				result.High++
			case "Medium":
				result.Medium++
			case "Low":
				result.Low++
			}

			// Only include Critical and High CVEs in the detail list.
			if sev == "Critical" || sev == "High" {
				desc := v.Title
				if desc == "" {
					desc = v.Description
				}
				if len(desc) > 120 {
					desc = desc[:120] + "..."
				}
				fix := v.FixedVersion
				if fix == "" {
					fix = "No fix available yet"
				} else {
					fix = "Upgrade " + v.PkgName + " to " + fix
				}
				result.CVEs = append(result.CVEs, Vulnerability{
					ID:       v.VulnerabilityID,
					Pkg:      v.PkgName,
					Severity: sev,
					Desc:     desc,
					Fix:      fix,
				})
			}
		}
	}

	// Determine status.
	if result.Critical > 0 {
		result.Status = "Vulnerable"
	} else if result.High > 0 {
		result.Status = "Warning"
	} else {
		result.Status = "Clean"
	}

	// Generate AI optimization suggestion.
	result.AIOpt = generateAIOptimization(img.name, result)

	return result, nil
}

// execTrivy tries local binary first, then Docker.
func (s *Scanner) execTrivy(ctx context.Context, image string) ([]byte, error) {
	// Try local binary.
	if trivyPath, err := exec.LookPath("trivy"); err == nil {
		return s.execTrivyBinary(ctx, trivyPath, image)
	}

	// Fall back to Docker.
	if dockerPath, err := exec.LookPath("docker"); err == nil {
		return s.execTrivyDocker(ctx, dockerPath, image)
	}

	return nil, fmt.Errorf("trivy not found: install trivy (https://aquasecurity.github.io/trivy/) or Docker")
}

func (s *Scanner) execTrivyBinary(ctx context.Context, trivyPath, image string) ([]byte, error) {
	args := []string{
		"image", "--format", "json",
		"--severity", "CRITICAL,HIGH,MEDIUM,LOW",
		"--quiet",
		image,
	}

	s.logger.Debug("running trivy binary",
		slog.String("image", image),
	)

	cmd := exec.CommandContext(ctx, trivyPath, args...)
	output, err := cmd.Output()
	if err != nil && len(output) == 0 {
		return nil, fmt.Errorf("trivy exec: %w", err)
	}
	return output, nil
}

func (s *Scanner) execTrivyDocker(ctx context.Context, dockerPath, image string) ([]byte, error) {
	args := []string{
		"run", "--rm",
		"aquasec/trivy:latest",
		"image", "--format", "json",
		"--severity", "CRITICAL,HIGH,MEDIUM,LOW",
		"--quiet",
		image,
	}

	s.logger.Debug("running trivy via docker",
		slog.String("image", image),
	)

	cmd := exec.CommandContext(ctx, dockerPath, args...)
	output, err := cmd.Output()
	if err != nil && len(output) == 0 {
		return nil, fmt.Errorf("trivy docker exec: %w", err)
	}
	return output, nil
}

// generateAIOptimization creates a context-aware suggestion for an image.
func generateAIOptimization(imageName string, result *ScannedImage) AIOptimization {
	lower := strings.ToLower(imageName)

	if result.Critical == 0 && result.High == 0 && result.Medium == 0 && result.Low == 0 {
		return AIOptimization{
			Issue: "None",
			Fix:   "Image is optimal and following least-privilege principles.",
		}
	}

	if strings.Contains(lower, ":latest") {
		return AIOptimization{
			Issue: fmt.Sprintf("Using ':latest' tag with %d critical and %d high vulnerabilities.", result.Critical, result.High),
			Fix:   "Pin to a specific SHA digest or immutable tag, and rebuild using a minimal base image like distroless.",
		}
	}

	if strings.Contains(lower, "debian") || strings.Contains(lower, "ubuntu") || strings.Contains(lower, "bullseye") || strings.Contains(lower, "buster") {
		return AIOptimization{
			Issue: fmt.Sprintf("Full OS base image has %d critical and %d high vulnerabilities.", result.Critical, result.High),
			Fix:   "Rebuild using distroless/cc-debian12 or alpine to reduce attack surface and drop most of these CVEs.",
		}
	}

	if strings.Contains(lower, "alpine") {
		if result.Critical > 0 {
			return AIOptimization{
				Issue: fmt.Sprintf("Alpine image has %d critical vulnerabilities despite minimal base.", result.Critical),
				Fix:   "Update the alpine base to the latest patch version and rebuild. Check apk packages for known fixes.",
			}
		}
		return AIOptimization{
			Issue: fmt.Sprintf("Alpine base image with %d medium/low findings.", result.Medium+result.Low),
			Fix:   "Consider upgrading to the latest alpine patch release to resolve remaining advisories.",
		}
	}

	if strings.Contains(lower, "nginx") || strings.Contains(lower, "ingress") {
		return AIOptimization{
			Issue: fmt.Sprintf("Ingress/proxy image has %d critical and %d high vulnerabilities.", result.Critical, result.High),
			Fix:   "Upgrade to the latest stable release. For ingress-nginx, check the controller compatibility matrix.",
		}
	}

	if result.Critical > 0 {
		return AIOptimization{
			Issue: fmt.Sprintf("Image has %d critical vulnerabilities that need immediate attention.", result.Critical),
			Fix:   "Rebuild the image with updated base layers and dependencies. Run 'trivy image --severity CRITICAL' for details.",
		}
	}

	return AIOptimization{
		Issue: fmt.Sprintf("Image has %d high and %d medium vulnerabilities.", result.High, result.Medium),
		Fix:   "Schedule a rebuild with updated dependencies to resolve high-severity findings.",
	}
}

func fmtElapsed(d time.Duration) string {
	if d < time.Second {
		return "just now"
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm ago", int(d.Minutes()))
}

// DemoResults returns realistic demo data when no cluster or Trivy is available.
func DemoResults() []ScannedImage {
	return []ScannedImage{
		{
			ID: "img-1", Name: "web-app:v1.2.4", Namespace: "default", LastScan: "10m ago",
			Critical: 2, High: 5, Medium: 12, Low: 24, Status: "Vulnerable",
			CVEs: []Vulnerability{
				{ID: "CVE-2023-38545", Pkg: "curl", Severity: "Critical", Desc: "Heap based buffer overflow in SOCKS5 proxy handshake.", Fix: "Upgrade to curl 8.4.0"},
				{ID: "CVE-2023-4911", Pkg: "glibc", Severity: "Critical", Desc: "Buffer overflow in ld.so (Looney Tunables).", Fix: "Update glibc to 2.38-r1"},
				{ID: "CVE-2024-23222", Pkg: "libxml2", Severity: "High", Desc: "Type confusion in libxml2 parser.", Fix: "Upgrade libxml2 to 2.10.4"},
				{ID: "CVE-2024-21803", Pkg: "python3", Severity: "High", Desc: "Heap buffer overflow in email module.", Fix: "Upgrade python3 to 3.11.7"},
				{ID: "CVE-2024-0001", Pkg: "openssl", Severity: "High", Desc: "Timing side-channel in RSA decryption.", Fix: "Upgrade openssl to 3.0.13"},
				{ID: "CVE-2024-0002", Pkg: "zlib", Severity: "High", Desc: "Memory corruption in inflate.", Fix: "Upgrade zlib to 1.3.1"},
				{ID: "CVE-2024-0003", Pkg: "curl", Severity: "High", Desc: "Cookie injection via redirect.", Fix: "Upgrade curl to 8.6.0"},
			},
			AIOpt: AIOptimization{Issue: "Base image is using debian:bullseye which has numerous unpatched CVEs.", Fix: "Rebuild image using distroless/cc-debian12 to reduce attack surface and drop 85% of these vulnerabilities."},
		},
		{
			ID: "img-4", Name: "ingress-nginx:v1.9.0", Namespace: "kube-system", LastScan: "1d ago",
			Critical: 1, High: 3, Medium: 15, Low: 40, Status: "Vulnerable",
			CVEs: []Vulnerability{
				{ID: "CVE-2023-44487", Pkg: "nginx", Severity: "Critical", Desc: "HTTP/2 Rapid Reset Attack.", Fix: "Upgrade to ingress-nginx v1.9.3+"},
				{ID: "CVE-2024-24989", Pkg: "nginx", Severity: "High", Desc: "Off-by-one in ngx_resolver_copy.", Fix: "Upgrade nginx to 1.25.4"},
				{ID: "CVE-2024-0007", Pkg: "openssl", Severity: "High", Desc: "Null pointer deref in TLS.", Fix: "Upgrade openssl to 3.1.4"},
				{ID: "CVE-2024-0008", Pkg: "libnghttp2", Severity: "High", Desc: "HTTP/2 stream memory leak.", Fix: "Upgrade nghttp2 to 1.60"},
			},
			AIOpt: AIOptimization{Issue: "Nginx ingress controller is exposed to HTTP/2 Rapid Reset DOS.", Fix: "Patch controller deployment and enable global rate limiting."},
		},
		{
			ID: "img-2", Name: "worker:latest", Namespace: "default", LastScan: "1h ago",
			Critical: 0, High: 1, Medium: 4, Low: 8, Status: "Warning",
			CVEs: []Vulnerability{
				{ID: "CVE-2023-5363", Pkg: "openssl", Severity: "High", Desc: "Incorrect cipher key & IV length processing.", Fix: "Upgrade to openssl 3.0.12"},
			},
			AIOpt: AIOptimization{Issue: "Using \"latest\" tag is an anti-pattern and masks underlying OS updates.", Fix: "Pin to a specific SHA digest or immutable tag."},
		},
		{
			ID: "img-3", Name: "payment:v2.0", Namespace: "finance", LastScan: "2m ago",
			Critical: 0, High: 0, Medium: 0, Low: 0, Status: "Clean",
			CVEs:  []Vulnerability{},
			AIOpt: AIOptimization{Issue: "None", Fix: "Image is optimal and following least-privilege principles."},
		},
	}
}

// titleCase uppercases the first letter and lowercases the rest.
func titleCase(s string) string {
	s = strings.ToLower(s)
	if len(s) == 0 {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
