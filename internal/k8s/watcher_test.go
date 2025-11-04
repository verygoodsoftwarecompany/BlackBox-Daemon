package k8s

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/verygoodsoftwarecompany/blackbox-daemon/internal/ringbuffer"
	"github.com/verygoodsoftwarecompany/blackbox-daemon/pkg/types"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func TestNewWatcher(t *testing.T) {
	t.Run("creates watcher successfully", func(t *testing.T) {
		buffer := ringbuffer.New(100, time.Hour)
		clientset := fake.NewSimpleClientset()

		watcher, err := NewWatcher(clientset, buffer, "test-namespace", []string{"deployments", "pods"})

		if err != nil {
			t.Fatalf("Expected no error creating watcher, got %v", err)
		}
		if watcher == nil {
			t.Fatal("Expected watcher to be created")
		}
		if watcher.namespace != "test-namespace" {
			t.Errorf("Expected namespace 'test-namespace', got %v", watcher.namespace)
		}
		if len(watcher.resources) != 2 {
			t.Errorf("Expected 2 resources, got %v", len(watcher.resources))
		}
	})

	t.Run("handles empty resources list", func(t *testing.T) {
		buffer := ringbuffer.New(100, time.Hour)
		clientset := fake.NewSimpleClientset()

		watcher, err := NewWatcher(clientset, buffer, "default", []string{})

		if err != nil {
			t.Fatalf("Expected no error with empty resources, got %v", err)
		}
		if len(watcher.resources) != 0 {
			t.Errorf("Expected 0 resources, got %v", len(watcher.resources))
		}
	})

	t.Run("validates supported resources", func(t *testing.T) {
		buffer := ringbuffer.New(100, time.Hour)
		clientset := fake.NewSimpleClientset()

		tests := []struct {
			name      string
			resources []string
			expectErr bool
		}{
			{
				name:      "valid resources",
				resources: []string{"pods", "deployments", "services"},
				expectErr: false,
			},
			{
				name:      "invalid resource",
				resources: []string{"pods", "invalid-resource"},
				expectErr: true,
			},
			{
				name:      "mixed valid and invalid",
				resources: []string{"deployments", "badresource", "services"},
				expectErr: true,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, err := NewWatcher(clientset, buffer, "default", tt.resources)

				if tt.expectErr && err == nil {
					t.Error("Expected error for invalid resources")
				}
				if !tt.expectErr && err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			})
		}
	})
}

