# BlackBox-Daemon Deployment Guide

This guide covers deploying BlackBox-Daemon in various environments including Kubernetes, Docker, and cloud platforms.

## Prerequisites

- **Kubernetes cluster** v1.20+ (for production deployment)
- **Docker** v20.10+ (for containerized deployment)
- **Go** 1.25+ (for building from source)
- **kubectl** configured for your cluster
- **Sufficient cluster resources**:
  - CPU: 100-500m per node
  - Memory: 128-512Mi per node
  - Storage: 1-10Gi per node (for logs)

## Kubernetes Deployment (Recommended)

### 1. Quick Deployment

```bash
# Clone repository
git clone https://github.com/verygoodsoftwarecompany/blackbox-daemon.git
cd blackbox-daemon

# Deploy with default configuration
kubectl apply -f deployments/kubernetes.yaml
```

### 2. Production Deployment

#### Step 1: Create Namespace

```yaml
# namespace.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: blackbox-system
  labels:
    name: blackbox-system
```

```bash
kubectl apply -f namespace.yaml
```

#### Step 2: Create Secret for API Key

```bash
# Generate secure API key
API_KEY=$(openssl rand -base64 32)

# Create secret
kubectl create secret generic blackbox-daemon-secret \
  --from-literal=api-key="$API_KEY" \
  -n blackbox-system
```

#### Step 3: Configure RBAC

```yaml
# rbac.yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: blackbox-daemon
  namespace: blackbox-system

---
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
- apiGroups: ["metrics.k8s.io"]
  resources: ["nodes", "pods"]
  verbs: ["get", "list"]

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
  namespace: blackbox-system
```

#### Step 4: Deploy DaemonSet

```yaml
# daemonset.yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: blackbox-daemon
  namespace: blackbox-system
  labels:
    app: blackbox-daemon
spec:
  selector:
    matchLabels:
      app: blackbox-daemon
  template:
    metadata:
      labels:
        app: blackbox-daemon
    spec:
      serviceAccountName: blackbox-daemon
      hostPID: true
      hostNetwork: false
      containers:
      - name: blackbox-daemon
        image: blackbox-daemon:latest
        imagePullPolicy: Always
        securityContext:
          privileged: true
          runAsUser: 0
        env:
        - name: NODE_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: BLACKBOX_API_KEY
          valueFrom:
            secretKeyRef:
              name: blackbox-daemon-secret
              key: api-key
        - name: BLACKBOX_BUFFER_WINDOW_SIZE
          value: "300s"
        - name: BLACKBOX_COLLECTION_INTERVAL
          value: "5s"
        - name: BLACKBOX_OUTPUT_FORMATTERS
          value: "json,csv"
        - name: BLACKBOX_LOG_LEVEL
          value: "info"
        - name: BLACKBOX_API_PORT
          value: "8080"
        - name: BLACKBOX_METRICS_PORT
          value: "9090"
        ports:
        - containerPort: 8080
          name: api
          protocol: TCP
        - containerPort: 9090
          name: metrics
          protocol: TCP
        volumeMounts:
        - name: proc
          mountPath: /host/proc
          readOnly: true
        - name: sys
          mountPath: /host/sys
          readOnly: true
        - name: logs
          mountPath: /var/log/blackbox
        - name: varrun
          mountPath: /var/run/docker.sock
          readOnly: true
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 512Mi
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
      volumes:
      - name: proc
        hostPath:
          path: /proc
      - name: sys
        hostPath:
          path: /sys
      - name: logs
        hostPath:
          path: /var/log/blackbox
          type: DirectoryOrCreate
      - name: varrun
        hostPath:
          path: /var/run/docker.sock
      terminationGracePeriodSeconds: 30
      tolerations:
      - effect: NoSchedule
        operator: Exists
      - effect: NoExecute
        operator: Exists
```

#### Step 5: Create Services

```yaml
# services.yaml
apiVersion: v1
kind: Service
metadata:
  name: blackbox-daemon-api
  namespace: blackbox-system
  labels:
    app: blackbox-daemon
spec:
  type: ClusterIP
  ports:
  - port: 8080
    targetPort: 8080
    name: api
  selector:
    app: blackbox-daemon

---
apiVersion: v1
kind: Service
metadata:
  name: blackbox-daemon-metrics
  namespace: blackbox-system
  labels:
    app: blackbox-daemon
spec:
  type: ClusterIP
  ports:
  - port: 9090
    targetPort: 9090
    name: metrics
  selector:
    app: blackbox-daemon
```

### 3. Deploy All Resources

```bash
kubectl apply -f rbac.yaml
kubectl apply -f daemonset.yaml
kubectl apply -f services.yaml

# Verify deployment
kubectl get pods -n blackbox-system -l app=blackbox-daemon
kubectl logs -n blackbox-system -l app=blackbox-daemon
```

## Docker Deployment

### 1. Simple Docker Run

