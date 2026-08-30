package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"slices"
	"strings"
	"testing"
)

func TestDevProvisionerCreatesAndActivatesBot(t *testing.T) {
	t.Parallel()

	var subscribed []string
	provisioner := mustDevProvisioner(t, "http://traq.test/api/v3")
	provisioner.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.Method + " " + r.URL.Path {
		case "POST /api/v3/users":
			return testResponse(http.StatusCreated, `{"id":"dev-user"}`), nil
		case "POST /api/v3/login":
			res := testResponse(http.StatusNoContent, "")
			res.Header.Add("Set-Cookie", "r_session=session; Path=/; HttpOnly")
			return res, nil
		case "GET /api/v3/users":
			requireSessionCookie(t, r)
			if r.URL.Query().Get("name") != devBotUserName {
				t.Errorf("name query = %q", r.URL.Query().Get("name"))
			}
			return testResponse(http.StatusOK, `[]`), nil
		case "POST /api/v3/bots":
			requireSessionCookie(t, r)
			var body map[string]string
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode create bot request: %v", err)
			}
			if body["endpoint"] != "http://bot-traq:8080" {
				t.Errorf("endpoint = %q", body["endpoint"])
			}
			return testResponse(http.StatusCreated, `{
                    "id":"bot-id",
                    "botUserId":"bot-user-id",
                    "subscribeEvents":[],
                    "tokens":{"accessToken":"access-token","verificationToken":"verification-token"}
                }`), nil
		case "PATCH /api/v3/bots/bot-id":
			requireSessionCookie(t, r)
			var body struct {
				Endpoint        string   `json:"endpoint"`
				SubscribeEvents []string `json:"subscribeEvents"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode patch bot request: %v", err)
			}
			subscribed = body.SubscribeEvents
			if body.Endpoint != "http://bot-traq:8080" {
				t.Errorf("endpoint = %q", body.Endpoint)
			}
			return testResponse(http.StatusNoContent, ""), nil
		case "POST /api/v3/bots/bot-id/actions/activate":
			requireSessionCookie(t, r)
			return testResponse(http.StatusAccepted, ""), nil
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
			return nil, nil
		}
	})

	bot, err := provisioner.provision(context.Background())
	if err != nil {
		t.Fatalf("provision() error = %v", err)
	}
	if bot.ID != "bot-id" || bot.AccessToken != "access-token" || bot.VerificationToken != "verification-token" {
		t.Errorf("provisioned bot = %+v", bot)
	}
	if !slices.Equal(subscribed, []string{mentionEvent}) {
		t.Errorf("subscribeEvents = %v", subscribed)
	}
	if err := provisioner.activate(context.Background(), bot.ID); err != nil {
		t.Fatalf("activate() error = %v", err)
	}
}

func TestDevProvisionerReusesBotAndPreservesSubscriptions(t *testing.T) {
	t.Parallel()

	var subscribed []string
	provisioner := mustDevProvisioner(t, "http://traq.test/api/v3")
	provisioner.httpClient.Transport = roundTripFunc(func(r *http.Request) (*http.Response, error) {
		switch r.Method + " " + r.URL.Path {
		case "POST /api/v3/users":
			return testResponse(http.StatusConflict, ""), nil
		case "POST /api/v3/login":
			res := testResponse(http.StatusNoContent, "")
			res.Header.Add("Set-Cookie", "r_session=session; Path=/; HttpOnly")
			return res, nil
		case "GET /api/v3/users":
			return testResponse(http.StatusOK, `[{"id":"bot-user-id","name":"BOT_traq"}]`), nil
		case "GET /api/v3/bots":
			return testResponse(http.StatusOK, `[{"id":"bot-id","botUserId":"bot-user-id","subscribeEvents":["MESSAGE_CREATED"]}]`), nil
		case "PATCH /api/v3/bots/bot-id":
			var body struct {
				Endpoint        string   `json:"endpoint"`
				SubscribeEvents []string `json:"subscribeEvents"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatalf("decode patch bot request: %v", err)
			}
			subscribed = body.SubscribeEvents
			if body.Endpoint != "http://bot-traq:8080" {
				t.Errorf("endpoint = %q", body.Endpoint)
			}
			return testResponse(http.StatusNoContent, ""), nil
		case "GET /api/v3/bots/bot-id":
			if r.URL.Query().Get("detail") != "true" {
				t.Errorf("detail query = %q", r.URL.Query().Get("detail"))
			}
			return testResponse(http.StatusOK, `{
                    "id":"bot-id",
                    "botUserId":"bot-user-id",
                    "endpoint":"https://old.example.com",
                    "subscribeEvents":["MESSAGE_CREATED"],
                    "tokens":{"accessToken":"existing-access","verificationToken":"existing-verification"}
                }`), nil
		default:
			t.Fatalf("unexpected request: %s %s", r.Method, r.URL.String())
			return nil, nil
		}
	})

	bot, err := provisioner.provision(context.Background())
	if err != nil {
		t.Fatalf("provision() error = %v", err)
	}
	if bot.AccessToken != "existing-access" || bot.VerificationToken != "existing-verification" {
		t.Errorf("provisioned bot = %+v", bot)
	}
	if !slices.Equal(subscribed, []string{"MENTION_MESSAGE_CREATED", "MESSAGE_CREATED"}) {
		t.Errorf("subscribeEvents = %v", subscribed)
	}
}

func testResponse(status int, body string) *http.Response {
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func mustDevProvisioner(t *testing.T, baseURL string) *devProvisioner {
	t.Helper()
	p, err := newDevProvisioner(config{
		APIBaseURL:      baseURL,
		BotEndpoint:     "http://bot-traq:8080",
		DevUserName:     "qbot_dev",
		DevUserPassword: "qbot-dev-password",
	})
	if err != nil {
		t.Fatalf("newDevProvisioner() error = %v", err)
	}
	return p
}

func requireSessionCookie(t *testing.T, r *http.Request) {
	t.Helper()
	cookie, err := r.Cookie("r_session")
	if err != nil || cookie.Value != "session" {
		t.Errorf("session cookie = %v, err = %v", cookie, err)
	}
}
