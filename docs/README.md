# BlackBox-Daemon Technical Documentation

This directory contains comprehensive technical documentation for the BlackBox-Daemon project.

## Documentation Structure

- [**Architecture Overview**](architecture.md) - System design, components, and data flow
- [**API Reference**](api-reference.md) - Complete REST API documentation  
- [**Build & Test Guide**](BUILD.md) - Comprehensive building, testing, and CI/CD instructions
- [**Configuration Guide**](configuration.md) - Environment variables and settings
- [**Deployment Guide**](deployment.md) - Kubernetes and Docker deployment instructions
- [**Development Guide**](development.md) - Building, testing, and extending the system
- [**Monitoring & Metrics**](monitoring.md) - Prometheus metrics and observability
- [**Troubleshooting**](troubleshooting.md) - Common issues and debugging

## Component Documentation

- [**Ring Buffer**](components/ringbuffer.md) - In-memory telemetry storage system
- [**System Telemetry**](components/telemetry.md) - Linux system metrics collection
- [**Kubernetes Integration**](components/k8s.md) - Pod monitoring and crash detection  
- [**API Server**](components/api.md) - REST API for sidecar communication
- [**Metrics Collector**](components/metrics.md) - Prometheus metrics and observability
- [**Output Formatters**](components/formatter.md) - Data formatting and output destinations
- [**Metrics Collector**](components/metrics-collector.md) - Prometheus metrics framework
- [**Formatter Chain**](components/formatter-chain.md) - Configurable output formatting

## Integration Documentation

- [**Sidecar Integration**](integration/sidecar-integration.md) - Building application sidecars
- [**JVM Sidecar Example**](integration/jvm-sidecar.md) - Java/JVM telemetry collection
- [**.NET Sidecar Example**](integration/dotnet-sidecar.md) - .NET runtime telemetry
- [**Go Sidecar Example**](integration/go-sidecar.md) - Go application telemetry
- [**Python Sidecar Example**](integration/python-sidecar.md) - Python runtime telemetry

## Additional Resources

- [**Performance Tuning**](performance-tuning.md) - Optimizing for high-throughput environments
- [**Security Considerations**](security.md) - Authentication, RBAC, and hardening
- [**Examples**](examples/) - Sample configurations and use cases
- [**FAQ**](faq.md) - Frequently asked questions

## Quick Links

- [Main README](../README.md)
- [API Swagger Documentation](http://localhost:8080/swagger/) (when enabled)
- [Prometheus Metrics](http://localhost:9090/metrics)
- [GitHub Repository](https://github.com/verygoodsoftwarecompany/blackbox-daemon)