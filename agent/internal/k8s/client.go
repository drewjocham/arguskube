package k8s

import (
	"context"
	"log/slog"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	clientset       *kubernetes.Clientset
	informerFactory informers.SharedInformerFactory
	logger          *slog.Logger
}

func NewClient(ctx context.Context, logger *slog.Logger) (*Client, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		logger.Info("not running in cluster, falling back to local kubeconfig")
		loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
		configOverrides := &clientcmd.ConfigOverrides{}
		config, err = clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides).ClientConfig()
		if err != nil {
			return nil, err
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	factory := informers.NewSharedInformerFactory(clientset, 10*time.Minute)

	return &Client{
		clientset:       clientset,
		informerFactory: factory,
		logger:          logger,
	}, nil
}

func (c *Client) StartInformers(ctx context.Context) error {
	podInformer := c.informerFactory.Core().V1().Pods().Informer()
	nodeInformer := c.informerFactory.Core().V1().Nodes().Informer()
	svcInformer := c.informerFactory.Core().V1().Services().Informer()
	depInformer := c.informerFactory.Apps().V1().Deployments().Informer()
	eventInformer := c.informerFactory.Core().V1().Events().Informer()

	_, err := podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)
			c.logger.Debug("pod added", "namespace", pod.Namespace, "name", pod.Name)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			pod := newObj.(*corev1.Pod)
			c.logger.Debug("pod updated", "namespace", pod.Namespace, "name", pod.Name, "status", pod.Status.Phase)
		},
		DeleteFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)
			c.logger.Debug("pod deleted", "namespace", pod.Namespace, "name", pod.Name)
		},
	})
	if err != nil {
		return err
	}

	c.informerFactory.Start(ctx.Done())
	c.logger.Info("started informers, waiting for cache sync")

	if !cache.WaitForCacheSync(ctx.Done(), podInformer.HasSynced, nodeInformer.HasSynced, svcInformer.HasSynced, depInformer.HasSynced, eventInformer.HasSynced) {
		return context.DeadlineExceeded
	}

	c.logger.Info("caches synced successfully")
	return nil
}

func (c *Client) GetClientset() *kubernetes.Clientset {
	return c.clientset
}

func (c *Client) GetInformerFactory() informers.SharedInformerFactory {
	return c.informerFactory
}

func (c *Client) GetPods() []corev1.Pod {
	ptrs, err := c.informerFactory.Core().V1().Pods().Lister().List(labels.Everything())
	if err != nil {
		c.logger.Error("failed to list pods from cache", "error", err)
		return nil
	}
	out := make([]corev1.Pod, len(ptrs))
	for i, p := range ptrs {
		out[i] = *p
	}
	return out
}

func (c *Client) GetNodes() []corev1.Node {
	ptrs, err := c.informerFactory.Core().V1().Nodes().Lister().List(labels.Everything())
	if err != nil {
		c.logger.Error("failed to list nodes from cache", "error", err)
		return nil
	}
	out := make([]corev1.Node, len(ptrs))
	for i, p := range ptrs {
		out[i] = *p
	}
	return out
}

func (c *Client) GetServices() []corev1.Service {
	ptrs, err := c.informerFactory.Core().V1().Services().Lister().List(labels.Everything())
	if err != nil {
		c.logger.Error("failed to list services from cache", "error", err)
		return nil
	}
	out := make([]corev1.Service, len(ptrs))
	for i, p := range ptrs {
		out[i] = *p
	}
	return out
}

func (c *Client) GetDeployments() []appsv1.Deployment {
	ptrs, err := c.informerFactory.Apps().V1().Deployments().Lister().List(labels.Everything())
	if err != nil {
		c.logger.Error("failed to list deployments from cache", "error", err)
		return nil
	}
	out := make([]appsv1.Deployment, len(ptrs))
	for i, p := range ptrs {
		out[i] = *p
	}
	return out
}

func (c *Client) GetEvents() []corev1.Event {
	ptrs, err := c.informerFactory.Core().V1().Events().Lister().List(labels.Everything())
	if err != nil {
		c.logger.Error("failed to list events from cache", "error", err)
		return nil
	}
	out := make([]corev1.Event, len(ptrs))
	for i, p := range ptrs {
		out[i] = *p
	}
	return out
}
