package k8s

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func (c *Client) listPVCs(ctx context.Context, ns string) (*ResourceListResult, error) {
	list, err := c.cs.CoreV1().PersistentVolumeClaims(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	schema := ResourceSchema{
		Kind: "PersistentVolumeClaim",
		Columns: []ResourceColumn{
			{Key: "pvc_status", Header: "Status"},
			{Key: "volume", Header: "Volume"},
			{Key: "capacity", Header: "Capacity"},
			{Key: "access_modes", Header: "Access Modes"},
			{Key: "storage_class", Header: "Storage Class"},
		},
	}

	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		pvc := &list.Items[i]
		status := string(pvc.Status.Phase)
		color := "green"
		if pvc.Status.Phase == corev1.ClaimPending {
			color = "amber"
		}

		capacity := "—"
		if pvc.Status.Capacity != nil {
			if q, ok := pvc.Status.Capacity[corev1.ResourceStorage]; ok {
				capacity = q.String()
			}
		}

		modes := fmtAccessModes(pvc.Spec.AccessModes)
		sc := "—"
		if pvc.Spec.StorageClassName != nil {
			sc = *pvc.Spec.StorageClassName
		}

		items = append(items, ResourceItem{
			Name:        pvc.Name,
			Namespace:   pvc.Namespace,
			Status:      status,
			StatusColor: color,
			Age:         fmtAge(pvc.CreationTimestamp.Time),
			Fields: map[string]string{
				"pvc_status":    status,
				"volume":        orDash(pvc.Spec.VolumeName),
				"capacity":      capacity,
				"access_modes":  modes,
				"storage_class": sc,
			},
		})
	}

	return &ResourceListResult{Schema: schema, Items: items, Total: len(items)}, nil
}

func (c *Client) listPVs(ctx context.Context) (*ResourceListResult, error) {
	list, err := c.cs.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	schema := ResourceSchema{
		Kind: "PersistentVolume",
		Columns: []ResourceColumn{
			{Key: "capacity", Header: "Capacity"},
			{Key: "access_modes", Header: "Access Modes"},
			{Key: "reclaim_policy", Header: "Reclaim Policy"},
			{Key: "pv_status", Header: "Status"},
			{Key: "claim", Header: "Claim"},
			{Key: "storage_class", Header: "Storage Class"},
		},
	}

	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		pv := &list.Items[i]
		capacity := "—"
		if q, ok := pv.Spec.Capacity[corev1.ResourceStorage]; ok {
			capacity = q.String()
		}

		claim := "—"
		if pv.Spec.ClaimRef != nil {
			claim = pv.Spec.ClaimRef.Namespace + "/" + pv.Spec.ClaimRef.Name
		}

		color := "green"
		if pv.Status.Phase == corev1.VolumeReleased {
			color = "amber"
		}
		if pv.Status.Phase == corev1.VolumeFailed {
			color = "red"
		}

		items = append(items, ResourceItem{
			Name:        pv.Name,
			Namespace:   "",
			Status:      string(pv.Status.Phase),
			StatusColor: color,
			Age:         fmtAge(pv.CreationTimestamp.Time),
			Fields: map[string]string{
				"capacity":       capacity,
				"access_modes":   fmtAccessModes(pv.Spec.AccessModes),
				"reclaim_policy": string(pv.Spec.PersistentVolumeReclaimPolicy),
				"pv_status":      string(pv.Status.Phase),
				"claim":          claim,
				"storage_class":  orDash(pv.Spec.StorageClassName),
			},
		})
	}

	return &ResourceListResult{Schema: schema, Items: items, Total: len(items)}, nil
}

func (c *Client) listStorageClasses(ctx context.Context) (*ResourceListResult, error) {
	list, err := c.cs.StorageV1().StorageClasses().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	schema := ResourceSchema{
		Kind: "StorageClass",
		Columns: []ResourceColumn{
			{Key: "provisioner", Header: "Provisioner"},
			{Key: "reclaim_policy", Header: "Reclaim Policy"},
			{Key: "binding_mode", Header: "Binding Mode"},
			{Key: "default", Header: "Default"},
		},
	}

	items := make([]ResourceItem, 0, len(list.Items))
	for i := range list.Items {
		sc := &list.Items[i]
		reclaimPolicy := "Delete"
		if sc.ReclaimPolicy != nil {
			reclaimPolicy = string(*sc.ReclaimPolicy)
		}
		bindingMode := "Immediate"
		if sc.VolumeBindingMode != nil {
			bindingMode = string(*sc.VolumeBindingMode)
		}
		isDefault := "false"
		if sc.Annotations["storageclass.kubernetes.io/is-default-class"] == "true" {
			isDefault = "true"
		}

		items = append(items, ResourceItem{
			Name:        sc.Name,
			Namespace:   "",
			Status:      "Active",
			StatusColor: "blue",
			Age:         fmtAge(sc.CreationTimestamp.Time),
			Fields: map[string]string{
				"provisioner":    sc.Provisioner,
				"reclaim_policy": reclaimPolicy,
				"binding_mode":   bindingMode,
				"default":        isDefault,
			},
		})
	}

	return &ResourceListResult{Schema: schema, Items: items, Total: len(items)}, nil
}
