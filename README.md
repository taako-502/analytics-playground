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

http://localhost:3000 でMetabaseの初期設定を行い、PostgreSQLデータベースを追加します。ホスト `analytics`、ポート `5432`、DB `analytics`、ユーザー `postgres`、パスワード `playground`。`mart.daily_sales`、`mart.daily_product_sales` を使い、質問とダッシュボードを作成します。ダッシュボードは自動作成されません。

## 制約

学習用の全件スナップショット方式です。レイクからの読み込みは最新バッチのみで、過去バッチの指定、差分更新、実行履歴、認証、大量データ向けの効率化は未実装です。データとMetabaseの設定はDockerボリュームに保存されます。`docker compose down -v` はデータを削除します。

## 構成

- `main.go`: Go HTTPサーバー、抽出・ロード・処理実行。PostgreSQLにはpsql経由で接続。
- `sql/warehouse.sql`: rawから分析用明細へ変換。
- `sql/marts.sql`: DWHから日別売上へ集計。
- `web/`: 各層の表示と手動実行画面。
