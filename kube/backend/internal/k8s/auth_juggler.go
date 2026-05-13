package k8s

import (
	"context"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type TokenStatus struct {
	Provider  string `json:"provider"`
	Expired   bool   `json:"expired"`
	ExpiresAt string `json:"expiresAt,omitempty"`
	Issuer    string `json:"issuer,omitempty"`
}

type AuthCheckResult struct {
	Tokens    []TokenStatus `json:"tokens"`
	AllValid  bool          `json:"allValid"`
	ClusterOK bool          `json:"clusterOK"`
}

func (c *Client) CheckAuthStatus(ctx context.Context) (*AuthCheckResult, error) {
	result := &AuthCheckResult{AllValid: true}

	// Check cluster connectivity.
	_, err := c.cs.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: 1})
	result.ClusterOK = err == nil

	// Check kubeconfig auth.
	kubeToken, err := c.getKubeToken()
	tokens := []TokenStatus{
		{
			Provider: "kubernetes",
			Expired:  err != nil || kubeToken == "",
			ExpiresAt: "",
			Issuer:   "kubeconfig",
		},
	}

	// Check AWS token if applicable.
	if awsInfo := c.detectAWS(); awsInfo != "" {
		awsExpired, expiry := c.checkAWSToken()
		tokens = append(tokens, TokenStatus{
			Provider:  "aws",
			Expired:   awsExpired,
			ExpiresAt: expiry,
		})
		if awsExpired {
			result.AllValid = false
		}
	}

	// Check GCP token if applicable.
	if gcpInfo := c.detectGCP(); gcpInfo != "" {
		gcpExpired, expiry := c.checkGCPToken()
		tokens = append(tokens, TokenStatus{
			Provider:  "gcp",
			Expired:   gcpExpired,
			ExpiresAt: expiry,
		})
		if gcpExpired {
			result.AllValid = false
		}
	}

	if kubeToken == "" {
		result.AllValid = false
	}

	result.Tokens = tokens
	return result, nil
}

func (c *Client) getKubeToken() (string, error) {
	if c.restCfg == nil {
		return "", fmt.Errorf("no rest config")
	}
	if c.restCfg.BearerToken != "" {
		return "configured", nil
	}
	if c.restCfg.BearerTokenFile != "" {
		return "file", nil
	}
	if c.restCfg.ExecProvider != nil {
		return "exec", nil
	}
	return "", fmt.Errorf("no token found")
}

func (c *Client) detectAWS() string {
	info := c.restCfg
	if info == nil {
		return ""
	}
	if info.Host != "" && (strings.Contains(info.Host, "amazonaws.com") || strings.Contains(info.Host, "eks")) {
		return "eks"
	}
	return ""
}

func (c *Client) detectGCP() string {
	info := c.restCfg
	if info == nil {
		return ""
	}
	if strings.Contains(info.Host, "gke") || strings.Contains(info.Host, "googleapis.com") {
		return "gke"
	}
	return ""
}

func (c *Client) checkAWSToken() (bool, string) {
	_, err := c.cs.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{Limit: 1})
	if err != nil && strings.Contains(err.Error(), "expired") {
		return true, time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	}
	return false, time.Now().Add(12 * time.Hour).Format(time.RFC3339)
}

func (c *Client) checkGCPToken() (bool, string) {
	_, err := c.cs.CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{Limit: 1})
	if err != nil && strings.Contains(err.Error(), "expired") {
		return true, time.Now().Add(-1 * time.Hour).Format(time.RFC3339)
	}
	return false, time.Now().Add(12 * time.Hour).Format(time.RFC3339)
}

type BlastRadiusInfo struct {
	ClusterName string `json:"clusterName"`
	Environment string `json:"environment"`
	IsProd      bool   `json:"isProd"`
}

func (c *Client) GetBlastRadiusInfo(ctx context.Context) (*BlastRadiusInfo, error) {
	info := &BlastRadiusInfo{
		ClusterName: "unknown",
		Environment: "unknown",
	}

	namespaces, err := c.cs.CoreV1().Namespaces().List(ctx, metav1.ListOptions{Limit: 50})
	if err != nil {
		return info, nil
	}

	clusterName := ""
	for i := range namespaces.Items {
		ns := &namespaces.Items[i]
		if v, ok := ns.Labels["kubernetes.io/cluster"]; ok && v != "" {
			clusterName = v
		}
	}
	if clusterName == "" {
		clusterName = c.restCfg.Host
	}
	info.ClusterName = clusterName

	nsNames := make([]string, 0)
	prodKeywords := []string{"prod", "production", "live", "prd"}
	for i := range namespaces.Items {
		nsNames = append(nsNames, namespaces.Items[i].Name)
		lower := strings.ToLower(namespaces.Items[i].Name)
		for _, kw := range prodKeywords {
			if strings.Contains(lower, kw) {
				info.IsProd = true
				info.Environment = "production"
				break
			}
		}
	}
	if !info.IsProd {
		for _, kw := range []string{"staging", "dev", "test"} {
			for _, name := range nsNames {
				if strings.Contains(strings.ToLower(name), kw) {
					info.Environment = "staging"
					break
				}
			}
			if info.Environment != "unknown" {
				break
			}
		}
	}

	return info, nil
}