```bash
# Build image
docker build -t blackbox-daemon:latest .

# Run container
docker run -d \
  --name blackbox-daemon \
  --privileged \
  --pid=host \
  --network=host \
  -v /proc:/host/proc:ro \
  -v /sys:/host/sys:ro \
  -v /var/run/docker.sock:/var/run/docker.sock:ro \
  -v /var/log/blackbox:/var/log/blackbox \
  -e BLACKBOX_API_KEY="your-secure-api-key" \
  -e BLACKBOX_BUFFER_WINDOW_SIZE="60s" \
  -e BLACKBOX_COLLECTION_INTERVAL="1s" \
  -p 8080:8080 \
  -p 9090:9090 \
  blackbox-daemon:latest
```

### 2. Docker Compose

```yaml
# docker-compose.yml
version: '3.8'

services:
  blackbox-daemon:
    build: .
    container_name: blackbox-daemon
    privileged: true
    pid: host
    network_mode: host
    environment:
      - BLACKBOX_API_KEY=test-api-key-12345
      - BLACKBOX_BUFFER_WINDOW_SIZE=60s
      - BLACKBOX_COLLECTION_INTERVAL=1s
      - BLACKBOX_OUTPUT_FORMATTERS=default,json
      - BLACKBOX_LOG_LEVEL=info
    volumes:
      - /proc:/host/proc:ro
      - /sys:/host/sys:ro
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - ./logs:/var/log/blackbox
    ports:
      - "8080:8080"
      - "9090:9090"
    restart: unless-stopped
    healthcheck:
      test: ["CMD", "wget", "-q", "--spider", "http://localhost:8080/health"]
      interval: 30s
      timeout: 10s
      retries: 3

  # Optional: Prometheus for metrics collection
  prometheus:
    image: prom/prometheus:latest
    container_name: prometheus
    ports:
      - "9091:9090"
    volumes:
      - ./prometheus.yml:/etc/prometheus/prometheus.yml
    command:
      - '--config.file=/etc/prometheus/prometheus.yml'
      - '--storage.tsdb.path=/prometheus'
      - '--web.console.libraries=/etc/prometheus/console_libraries'
      - '--web.console.templates=/etc/prometheus/consoles'
    depends_on:
      - blackbox-daemon
```

```bash
# Start services
docker-compose up -d

# View logs
docker-compose logs blackbox-daemon

# Stop services
docker-compose down
```

## Cloud Platform Deployments

### AWS EKS

#### Prerequisites

```bash
# Install eksctl
curl --silent --location "https://github.com/weaveworks/eksctl/releases/latest/download/eksctl_$(uname -s)_amd64.tar.gz" | tar xz -C /tmp
sudo mv /tmp/eksctl /usr/local/bin

# Create EKS cluster
eksctl create cluster --name blackbox-cluster --region us-west-2 --nodes 3
```

#### EKS-Specific Configuration

```yaml
# Add EKS-specific annotations and configurations
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: blackbox-daemon
  annotations:
    eks.amazonaws.com/role-arn: arn:aws:iam::ACCOUNT:role/blackbox-daemon-role
spec:
  # ... rest of configuration
  template:
    spec:
      containers:
      - name: blackbox-daemon
        # EKS optimized configuration
        env:
        - name: AWS_REGION
          value: us-west-2
        - name: BLACKBOX_OUTPUT_PATH
          value: /var/log/blackbox
        volumeMounts:
        - name: aws-logs
          mountPath: /var/log/blackbox
      volumes:
      - name: aws-logs
        persistentVolumeClaim:
          claimName: blackbox-logs-pvc
```

### Google GKE

#### Prerequisites

```bash
# Create GKE cluster
gcloud container clusters create blackbox-cluster \
    --zone us-central1-a \
    --num-nodes 3 \
    --enable-autorepair \
    --enable-autoupgrade
```

#### GKE-Specific Configuration

```yaml
# Add GKE-specific service account binding
apiVersion: v1
kind: ServiceAccount
metadata:
  name: blackbox-daemon
  annotations:
    iam.gke.io/gcp-service-account: blackbox-daemon@PROJECT-ID.iam.gserviceaccount.com
```

### Azure AKS

#### Prerequisites

```bash
# Create AKS cluster
az aks create \
    --resource-group blackbox-rg \
    --name blackbox-cluster \
    --node-count 3 \
    --enable-addons monitoring \
    --generate-ssh-keys
```

#### AKS-Specific Configuration

```yaml
# Add Azure-specific annotations
metadata:
  annotations:
    azure.workload.identity/service-account-name: blackbox-daemon
```

## Monitoring Integration

### Prometheus ServiceMonitor

```yaml
# servicemonitor.yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: blackbox-daemon
  namespace: blackbox-system
  labels:
    app: blackbox-daemon
spec:
  selector:
    matchLabels:
      app: blackbox-daemon
  endpoints:
  - port: metrics
    interval: 30s
    path: /metrics
    scrapeTimeout: 10s
```