func TestWatcherStart(t *testing.T) {
	t.Run("starts watching successfully", func(t *testing.T) {
		buffer := ringbuffer.New(100, time.Hour)
		clientset := fake.NewSimpleClientset()

		// Add a reaction to handle the watch request
		clientset.PrependWatchReactor("pods", func(action ktesting.Action) (handled bool, ret watch.Interface, err error) {
			return true, watch.NewEmptyWatch(), nil
		})

		watcher, err := NewWatcher(clientset, buffer, "default", []string{"pods"})
		if err != nil {
			t.Fatalf("Failed to create watcher: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err = watcher.Start(ctx)

		// Should not return error even if context times out
		if err != nil {
			t.Errorf("Expected no error starting watcher, got %v", err)
		}
	})

	t.Run("handles context cancellation", func(t *testing.T) {
		buffer := ringbuffer.New(100, time.Hour)
		clientset := fake.NewSimpleClientset()

		watcher, err := NewWatcher(clientset, buffer, "default", []string{"pods"})
		if err != nil {
			t.Fatalf("Failed to create watcher: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		err = watcher.Start(ctx)

		// Should handle cancellation gracefully
		if err != nil {
			t.Errorf("Expected no error with cancelled context, got %v", err)
		}
	})

	t.Run("handles no resources to watch", func(t *testing.T) {
		buffer := ringbuffer.New(100, time.Hour)
		clientset := fake.NewSimpleClientset()

		watcher, err := NewWatcher(clientset, buffer, "default", []string{})
		if err != nil {
			t.Fatalf("Failed to create watcher: %v", err)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		err = watcher.Start(ctx)

		if err != nil {
			t.Errorf("Expected no error with no resources, got %v", err)
		}
	})
}

func TestWatcherProcessPodEvent(t *testing.T) {
	buffer := ringbuffer.New(100, time.Hour)
	clientset := fake.NewSimpleClientset()

	watcher, err := NewWatcher(clientset, buffer, "default", []string{"pods"})
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}

	tests := []struct {
		name      string
		eventType watch.EventType
		pod       *corev1.Pod
		expectAdd bool
	}{
		{
			name:      "pod added",
			eventType: watch.Added,
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
				},
			},
			expectAdd: true,
		},
		{
			name:      "pod modified",
			eventType: watch.Modified,
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
				Status: corev1.PodStatus{
					Phase: corev1.PodFailed,
				},
			},
			expectAdd: true,
		},
		{
			name:      "pod deleted",
			eventType: watch.Deleted,
			pod: &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pod",
					Namespace: "default",
				},
			},
			expectAdd: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			initialCount := buffer.Count()

			event := watch.Event{
				Type:   tt.eventType,
				Object: tt.pod,
			}

			watcher.processPodEvent(event)

			if tt.expectAdd {
				if buffer.Count() != initialCount+1 {
					t.Errorf("Expected buffer count to increase by 1, got %v -> %v", initialCount, buffer.Count())
				}

				// Verify the entry was added correctly
				entries := buffer.GetAll()
				lastEntry := entries[len(entries)-1]

				if lastEntry.Source != types.SourceKubernetes {
					t.Errorf("Expected source kubernetes, got %v", lastEntry.Source)
				}
				if lastEntry.Type != types.TypeKubernetes {
					t.Errorf("Expected type kubernetes, got %v", lastEntry.Type)
				}
				if lastEntry.Name != "pod_event" {
					t.Errorf("Expected name 'pod_event', got %v", lastEntry.Name)
				}

				// Check tags
				expectedTags := map[string]string{
					"event_type": string(tt.eventType),
					"resource":   "pod",
					"name":       "test-pod",
					"namespace":  "default",
				}

				if tt.eventType != watch.Deleted {
					expectedTags["phase"] = string(tt.pod.Status.Phase)
				}

				if !reflect.DeepEqual(lastEntry.Tags, expectedTags) {
					t.Errorf("Expected tags %v, got %v", expectedTags, lastEntry.Tags)
				}
			}
		})
	}
}

func TestWatcherProcessDeploymentEvent(t *testing.T) {
	buffer := ringbuffer.New(100, time.Hour)
	clientset := fake.NewSimpleClientset()

	watcher, err := NewWatcher(clientset, buffer, "default", []string{"deployments"})
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}

	replicas := int32(3)
	readyReplicas := int32(2)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-deployment",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
		},
		Status: appsv1.DeploymentStatus{
			Replicas:      replicas,
			ReadyReplicas: readyReplicas,
		},
	}

	event := watch.Event{
		Type:   watch.Modified,
		Object: deployment,
	}

	initialCount := buffer.Count()
	watcher.processDeploymentEvent(event)

	if buffer.Count() != initialCount+1 {
		t.Errorf("Expected buffer count to increase by 1")
	}

	entries := buffer.GetAll()
	lastEntry := entries[len(entries)-1]

	expectedTags := map[string]string{
		"event_type":     "MODIFIED",
		"resource":       "deployment",
		"name":           "test-deployment",
		"namespace":      "default",
		"replicas":       "3",
		"ready_replicas": "2",
	}

	if !reflect.DeepEqual(lastEntry.Tags, expectedTags) {
		t.Errorf("Expected tags %v, got %v", expectedTags, lastEntry.Tags)
	}
}

