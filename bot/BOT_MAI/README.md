# BOT_MAI

BOT謎で使用するHTTP BOT。表示名は「まい」。`BOT_MAI`へのメンションに続く2単語を4×4盤面上のナイト移動で解釈し、選ばれたコマンドをtraQ serverへ依頼して同じチャンネルへ結果を返す。

新規アカウントの作成時には、そのユーザーの `#general/2` に「BOTコマンド仕様書」のPNG版とPDF版を添付投稿する。

## 現在の入力と応答

入力はメンションの後に、有効な単語を2つ指定する。

```text
@BOT_MAI <1語目> <2語目>
```

1語目のナイト移動先にあるCOMMANDと、2語目のナイト移動先にあるTARGETの直積を解釈候補とする。候補が複数ならランダムに1件を実行する。ただし `reset BOT` は唯一の候補になった場合だけ選ぶ。

```text
@BOT_MAI file message
```

この入力は `reset BOT` だけに解釈できるため、BOTを復旧してゲームをクリアする。複数候補になる入力では、実際に選ばれた1件の結果だけを返す。

```text
@BOT_MAI BOT count
```

引数が2つでない、盤面に存在しない単語がある、または有効な移動先がない場合は構文エラーを返す。盤面と全移動候補は [`docs/q_bot.md`](../../../docs/q_bot.md) を参照する。

`reset BOT`でBOTを復旧した後は、そのユーザーの入力に限りナイト移動による変換を行わず、指定されたコマンドと対象をそのまま実行する。復旧状態はユーザーごとに保存される。

## コマンド実装の構成

解釈後の `COMMAND TARGET` を実行する共通基盤は `command_executor.go`、型定義は `command_types.go` に置く。COMMANDごとの処理は次のファイルに分け、各関数内で7種類のTARGETを扱う。

```text
command_count.go
command_list.go
command_open.go
command_send.go
command_delete.go
command_reset.go
command_debug.go
```

各COMMAND関数は共通の `POST /api/v3/qbot/commands` を呼ぶ。集計・一覧・削除・状態変更はserver側で実行し、`send` だけは応答本文とは別の投稿内容を受け取ってBOTが追加投稿する。これにより、BOTのAccess Tokenを持つプロセスと、DBやWebSocketを扱うserverの責務を分離している。

`open` と添付の疑似削除はserverから対象ユーザーだけへ送る `QBOT_ACTION` WebSocketイベントで専用クライアントに反映する。ゲームのクリア状態、最後の画面操作リビジョン、`message ID + file ID` 単位の疑似削除はDBに保存され、再接続・再起動後も `GET /api/v3/qbot/state` から復元される。

## 使用ライブラリとイベント

