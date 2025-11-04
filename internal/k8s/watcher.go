// Package k8s provides Kubernetes integration for monitoring pods and detecting crashes.
// It uses the Kubernetes API to watch pod events and report incidents when crashes occur.
package k8s

import (
	"context"
	"fmt"
	"time"

	"github.com/verygoodsoftwarecompany/blackbox-daemon/pkg/types"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// PodWatcher monitors pods on the current node and detects crashes by watching
// Kubernetes pod events and analyzing container exit codes and restart patterns.
type PodWatcher struct {
	clientset    kubernetes.Interface
	nodeName     string
	eventHandler EventHandler
}

// EventHandler defines the interface for handling pod events and lifecycle changes.
type EventHandler interface {
	OnPodCrash(report types.IncidentReport)
	OnPodStart(pod *corev1.Pod)
	OnPodStop(pod *corev1.Pod)
}

// NewPodWatcher creates a new Kubernetes pod watcher that monitors pods on the specified node.
// It supports both in-cluster configuration and external kubeconfig files.
func NewPodWatcher(kubeConfig, nodeName string, eventHandler EventHandler) (*PodWatcher, error) {
	var config *rest.Config
	var err error

	if kubeConfig != "" {
		config, err = clientcmd.BuildConfigFromFlags("", kubeConfig)
	} else {
		config, err = rest.InClusterConfig()
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes clientset: %w", err)
	}

	return &PodWatcher{
		clientset:    clientset,
		nodeName:     nodeName,
		eventHandler: eventHandler,
	}, nil
}

// Start begins monitoring pods on the node, synchronizing initial state and watching
// for pod events until the context is cancelled.
func (pw *PodWatcher) Start(ctx context.Context) error {
	// Get initial list of pods on this node
	if err := pw.syncInitialPods(ctx); err != nil {
		return fmt.Errorf("failed to sync initial pods: %w", err)
	}

	// Watch for pod events
	fieldSelector := fields.OneTermEqualSelector("spec.nodeName", pw.nodeName).String()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			if err := pw.watchPods(ctx, fieldSelector); err != nil {
				fmt.Printf("Pod watcher error (retrying): %v\n", err)
				time.Sleep(5 * time.Second)
				continue
			}
		}
	}
}

// syncInitialPods gets the current state of pods on this node and notifies the
// event handler of any running pods to establish initial state.
func (pw *PodWatcher) syncInitialPods(ctx context.Context) error {
	fieldSelector := fields.OneTermEqualSelector("spec.nodeName", pw.nodeName).String()

	pods, err := pw.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
	})
	if err != nil {
		return err
	}

	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodRunning {
			pw.eventHandler.OnPodStart(&pod)
		}
	}

	return nil
}

// watchPods watches for pod events on this node using the Kubernetes watch API
// and processes add, modify, and delete events.
func (pw *PodWatcher) watchPods(ctx context.Context, fieldSelector string) error {
	watcher, err := pw.clientset.CoreV1().Pods("").Watch(ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
		Watch:         true,
	})
	if err != nil {
		return err
	}
	defer watcher.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return fmt.Errorf("watch channel closed")
			}

			pod, ok := event.Object.(*corev1.Pod)
			if !ok {
				continue
			}

			switch event.Type {
			case watch.Added, watch.Modified:
				pw.handlePodEvent(pod)
			case watch.Deleted:
				pw.eventHandler.OnPodStop(pod)
			}
		}
	}
}

// handlePodEvent processes pod status changes and generates incident reports
// for failed pods or crashed containers.
func (pw *PodWatcher) handlePodEvent(pod *corev1.Pod) {
	switch pod.Status.Phase {
	case corev1.PodRunning:
		pw.eventHandler.OnPodStart(pod)

	case corev1.PodFailed:
		// Pod has failed - create incident report
		report := types.IncidentReport{
			ID:        fmt.Sprintf("pod-crash-%s-%d", pod.Name, time.Now().Unix()),
			Timestamp: time.Now(),
			PodName:   pod.Name,
			Namespace: pod.Namespace,
			Severity:  types.SeverityCritical,
			Type:      types.IncidentCrash,
			Message:   fmt.Sprintf("Pod %s/%s failed with phase: %s", pod.Namespace, pod.Name, pod.Status.Phase),
			Context: map[string]interface{}{
				"reason":  pod.Status.Reason,
				"message": pod.Status.Message,
				"phase":   string(pod.Status.Phase),
			},
		}
		pw.eventHandler.OnPodCrash(report)

	case corev1.PodSucceeded:
		// Pod completed successfully
		pw.eventHandler.OnPodStop(pod)
	}

	// Check container statuses for crashes
	pw.checkContainerStatuses(pod)
}