func TestWatcherProcessServiceEvent(t *testing.T) {
	buffer := ringbuffer.New(100, time.Hour)
	clientset := fake.NewSimpleClientset()

	watcher, err := NewWatcher(clientset, buffer, "default", []string{"services"})
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "default",
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Ports: []corev1.ServicePort{
				{
					Name: "http",
					Port: 80,
				},
				{
					Name: "https",
					Port: 443,
				},
			},
		},
	}

	event := watch.Event{
		Type:   watch.Added,
		Object: service,
	}

	initialCount := buffer.Count()
	watcher.processServiceEvent(event)

	if buffer.Count() != initialCount+1 {
		t.Errorf("Expected buffer count to increase by 1")
	}

	entries := buffer.GetAll()
	lastEntry := entries[len(entries)-1]

	expectedTags := map[string]string{
		"event_type": "ADDED",
		"resource":   "service",
		"name":       "test-service",
		"namespace":  "default",
		"type":       "ClusterIP",
		"ports":      "http:80,https:443",
	}

	if !reflect.DeepEqual(lastEntry.Tags, expectedTags) {
		t.Errorf("Expected tags %v, got %v", expectedTags, lastEntry.Tags)
	}
}

func TestWatcherProcessReplicaSetEvent(t *testing.T) {
	buffer := ringbuffer.New(100, time.Hour)
	clientset := fake.NewSimpleClientset()

	watcher, err := NewWatcher(clientset, buffer, "default", []string{"replicasets"})
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}

	replicas := int32(3)
	readyReplicas := int32(2)

	replicaSet := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-rs",
			Namespace: "default",
		},
		Spec: appsv1.ReplicaSetSpec{
			Replicas: &replicas,
		},
		Status: appsv1.ReplicaSetStatus{
			Replicas:      replicas,
			ReadyReplicas: readyReplicas,
		},
	}

	event := watch.Event{
		Type:   watch.Modified,
		Object: replicaSet,
	}

	initialCount := buffer.Count()
	watcher.processReplicaSetEvent(event)

	if buffer.Count() != initialCount+1 {
		t.Errorf("Expected buffer count to increase by 1")
	}

	entries := buffer.GetAll()
	lastEntry := entries[len(entries)-1]

	expectedTags := map[string]string{
		"event_type":     "MODIFIED",
		"resource":       "replicaset",
		"name":           "test-rs",
		"namespace":      "default",
		"replicas":       "3",
		"ready_replicas": "2",
	}

	if !reflect.DeepEqual(lastEntry.Tags, expectedTags) {
		t.Errorf("Expected tags %v, got %v", expectedTags, lastEntry.Tags)
	}
}

func TestWatcherHandleInvalidEvent(t *testing.T) {
	buffer := ringbuffer.New(100, time.Hour)
	clientset := fake.NewSimpleClientset()

	watcher, err := NewWatcher(clientset, buffer, "default", []string{"pods"})
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}

	t.Run("handles nil object", func(t *testing.T) {
		event := watch.Event{
			Type:   watch.Added,
			Object: nil,
		}

		initialCount := buffer.Count()
		watcher.processPodEvent(event)

		// Should not add entry for nil object
		if buffer.Count() != initialCount {
			t.Error("Expected no buffer change for nil object")
		}
	})

	t.Run("handles wrong object type", func(t *testing.T) {
		// Pass a deployment object to pod event processor
		deployment := &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "test-deployment",
			},
		}

		event := watch.Event{
			Type:   watch.Added,
			Object: deployment,
		}

		initialCount := buffer.Count()
		watcher.processPodEvent(event)

		// Should not add entry for wrong object type
		if buffer.Count() != initialCount {
			t.Error("Expected no buffer change for wrong object type")
		}
	})

	t.Run("handles unknown event type", func(t *testing.T) {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-pod",
				Namespace: "default",
			},
		}

		event := watch.Event{
			Type:   "UNKNOWN",
			Object: pod,
		}

		initialCount := buffer.Count()
		watcher.processPodEvent(event)

		// Should still add entry for unknown event type
		if buffer.Count() != initialCount+1 {
			t.Error("Expected buffer count to increase for unknown event type")
		}

		entries := buffer.GetAll()
		lastEntry := entries[len(entries)-1]
		if lastEntry.Tags["event_type"] != "UNKNOWN" {
			t.Errorf("Expected event_type 'UNKNOWN', got %v", lastEntry.Tags["event_type"])
		}
	})
}

