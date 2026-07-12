# ---------------------------------------------------
# 1. ビルド用ステージ (builder)
# ---------------------------------------------------
FROM golang:1.26.1-alpine AS builder

WORKDIR /app

# モジュールファイルのコピーと依存関係のダウンロード
# (同じディレクトリに go.mod と go.sum がある想定)
COPY go.mod go.sum ./
RUN go mod download

# ソースコードをコピー
COPY . .

# CGOを無効化し、静的リンクされたバイナリをビルド
# CGO_ENABLED=0:Goのプログラムをコンパイルする際、C言語のライブラリに依存しない「完全な静的リンクバイナリ」を作成する設定。
# 実行環境(Alpine Linux)のOS標準ライブラリの違いによる動作不良を防ぐことができる。
# GOOSとGOARCHを指定し、Linux環境で確実に動くようにする
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o kubeguard main.go

# ---------------------------------------------------
# 2. 実行用ステージ (runtime)
# ---------------------------------------------------
# 実行環境は最小限の構成にしてセキュリティを高める
FROM alpine:latest

# SlackのWebhook(HTTPS)と通信するためにルート証明書を追加
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# ビルド用ステージから完成したバイナリだけをコピー
COPY --from=builder /app/kubeguard .

# 実行権限を付与
RUN chmod +x ./kubeguard

# コンテナ起動時のデフォルトコマンド
ENTRYPOINT ["./kubeguard"]
