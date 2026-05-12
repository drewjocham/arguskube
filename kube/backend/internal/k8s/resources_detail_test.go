package k8s

import (
	"context"
	"log/slog"
	"os"
	"testing"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/argues/argus/internal/config"
)

func detailClient(objects ...interface{}) *Client {
	cs := fake.NewSimpleClientset()
	ctx := context.Background()
	for _, obj := range objects {
		switch o := obj.(type) {
		case *corev1.PersistentVolume:
			_, _ = cs.CoreV1().PersistentVolumes().Create(ctx, o, metav1.CreateOptions{})
		case *storagev1.StorageClass:
			_, _ = cs.StorageV1().StorageClasses().Create(ctx, o, metav1.CreateOptions{})
		}
	}
	return &Client{
		cs:     cs,
		cfg:    &config.OnlineDataConfig{},
		logger: slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	}
}

// TestGetResourceDetail_PV verifies the bug fix: clicking a PV in the
// Volumes view used to fall through to the generic handler and surface
// "Detail view not yet implemented for this resource type." Now it
// returns a populated detail with capacity, source, claim ref, etc.
func TestGetResourceDetail_PV(t *testing.T) {
	hostPath := corev1.HostPathVolumeSource{Path: "/data/pv1"}
	mode := corev1.PersistentVolumeFilesystem
	storageClass := "fast-ssd"
	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "pv-1"},
		Spec: corev1.PersistentVolumeSpec{
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse("10Gi"),
			},
			AccessModes:                   []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain,
			StorageClassName:              storageClass,
			VolumeMode:                    &mode,
			ClaimRef: &corev1.ObjectReference{
				Namespace: "default",
				Name:      "data-claim",
			},
			PersistentVolumeSource: corev1.PersistentVolumeSource{HostPath: &hostPath},
		},
		Status: corev1.PersistentVolumeStatus{Phase: corev1.VolumeBound},
	}

	c := detailClient(pv)
	got, err := c.GetResourceDetail(context.Background(), "pvs", "", "pv-1")
	if err != nil {
		t.Fatalf("GetResourceDetail(pvs): %v", err)
	}

	if got.Kind != "PersistentVolume" {
		t.Errorf("Kind = %q, want PersistentVolume", got.Kind)
	}
	if got.Name != "pv-1" {
		t.Errorf("Name = %q, want pv-1", got.Name)
	}

	// The whole point of this fix: NOT the generic fallback message.
	for _, p := range got.Properties {
		if p.Key == "Note" {
			t.Errorf("got generic fallback Note property: %q — PV detail handler missing", p.Value)
		}
	}

	props := propsByKey(got.Properties)
	if props["Capacity"] != "10Gi" {
		t.Errorf("Capacity = %q, want 10Gi", props["Capacity"])
	}
	if props["Status"] != string(corev1.VolumeBound) {
		t.Errorf("Status = %q, want %s", props["Status"], corev1.VolumeBound)
	}
	if props["Storage Class"] != "fast-ssd" {
		t.Errorf("Storage Class = %q, want fast-ssd", props["Storage Class"])
	}
	if props["Reclaim Policy"] != string(corev1.PersistentVolumeReclaimRetain) {
		t.Errorf("Reclaim Policy = %q, want Retain", props["Reclaim Policy"])
	}
	if props["Claim"] != "default/data-claim" {
		t.Errorf("Claim = %q, want default/data-claim", props["Claim"])
	}
	if props["Source"] != "hostPath: /data/pv1" {
		t.Errorf("Source = %q, want hostPath: /data/pv1", props["Source"])
	}
}

func TestGetResourceDetail_PV_CSISource(t *testing.T) {
	pv := &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: "pv-csi"},
		Spec: corev1.PersistentVolumeSpec{
			Capacity:    corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("5Gi")},
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				CSI: &corev1.CSIPersistentVolumeSource{
					Driver:       "ebs.csi.aws.com",
					VolumeHandle: "vol-0abc123",
				},
			},
		},
	}
	c := detailClient(pv)
	got, err := c.GetResourceDetail(context.Background(), "pvs", "", "pv-csi")
	if err != nil {
		t.Fatalf("GetResourceDetail: %v", err)
	}
	src := propsByKey(got.Properties)["Source"]
	if src != "CSI ebs.csi.aws.com (vol-0abc123)" {
		t.Errorf("CSI Source = %q, want 'CSI ebs.csi.aws.com (vol-0abc123)'", src)
	}
}

func TestGetResourceDetail_StorageClass(t *testing.T) {
	allowExpand := true
	reclaim := corev1.PersistentVolumeReclaimDelete
	binding := storagev1.VolumeBindingWaitForFirstConsumer
	sc := &storagev1.StorageClass{
		ObjectMeta: metav1.ObjectMeta{
			Name: "fast-ssd",
			Annotations: map[string]string{
				"storageclass.kubernetes.io/is-default-class": "true",
			},
		},
		Provisioner:          "ebs.csi.aws.com",
		ReclaimPolicy:        &reclaim,
		VolumeBindingMode:    &binding,
		AllowVolumeExpansion: &allowExpand,
		Parameters: map[string]string{
			"type": "gp3",
		},
	}
	c := detailClient(sc)
	got, err := c.GetResourceDetail(context.Background(), "storageclasses", "", "fast-ssd")
	if err != nil {
		t.Fatalf("GetResourceDetail(storageclasses): %v", err)
	}
	if got.Kind != "StorageClass" {
		t.Errorf("Kind = %q, want StorageClass", got.Kind)
	}
	props := propsByKey(got.Properties)
	if props["Provisioner"] != "ebs.csi.aws.com" {
		t.Errorf("Provisioner = %q", props["Provisioner"])
	}
	if props["Reclaim Policy"] != string(reclaim) {
		t.Errorf("Reclaim Policy = %q", props["Reclaim Policy"])
	}
	if props["Volume Binding Mode"] != string(binding) {
		t.Errorf("Volume Binding Mode = %q", props["Volume Binding Mode"])
	}
	if props["Allow Volume Expansion"] != "true" {
		t.Errorf("Allow Volume Expansion = %q", props["Allow Volume Expansion"])
	}
	if props["Default"] != "true" {
		t.Errorf("Default = %q (should reflect is-default-class annotation)", props["Default"])
	}
	if props["param: type"] != "gp3" {
		t.Errorf("param: type = %q", props["param: type"])
	}
}

func propsByKey(props []KeyValue) map[string]string {
	m := make(map[string]string, len(props))
	for _, p := range props {
		m[p.Key] = p.Value
	}
	return m
}