イベント受信には [traPtitech/traq-bot](https://github.com/traPtitech/traq-bot) v1.0.3を使用する。このライブラリは、traQから届くHTTPイベントの検証トークン確認とpayloadのデコードを担当する。traQへリクエストを送るAPIクライアントは含まれないため、メッセージ投稿部分はこのBOT内のHTTPクライアントで実装している。

traQのBOT設定では `MENTION_MESSAGE_CREATED` を購読する。現在のtraQ serverは、この購読設定を使ってメンション対象BOTを選んだ後、BOTのHTTP endpointには `X-TRAQ-BOT-EVENT: MESSAGE_CREATED` として配送する。そのため、コードでは `traqbot.EventHandlers.SetMessageCreatedHandler` を登録している。

## 環境変数

| 名前 | 必須 | 説明 | 例 |
|---|---|---|---|
| `TRAQ_API_BASE_URL` | 必須 | traQ API v3のベースURL。末尾を `/api/v3` にする | `http://localhost:3000/api/v3` |
| `TRAQ_PUBLIC_ORIGIN` | 任意 | 投稿する添付URLの、ブラウザから到達できるtraQオリジン。未指定時はAPI URLから推測する | `http://localhost:3000` |
| `TRAQ_BOT_ACCESS_TOKEN` | 通常起動時 | BOT詳細画面で発行されたAccess Token | — |
| `TRAQ_BOT_VERIFICATION_TOKEN` | 通常起動時 | BOT詳細画面で発行されたVerification Token | — |
| `TRAQ_BOT_AUTO_REGISTER` | 任意 | dev環境でBOTを自動発行・設定する。`true`の場合は手動トークンが不要 | `true` |
| `TRAQ_BOT_ENDPOINT` | 自動登録時 | traQ backendから到達できるBOTのHTTP endpoint | `http://bot-mai:8080` |
| `TRAQ_DEV_USER_NAME` | 自動登録時 | BOTを所有するdev専用ユーザー名 | `qbot_dev` |
| `TRAQ_DEV_USER_PASSWORD` | 自動登録時 | dev専用ユーザーのパスワード | — |
| `PORT` | 任意 | HTTP待受ポート。未指定時は`8080` | `8080` |

`TRAQ_BOT_AUTO_REGISTER`はローカルのCompose環境専用であり、デプロイ環境では使用しない。デプロイ時のトークンをリポジトリへコミットしないこと。

## dev環境での起動

serverリポジトリのルートで次を実行するだけで、ローカルclient、traQ server、BOTが起動する。`1m26_02-client` と `1m26_02-server` は同じ親ディレクトリに置く。

```bash
make up
```

Compose内ではBOTを `bot-mai` サービスとして起動する。BOTはtraQ APIの準備完了を待ち、次を自動で行う。

1. dev専用ユーザー `qbot_dev` を作成または再利用してログインする。
2. HTTP BOT `BOT_MAI` を初回だけ発行し、再起動時は既存BOTを再利用する。
3. 既存の購読イベントを残したまま `MENTION_MESSAGE_CREATED` を追加する。
4. 発行済みのAccess TokenとVerification Tokenを取得してBOTサーバーを起動する。
5. BOTを有効化する。

traQ backendからBOTへはCompose内の `http://bot-mai:8080` で接続するため、Cloudflare Tunnelやngrokは不要である。traQは <http://localhost:3000>、BOTの死活確認は <http://localhost:3003/healthz> で開ける。

死活確認:

```bash
curl -i http://localhost:3003/healthz
```

`204 No Content` が返れば起動している。traQ上で `@BOT_MAI file message` を投稿し、「BOTの復旧が完了しました。」と返ってクライアントにも復旧通知が出ることを確認する。

自動登録の進行状況は次で確認できる。

```bash
docker compose logs -f bot-mai
```

`qbot_dev`が既に存在し、Composeに設定されたものと異なるパスワードを持つ場合、自動ログインは失敗する。その場合は`compose.yaml`の`TRAQ_DEV_USER_NAME`と`TRAQ_DEV_USER_PASSWORD`を既存ユーザーに合わせる。

## テストとビルド

このディレクトリはserver本体とは独立したGo moduleになっている。

```bash
go test ./...
go build ./...
docker build -t bot-mai .
```

serverルートの `go test ./...` にはこの入れ子のmoduleは含まれないため、BOTのテストはこのディレクトリで別に実行する。

手動で`go run .`する場合は自動登録を無効にし、従来どおり `TRAQ_BOT_ACCESS_TOKEN` と `TRAQ_BOT_VERIFICATION_TOKEN` を設定する。

## NeoShowcaseでの起動

serverと同じGitHubリポジトリから、BOT用のApplicationを別に作成する。

| 設定 | 値 |
|---|---|
| Deploy Type | `Runtime` |
| Build Type | `Dockerfile` |
| Context | `bot/BOT_MAI` |
| Dockerfile | `Dockerfile` |
| Entrypoint | 空欄 |
| Command | 空欄 |
| HTTP Port | `8080` |
| Path | `/` |
| Strip Prefix | OFF |
| h2c | OFF |
| Auto Shutdown | OFF |
| Database | なし |

NeoShowcaseの環境変数には次を設定する。

```text
TRAQ_API_BASE_URL=https://<traQのホスト>/api/v3
TRAQ_BOT_ACCESS_TOKEN=<Access Token>
TRAQ_BOT_VERIFICATION_TOKEN=<Verification Token>
PORT=8080
```

Access TokenとVerification TokenはSecretとして登録する。

デプロイ手順:

1. BOT用Applicationを作成して公開URLを確定する。
2. traQでHTTP BOTを作成し、EndpointにNeoShowcaseの公開URLを設定する。
3. `MENTION_MESSAGE_CREATED` を購読する。
4. 発行された2つのトークンをNeoShowcaseの環境変数へ設定する。
5. Applicationを再デプロイし、Auto ShutdownをOFFにする。
6. traQ側でBOTを有効化し、メンション応答を確認する。

公開URLのrootをtraQのEndpointへ設定する。`/healthz`はNeoShowcaseや外部監視からの死活確認に利用できる。
