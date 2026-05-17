package cd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
)

type Applier struct {
	logger        *slog.Logger
	dynamicClient dynamic.Interface
	restMapper    meta.RESTMapper
	fieldManager  string
}

type ApplierOptions struct {
	Logger        *slog.Logger
	DynamicClient dynamic.Interface
	RESTMapper    meta.RESTMapper
	FieldManager  string
}

func NewApplier(opts ApplierOptions) *Applier {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	if opts.FieldManager == "" {
		opts.FieldManager = "arguscd"
	}

	return &Applier{
		logger:        opts.Logger,
		dynamicClient: opts.DynamicClient,
		restMapper:    opts.RESTMapper,
		fieldManager:  opts.FieldManager,
	}
}

func (a *Applier) ApplyManifest(ctx context.Context, yamlContent []byte) error {
	a.logger.Info("Applying manifest via Server-Side Apply", slog.Int("bytes", len(yamlContent)))

	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(yamlContent), 4096)
	forceApply := true

	for {
		var rawObj map[string]interface{}
		err := decoder.Decode(&rawObj)
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to decode YAML document: %w", err)
		}
		if len(rawObj) == 0 {
			continue
		}

		obj := &unstructured.Unstructured{Object: rawObj}
		gvk := obj.GroupVersionKind()

		mapping, err := a.restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			return fmt.Errorf("failed to find REST mapping for %s: %w", gvk.String(), err)
		}

		var dr dynamic.ResourceInterface
		if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
			namespace := obj.GetNamespace()
			if namespace == "" {
				namespace = "default"
			}
			dr = a.dynamicClient.Resource(mapping.Resource).Namespace(namespace)
		} else {
			dr = a.dynamicClient.Resource(mapping.Resource)
		}

		patchData, err := json.Marshal(obj.Object)
		if err != nil {
			return fmt.Errorf("failed to marshal object for patching: %w", err)
		}

		_, err = dr.Patch(ctx, obj.GetName(), types.ApplyPatchType, patchData, metav1.PatchOptions{
			FieldManager: a.fieldManager,
			Force:        &forceApply,
		})
		if err != nil {
			return fmt.Errorf("failed to apply %s/%s: %w", obj.GetKind(), obj.GetName(), err)
		}

		a.logger.Debug("Successfully applied resource",
			slog.String("kind", obj.GetKind()),
			slog.String("name", obj.GetName()),
		)
	}

	a.logger.Info("Manifest application complete")
	return nil
}
