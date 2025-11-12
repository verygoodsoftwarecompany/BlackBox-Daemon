# Kubernetes Integration Component

## Overview

The Kubernetes Integration component provides native pod monitoring and crash detection by integrating with the Kubernetes API server. It watches for pod events on the current node, detects crashes and failures, and triggers incident reporting with comprehensive telemetry data.

## Architecture

### Kubernetes API Integration
- **Client-Go Library**: Official Kubernetes client for Go applications
- **Watch API**: Efficient event streaming for real-time pod monitoring  
- **In-Cluster Configuration**: Automatic authentication using service account
- **RBAC Integration**: Minimal required permissions for security
- **Node-Scoped Monitoring**: Only monitors pods on the same node

### Event Processing Pipeline
1. **Pod Discovery**: Initial synchronization of running pods
2. **Event Streaming**: Continuous monitoring of pod lifecycle events
3. **Crash Detection**: Analysis of pod status and container exit codes
4. **Incident Generation**: Creation of detailed incident reports
5. **Telemetry Correlation**: Association with system and sidecar data

## Key Features

### Pod Lifecycle Monitoring
**Events Monitored**:
- **Pod Start**: New pods starting on the node
- **Pod Running**: Successful pod startup and health checks
- **Pod Failed**: Pod crashes, OOM kills, image pull failures  
- **Pod Deleted**: Graceful or forced pod termination
- **Container Restart**: Individual container restarts within pods

**Status Analysis**:
- **Exit Codes**: Container exit status interpretation
- **Restart Counts**: Tracking container restart patterns
- **Resource Limits**: OOM kill detection from resource constraints
- **Image Issues**: Failed image pulls and startup errors

### Crash Detection Logic
```go
func (pw *PodWatcher) handlePodEvent(pod *corev1.Pod) {
    switch pod.Status.Phase {
    case corev1.PodFailed:
        // Pod has failed - analyze containers for root cause
        incident := analyzeFailedPod(pod)
        pw.eventHandler.OnPodCrash(incident)
        
    case corev1.PodRunning:
        // Check for container restarts indicating crashes
        for _, container := range pod.Status.ContainerStatuses {
            if container.RestartCount > previousCount {
                incident := analyzeContainerRestart(pod, container)
                pw.eventHandler.OnPodCrash(incident)
            }
        }
    }
}
```

### Incident Report Generation
When crashes are detected, detailed incident reports are created:

```go
type IncidentReport struct {
    ID          string                 // Unique incident identifier
    Timestamp   time.Time             // When incident was detected  
    Severity    IncidentSeverity      // Critical, High, Medium, Low
    Type        IncidentType          // Crash, OOM, Timeout, etc.
    Message     string                // Human-readable description
    PodName     string                // Affected pod name
    Namespace   string                // Kubernetes namespace
    NodeName    string                // Node where incident occurred
    Containers  []ContainerInfo       // Container-specific details
    Metadata    map[string]interface{} // Additional context
}
```

## Implementation Details

### PodWatcher Structure
```go
type PodWatcher struct {
    clientset    kubernetes.Interface  // Kubernetes API client
    nodeName     string               // Current node name
    eventHandler EventHandler         // Incident processing interface
}
```

### Kubernetes Client Configuration
```go
func NewPodWatcher(kubeConfig, nodeName string, eventHandler EventHandler) (*PodWatcher, error) {
    var config *rest.Config
    var err error
    
    if kubeConfig != "" {
        // External kubeconfig file (development)
        config, err = clientcmd.BuildConfigFromFlags("", kubeConfig)
    } else {
        // In-cluster configuration (production)
        config, err = rest.InClusterConfig()
    }
    
    clientset, err := kubernetes.NewForConfig(config)
    return &PodWatcher{
        clientset:    clientset,
        nodeName:     nodeName,
        eventHandler: eventHandler,
    }
}
```

### Event Streaming Implementation
```go
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
```

