# Analytics Playground

架空のECを題材に、業務DB → データレイク → raw → DWH → データマート → BIを観察する学習用プロジェクトです。

## 起動

Docker Desktopを起動し、次を実行します。

```sh
docker compose up --build -d
```

http://localhost:8080 を開き、「すべて実行」を押します。画面の各ボタンから段階的にも実行できます。抽出時には一貫した業務DBのスナップショットをレイクにJSONとして保存します。最新バッチをrawに読み込み、SQLでDWHとマートを全件再構築します。各段階は独立しているため、前段階の変更は明示的な実行まで反映されません。

返品追加ボタンを押し、業務DB → レイク → raw → DWH → マートの順に実行して変化を確認してください。実行後に同じ処理を再実行しても二重計上されません。レイクはバッチごとに追記され、上書きされません。集計は日本時間、売上は決済日、返品は返品日に計上します。商品別売上は返品控除前です。

## BI

### 初期設定と分析DBの接続

1. http://localhost:8080 で「すべて実行」を押し、マートまでの処理を完了させます。
2. http://localhost:3000 を開き、Metabaseの初期設定で管理者アカウントを作成します。ローカル学習用のサンプルとして、メールアドレスは `admin@example.com`、パスワードは `Playground123!` を入力します。後述のダッシュボード作成時にも同じ値を使用します。
3. MetabaseにPostgreSQLデータベースを追加し、次の接続情報を入力します。初期設定を済ませている場合は、管理者設定のデータベース追加から登録します。

| 項目 | 入力値 |
| --- | --- |
| データベースの種類 | PostgreSQL |
| 表示名 | `analytics` |
| ホスト | `analytics` |
| ポート | `5432` |
| データベース名 | `analytics` |
| ユーザー名 | `postgres` |
| パスワード | `playground` |

MetabaseはDocker内から接続するため、ホストには `analytics`、ポートには `5432` を指定します。

### ダッシュボードの自動作成

ローカルにGo 1.23以降が必要です。リポジトリのルートで次を実行します。以前の `python3 scripts/metabase_dashboard.py` はGo版に置き換わっています。

```sh
go run ./cmd/metabase-dashboard
```

入力の流れは次のとおりです。

1. **Metabase 管理者メールアドレス**: 初期設定で作成した管理者のメールアドレス（サンプル: `admin@example.com`）を入力します。
2. **パスワード**: その管理者のパスワード（サンプル: `Playground123!`）を入力します。入力中は文字が表示されません。DB接続用の `playground` とは別の認証情報です。
3. **分析DBのID**: ログイン後に表示される一覧から、上記の分析DBとして登録した接続の左側の数字を入力します。

例えば、表示名を `analytics` として登録したDBが `2: analytics` と表示された場合は、`2` を入力してEnterを押します。

```text
Metabase 管理者メールアドレス: admin@example.com
パスワード:
2: analytics
分析DBのID: 2
```

`2` はMetabaseが割り当てたDBのID、`analytics` は登録時に付けた表示名です。IDは環境によって異なるため、実際に表示された番号を指定してください。複数の候補がある場合は、分析DB `analytics` に接続しているものを選びます。

完了すると `作成しました: http://localhost:3000/dashboard/…` と表示されます。そのURLを開くと、日別売上と返品・商品別売上・月別リピート率の3つのグラフを確認できます。再実行すると別のダッシュボードが作成されます。

手動で質問やダッシュボードを作成する場合は、`mart.daily_sales`、`mart.daily_product_sales`、`mart.monthly_customer_activity` を使用します。

## 制約

業務DBからの抽出は全件スナップショット方式です。rawの差分更新は、最新スナップショットと既存rawをIDで比較し、変更・削除を反映します。変更されない行のバッチIDは以前の値を保ちます。複数回実行しても同じ結果になります。差分更新後はDWHを更新してください。過去バッチ指定、業務DBからの差分抽出、認証、大量データ向けの効率化は未実装です。データとMetabaseの設定はDockerボリュームに保存されます。`docker compose down -v` はデータを削除します。

期間指定の再集計は開始日・終了日を含む日本時間の日付で指定します。日別マートは指定日だけを、月別マートは指定日に重なる月全体を再集計します。先にDWHを最新化してください。

## 構成

- `cmd/server/`: Webサーバーの実行入口とHTTP API。
- `cmd/metabase-dashboard/`: Metabaseダッシュボード作成コマンド。
- `internal/pipeline/`: 抽出・ロード・集計、各層の参照、実行履歴。PostgreSQLにはpsql経由で接続。
- `internal/metabase/`: Metabase APIとの通信処理。
- `sql/warehouse.sql`: rawから分析用明細へ変換。
- `sql/marts.sql`: DWHから日別売上へ集計。
- `web/`: 各層の表示と手動実行画面。

## 実験と指標

「サンプルデータを追加」で1,000顧客、20商品、約1万注文を作成します。既存IDは上書きせず、繰り返し実行できます。その後に抽出から実行してください。注文IDを入力すると各層を追跡できます。マートは日別集計なので、その日の他の注文も合算されます。

`mart.monthly_customer_activity` のリピート率は、同じ月に2回以上注文した顧客数を、その月の購入顧客数で割ったものです。処理履歴はレイクボリュームの `runs.json` に最新100件を保存します。

WebサーバーをDocker外で起動する場合は、Go 1.23以降と `psql` を用意し、`docker compose up -d source analytics` でDBを起動してから、リポジトリのルートで `go run ./cmd/server` を実行します。`web/` と `sql/` を相対パスで参照するため、作業ディレクトリはルートにしてください。Dockerのappサービスが起動中の場合は、先に `docker compose stop app` で8080番ポートを空けます。

## Goのテスト

`go test ./...` は通常テストです。結合テストは新規のDockerボリュームで、`go test -tags=integration ./tests -run ^TestPipeline$ -v` を実行します。Metabaseの初期化テストは `-run ^TestMetabase$` で別途実行します。既存データには実行しないでください。Pythonは不要です。対話ツールのパスワード非表示入力にはmacOS/Linuxのsttyを使用します。
