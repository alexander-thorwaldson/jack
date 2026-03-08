//go:build testing

package msg

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	jtesting "github.com/zoobzio/jack/testing"
)

func TestClientRegister(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jtesting.AssertEqual(t, r.URL.Path, "/_matrix/client/v3/register")
		jtesting.AssertEqual(t, r.Method, http.MethodPost)
		json.NewEncoder(w).Encode(Registration{
			UserID:      "@agent:localhost",
			AccessToken: "tok_123",
			DeviceID:    "DEV1",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	reg, err := client.Register("agent", "pass", "jack")
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, reg.UserID, "@agent:localhost")
	jtesting.AssertEqual(t, reg.AccessToken, "tok_123")
}

func TestClientLogin(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jtesting.AssertEqual(t, r.URL.Path, "/_matrix/client/v3/login")
		json.NewEncoder(w).Encode(Registration{
			UserID:      "@operator:localhost",
			AccessToken: "tok_456",
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	reg, err := client.Login("operator", "pass")
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, reg.UserID, "@operator:localhost")
	jtesting.AssertEqual(t, reg.AccessToken, "tok_456")
}

func TestClientSend(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jtesting.AssertEqual(t, r.Method, http.MethodPut)
		json.NewEncoder(w).Encode(map[string]string{"event_id": "$evt1"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "tok_abc")
	eventID, err := client.Send("!room:localhost", "hello")
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, eventID, "$evt1")
}

func TestClientMessages(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jtesting.AssertEqual(t, r.Method, http.MethodGet)
		json.NewEncoder(w).Encode(Messages{
			Chunk: []Message{
				{Sender: "@bob:localhost", Type: "m.room.message", Content: map[string]interface{}{"body": "hi"}},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "tok_abc")
	msgs, err := client.Messages("!room:localhost", 10)
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, len(msgs.Chunk), 1)
	jtesting.AssertEqual(t, msgs.Chunk[0].Sender, "@bob:localhost")
}

func TestClientErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(matrixError{ErrCode: "M_FORBIDDEN", Error: "not allowed"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	_, err := client.Login("x", "y")
	jtesting.AssertError(t, err)
}

func TestTokenFromEnv(t *testing.T) {
	t.Setenv("JACK_MSG_TOKEN", "tok_env")
	token, err := TokenFromEnv()
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, token, "tok_env")
}

func TestTokenFromEnvMissing(t *testing.T) {
	t.Setenv("JACK_MSG_TOKEN", "")
	_, err := TokenFromEnv()
	jtesting.AssertError(t, err)
}

func TestTokenFromEnvFile(t *testing.T) {
	t.Setenv("JACK_MSG_TOKEN", "")
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, ".env"), []byte("export JACK_MSG_TOKEN=tok_file\nexport JACK_TEAM=blue\n"), 0o600)
	t.Chdir(dir)
	token, err := TokenFromEnv()
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, token, "tok_file")
}

func TestServerName(t *testing.T) {
	jtesting.AssertEqual(t, ServerName("http://localhost:8008"), "localhost")
	jtesting.AssertEqual(t, ServerName("https://matrix.example.com"), "matrix.example.com")
	jtesting.AssertEqual(t, ServerName("https://matrix.example.com:8448"), "matrix.example.com")
}

func TestClientGetProfile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jtesting.AssertEqual(t, r.Method, http.MethodGet)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"displayname":"Alice"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "tok_abc")
	err := client.GetProfile("@alice:localhost")
	jtesting.AssertNoError(t, err)
}

func TestClientGetProfileNotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"errcode":"M_NOT_FOUND","error":"User not found"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "tok_abc")
	err := client.GetProfile("@ghost:localhost")
	jtesting.AssertError(t, err)
}

func TestClientSetRoomAlias(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jtesting.AssertEqual(t, r.Method, http.MethodPut)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
	}))
	defer server.Close()

	client := NewClient(server.URL, "tok_abc")
	err := client.SetRoomAlias("#dm-alice-bob:localhost", "!room:localhost")
	jtesting.AssertNoError(t, err)
}

func TestClientEventContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"end": "tok_end"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "tok_abc")
	token, err := client.EventContext("!room:localhost", "$evt1")
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, token, "tok_end")
}

func TestClientMessagesFrom(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(Messages{
			Chunk: []Message{
				{Sender: "@alice:localhost", Type: "m.room.message", Content: map[string]interface{}{"body": "new"}},
			},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "tok_abc")
	msgs, err := client.MessagesFrom("!room:localhost", "tok_start", 10, "f")
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, len(msgs.Chunk), 1)
}

func TestClientWhoAmI(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jtesting.AssertEqual(t, r.URL.Path, "/_matrix/client/v3/account/whoami")
		json.NewEncoder(w).Encode(WhoAmIResponse{UserID: "@blue-vicky:localhost"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "tok_abc")
	resp, err := client.WhoAmI()
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, resp.UserID, "@blue-vicky:localhost")
}

func TestClientResolveAlias(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(AliasResponse{RoomID: "!abc:localhost"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "tok_abc")
	resp, err := client.ResolveAlias("#general:localhost")
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, resp.RoomID, "!abc:localhost")
}

func TestClientJoin(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jtesting.AssertEqual(t, r.Method, http.MethodPost)
		json.NewEncoder(w).Encode(map[string]string{"room_id": "!abc:localhost"})
	}))
	defer server.Close()

	client := NewClient(server.URL, "tok_abc")
	roomID, err := client.Join("!abc:localhost")
	jtesting.AssertNoError(t, err)
	jtesting.AssertEqual(t, roomID, "!abc:localhost")
}
