# Analytics Playground

ECの分析基盤を学ぶためのプロジェクト。現在は初期構成のみです。

## 起動

Docker Desktopを起動して `docker compose up -d` を実行します。業務DBはlocalhost:15432、分析DBはlocalhost:15433、Metabaseはlocalhost:3000です。DBユーザーはpostgres、パスワードはplaygroundです。Metabaseの接続先ホストは `analytics`、ポートは5432、データベース名はanalyticsです。

SQL初期化は空のボリュームに対してのみ実行されます。

## 未実装

レイクへの抽出、rawへのロード、DWH・マートの変換、Go API、可視化画面、テスト、Metabaseダッシュボード。
