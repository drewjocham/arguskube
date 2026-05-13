package k8s

import (
	"context"
	"fmt"

	authv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

type CanIResult struct {
	Verb      string `json:"verb"`
	Resource  string `json:"resource"`
	Namespace string `json:"namespace"`
	Allowed   bool   `json:"allowed"`
	Reason    string `json:"reason,omitempty"`
}

type ImpersonationView struct {
	User         string        `json:"user"`
	Group        string        `json:"group,omitempty"`
	Capabilities []CanIResult  `json:"capabilities"`
}

func (c *Client) CheckCanI(ctx context.Context, namespace, verb, resource, apiGroup string) (*CanIResult, error) {
	sar := &authv1.SelfSubjectAccessReview{
		Spec: authv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authv1.ResourceAttributes{
				Namespace: namespace,
				Verb:      verb,
				Group:     apiGroup,
				Resource:  resource,
			},
		},
	}

	result, err := c.cs.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, sar, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("selfsubjectaccessreview: %w", err)
	}

	return &CanIResult{
		Verb:     verb,
		Resource: resource,
		Allowed:  result.Status.Allowed,
		Reason:   result.Status.Reason,
	}, nil
}

func (c *Client) BatchCheckCanI(ctx context.Context, namespace string, checks []CanIResult) ([]CanIResult, error) {
	results := make([]CanIResult, 0, len(checks))
	for _, check := range checks {
		result, err := c.CheckCanI(ctx, namespace, check.Verb, check.Resource, "")
		if err != nil {
			results = append(results, CanIResult{
				Verb:     check.Verb,
				Resource: check.Resource,
				Allowed:  false,
				Reason:   err.Error(),
			})
			continue
		}
		results = append(results, *result)
	}
	return results, nil
}

func (c *Client) GetCommonPermissions(ctx context.Context, namespace string) ([]CanIResult, error) {
	commonVerbs := []string{"get", "list", "watch", "create", "update", "patch", "delete"}
	commonResources := []string{"pods", "deployments", "services", "configmaps", "secrets", "ingresses", "events"}

	var checks []CanIResult
	for _, resource := range commonResources {
		for _, verb := range commonVerbs {
			if verb == "delete" || verb == "create" || verb == "update" || verb == "patch" {
				continue
			}
			checks = append(checks, CanIResult{
				Verb:     verb,
				Resource: resource,
			})
		}
	}

	return c.BatchCheckCanI(ctx, namespace, checks)
}

func (c *Client) ImpersonateUser(ctx context.Context, user string, groups []string) (*ImpersonationView, error) {
	if user == "" {
		user = "system:serviceaccount:kube-system:default"
	}

	impersonateCfg := rest.CopyConfig(c.restCfg)
	impersonateCfg.Impersonate = rest.ImpersonationConfig{
		UserName: user,
		Groups:   groups,
	}

	view := &ImpersonationView{
		User:  user,
		Group: "",
	}
	if len(groups) > 0 {
		view.Group = groups[0]
	}

	return view, nil
}


