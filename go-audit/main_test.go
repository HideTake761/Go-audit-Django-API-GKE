package main

import (
	"sync"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// labelsMatch(),auditPod(),auditDeployment(),auditService()はテスト対象のmain.goで定義した関数。
// 本体のコード(main.go)とテストコード(main_test.go)が同じフォルダにあり、同じパッケージ名(package main)
// を名乗っているから使える。

// labelsMatch 関数のテスト
func TestLabelsMatch(t *testing.T) {
	tests := []struct {
		name     string
		selector map[string]string
		labels   map[string]string
		expected bool
	}{
		{"完全一致", map[string]string{"app": "nginx"}, map[string]string{"app": "nginx", "env": "prod"}, true},
		{"不一致", map[string]string{"app": "nginx"}, map[string]string{"app": "redis"}, false},
		{"ラベルなし", map[string]string{"app": "nginx"}, nil, false},
		{"セレクタなし", map[string]string{}, map[string]string{"app": "nginx"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := labelsMatch(tt.selector, tt.labels); got != tt.expected {
				t.Errorf("labelsMatch() = %v, want %v", got, tt.expected)
			}
		})
	}
}

// auditPod 関数のテスト
func TestAuditPod(t *testing.T) {
	wg := &sync.WaitGroup{}
	resultsCh := make(chan AuditResult, 10)

	// ケース1: 正常なPod
	podNormal := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "healthy-pod", Namespace: "default"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:          "nginx",
					LivenessProbe: &corev1.Probe{},
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU: resource.MustParse("100m"),
						},
					},
				},
			},
		},
	}

	// ケース2: CrashLoopBackOffのPod
	podCrash := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "crashing-pod", Namespace: "default"},
		Status: corev1.PodStatus{
			ContainerStatuses: []corev1.ContainerStatus{
				{
					Name: "bad-container",
					State: corev1.ContainerState{
						Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"},
					},
				},
			},
		},
	}

	wg.Add(1)
	go auditPod(podNormal, wg, resultsCh)
	wg.Add(1)
	go auditPod(podCrash, wg, resultsCh)

	wg.Wait()
	close(resultsCh)

	var results []AuditResult
	for res := range resultsCh {
		results = append(results)
		if res.Name == "crashing-pod" && res.Severity != "Critical" {
			t.Errorf("Expected Critical for crashing-pod, got %s", res.Severity)
		}
	}
	// 正常なPodは警告が出るはず(CPUリクエスト等は設定してあるが、追加のチェックがあれば増える)
}

// auditDeployment 関数のテスト
func TestAuditDeployment(t *testing.T) {
	wg := &sync.WaitGroup{}
	resultsCh := make(chan AuditResult, 10)

	replicas := int32(1)
	deploy := appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "low-replica-deploy", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
		},
	}

	wg.Add(1)
	go auditDeployment(deploy, wg, resultsCh)
	wg.Wait()
	close(resultsCh)

	foundLowReplicas := false
	foundRecreate := false
	for res := range resultsCh {
		if res.Issue == "Using Recreate strategy. RollingUpdate is recommended for zero-downtime" {
			foundRecreate = true
		}
		if res.Severity == "Warning" {
			foundLowReplicas = true
		}
	}

	if !foundLowReplicas || !foundRecreate {
		t.Error("Failed to detect low replicas or Recreate strategy")
	}
}

// auditService 関数のテスト
func TestAuditService(t *testing.T) {
	wg := &sync.WaitGroup{}
	resultsCh := make(chan AuditResult, 10)

	// セレクタが一致しないService
	svc := corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "orphan-svc", Namespace: "default"},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{"app": "missing"},
			Type:     corev1.ServiceTypeLoadBalancer,
		},
	}

	// 既存のPod（ラベルが違う）
	pods := []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "nginx-pod",
				Namespace: "default",
				Labels:    map[string]string{"app": "nginx"},
			},
		},
	}

	wg.Add(1)
	go auditService(svc, pods, wg, resultsCh)
	wg.Wait()
	close(resultsCh)

	foundCritical := false
	foundExposedWarning := false
	for res := range resultsCh {
		if res.Severity == "Critical" {
			foundCritical = true // No running Pods match
		}
		if res.Severity == "Warning" {
			foundExposedWarning = true // LoadBalancer warning
		}
	}

	if !foundCritical {
		t.Error("Expected Critical error for orphan service not found")
	}
	if !foundExposedWarning {
		t.Error("Expected Warning for LoadBalancer service not found")
	}
}