## Crash Detection Patterns

### Exit Code Analysis
```go
func analyzeExitCode(exitCode int32) (IncidentType, IncidentSeverity) {
    switch exitCode {
    case 0:
        return IncidentTypeGraceful, SeverityLow
    case 1:
        return IncidentTypeError, SeverityMedium
    case 2:
        return IncidentTypeMisuse, SeverityMedium
    case 125:
        return IncidentTypeDockerError, SeverityHigh
    case 126:
        return IncidentTypeNotExecutable, SeverityHigh
    case 127:
        return IncidentTypeNotFound, SeverityHigh
    case 128:
        return IncidentTypeInvalidArg, SeverityMedium
    case 130:
        return IncidentTypeInterrupted, SeverityLow
    case 137:
        return IncidentTypeKilled, SeverityHigh  // SIGKILL (often OOM)
    case 143:
        return IncidentTypeTerminated, SeverityMedium // SIGTERM
    default:
        if exitCode > 128 {
            return IncidentTypeSignal, SeverityMedium
        }
        return IncidentTypeUnknown, SeverityMedium
    }
}
```

### OOM Kill Detection
```go
func detectOOMKill(pod *corev1.Pod, container corev1.ContainerStatus) bool {
    // Check exit code 137 (SIGKILL)
    if container.LastTerminationState.Terminated != nil {
        if container.LastTerminationState.Terminated.ExitCode == 137 {
            return true
        }
        
        // Check termination reason
        reason := container.LastTerminationState.Terminated.Reason
        if reason == "OOMKilled" {
            return true
        }
    }
    
    // Check container state reason
    if container.State.Waiting != nil {
        if container.State.Waiting.Reason == "OOMKilled" {
            return true
        }
    }
    
    return false
}
```

### Resource Constraint Analysis
```go
func analyzeResourceConstraints(pod *corev1.Pod) ResourceAnalysis {
    analysis := ResourceAnalysis{}
    
    for _, container := range pod.Spec.Containers {
        if container.Resources.Limits != nil {
            // Memory limits
            if memLimit := container.Resources.Limits.Memory(); memLimit != nil {
                analysis.MemoryLimitBytes = memLimit.Value()
            }
            
            // CPU limits  
            if cpuLimit := container.Resources.Limits.Cpu(); cpuLimit != nil {
                analysis.CPULimitMillicores = cpuLimit.MilliValue()
            }
        }
        
        if container.Resources.Requests != nil {
            // Memory requests
            if memReq := container.Resources.Requests.Memory(); memReq != nil {
                analysis.MemoryRequestBytes = memReq.Value()
            }
            
            // CPU requests
            if cpuReq := container.Resources.Requests.Cpu(); cpuReq != nil {
                analysis.CPURequestMillicores = cpuReq.MilliValue()
            }
        }
    }
    
    return analysis
}
```

## RBAC Configuration

### Required Permissions
The component requires minimal Kubernetes permissions:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: blackbox-daemon
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list", "watch"]
- apiGroups: [""]  
  resources: ["events"]
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources: ["nodes"] 
  verbs: ["get", "list"]
```

### Service Account Setup
```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: blackbox-daemon
  namespace: kube-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: blackbox-daemon
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: blackbox-daemon
subjects:
- kind: ServiceAccount
  name: blackbox-daemon
  namespace: kube-system
```

### Security Considerations
- **Read-Only Access**: Only requires read permissions
- **Node Scoping**: Filters pods by node name automatically
- **No Cluster Admin**: Doesn't require cluster administrator privileges
- **Minimal Surface**: Limited API surface reduces security risk

## Configuration

### Environment Variables
```bash
NODE_NAME=<node-name>                    # Kubernetes node name (required)
POD_NAMESPACE=<namespace>                # Current pod namespace
KUBECONFIG=/path/to/kubeconfig           # External config (optional)
BLACKBOX_K8S_TIMEOUT=30s                 # API client timeout
BLACKBOX_K8S_RETRY_INTERVAL=5s           # Retry interval on API failures
```

### DaemonSet Configuration
```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: blackbox-daemon
spec:
  template:
    spec:
      serviceAccountName: blackbox-daemon
      containers:
      - name: blackbox-daemon
        env:
        - name: NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
