# BlackBox-Daemon Troubleshooting Guide

This guide helps diagnose and resolve common issues with BlackBox-Daemon deployment and operation.

## Common Issues and Solutions

### 1. Deployment Issues

#### Pod Won't Start

**Symptoms:**
- Pod remains in `Pending` or `CrashLoopBackOff` state
- Container exits immediately

**Diagnosis:**
```bash
kubectl describe pod blackbox-daemon-xyz -n blackbox-system
kubectl logs blackbox-daemon-xyz -n blackbox-system
```

**Common Causes & Solutions:**

**Missing API Key:**
```
Error: BLACKBOX_API_KEY environment variable is required
```
Solution:
```bash
# Verify secret exists
kubectl get secret blackbox-daemon-secret -n blackbox-system

# Create if missing
kubectl create secret generic blackbox-daemon-secret \
  --from-literal=api-key="$(openssl rand -base64 32)" \
  -n blackbox-system
```

**RBAC Permissions:**
```
Error: pods is forbidden: User "system:serviceaccount:blackbox-system:blackbox-daemon" cannot get resource "pods"
```
Solution:
```bash
# Apply RBAC configuration
kubectl apply -f deployments/rbac.yaml

# Verify permissions
kubectl auth can-i get pods --as=system:serviceaccount:blackbox-system:blackbox-daemon
```

**Privileged Access Denied:**
```
Error: container has runAsNonRoot and image will run as root
```
Solution:
```yaml
# Update DaemonSet security context
securityContext:
  privileged: true
  runAsUser: 0
```

**Host Path Mount Issues:**
```
Error: failed to mount "/proc" on "/host/proc": permission denied
```
Solution:
```yaml
# Ensure correct host path mounts
volumeMounts:
- name: proc
  mountPath: /host/proc
  readOnly: true
volumes:
- name: proc
  hostPath:
    path: /proc
```

#### Image Pull Failures

**Symptoms:**
```
Failed to pull image "blackbox-daemon:latest": rpc error: code = NotFound
```

**Solutions:**
```bash
# Build and tag image locally
docker build -t blackbox-daemon:latest .

# For Kubernetes (kind/minikube)
kind load docker-image blackbox-daemon:latest

# For remote registry
docker tag blackbox-daemon:latest your-registry/blackbox-daemon:latest
docker push your-registry/blackbox-daemon:latest
```

### 2. Runtime Issues

#### API Server Not Responding

**Symptoms:**
- Health check fails
- API requests timeout
- Metrics endpoint unreachable

**Diagnosis:**
```bash
# Check if process is running
kubectl exec blackbox-daemon-xyz -- ps aux | grep blackbox

# Test internal connectivity
kubectl exec blackbox-daemon-xyz -- wget -qO- http://localhost:8080/health

# Check port bindings
kubectl exec blackbox-daemon-xyz -- netstat -tulpn | grep :8080
```

**Common Causes & Solutions:**

**Port Conflicts:**
```bash
# Check if ports are in use
kubectl exec blackbox-daemon-xyz -- lsof -i :8080
kubectl exec blackbox-daemon-xyz -- lsof -i :9090

# Use different ports if needed
env:
- name: BLACKBOX_API_PORT
  value: "8081"
- name: BLACKBOX_METRICS_PORT
  value: "9091"
```

**Firewall/Network Policies:**
```bash
# Test connectivity from another pod
kubectl run test-pod --rm -it --image=alpine/curl -- \
  curl http://blackbox-daemon-api.blackbox-system.svc.cluster.local:8080/health

# Check network policies
kubectl get networkpolicy -n blackbox-system
```

#### High Memory Usage

**Symptoms:**
- Pod gets OOM killed
- Performance degradation
- Buffer overflow errors

**Diagnosis:**
```bash
# Check memory usage
kubectl top pod blackbox-daemon-xyz -n blackbox-system

# Monitor buffer metrics
curl http://localhost:9090/metrics | grep blackbox_buffer

# Check memory limits
kubectl describe pod blackbox-daemon-xyz | grep -A5 "Limits:"
```

