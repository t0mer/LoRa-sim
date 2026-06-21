package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/t0mer/cylon/internal/db"
	"github.com/t0mer/cylon/internal/secret"
	"github.com/t0mer/cylon/internal/store"
)

func testRouter(t *testing.T) (http.Handler, *store.Store) {
	t.Helper()
	database, err := db.Open(filepath.Join(t.TempDir(), "c.db"))
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	if err := db.Migrate(database, "up"); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	cipher, _ := secret.New("000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f")
	st := store.New(database, cipher)
	if _, err := st.Gateway().EnsureEUI(context.Background(), "0102030405060708", ""); err != nil {
		t.Fatalf("ensure eui: %v", err)
	}
	a := NewAPI(st, NewHub(), nil, nil, "1.2.3", "0102030405060708")
	return NewRouter(a, nil), st
}

func do(t *testing.T, h http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, rdr)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHealthz(t *testing.T) {
	h, _ := testRouter(t)
	rec := do(t, h, "GET", "/healthz", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var resp HealthResponse
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Status != "ok" || resp.Version != "1.2.3" || resp.EUI != "0102030405060708" {
		t.Errorf("health = %+v", resp)
	}
}

func TestGatewayGet(t *testing.T) {
	h, _ := testRouter(t)
	rec := do(t, h, "GET", "/api/gateway", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	var dto gatewayDTO
	json.Unmarshal(rec.Body.Bytes(), &dto)
	if dto.EUI != "0102030405060708" || dto.Status != "disabled" {
		t.Errorf("gateway = %+v", dto)
	}
}

func TestTagCRUDAndMasking(t *testing.T) {
	h, _ := testRouter(t)

	// Create
	rec := do(t, h, "POST", "/api/tags", createTagReq{
		DevEUI: "0101010101010101", JoinEUI: "0202020202020202",
		AppKey: "00112233445566778899aabbccddeeff", DefaultDR: 5,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d: %s", rec.Code, rec.Body)
	}
	var created []tagDTO
	json.Unmarshal(rec.Body.Bytes(), &created)
	if len(created) != 1 {
		t.Fatalf("created %d tags", len(created))
	}
	if created[0].AppKeyMasked != "****eeff" {
		t.Errorf("AppKeyMasked = %q, want ****eeff", created[0].AppKeyMasked)
	}

	// List
	rec = do(t, h, "GET", "/api/tags", nil)
	var list []tagDTO
	json.Unmarshal(rec.Body.Bytes(), &list)
	if len(list) != 1 {
		t.Errorf("list len = %d", len(list))
	}

	// Get detail includes session
	id := created[0].ID
	rec = do(t, h, "GET", "/api/tags/"+itoa(id), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("get status = %d", rec.Code)
	}
	var detail tagDTO
	json.Unmarshal(rec.Body.Bytes(), &detail)
	if detail.Session == nil || detail.Session.Joined {
		t.Errorf("session = %+v, want present and not joined", detail.Session)
	}

	// Delete
	rec = do(t, h, "DELETE", "/api/tags/"+itoa(id), nil)
	if rec.Code != http.StatusNoContent {
		t.Errorf("delete status = %d", rec.Code)
	}
	rec = do(t, h, "GET", "/api/tags/"+itoa(id), nil)
	if rec.Code != http.StatusNotFound {
		t.Errorf("get after delete = %d, want 404", rec.Code)
	}
}

func TestFleetCreate(t *testing.T) {
	h, _ := testRouter(t)
	rec := do(t, h, "POST", "/api/tags", createTagReq{Count: 5, DefaultDR: 3})
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body)
	}
	var created []tagDTO
	json.Unmarshal(rec.Body.Bytes(), &created)
	if len(created) != 5 {
		t.Fatalf("fleet created %d, want 5", len(created))
	}
	// Auto-generated DevEUIs must be distinct.
	seen := map[string]bool{}
	for _, c := range created {
		if seen[c.DevEUI] {
			t.Errorf("duplicate DevEUI %s", c.DevEUI)
		}
		seen[c.DevEUI] = true
	}
}

func TestJoinWithoutGatewayIs503(t *testing.T) {
	h, _ := testRouter(t)
	rec := do(t, h, "POST", "/api/tags", createTagReq{DefaultDR: 5})
	var created []tagDTO
	json.Unmarshal(rec.Body.Bytes(), &created)
	rec = do(t, h, "POST", "/api/tags/"+itoa(created[0].ID)+"/join", nil)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("join without gateway = %d, want 503", rec.Code)
	}
}

func TestEventsEndpoint(t *testing.T) {
	h, st := testRouter(t)
	st.Events().Append(context.Background(), store.Event{Direction: "up", Kind: "data"})
	rec := do(t, h, "GET", "/api/events?limit=10", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var events []store.Event
	json.Unmarshal(rec.Body.Bytes(), &events)
	if len(events) != 1 {
		t.Errorf("events = %d, want 1", len(events))
	}
}

func itoa(i int64) string { return strconv.FormatInt(i, 10) }