```

## Integration with Incident System

### Event Handler Interface
```go
type EventHandler interface {
    OnPodCrash(report IncidentReport)     // Pod crash detected
    OnPodStart(pod *corev1.Pod)          // Pod started successfully  
    OnPodStop(pod *corev1.Pod)           // Pod stopped/deleted
}
```

### Incident Processing Flow
1. **Event Detection**: Kubernetes API event received
2. **Status Analysis**: Pod and container status evaluation
3. **Incident Creation**: Generate structured incident report
4. **Telemetry Correlation**: Gather related system/sidecar data
5. **Report Generation**: Format and output incident report

### Telemetry Correlation
When incidents are detected, the system correlates:
- **System Metrics**: CPU, memory, disk, network data from time window
- **Sidecar Telemetry**: Application-specific metrics if available  
- **Kubernetes Events**: Related cluster events and warnings
- **Resource Usage**: Pod resource consumption patterns

## Performance Characteristics

### Resource Usage
- **Memory**: ~10-30MB for Kubernetes client and event processing
- **CPU**: <50m (0.05 CPU cores) for event processing
- **Network**: Minimal (only Kubernetes API calls)
- **Storage**: No persistent storage required

### Scalability
- **Node Scope**: Each daemon instance monitors one node only
- **Event Volume**: Handles 100+ pod events/second per node
- **API Efficiency**: Uses watch streams to minimize API calls
- **Memory Bounded**: Event processing has constant memory usage

### Reliability
- **Reconnection**: Automatic reconnection on API server failures
- **Retry Logic**: Exponential backoff for transient errors
- **Graceful Degradation**: Continues operating with reduced functionality
- **Error Isolation**: API failures don't affect other components

## Monitoring and Observability

### Kubernetes Integration Metrics
```promql
# Pod monitoring metrics
k8s_pods_watched_total                    # Total pods being monitored
k8s_pod_events_total{type="crash"}        # Pod crash events detected  
k8s_pod_events_total{type="start"}        # Pod start events
k8s_pod_events_total{type="stop"}         # Pod stop events

# API client metrics
k8s_api_requests_total{verb="watch"}      # Kubernetes API watch requests
k8s_api_request_duration_seconds          # API request latency
k8s_api_errors_total{code="403"}          # Permission denied errors
k8s_api_errors_total{code="504"}          # API server timeout errors

# Watch stream health
k8s_watch_reconnections_total             # Watch stream reconnection count
k8s_watch_events_total                    # Total events received
k8s_watch_last_event_timestamp            # Timestamp of last event
```

### Health Indicators  
- **API Connectivity**: Successful API server communication
- **Event Freshness**: Recency of last received Kubernetes event
- **Watch Stream Status**: Health of event streaming connection
- **Permission Status**: RBAC permission validation

### Alerting Rules
```yaml
groups:
- name: blackbox-k8s-integration
  rules:
  - alert: KubernetesAPIDown
    expr: up{job="blackbox-daemon"} == 0
    for: 2m
    
  - alert: NoKubernetesEvents
    expr: time() - k8s_watch_last_event_timestamp > 300
    for: 5m
    
  - alert: HighPodCrashRate  
    expr: rate(k8s_pod_events_total{type="crash"}[5m]) > 0.1
    for: 2m