**Solutions:**
```yaml
# Reduce buffer window size
env:
- name: BLACKBOX_BUFFER_WINDOW_SIZE
  value: "30s"  # Reduce from 60s

# Increase collection interval
- name: BLACKBOX_COLLECTION_INTERVAL
  value: "5s"   # Reduce from 1s

# Increase memory limits
resources:
  limits:
    memory: "1Gi"  # Increase from 512Mi
```

#### System Metrics Collection Failures

**Symptoms:**
```
Error reading /proc/stat: permission denied
Error reading /sys/class/net/eth0/statistics: no such file
```

**Diagnosis:**
```bash
# Check host filesystem access
kubectl exec blackbox-daemon-xyz -- ls -la /host/proc/
kubectl exec blackbox-daemon-xyz -- cat /host/proc/meminfo
kubectl exec blackbox-daemon-xyz -- ls -la /host/sys/class/net/
```

**Solutions:**
```yaml
# Ensure proper host path mounts
volumeMounts:
- name: proc
  mountPath: /host/proc
  readOnly: true
- name: sys
  mountPath: /host/sys
  readOnly: true

volumes:
- name: proc
  hostPath:
    path: /proc
- name: sys
  hostPath:
    path: /sys

# Ensure privileged access
securityContext:
  privileged: true
  capabilities:
    add:
    - SYS_ADMIN
    - SYS_PTRACE
```

### 3. Kubernetes Integration Issues

#### Pod Watcher Not Working

**Symptoms:**
- No pod events detected
- Missing pod metrics
- Incident reports not generated

**Diagnosis:**
```bash
# Check Kubernetes API connectivity
kubectl exec blackbox-daemon-xyz -- \
  wget -qO- --header="Authorization: Bearer $(cat /var/run/secrets/kubernetes.io/serviceaccount/token)" \
  https://kubernetes.default.svc.cluster.local/api/v1/pods

# Verify RBAC permissions
kubectl auth can-i list pods --as=system:serviceaccount:blackbox-system:blackbox-daemon
kubectl auth can-i watch pods --as=system:serviceaccount:blackbox-system:blackbox-daemon
```

**Solutions:**
```yaml
# Ensure proper RBAC configuration
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: blackbox-daemon
rules:
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources: ["nodes"]
  verbs: ["get", "list"]

# Check service account binding
kubectl get clusterrolebinding blackbox-daemon -o yaml
```

#### Node Name Detection Issues

**Symptoms:**
```
Error: NODE_NAME environment variable not set
Failed to determine current node
```

**Solutions:**
```yaml
# Ensure NODE_NAME is set via downward API
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

### 4. Performance Issues

#### High CPU Usage

**Symptoms:**
- Pod consuming excessive CPU
- System responsiveness degraded
- Frequent context switching

**Diagnosis:**
```bash
# Monitor CPU usage
kubectl top pod blackbox-daemon-xyz -n blackbox-system --containers

# Check collection intervals
kubectl exec blackbox-daemon-xyz -- env | grep COLLECTION_INTERVAL

# Profile CPU usage
kubectl exec blackbox-daemon-xyz -- top -p 1
```

**Solutions:**
```yaml
# Increase collection interval
env:
- name: BLACKBOX_COLLECTION_INTERVAL
  value: "10s"  # Reduce frequency

# Set CPU limits
resources:
  limits:
    cpu: "200m"  # Limit CPU usage
  requests:
    cpu: "100m"
```

#### Disk I/O Issues

**Symptoms:**
- High disk wait times
- Slow log writes
- Storage errors

**Diagnosis:**
```bash
# Check disk usage
kubectl exec blackbox-daemon-xyz -- df -h /var/log/blackbox

# Monitor I/O
kubectl exec blackbox-daemon-xyz -- iostat -x 1 5

# Check log rotation
kubectl exec blackbox-daemon-xyz -- ls -lah /var/log/blackbox/
```

**Solutions:**
```yaml
# Use faster storage class
volumeClaimTemplates:
- metadata:
    name: blackbox-logs
  spec:
    storageClassName: ssd  # Use SSD storage
    accessModes: ["ReadWriteOnce"]
    resources:
      requests:
        storage: 10Gi

