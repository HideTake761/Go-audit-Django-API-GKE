package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"text/tabwriter"

	appsv1 "k8s.io/api/apps/v1"
	// appsv1:Deploymentの構造体を扱うためのパッケージ
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	// metav1:KubernetesのGoクライアント(client-go)において「メタデータ(Metadata)」を扱うための標準パッケージ。
	// Kubernetesのすべてのリソース(Pod、Service、Deploymentなど)に共通する「基本情報」(ObjectMeta,TypeMeta)や、
	// KubernetesのAPIサーバー(kube-apiserver)と通信する際の「条件」を定義。

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// 監査結果を格納する構造体
type AuditResult struct {
	Namespace string `json:"namespace"`
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Issue     string `json:"issue"`
	Severity  string `json:"severity"` // "Critical", "Warning" など
}

// Slackへ送るJSONの構造体
type SlackMessage struct {
	Text string `json:"text"`
}

func main() {
	outputFormat := flag.String("output", "table", "Output format: table or json")
	flag.Parse()

	clientset, err := getK8sClient()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating k8s client: %v\n", err)
		os.Exit(1) // CI/CDパイプラインを失敗させるために終了コード1を返す
	}

	ctx := context.Background()

	// 1. 各リソースの一覧取得
	// 上のos.Exit(1)とは別にまた設定するのは、認証(ログイン)は成功しても、
	// RBAC(権限管理)によるアクセス拒否の可能性があるから
	pods, err := clientset.CoreV1().Pods("").List(ctx, metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing pods: %v\n", err)
		os.Exit(1)
	}

	deployments, err := clientset.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing deployments: %v\n", err)
		os.Exit(1)
	}

	services, err := clientset.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error listing services: %v\n", err)
		os.Exit(1)
	}

	var wg sync.WaitGroup
	// チャンネルのバッファサイズを余裕を持って確保
	resultsCh := make(chan AuditResult, (len(pods.Items)+len(deployments.Items)+len(services.Items))*3)
	// Goroutineを使った並行スキャン
	// 2. Podの監査
	for _, pod := range pods.Items {
		wg.Add(1)
		go auditPod(pod, &wg, resultsCh)
	}
	// 3. Deploymentの監査
	for _, deploy := range deployments.Items {
		wg.Add(1)
		go auditDeployment(deploy, &wg, resultsCh)
	}
	// 4. Serviceの監査 (Podのラベルと照合するため、Pod一覧も渡す)
	for _, svc := range services.Items {
		wg.Add(1)
		go auditService(svc, pods.Items, &wg, resultsCh)
	}
	// 全てのGoroutineが完了するのを待機し、Channelを閉じる
	go func() {
		wg.Wait()
		close(resultsCh)
	}()

	// 結果の集計
	var results []AuditResult
	var criticalResults []AuditResult
	hasCritical := false

	for res := range resultsCh {
		results = append(results, res)
		if res.Severity == "Critical" {
			hasCritical = true
			criticalResults = append(criticalResults, res)
		}
	}

	// 標準出力への結果表示(CLI/CI向け)
	printResults(results, *outputFormat)

	// 監査サマリー
	fmt.Println("\n=== Scan Summary ===")
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "RESOURCE TYPE\tCOUNT")
	fmt.Fprintf(w, "Pod\t%d\n", len(pods.Items))
	fmt.Fprintf(w, "Deployment\t%d\n", len(deployments.Items))
	fmt.Fprintf(w, "Service\t%d\n", len(services.Items))
	w.Flush() // 監査サマリー終わり

	// Criticalな問題がある場合のみSlackに通知を飛ばす
	if hasCritical {
		err := sendSlackNotification(criticalResults)
		if err != nil {
			// 通知に失敗してもプログラム自体はパニックさせず、エラーログだけ残す
			fmt.Fprintf(os.Stderr, "Warning: Failed to send Slack notification: %v\n", err)
		}
		os.Exit(1)
	}
}

