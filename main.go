package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/component-base/featuregate"
	"k8s.io/utils/pointer"
)

func main() {
	// Enable the WatchListClient feature gate
	featureGate := featuregate.NewFeatureGate()
	// Register the WatchListClient feature gate
	err := featureGate.Add(map[featuregate.Feature]featuregate.FeatureSpec{
		"WatchListClient": {Default: true}, // The feature is enabled by default in 1.32
	})
	if err != nil {
		fmt.Printf("Failed to add feature gate: %v\n", err)
		return
	}

	// Enable the feature
	err = featureGate.SetFromMap(map[string]bool{
		"WatchListClient": true,
	})
	if err != nil {
		fmt.Printf("Failed to set feature gates: %v\n", err)
		return
	}

	// Verify feature gate is enabled
	fmt.Printf("WatchListClient feature gate enabled: %v\n", featureGate.Enabled("WatchListClient"))

	// Get the default kubeconfig path for Kind
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Failed to get home directory: %v\n", err)
		return
	}
	kubeconfig := filepath.Join(homeDir, ".kube", "config")

	// Create the client config
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		fmt.Printf("Failed to create config: %v\n", err)
		return
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		fmt.Printf("Failed to create clientset: %v\n", err)
		return
	}

	fmt.Println("Connected to Kind cluster successfully")
	listPodsUsingWatch(clientset, "default")
}

func listPodsUsingWatch(clientset *kubernetes.Clientset, namespace string) {
	fmt.Printf("Starting to watch pods in namespace: %s\n", namespace)

	// Create a watch on pods with sendInitialEvents=true
	watchOptions := metav1.ListOptions{
		SendInitialEvents:    pointer.Bool(true), // Request the initial list via watch
		ResourceVersionMatch: "NotOlderThan",
		AllowWatchBookmarks:  true, // Enable bookmark events
	}
	fmt.Printf("Watch options: %+v\n", watchOptions)

	watcher, err := clientset.CoreV1().Pods(namespace).Watch(context.Background(), watchOptions)
	if err != nil {
		fmt.Printf("Error creating watcher: %v\n", err)
		return
	}
	defer watcher.Stop()

	// Process the watch events
	for event := range watcher.ResultChan() {
		fmt.Printf("Received event type: %s\n", event.Type)

		// Handle bookmark events separately
		if event.Type == watch.Bookmark {
			fmt.Printf("Received bookmark event\n")
			if pod, ok := event.Object.(*v1.Pod); ok {
				annotations := pod.GetAnnotations()
				fmt.Printf("Bookmark annotations: %+v\n", annotations)
				if annotations != nil && annotations["k8s.io/initial-events-end"] == "true" {
					fmt.Println("Initial pod list complete, now watching for changes")
				}
			}
			continue
		}

		// Handle pod events
		pod, ok := event.Object.(*v1.Pod)
		if !ok {
			fmt.Printf("Received non-pod object of type %T\n", event.Object)
			continue
		}

		// Process the pod based on the event type
		switch event.Type {
		case watch.Added:
			fmt.Printf("Pod added: %s (Phase: %s)\n", pod.Name, pod.Status.Phase)
		case watch.Modified:
			fmt.Printf("Pod modified: %s (Phase: %s)\n", pod.Name, pod.Status.Phase)
		case watch.Deleted:
			fmt.Printf("Pod deleted: %s\n", pod.Name)
		case watch.Error:
			fmt.Printf("Error event received: %v\n", event.Object)
		default:
			fmt.Printf("Unknown event type: %s for pod: %s\n", event.Type, pod.Name)
		}
	}
}