// checkContainerStatuses examines individual container statuses for crashes
func (pw *PodWatcher) checkContainerStatuses(pod *corev1.Pod) {
	for _, containerStatus := range pod.Status.ContainerStatuses {
		// Check for restarts indicating crashes
		if containerStatus.RestartCount > 0 && containerStatus.State.Running != nil {
			// Container has been restarted
			var reason, message string
			if containerStatus.LastTerminationState.Terminated != nil {
				reason = containerStatus.LastTerminationState.Terminated.Reason
				message = containerStatus.LastTerminationState.Terminated.Message
			}

			var incidentType types.IncidentType = types.IncidentCrash
			var severity types.IncidentSeverity = types.SeverityHigh

			// Detect OOM kills
			if reason == "OOMKilled" {
				incidentType = types.IncidentOOM
				severity = types.SeverityCritical
			}

			report := types.IncidentReport{
				ID:          fmt.Sprintf("container-restart-%s-%s-%d", pod.Name, containerStatus.Name, time.Now().Unix()),
				Timestamp:   time.Now(),
				PodName:     pod.Name,
				Namespace:   pod.Namespace,
				ContainerID: containerStatus.ContainerID,
				Severity:    severity,
				Type:        incidentType,
				Message:     fmt.Sprintf("Container %s in pod %s/%s restarted (count: %d)", containerStatus.Name, pod.Namespace, pod.Name, containerStatus.RestartCount),
				Context: map[string]interface{}{
					"container_name": containerStatus.Name,
					"restart_count":  containerStatus.RestartCount,
					"exit_code":      containerStatus.LastTerminationState.Terminated.ExitCode,
					"reason":         reason,
					"message":        message,
					"started_at":     containerStatus.State.Running.StartedAt,
				},
			}

			pw.eventHandler.OnPodCrash(report)
		}

		// Check for currently failed containers
		if containerStatus.State.Terminated != nil && containerStatus.State.Terminated.ExitCode != 0 {
			var incidentType types.IncidentType = types.IncidentCrash
			var severity types.IncidentSeverity = types.SeverityHigh

			if containerStatus.State.Terminated.Reason == "OOMKilled" {
				incidentType = types.IncidentOOM
				severity = types.SeverityCritical
			}

			report := types.IncidentReport{
				ID:          fmt.Sprintf("container-failed-%s-%s-%d", pod.Name, containerStatus.Name, time.Now().Unix()),
				Timestamp:   time.Now(),
				PodName:     pod.Name,
				Namespace:   pod.Namespace,
				ContainerID: containerStatus.ContainerID,
				Severity:    severity,
				Type:        incidentType,
				Message:     fmt.Sprintf("Container %s in pod %s/%s failed with exit code %d", containerStatus.Name, pod.Namespace, pod.Name, containerStatus.State.Terminated.ExitCode),
				Context: map[string]interface{}{
					"container_name": containerStatus.Name,
					"exit_code":      containerStatus.State.Terminated.ExitCode,
					"reason":         containerStatus.State.Terminated.Reason,
					"message":        containerStatus.State.Terminated.Message,
					"finished_at":    containerStatus.State.Terminated.FinishedAt,
				},
			}

			pw.eventHandler.OnPodCrash(report)
		}
	}
}

// GetPodsOnNode returns all pods currently running on this node
func (pw *PodWatcher) GetPodsOnNode(ctx context.Context) ([]*corev1.Pod, error) {
	fieldSelector := fields.OneTermEqualSelector("spec.nodeName", pw.nodeName).String()

	pods, err := pw.clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
	})
	if err != nil {
		return nil, err
	}

	result := make([]*corev1.Pod, len(pods.Items))
	for i := range pods.Items {
		result[i] = &pods.Items[i]
	}

	return result, nil
}