### Grafana Dashboard

```json
{
  "dashboard": {
    "title": "BlackBox Daemon Metrics",
    "panels": [
      {
        "title": "Buffer Utilization",
        "type": "stat",
        "targets": [
          {
            "expr": "blackbox_buffer_utilization_percent"
          }
        ]
      },
      {
        "title": "Telemetry Entries per Second",
        "type": "graph", 
        "targets": [
          {
            "expr": "rate(blackbox_telemetry_entries_total[5m])"
          }
        ]
      }
    ]
  }
}
```

## Scaling Considerations

### Node Resources

Calculate resources per node:

- **CPU**: Base 100m + (10m × pods per node)
- **Memory**: Base 128Mi + (1Mi × active pods)
- **Storage**: 1Gi per day of logs (configurable)

### High-Availability Setup

For critical environments:

```yaml
# Multiple replicas with anti-affinity
spec:
  replicas: 2
  template:
    spec:
      affinity:
        podAntiAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
          - labelSelector:
              matchLabels:
                app: blackbox-daemon
            topologyKey: kubernetes.io/hostname
```

## Security Considerations

### Network Policies

```yaml
# network-policy.yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: blackbox-daemon-netpol
  namespace: blackbox-system
spec:
  podSelector:
    matchLabels:
      app: blackbox-daemon
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector: {}
    ports:
    - protocol: TCP
      port: 8080
    - protocol: TCP
      port: 9090
  egress:
  - to: []
    ports:
    - protocol: TCP
      port: 443  # Kubernetes API
    - protocol: TCP
      port: 53   # DNS
    - protocol: UDP
      port: 53   # DNS
```

### Pod Security Standards

```yaml
# pod-security.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: blackbox-system
  labels:
    pod-security.kubernetes.io/enforce: baseline
    pod-security.kubernetes.io/audit: restricted
    pod-security.kubernetes.io/warn: restricted
```

## Troubleshooting Deployment

### Common Issues

#### 1. Permission Denied

```bash
# Check RBAC
kubectl auth can-i get pods --as=system:serviceaccount:blackbox-system:blackbox-daemon

# Fix: Update RBAC configuration
kubectl apply -f rbac.yaml
```

#### 2. Mount Issues

```bash
# Check host paths exist
kubectl exec -it blackbox-daemon-xyz -- ls -la /host/proc
kubectl exec -it blackbox-daemon-xyz -- ls -la /host/sys

# Fix: Ensure host paths are accessible
```

#### 3. API Key Issues

```bash
# Check secret exists
kubectl get secret blackbox-daemon-secret -n blackbox-system

# Check environment variable
kubectl exec -it blackbox-daemon-xyz -- env | grep BLACKBOX_API_KEY
```

#### 4. Network Issues

```bash
# Test API connectivity
kubectl port-forward svc/blackbox-daemon-api 8080:8080 -n blackbox-system
curl http://localhost:8080/health

# Test metrics
kubectl port-forward svc/blackbox-daemon-metrics 9090:9090 -n blackbox-system
curl http://localhost:9090/metrics
```

### Debugging Commands

```bash
# View pod logs
kubectl logs -n blackbox-system -l app=blackbox-daemon -f

# Describe pod
kubectl describe pod -n blackbox-system -l app=blackbox-daemon

# Check events
kubectl get events -n blackbox-system --sort-by='.lastTimestamp'

# Exec into pod
kubectl exec -it blackbox-daemon-xyz -n blackbox-system -- /bin/sh

# Test internal connectivity
kubectl run test-pod --rm -it --image=alpine/curl -- \
  curl http://blackbox-daemon-api.blackbox-system.svc.cluster.local:8080/health
```

## Upgrade Process

### Rolling Update

```bash
# Update image version
kubectl set image daemonset/blackbox-daemon \
  blackbox-daemon=blackbox-daemon:v2.0.0 \
  -n blackbox-system

# Check rollout status
kubectl rollout status daemonset/blackbox-daemon -n blackbox-system

# Rollback if needed
kubectl rollout undo daemonset/blackbox-daemon -n blackbox-system
```

### Zero-Downtime Upgrade

For environments requiring zero downtime:

1. Deploy new version alongside old version
2. Gradually shift traffic using service selectors
3. Remove old version once validated

## Backup and Recovery

### Configuration Backup

```bash
# Export current configuration
kubectl get all,secrets,configmaps,pv,pvc -n blackbox-system -o yaml > blackbox-backup.yaml

# Restore from backup
kubectl apply -f blackbox-backup.yaml
```

### Log Data Backup

```bash
# Backup logs from nodes
for node in $(kubectl get nodes -o name); do
  kubectl debug $node -it --image=alpine -- tar czf - /host/var/log/blackbox
done > blackbox-logs-backup.tar.gz
```

This deployment guide should cover most production scenarios. Adjust configurations based on your specific requirements and security policies.