// 認証情報の設定(ローカルkubeconfig、またはクラスタ内ServiceAccount)
// Kubernetesクラスタに接続するための設定。APIサーバと通信するためのクライアントを作成。
// kubernetes.Clientset:Go言語からKubernetesとやり取りするための「公式リモコン」の本体。
// client-goライブラリに含まれる k8s.io/client-go/kubernetes パッケージで定義。
// KubernetesのAPIは、リソースの種類ごとに細かく分かれている(Pod用、Deployment用、Service用など)。
// Clientsetは、これらのAPIグループを一つにまとめ、直感的に操作できるようにしたインターフェース。
func getK8sClient() (*kubernetes.Clientset, error) {
	// 短縮変数宣言(:=)を使用しているので、var configという形で変数定義をする必要はない。
	// rest.InClusterConfig():内蔵のサービスアカウント。
	// ツール自身がPodとしてクラスタ内部で動いている場合を想定。
	config, err := rest.InClusterConfig()
	// 上記に失敗した場合、開発者のローカルPCで実行されていると判断し、
	// ~/.kube/config を読み込んで接続。
	if err != nil {
		kubeconfig := filepath.Join(homedir.HomeDir(), ".kube", "config")
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, err
		}
	}
	return kubernetes.NewForConfig(config)
}

// Podの監査
func auditPod(pod corev1.Pod, wg *sync.WaitGroup, resultsCh chan<- AuditResult) {
	defer wg.Done()
	// 1つのPodを受け取り、その中のコンテナの状態(Status)や設定(Spec)をチェック
	// Spec:Kubernetesのほぼすべてのリソースに共通して存在する設定項目
	// Podの場合は「このコンテナイメージを使って、CPUはこれくらい割り当てて動かしてほしい」
	// &&:論理積(AND)。両方ともtrue(真)の場合のみtrue
	// CrashLoopBackOff:コンテナが起動とクラッシュを繰り返している状態。
	// ImagePullBackOff:コンテナイメージの取得(Pull)に失敗している状態。
	// ContainerStatuses:PodStatus構造体(struct)のフィールド名。
	// k8s.io/api/core/v1 パッケージの中で定義されている。
	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.State.Waiting != nil && containerStatus.State.Waiting.Reason == "CrashLoopBackOff" {
			resultsCh <- AuditResult{pod.Namespace, "Pod", pod.Name, "CrashLoopBackOff in container: " + containerStatus.Name, "Critical"}
		}
		if containerStatus.State.Waiting != nil && containerStatus.State.Waiting.Reason == "ImagePullBackOff" {
			resultsCh <- AuditResult{pod.Namespace, "Pod", pod.Name, "ImagePullBackOff in container: " + containerStatus.Name, "Critical"}
		}
		if containerStatus.RestartCount > 5 {
			resultsCh <- AuditResult{pod.Namespace, "Pod", pod.Name, fmt.Sprintf("High restart count (%d) in container: %s", containerStatus.RestartCount, containerStatus.Name), "Warning"}
		}
	}
	// Liveness Probe missing:死活監視の設定がない(ハング時に自動復旧できない)。
	// CPU requests missing:CPUのリクエスト値が未設定(リソース競合の原因になり得る)。
	for _, container := range pod.Spec.Containers {
		if container.LivenessProbe == nil {
			resultsCh <- AuditResult{pod.Namespace, "Pod", pod.Name, "Liveness Probe missing in container: " + container.Name, "Warning"}
		}
		if container.Resources.Requests.Cpu().IsZero() {
			resultsCh <- AuditResult{pod.Namespace, "Pod", pod.Name, "CPU requests missing in container: " + container.Name, "Warning"}
		}
	}
}

// Deploymentの監査
func auditDeployment(deploy appsv1.Deployment, wg *sync.WaitGroup, resultsCh chan<- AuditResult) {
	defer wg.Done()

	// 1. Replicasのチェック。SPOF(単一障害点)の防止
	// High Availability(高可用性)を意識した設定(Spec)になっているか、をチェック
	// DeploymentのSpec:「Podを常に〇個(Replicas)動かしておいてほしい」
	if deploy.Spec.Replicas != nil && *deploy.Spec.Replicas < 2 {
		resultsCh <- AuditResult{deploy.Namespace, "Deployment", deploy.Name, fmt.Sprintf("Low replicas count (%d). Recommended >= 2 for High Availability", *deploy.Spec.Replicas), "Warning"}
	}

	// 2. RollingUpdate戦略のチェック
	// Strategy.Type,RecreateDeploymentStrategyType:Deploymentが「新しいバージョンのアプリをデプロイする(Podを更新する)ときの作戦Strategy」に関する設定項目。
	// Strategy.Type:「どのアップデート戦略を使うか」を指定する場所。
	// RecreateDeploymentStrategyType:アップデート戦略の種類の一つ。「いま動いている古いPodを一度すべて削除してから、新しいPodをまとめて作成する」という動き。
	// Recreate戦略は、古いPodが消えてから新しいPodが起動するまでの間、必ずシステムが停止する(ダウンタイムが発生する)。
	// 実運用では、ユーザーに影響を与えずに少しずつPodを入れ替える「RollingUpdate(ローリングアップデート)」という戦略を使うのがベストプラクティスとされているため、
	// Recreateになっている場合は「これで本当に大丈夫ですか？」とと確認としてWarning
	if deploy.Spec.Strategy.Type == appsv1.RecreateDeploymentStrategyType {
		resultsCh <- AuditResult{deploy.Namespace, "Deployment", deploy.Name, "Using Recreate strategy. RollingUpdate is recommended for zero-downtime", "Warning"}
	}
}

