package k8s

import (
	"context"
	"fmt"

	discoveryv1 "k8s.io/api/discovery/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type EndpointSliceInfo struct {
	Name       string              `json:"name"`
	Namespace  string              `json:"namespace"`
	AddressType string             `json:"addressType"`
	Endpoints  int                 `json:"endpoints"`
	ReadyCount int                 `json:"readyCount"`
	ZoneCount  map[string]int      `json:"zoneCount"`
	ByService  string              `json:"byService,omitempty"`
}

type ZoneDistribution struct {
	TotalEndpoints int            `json:"totalEndpoints"`
	ZoneCounts     map[string]int `json:"zoneCounts"`
	Imbalanced     bool           `json:"imbalanced"`
	MaxZonePct     float64        `json:"maxZonePct"`
}

func (c *Client) ListEndpointSlices(ctx context.Context, namespace string) ([]EndpointSliceInfo, error) {
	list, err := c.cs.DiscoveryV1().EndpointSlices(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list endpointslices: %w", err)
	}

	infos := make([]EndpointSliceInfo, 0, len(list.Items))
	for i := range list.Items {
		es := &list.Items[i]

		zoneCount := make(map[string]int)
		readyCount := 0

		for _, ep := range es.Endpoints {
			zone := ep.Zone
			if zone == nil || *zone == "" {
				zone = strPtr("unknown")
			}
			zoneCount[*zone]++

			if isEndpointReady(ep) {
				readyCount++
			}
		}

		svcName := ""
		if len(es.Labels) > 0 {
			if name, ok := es.Labels["kubernetes.io/service-name"]; ok {
				svcName = name
			}
		}

		infos = append(infos, EndpointSliceInfo{
			Name:        es.Name,
			Namespace:   es.Namespace,
			AddressType: string(es.AddressType),
			Endpoints:   len(es.Endpoints),
			ReadyCount:  readyCount,
			ZoneCount:   zoneCount,
			ByService:   svcName,
		})
	}

	return infos, nil
}

func (c *Client) GetZoneDistribution(ctx context.Context, namespace, serviceName string) (*ZoneDistribution, error) {
	selector := ""
	if serviceName != "" {
		selector = fmt.Sprintf("kubernetes.io/service-name=%s", serviceName)
	}

	list, err := c.cs.DiscoveryV1().EndpointSlices(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: selector,
	})
	if err != nil {
		return nil, fmt.Errorf("list endpointslices: %w", err)
	}

	dist := &ZoneDistribution{
		ZoneCounts: make(map[string]int),
	}

	for i := range list.Items {
		es := &list.Items[i]
		for _, ep := range es.Endpoints {
			zone := ep.Zone
			if zone == nil || *zone == "" {
				zone = strPtr("unknown")
			}
			dist.ZoneCounts[*zone]++
			dist.TotalEndpoints++
		}
	}

	if dist.TotalEndpoints > 0 {
		maxCount := 0
		for _, count := range dist.ZoneCounts {
			if count > maxCount {
				maxCount = count
			}
		}
		dist.MaxZonePct = float64(maxCount) / float64(dist.TotalEndpoints) * 100
		dist.Imbalanced = dist.MaxZonePct > 80
	}

	return dist, nil
}

func (c *Client) CheckTopologyAwareRouting(ctx context.Context, namespace string) ([]TopologyWarning, error) {
	slices, err := c.ListEndpointSlices(ctx, namespace)
	if err != nil {
		return nil, err
	}

	var warnings []TopologyWarning
	for _, slice := range slices {
		if slice.Endpoints == 0 {
			continue
		}

		maxCount := 0
		total := slice.Endpoints
		for _, count := range slice.ZoneCount {
			if count > maxCount {
				maxCount = count
			}
		}

		pct := float64(maxCount) / float64(total) * 100
		if pct > 80 {
			warnings = append(warnings, TopologyWarning{
				ServiceName:  slice.ByService,
				Namespace:    slice.Namespace,
				MaxZonePct:   pct,
				TotalInZone:  maxCount,
				TotalEndpoints: total,
				Message:      fmt.Sprintf("%s: %.0f%% of endpoints in a single zone — risk of traffic imbalance", slice.ByService, pct),
			})
		}
	}

	return warnings, nil
}

type TopologyWarning struct {
	ServiceName   string  `json:"serviceName"`
	Namespace     string  `json:"namespace"`
	MaxZonePct    float64 `json:"maxZonePct"`
	TotalInZone   int     `json:"totalInZone"`
	TotalEndpoints int    `json:"totalEndpoints"`
	Message       string  `json:"message"`
}

func isEndpointReady(ep discoveryv1.Endpoint) bool {
	if ep.Conditions.Ready == nil {
		return true
	}
	return *ep.Conditions.Ready
}

func strPtr(s string) *string {
	return &s
}


