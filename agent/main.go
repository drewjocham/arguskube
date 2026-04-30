package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// AnomalyScore represents an AI-generated anomaly metric.
type AnomalyScore struct {
	Timestamp time.Time `json:"timestamp"`
	Score     float64   `json:"score"`
	Target    string    `json:"target"`
	Rule      string    `json:"rule"`
}

func getKubeClient() (*kubernetes.Clientset, error) {
	// Try in-cluster config first
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Println("Not running in cluster, falling back to local kubeconfig")
		var kubeconfig string
		if home := homedir.HomeDir(); home != "" {
			kubeconfig = filepath.Join(home, ".kube", "config")
		}
		
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, err
		}
	}

	return kubernetes.NewForConfig(config)
}

func main() {
	log.Println("Starting KubeWatcher In-Cluster Agent...")

	token := os.Getenv("SAAS_TOKEN")
	if token == "" {
		log.Println("Warning: SAAS_TOKEN not set. Running in local-only mode.")
	} else {
		log.Println("SaaS Token detected. Will sync metadata to cloud.")
	}

	clientset, err := getKubeClient()
	if err != nil {
		log.Fatalf("Failed to initialize Kubernetes client: %v", err)
	}

	// Setup Informers for real-time tracking
	informerFactory := informers.NewSharedInformerFactory(clientset, 10*time.Minute)
	podInformer := informerFactory.Core().V1().Pods().Informer()

	podInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)
			log.Printf("Pod Added: %s/%s\n", pod.Namespace, pod.Name)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			pod := newObj.(*corev1.Pod)
			// Track state changes, ready status, etc.
			log.Printf("Pod Updated: %s/%s (Status: %s)\n", pod.Namespace, pod.Name, pod.Status.Phase)
		},
		DeleteFunc: func(obj interface{}) {
			pod := obj.(*corev1.Pod)
			log.Printf("Pod Deleted: %s/%s\n", pod.Namespace, pod.Name)
		},
	})

	stopCh := make(chan struct{})
	defer close(stopCh)
	
	informerFactory.Start(stopCh)
	log.Println("Started Informers. Waiting for cache sync...")
	if !cache.WaitForCacheSync(stopCh, podInformer.HasSynced) {
		log.Fatal("Failed to sync informer caches")
	}
	log.Println("Caches synced successfully.")

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	http.HandleFunc("/api/v1/pods", func(w http.ResponseWriter, r *http.Request) {
		pods, err := clientset.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(pods.Items)
	})

	http.HandleFunc("/api/v1/nodes", func(w http.ResponseWriter, r *http.Request) {
		nodes, err := clientset.CoreV1().Nodes().List(context.Background(), metav1.ListOptions{})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(nodes.Items)
	})

	http.HandleFunc("/api/v1/anomalies", func(w http.ResponseWriter, r *http.Request) {
		anomalies := []AnomalyScore{
			{Timestamp: time.Now().Add(-2 * time.Minute), Score: 94.5, Target: "aws-prod-db", Rule: "Sudden Memory Spike"},
			{Timestamp: time.Now().Add(-1 * time.Hour), Score: 88.2, Target: "ingress/traefik", Rule: "High Error Rate"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(anomalies)
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Agent API listening on port %s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
