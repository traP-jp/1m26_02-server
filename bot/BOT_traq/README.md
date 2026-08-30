# BOT_traq

BOT謎で使用するHTTP BOT。現在は `BOT_traq` をメンションすると、同じチャンネルへ「呼びましたか？」と投稿する。

## 使用ライブラリとイベント

イベント受信には [traPtitech/traq-bot](https://github.com/traPtitech/traq-bot) v1.0.3を使用する。このライブラリは、traQから届くHTTPイベントの検証トークン確認とpayloadのデコードを担当する。traQへリクエストを送るAPIクライアントは含まれないため、メッセージ投稿部分はこのBOT内のHTTPクライアントで実装している。

traQのBOT設定では `MENTION_MESSAGE_CREATED` を購読する。現在のtraQ serverは、この購読設定を使ってメンション対象BOTを選んだ後、BOTのHTTP endpointには `X-TRAQ-BOT-EVENT: MESSAGE_CREATED` として配送する。そのため、コードでは `traqbot.EventHandlers.SetMessageCreatedHandler` を登録している。

## 環境変数

| 名前 | 必須 | 説明 | 例 |
|---|---|---|---|
| `TRAQ_API_BASE_URL` | 必須 | traQ API v3のベースURL。末尾を `/api/v3` にする | `http://localhost:3000/api/v3` |
| `TRAQ_BOT_ACCESS_TOKEN` | 通常起動時 | BOT詳細画面で発行されたAccess Token | — |
| `TRAQ_BOT_VERIFICATION_TOKEN` | 通常起動時 | BOT詳細画面で発行されたVerification Token | — |
| `TRAQ_BOT_AUTO_REGISTER` | 任意 | dev環境でBOTを自動発行・設定する。`true`の場合は手動トークンが不要 | `true` |
| `TRAQ_BOT_ENDPOINT` | 自動登録時 | traQ backendから到達できるBOTのHTTP endpoint | `http://bot-traq:8080` |
| `TRAQ_DEV_USER_NAME` | 自動登録時 | BOTを所有するdev専用ユーザー名 | `qbot_dev` |
| `TRAQ_DEV_USER_PASSWORD` | 自動登録時 | dev専用ユーザーのパスワード | — |
| `PORT` | 任意 | HTTP待受ポート。未指定時は`8080` | `8080` |

`TRAQ_BOT_AUTO_REGISTER`はローカルのCompose環境専用であり、デプロイ環境では使用しない。デプロイ時のトークンをリポジトリへコミットしないこと。

## dev環境での起動

serverリポジトリのルートで次を実行するだけで、traQとBOTが起動する。

```bash
make up
```

Compose内ではBOTを `bot-traq` サービスとして起動する。BOTはtraQ APIの準備完了を待ち、次を自動で行う。

1. dev専用ユーザー `qbot_dev` を作成または再利用してログインする。
2. HTTP BOT `BOT_traq` を初回だけ発行し、再起動時は既存BOTを再利用する。
3. 既存の購読イベントを残したまま `MENTION_MESSAGE_CREATED` を追加する。
4. 発行済みのAccess TokenとVerification Tokenを取得してBOTサーバーを起動する。
5. BOTを有効化する。

traQ backendからBOTへはCompose内の `http://bot-traq:8080` で接続するため、Cloudflare Tunnelやngrokは不要である。traQは <http://localhost:3000>、BOTの死活確認は <http://localhost:3003/healthz> で開ける。

死活確認:

```bash
curl -i http://localhost:3003/healthz
```

`204 No Content` が返れば起動している。traQ上で `@BOT_traq` を含むメッセージを投稿し、「呼びましたか？」と返ることを確認する。

自動登録の進行状況は次で確認できる。

```bash
docker compose logs -f bot-traq
```

`qbot_dev`が既に存在し、Composeに設定されたものと異なるパスワードを持つ場合、自動ログインは失敗する。その場合は`compose.yaml`の`TRAQ_DEV_USER_NAME`と`TRAQ_DEV_USER_PASSWORD`を既存ユーザーに合わせる。

## テストとビルド

このディレクトリはserver本体とは独立したGo moduleになっている。

```bash
go test ./...
go build ./...
docker build -t bot-traq .
```

serverルートの `go test ./...` にはこの入れ子のmoduleは含まれないため、BOTのテストはこのディレクトリで別に実行する。

手動で`go run .`する場合は自動登録を無効にし、従来どおり `TRAQ_BOT_ACCESS_TOKEN` と `TRAQ_BOT_VERIFICATION_TOKEN` を設定する。

## NeoShowcaseでの起動

serverと同じGitHubリポジトリから、BOT用のApplicationを別に作成する。

| 設定 | 値 |
|---|---|
| Deploy Type | `Runtime` |
| Build Type | `Dockerfile` |
| Context | `bot/BOT_traq` |
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