// Serviceの監査
func auditService(svc corev1.Service, pods []corev1.Pod, wg *sync.WaitGroup, resultsCh chan<- AuditResult) {
	defer wg.Done()

	// 1. 意図しない公開設定(NodePort / LoadBalancer)の検知
	// ServiceのSpec:「ポート80番へのアクセスを、このラベルを持つPodに流してほしい」
	// ServiceTypeNodePort,ServiceTypeLoadBalancer: KubernetesにおけるServiceの種類(外部への公開方法)
	// NodePort:k8sを動かしている各サーバー(Node)自体の特定のポート(例:30000番〜32767番)を開き、そこにアクセスが来たらPodへ通信を流す設定
	// LoadBalancer: GCPやAWSなどのクラウドプロバイダと連携し、クラウド上のロードバランサー(外部IPアドレス付き)を自動作成、そこからPodへ通信を流す設定
	// これらが設定されているということは、「そのシステムがクラスタの外部からアクセスできる状態になっている」ことを意味する。
	// 「意図して公開設定にしていますか？」と確認としてWarning
	if svc.Spec.Type == corev1.ServiceTypeNodePort || svc.Spec.Type == corev1.ServiceTypeLoadBalancer { // ||は論理和(..または..)
		resultsCh <- AuditResult{svc.Namespace, "Service", svc.Name, fmt.Sprintf("Service is exposed via %s. Verify if this is intended", svc.Spec.Type), "Warning"}
	}

	// 2. Selectorと合致するPodが存在するかチェック(迷子トラフィック防止)
	// Selector:Serviceの設定項目のひとつ。「app:webというラベルを持つPodにトラフィックを流す」
	if len(svc.Spec.Selector) > 0 {
		matched := false
		for _, pod := range pods {
			// 同一Namespace内でラベルが一致するか確認
			if pod.Namespace == svc.Namespace && labelsMatch(svc.Spec.Selector, pod.Labels) {
				matched = true
				break
			}
		}
		if !matched {
			resultsCh <- AuditResult{svc.Namespace, "Service", svc.Name, "No running Pods match this Service's selector", "Critical"}
		}
	}
}

// 補助関数: マップ(Selector)の全てのKey/Valueが、Podのラベルに存在するか判定
func labelsMatch(selector map[string]string, podLabels map[string]string) bool {
	if podLabels == nil {
		return false
	}
	for key, value := range selector {
		if podLabels[key] != value {
			return false
		}
	}
	return true
}

// 複数のCriticalな結果を1つのSlackメッセージにまとめて送信する
func sendSlackNotification(criticals []AuditResult) error {
	webhookURL := os.Getenv("SLACK_WEBHOOK_URL")
	// URLが設定されていなければサイレントにスキップ(ローカルテスト等で邪魔にならないため)
	if webhookURL == "" {
		return nil
	}

	// Slackのメッセージを組み立てる
	messageText := fmt.Sprintf("🚨 *Kubeguard Audit Alert*\nクラスタ内で `%d` 件のCriticalな問題を検出しました！\n", len(criticals))
	for _, c := range criticals {
		messageText += fmt.Sprintf("• [%s] %s `%s`: %s\n", c.Namespace, c.Kind, c.Name, c.Issue)
	}

	payload := SlackMessage{Text: messageText}
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %v", err)
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return fmt.Errorf("HTTP request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("slack api returned status code: %d", resp.StatusCode)
	}

	return nil
}

// 結果のフォーマット出力
func printResults(results []AuditResult, format string) {
	if len(results) == 0 {
		fmt.Println("No issues found. Cluster is healthy!")
		return
	}

	if format == "json" {
		jsonData, _ := json.MarshalIndent(results, "", "  ")
		fmt.Println(string(jsonData))
		return
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAMESPACE\tKIND\tNAME\tSEVERITY\tISSUE")
	for _, r := range results {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", r.Namespace, r.Kind, r.Name, r.Severity, r.Issue)
	}
	w.Flush()
}