# Reduce output formatters
env:
- name: BLACKBOX_OUTPUT_FORMATTERS
  value: "json"  # Single formatter instead of multiple
```

### 5. Network and Connectivity Issues

#### API Authentication Failures

**Symptoms:**
```
HTTP 401: Unauthorized
Invalid API key
Authentication header missing
```

**Diagnosis:**
```bash
# Test API key
API_KEY=$(kubectl get secret blackbox-daemon-secret -n blackbox-system -o jsonpath='{.data.api-key}' | base64 -d)
curl -H "Authorization: Bearer $API_KEY" http://localhost:8080/health

# Check environment variable
kubectl exec blackbox-daemon-xyz -- env | grep BLACKBOX_API_KEY
```

**Solutions:**
```bash
# Regenerate API key
kubectl delete secret blackbox-daemon-secret -n blackbox-system
kubectl create secret generic blackbox-daemon-secret \
  --from-literal=api-key="$(openssl rand -base64 32)" \
  -n blackbox-system

# Restart pods to pick up new secret
kubectl rollout restart daemonset/blackbox-daemon -n blackbox-system
```

#### Service Discovery Issues

**Symptoms:**
- Prometheus can't scrape metrics
- Service endpoints not found
- DNS resolution failures

**Diagnosis:**
```bash
# Check service endpoints
kubectl get endpoints blackbox-daemon-metrics -n blackbox-system

# Test DNS resolution
kubectl exec test-pod -- nslookup blackbox-daemon-api.blackbox-system.svc.cluster.local

# Check service configuration
kubectl describe svc blackbox-daemon-metrics -n blackbox-system
```

**Solutions:**
```yaml
# Verify service selector matches pod labels
spec:
  selector:
    app: blackbox-daemon  # Must match pod labels

# Check service ports
ports:
- port: 9090
  targetPort: 9090  # Must match container port
  name: metrics
```

### 6. Logging and Debugging

#### Enable Debug Logging

```yaml
# Temporary debug configuration
env:
- name: BLACKBOX_LOG_LEVEL
  value: "debug"
- name: BLACKBOX_LOG_JSON
  value: "false"  # Human-readable logs
```

#### Collect Diagnostic Information

```bash
#!/bin/bash
# diagnostic-collect.sh

NAMESPACE="blackbox-system"
POD=$(kubectl get pods -n $NAMESPACE -l app=blackbox-daemon -o jsonpath='{.items[0].metadata.name}')

echo "=== BlackBox-Daemon Diagnostics ==="
echo "Timestamp: $(date)"
echo "Pod: $POD"
echo

echo "=== Pod Status ==="
kubectl describe pod $POD -n $NAMESPACE

echo "=== Pod Logs ==="
kubectl logs $POD -n $NAMESPACE --tail=100

echo "=== Environment Variables ==="
kubectl exec $POD -n $NAMESPACE -- env | grep BLACKBOX

echo "=== Process Status ==="
kubectl exec $POD -n $NAMESPACE -- ps aux

echo "=== Network Status ==="
kubectl exec $POD -n $NAMESPACE -- netstat -tulpn

echo "=== File System Access ==="
kubectl exec $POD -n $NAMESPACE -- ls -la /host/proc/ | head -10
kubectl exec $POD -n $NAMESPACE -- ls -la /host/sys/class/net/

echo "=== Health Check ==="
kubectl exec $POD -n $NAMESPACE -- wget -qO- http://localhost:8080/health

echo "=== Metrics Sample ==="
kubectl exec $POD -n $NAMESPACE -- wget -qO- http://localhost:9090/metrics | head -20

echo "=== Events ==="
kubectl get events -n $NAMESPACE --sort-by='.lastTimestamp' | tail -10
```

### 7. Configuration Issues

#### Invalid Configuration Values

**Symptoms:**
```
Error: invalid duration format "invalid"
Error: buffer window size must be positive
Error: collection interval too small
```

**Solutions:**
```yaml
# Valid configuration examples
env:
- name: BLACKBOX_BUFFER_WINDOW_SIZE
  value: "60s"     # Valid: 30s, 2m, 1h
- name: BLACKBOX_COLLECTION_INTERVAL
  value: "1s"      # Valid: 100ms, 1s, 5s