func TestFormatPortsString(t *testing.T) {
	tests := []struct {
		name     string
		ports    []corev1.ServicePort
		expected string
	}{
		{
			name:     "empty ports",
			ports:    []corev1.ServicePort{},
			expected: "",
		},
		{
			name: "single port",
			ports: []corev1.ServicePort{
				{
					Name: "http",
					Port: 80,
				},
			},
			expected: "http:80",
		},
		{
			name: "multiple ports",
			ports: []corev1.ServicePort{
				{
					Name: "http",
					Port: 80,
				},
				{
					Name: "https",
					Port: 443,
				},
				{
					Name: "metrics",
					Port: 9090,
				},
			},
			expected: "http:80,https:443,metrics:9090",
		},
		{
			name: "port without name",
			ports: []corev1.ServicePort{
				{
					Port: 8080,
				},
			},
			expected: ":8080",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatPorts(tt.ports)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestWatcherConcurrency(t *testing.T) {
	t.Run("handles concurrent events", func(t *testing.T) {
		buffer := ringbuffer.New(1000, time.Hour)
		clientset := fake.NewSimpleClientset()

		watcher, err := NewWatcher(clientset, buffer, "default", []string{"pods"})
		if err != nil {
			t.Fatalf("Failed to create watcher: %v", err)
		}

		// Create multiple goroutines processing events concurrently
		numEvents := 100
		done := make(chan bool, numEvents)

		for i := 0; i < numEvents; i++ {
			go func(id int) {
				pod := &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod-" + string(rune(id)),
						Namespace: "default",
					},
					Status: corev1.PodStatus{
						Phase: corev1.PodRunning,
					},
				}

				event := watch.Event{
					Type:   watch.Added,
					Object: pod,
				}

				watcher.processPodEvent(event)
				done <- true
			}(i)
		}

		// Wait for all events to be processed
		for i := 0; i < numEvents; i++ {
			<-done
		}

		// Verify all events were processed
		if buffer.Count() != numEvents {
			t.Errorf("Expected %v events in buffer, got %v", numEvents, buffer.Count())
		}
	})
}

func TestWatcherResourceValidation(t *testing.T) {
	buffer := ringbuffer.New(100, time.Hour)
	clientset := fake.NewSimpleClientset()

	supportedResources := []string{"pods", "deployments", "services", "replicasets"}

	for _, resource := range supportedResources {
		t.Run("supports "+resource, func(t *testing.T) {
			_, err := NewWatcher(clientset, buffer, "default", []string{resource})
			if err != nil {
				t.Errorf("Expected %s to be supported, got error: %v", resource, err)
			}
		})
	}

	unsupportedResources := []string{"configmaps", "secrets", "nodes", "namespaces"}

	for _, resource := range unsupportedResources {
		t.Run("rejects "+resource, func(t *testing.T) {
			_, err := NewWatcher(clientset, buffer, "default", []string{resource})
			if err == nil {
				t.Errorf("Expected %s to be rejected", resource)
			}
		})
	}
}

func TestWatcherEventFiltering(t *testing.T) {
	buffer := ringbuffer.New(100, time.Hour)
	clientset := fake.NewSimpleClientset()

	watcher, err := NewWatcher(clientset, buffer, "default", []string{"pods"})
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}

	// Test that all event types are processed
	eventTypes := []watch.EventType{
		watch.Added,
		watch.Modified,
		watch.Deleted,
		watch.Bookmark,
		watch.Error,
	}

	for _, eventType := range eventTypes {
		t.Run("processes "+string(eventType)+" events", func(t *testing.T) {
			initialCount := buffer.Count()

			var object runtime.Object
			if eventType == watch.Error {
				// Error events typically don't have a valid object
				object = nil
			} else {
				object = &corev1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-pod",
						Namespace: "default",
					},
				}
			}

			event := watch.Event{
				Type:   eventType,
				Object: object,
			}

			watcher.processPodEvent(event)

			if eventType == watch.Error || object == nil {
				// Error events or nil objects should not add entries
				if buffer.Count() != initialCount {
					t.Errorf("Expected no buffer change for %s event with nil object", eventType)
				}
			} else {
				// Other events should add entries
				if buffer.Count() != initialCount+1 {
					t.Errorf("Expected buffer count to increase for %s event", eventType)
				}
			}
		})
	}
}