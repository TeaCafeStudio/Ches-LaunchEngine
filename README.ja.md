# ChesLaunchEngine
[![GitHub Stars](https://img.shields.io/github/stars/TeaCafeStudio/Ches-LaunchEngine?style=flat-square)](https://github.com/TeaCafeStudio/Ches-LaunchEngine/stargazers)
[![GitHub Forks](https://img.shields.io/github/forks/TeaCafeStudio/Ches-LaunchEngine?style=flat-square)](https://github.com/TeaCafeStudio/Ches-LaunchEngine/network/members)
[![GitHub Issues](https://img.shields.io/github/issues/TeaCafeStudio/Ches-LaunchEngine?style=flat-square)](https://github.com/TeaCafeStudio/Ches-LaunchEngine/issues)
[![GitHub Releases](https://img.shields.io/github/v/release/TeaCafeStudio/Ches-LaunchEngine?style=flat-square)](https://github.com/TeaCafeStudio/Ches-LaunchEngine/releases)

> **言語:** [English](README.md) · [简体中文](README.zh-CN.md) · [日本語](README.ja.md)

ChesLaunchEngine（CLE）は、Minecraft: Java Edition 向けのモジュラーで
クロスプラットフォームな Go バックエンドです。グラフィカルランチャーでは
なく、エンジンです。

## プログラム

- `cle`: バージョン表示、設定検証、Minecraft 起動を行う正式なワンショット CLI。
- `cle-debug`: 対話コマンド、アカウントプロファイル管理、ライブ起動イベント、
  ログ、タスク、プロセスを扱う独立した開発者向けターミナル。
- `pkg/sdk`: 共有エンジンを利用する Go 埋め込み API。

2 つの実行ファイルは同じ SDK/Core エンジンを使用します。`cmd/cle` は
ターミナルパッケージに依存しません。旧 `cle debug` 形式は移行メッセージを
表示します。`cle-debug` を使用してください。

## 現在のサポート

- Vanilla 起動とオフラインプロファイル。
- バージョンメタデータ、Java ランタイム検出、検証済みライブラリ/アセット、
  起動引数の組み立て、管理対象プロセス、Debug Terminal のイベント出力。
- オフライン、サードパーティ Yggdrasil、Microsoft（プレミアム）認証。
  オフラインおよび Yggdrasil/Microsoft の資格情報は OS のキーリングに保存
  されます。Mod ローダーと HTTP サーバー/API は未実装です。オフライン
  プロファイルはオンラインモードのサーバーで認証できません。
- Microsoft アカウントは OAuth 2.0 Device Code フロー、Xbox Live、XSTS、
  Minecraft Services ログイン、権利確認、ゲームプロファイル取得を使用します。
  Microsoft チェーンには `account.microsoft.clientId` または
  `CLE_MICROSOFT_CLIENT_ID` 環境変数で設定する Microsoft 公開クライアント
  アプリケーション ID が必要です。CLE は Client ID や Client Secret を
  ハードコードしません。
- Yggdrasil 起動では、公式 HTTPS 配布エンドポイントからダウンロードし
  SHA-256 検証した固定バージョンの Authlib Injector 1.2.8 エージェントを
  注入します。互換性は Minecraft/Authlib のバージョンと Yggdrasil サーバーに
  依存します。
- Yggdrasil および Microsoft のプロトコル、アカウント解決、JVM 引数の組み立て
  はモックサーバーによる自動テストでカバーされています。実際の認証、Minecraft
  メインメニューへの到達、OS 資格情報ストアの実行時挙動は未検証です。

## クイックスタート

リポジトリのルートから両プログラムをビルドします：

```powershell
go build -o cle.exe .\cmd\cle
go build -o cle-debug.exe .\cmd\cle-debug
```

```sh
go build -o cle ./cmd/cle
go build -o cle-debug ./cmd/cle-debug
```

バージョンを確認し、CLE 設定を検証します：

```text
cle -version
cle -check-config cle.json
```

[`examples/launch/vanilla.json`](examples/launch/vanilla.json) をマシンの
Java/ゲームパスに合わせて編集し、起動します：

```text
cle -launch ./examples/launch/vanilla.json
```

開発者ターミナルを起動し、オフライン、Yggdrasil、または Microsoft アカウント
を追加します：

```text
cle-debug
account add offline Player_Name
```

Microsoft アカウントは OAuth Device Code フローを使用し、Microsoft 公開
クライアントアプリケーション ID が必要です
（[アカウント](docs/en-us/configuration/accounts.md)を参照）：

```text
account add microsoft
```

Yggdrasil アカウントのパスワードは、非表示の対話プロンプトからのみ読み取られ
ます。ログインウェブページではなく、サーバーの Yggdrasil API ベース URL
（`/authserver/authenticate` に使うベースと同じもの）を使用してください：

```text
account add yggdrasil https://auth.example.org login-name
```

Yggdrasil サービスは HTTPS を使用する必要があります。HTTP は、プログラムでの
利用時に明示的に有効化したループバックテストサーバーでのみ許可され、CLI では
許可されません。初回の Yggdrasil 起動では Authlib Injector 1.2.8 がゲーム
ディレクトリの検証済みライブラリキャッシュにダウンロードされます。オフライン
起動には、その検証済み JAR が既に存在する必要があります。実際の Yggdrasil
アカウントやサービスは同梱されません。

ターミナルの詳細については `help` を実行するか、
[Debug Terminal コマンドガイド](docs/en-us/debug/commands.md)を参照してください。

## CLI JSON 出力

対応するワンショット CLI コマンドは `--json` を受け付けます。stdout には
1 つの JSON レスポンスが含まれ、診断情報は stderr に残ります：

```text
cle --json -version
cle --json -check-config cle.json
cle --json -config cle.json -launch launch.json
```

レスポンススキーマとエラーコードの動作については
[CLI JSON output](docs/en-us/cli/json-output.md) を参照してください。

## 設定とドキュメント

開発者向けドキュメントは [简体中文](docs/zh-cn/README.md)、
[English](docs/en-us/README.md)、[繁體中文](docs/zh-tw/README.md) で利用できます。

CLE ランタイム設定は [`configs/cle.schema.json`](configs/cle.schema.json) で
定義されます。起動リクエストは `pkg/models.LaunchRequest` で表される別の
JSON ドキュメントです。

[開発者向けドキュメントインデックス](docs/README.md) には、クイックスタート、
インストール、現在の CLI および Debug Terminal コマンド、起動/設定ガイド、
JSON 出力、Go SDK、ビルド/コントリビューションのメモへのリンクがあります。
HTTP API は **計画中 / 未実装** と明記されており、`api.enabled` を設定しても
サーバーは起動しません。

## ビルドと検証

```powershell
go test ./...
go vet ./...
.\scripts\build.ps1 all
```

```sh
go test ./...
go vet ./...
./scripts/build.sh all
```

ビルドスクリプトは、Windows、Linux、macOS の amd64 と arm64 向けに、CGO を
無効にして `cle` と `cle-debug` のバイナリを個別に生成します。
[開発/ビルド](docs/en-us/development/build.md)を参照してください。