- name: BLACKBOX_LOG_LEVEL
  value: "info"    # Valid: debug, info, warn, error
```

#### Formatter Configuration Issues

**Symptoms:**
```
Error: unknown formatter "invalid"
Error: failed to create output destination
```

**Solutions:**
```yaml
# Valid formatter configurations
env:
- name: BLACKBOX_OUTPUT_FORMATTERS
  value: "default,json,csv"  # Valid formatters
- name: BLACKBOX_OUTPUT_PATH
  value: "/var/log/blackbox"  # Writable directory
```

### 8. Recovery Procedures

#### Emergency Recovery

```bash
# Scale down daemon
kubectl scale daemonset blackbox-daemon --replicas=0 -n blackbox-system

# Clear problematic configuration
kubectl patch daemonset blackbox-daemon -n blackbox-system --type='merge' -p='{"spec":{"template":{"spec":{"containers":[{"name":"blackbox-daemon","env":[]}]}}}}'

# Restore minimal configuration
kubectl set env daemonset/blackbox-daemon -n blackbox-system \
  BLACKBOX_API_KEY="$(openssl rand -base64 32)" \
  BLACKBOX_LOG_LEVEL="debug"

# Scale back up
kubectl scale daemonset blackbox-daemon --replicas=1 -n blackbox-system
```

#### Data Recovery

```bash
# Export buffer contents before restart
kubectl exec $POD -n $NAMESPACE -- \
  wget -qO- "http://localhost:8080/api/v1/export?format=json" > blackbox-backup.json

# Restore from backup (if supported)
curl -H "Authorization: Bearer $API_KEY" \
     -X POST http://localhost:8080/api/v1/import \
     --data-binary @blackbox-backup.json
```

### 9. Monitoring and Alerting Issues

#### Missing Metrics

**Diagnosis:**
```bash
# Check metrics endpoint
curl http://localhost:9090/metrics | grep blackbox_

# Verify Prometheus configuration
kubectl get configmap prometheus-config -o yaml

# Check ServiceMonitor
kubectl get servicemonitor blackbox-daemon -o yaml
```

**Solutions:**
```yaml
# Verify metrics port in service
spec:
  ports:
  - name: metrics
    port: 9090
    targetPort: 9090

# Check ServiceMonitor selector
spec:
  selector:
    matchLabels:
      app: blackbox-daemon
```

### 10. Performance Tuning

#### Memory Optimization

```yaml
# Optimized configuration for memory-constrained environments
env:
- name: BLACKBOX_BUFFER_WINDOW_SIZE
  value: "30s"           # Smaller window
- name: BLACKBOX_COLLECTION_INTERVAL  
  value: "5s"            # Less frequent collection
- name: BLACKBOX_OUTPUT_FORMATTERS
  value: "json"          # Single formatter

resources:
  requests:
    memory: "64Mi"       # Lower request
  limits:
    memory: "256Mi"      # Conservative limit
```

#### CPU Optimization

```yaml
# CPU-optimized configuration
env:
- name: BLACKBOX_COLLECTION_INTERVAL
  value: "10s"           # Reduce CPU load

resources:
  requests:
    cpu: "50m"           # Lower CPU request
  limits:
    cpu: "200m"          # Limit CPU usage
```

### Support and Help

#### Getting Help

1. **Check Logs**: Always start with detailed logs
2. **Gather Diagnostics**: Use the diagnostic script above
3. **Review Configuration**: Verify all environment variables
4. **Test Components**: Isolate issues to specific components
5. **Community Support**: 
   - GitHub Issues: Report bugs and feature requests
   - Documentation: Check latest documentation for updates
   - Examples: Review deployment examples for reference

#### Escalation Process

1. **Level 1**: Check common issues in this guide
2. **Level 2**: Gather comprehensive diagnostics
3. **Level 3**: Create GitHub issue with full details:
   - Environment details (K8s version, node OS, etc.)
   - Complete configuration (sanitized)
   - Error logs and diagnostics
   - Steps to reproduce

This troubleshooting guide should help resolve most common issues with BlackBox-Daemon. Keep it updated as new issues and solutions are discovered.