```

## Troubleshooting

### Common Issues

1. **RBAC Permission Denied**
   ```bash
   # Check service account permissions
   kubectl auth can-i get pods --as=system:serviceaccount:kube-system:blackbox-daemon
   kubectl auth can-i watch pods --as=system:serviceaccount:kube-system:blackbox-daemon
   
   # Verify RBAC configuration
   kubectl describe clusterrolebinding blackbox-daemon
   ```

2. **API Server Connection Issues**
   ```bash
   # Test API connectivity from pod
   kubectl exec <blackbox-pod> -- wget -qO- https://kubernetes.default.svc/api/v1
   
   # Check service account token
   kubectl exec <blackbox-pod> -- cat /var/run/secrets/kubernetes.io/serviceaccount/token
   ```

3. **No Pod Events Detected**
   ```bash
   # Verify node name configuration  
   kubectl exec <blackbox-pod> -- printenv NODE_NAME
   kubectl get nodes
   
   # Check pod filtering
   kubectl get pods --field-selector spec.nodeName=<node-name>
   ```

4. **Watch Stream Failures**
   ```bash
   # Check for network policies blocking API access
   kubectl get networkpolicy -A
   
   # Monitor API server logs for errors
   kubectl logs -n kube-system -l component=kube-apiserver --tail=100
   ```

### Debug Commands
```bash
# Test Kubernetes API access
kubectl exec <blackbox-pod> -- curl -k -H "Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" \
  https://kubernetes.default.svc/api/v1/pods?fieldSelector=spec.nodeName=<node-name>

# Check event stream  
kubectl get events --field-selector involvedObject.kind=Pod,source.host=<node-name> --watch

# Verify pod watch functionality
kubectl get pods -w --field-selector spec.nodeName=<node-name>

# Monitor component logs
kubectl logs -n kube-system -l app=blackbox-daemon --tail=100 -f | grep k8s
```

## Testing

### Unit Test Coverage

The Kubernetes integration includes comprehensive unit tests with **69.4% coverage**, utilizing fake Kubernetes clients for reliable testing without requiring a real cluster.

**Test Categories**:
- **Pod Lifecycle Tests**: Validation of pod creation, running, failed, and succeeded events
- **Container Crash Detection**: Testing of restart detection, OOM kill handling, and failed containers
- **Error Handling**: Verification of nil pointer safety, invalid inputs, and edge cases
- **Concurrency Tests**: Thread-safe operation validation with concurrent pod events
- **Integration Tests**: Watch API integration using fake Kubernetes clientsets

**Key Testing Features**:
```go
// Thread-safe mock event handler for testing
type mockEventHandler struct {
    mu           sync.RWMutex
    crashReports []types.IncidentReport
    startedPods  []*corev1.Pod
    stoppedPods  []*corev1.Pod
}

// Comprehensive container crash detection tests
func TestContainerStatusCrashDetection(t *testing.T) {
    // Tests for container restarts, OOM kills, and exit code analysis
}
```

**Running Tests**:
```bash
# Run Kubernetes integration tests
go test -v ./internal/k8s

# Run with coverage
go test -cover ./internal/k8s

# Run specific test cases  
go test -v ./internal/k8s -run TestContainerStatusCrashDetection
```

## Best Practices

### Security
1. **Use minimal RBAC permissions** (read-only access to pods/events)
2. **Enable network policies** to restrict unnecessary API access
3. **Regular security scanning** of Kubernetes client dependencies  
4. **Monitor API access patterns** for unusual activity

### Reliability  
1. **Configure appropriate timeouts** for API client operations
2. **Implement retry logic** with exponential backoff
3. **Monitor watch stream health** and set up alerting
4. **Test failover scenarios** (API server restarts, network partitions)

### Performance
1. **Use node-scoped monitoring** to limit resource usage
2. **Configure reasonable buffer sizes** for event processing  
3. **Monitor API request rates** to avoid throttling
4. **Optimize field selectors** to minimize data transfer

### Operations
1. **Set up comprehensive monitoring** of Kubernetes integration health
2. **Create runbooks** for common failure scenarios  
3. **Document RBAC requirements** for different environments
4. **Test deployment** in development clusters before production