## About This Project

In this project, an API server and a continuous cluster auditing tool are built as microservices and deployed together on a single Kubernetes cluster. The infrastructure is managed via IaC, and deployments are automated through CI/CD pipelines. This system is designed for a cloud-native environment and operates on the technology stack detailed below.

Note: The **GCP infrastructure** is managed in a separate **Terraform** repository:
[Terraform/GCP_k8s](https://github.com/HideTake761/Terraform/tree/main/GCP_k8s)

## Architecture & Tech Stack:

Environment:
- Host OS: Windows 11 Home 25H2  
- IDE: Visual Studio Code 1.129.1  
- Languages: Go 1.26.1, Python 3.13.5  
- Backend: Django 5.2.5, Django REST Framework 3.16.1, Django-filter 25.1
- Docker 29.6.1, Kubernetes 1.36.1
- Terraform v1.13.4
- gcloud 576.0.0   

Macro Architecture (GCP Infrastructure):
<img src="./GCP GKE.jpg" alt="System Architecture Diagram" width="600" />

Micro Architecture (GKE Cluster Internals):
<img src="./GKE Cluster.jpg" alt="GKE Cluster Diagram" width="600" />

K8s Audit CLI Tool (Go / CronJob):

This is a Kubernetes auditing tool that connects to a running cluster and assesses the health of Pods, Deployments, and Services.

- Why Go?: Go was chosen because it integrates seamlessly with the official Kubernetes library (`client-go`) and 
leverages Go's powerful concurrency capabilities (Goroutines). This enabling fast and lightweight scanning of vast numbers of resources within the cluster.
- detects not only runtime errors like `CrashLoopBackOff` and `ImagePullBackOff`, but also deviations from best practices, such as missing liveness probes, insufficient replicas, and publicly exposed services.
- Output & Alerting: Supports CLI output (table format), JSON output, and Slack notifications when `Critical` issues are detected
- Principle of Least Privilege: RBAC permissions are limited to the `get` and `list` operations on the target resources, which is the minimum access required to retrieve cluster information.
- End-to-End Testing: Successfully verified the alerting flow by deploying a test Pod (bad-resources.yaml) intentionally designed to crash on the GKE cluster, confirming that the tool detects the anomaly and triggers a Slack notification.

API server (Python / Django REST Framework):

For details regarding the selection of backend frameworks and middleware, please refer to my
[AWS ECS Portfolio Repo](https://github.com/HideTake761/CI-CD-Django-REST-API-with-Docker-on-AWS-ECS-Fargate).

- records and stores product names(product) and prices(price)
- No authentication
- Searchable by product name
- No pagination
- Deployment Behavior: Upon deployment, the server starts with Gunicorn and automatically creates the required database tables.

Docker & Kubernetes:

Docker and Kubernetes were selected to unify the management of applications with distinct technology stacks (Go and DRF) and differing operational characteristics (a periodically executed CLI tool vs. an always-running API server). Using Kubernetes manifests allows for a unified API and appropriate resource allocation for each service.

- API server: defined as `Deployment` (Replicas: 2) to continuously process requests, guaranteeing high availability and redundancy.
- Go Audit CLI tool: defined as `CronJob` to perform periodic cluster diagnostics, ensuring it efficiently consumes resources only during execution.

## CI/CD Pipeline (via GitHub Actions):  
  
Secure Deployment via **OIDC (Workload Identity)**:

`Workload Identity Federation (OIDC)` is utilized for authentication when deploying from GitHub Actions to GCP. This "keyless authentication" approach ensures a high level of security by eliminating the need to store persistent service account keys (JSON files) as GitHub secrets.

Pipeline Steps:
- Trigger: Push, pull request & merge to the main branch
- CI: Automatically runs unit tests
- CD: If tests pass, builds a Docker image and deploys it to GCP GKE  
