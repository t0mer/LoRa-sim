package api

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/t0mer/cylon/internal/euid"
	"github.com/t0mer/cylon/internal/store"
)

// tagDTO is the API representation of a tag. The AppKey is never returned in
// full — only the last 4 hex chars are surfaced.
type tagDTO struct {
	ID            int64       `json:"id"`
	DevEUI        string      `json:"dev_eui"`
	JoinEUI       string      `json:"join_eui"`
	AppKeyMasked  string      `json:"app_key_masked"`
	Class         string      `json:"class"`
	Region        string      `json:"region"`
	SubBand       int         `json:"sub_band"`
	DefaultDR     int         `json:"default_dr"`
	FPort         int         `json:"fport"`
	PayloadType   string      `json:"payload_type"`
	PayloadConfig string      `json:"payload_config,omitempty"`
	Enabled       bool        `json:"enabled"`
	Running       bool        `json:"running"`
	CreatedAt     string      `json:"created_at"`
	UpdatedAt     string      `json:"updated_at"`
	Session       *sessionDTO `json:"session,omitempty"`
}

type sessionDTO struct {
	Joined   bool   `json:"joined"`
	DevAddr  string `json:"dev_addr,omitempty"`
	FCntUp   uint32 `json:"fcnt_up"`
	FCntDown uint32 `json:"fcnt_down"`
	DevNonce uint16 `json:"dev_nonce"`
}

func maskKey(k string) string {
	if len(k) <= 4 {
		return "****"
	}
	return "****" + k[len(k)-4:]
}

func (a *API) toTagDTO(t store.Tag, running map[int64]bool) tagDTO {
	return tagDTO{
		ID: t.ID, DevEUI: t.DevEUI, JoinEUI: t.JoinEUI, AppKeyMasked: maskKey(t.AppKey),
		Class: t.Class, Region: t.Region, SubBand: t.SubBand, DefaultDR: t.DefaultDR,
		FPort: t.FPort, PayloadType: t.PayloadType, PayloadConfig: t.PayloadConfig,
		Enabled: t.Enabled, Running: running[t.ID], CreatedAt: t.CreatedAt, UpdatedAt: t.UpdatedAt,
	}
}

func (a *API) runningSet() map[int64]bool {
	set := map[int64]bool{}
	if a.orch != nil {
		for _, id := range a.orch.Running() {
			set[id] = true
		}
	}
	return set
}

func (a *API) listTags(w http.ResponseWriter, r *http.Request) {
	tags, err := a.store.Tags().List(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	running := a.runningSet()
	out := make([]tagDTO, 0, len(tags))
	for _, t := range tags {
		dto := a.toTagDTO(t, running)
		if s, err := a.store.Sessions().Get(r.Context(), t.ID); err == nil {
			dto.Session = &sessionDTO{
				Joined: s.Joined, DevAddr: s.DevAddr, FCntUp: s.FCntUp,
				FCntDown: s.FCntDown, DevNonce: s.DevNonce,
			}
		}
		out = append(out, dto)
	}
	writeJSON(w, http.StatusOK, out)
}

type createTagReq struct {
	DevEUI        string `json:"dev_eui"`
	JoinEUI       string `json:"join_eui"`
	AppKey        string `json:"app_key"`
	Class         string `json:"class"`
	Region        string `json:"region"`
	SubBand       int    `json:"sub_band"`
	DefaultDR     int    `json:"default_dr"`
	FPort         int    `json:"fport"`
	PayloadType   string `json:"payload_type"`
	PayloadConfig string `json:"payload_config"`
	Enabled       *bool  `json:"enabled"`
	Count         int    `json:"count"` // >1 creates a fleet with generated DevEUIs
}

func (a *API) createTags(w http.ResponseWriter, r *http.Request) {
	var req createTagReq
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	count := req.Count
	if count <= 0 {
		count = 1
	}

	created := make([]tagDTO, 0, count)
	for i := 0; i < count; i++ {
		nt, err := req.toNewTag(count > 1)
		if err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
		t, err := a.store.Tags().Create(r.Context(), nt)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		created = append(created, a.toTagDTO(*t, nil))
	}
	writeJSON(w, http.StatusCreated, created)
}

func (req createTagReq) toNewTag(fleet bool) (store.NewTag, error) {
	devEUI := req.DevEUI
	if devEUI == "" || fleet {
		g, err := euid.GenerateEUI("")
		if err != nil {
			return store.NewTag{}, err
		}
		devEUI = g
	}
	joinEUI := req.JoinEUI
	if joinEUI == "" {
		joinEUI = "0000000000000000"
	}
	appKey := req.AppKey
	if appKey == "" {
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			return store.NewTag{}, err
		}
		appKey = hex.EncodeToString(b)
	}
	class := req.Class
	if class == "" {
		class = "A"
	}
	region := req.Region
	if region == "" {
		region = "EU868"
	}
	payloadType := req.PayloadType
	if payloadType == "" {
		payloadType = "counter"
	}
	fport := req.FPort
	if fport == 0 {
		fport = 10
	}
	subBand := req.SubBand
	if subBand == 0 {
		subBand = 2
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	return store.NewTag{
		DevEUI: devEUI, JoinEUI: joinEUI, AppKey: appKey, Class: class, Region: region,
		SubBand: subBand, DefaultDR: req.DefaultDR, FPort: fport, PayloadType: payloadType,
		PayloadConfig: req.PayloadConfig, Enabled: enabled,
	}, nil
}

func (a *API) tagID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid tag id")
		return 0, false
	}
	return id, true
}

func (a *API) getTag(w http.ResponseWriter, r *http.Request) {
	id, ok := a.tagID(w, r)
	if !ok {
		return
	}
	t, err := a.store.Tags().Get(r.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "tag not found")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	dto := a.toTagDTO(*t, a.runningSet())
	if s, err := a.store.Sessions().Get(r.Context(), id); err == nil {
		dto.Session = &sessionDTO{
			Joined: s.Joined, DevAddr: s.DevAddr, FCntUp: s.FCntUp,
			FCntDown: s.FCntDown, DevNonce: s.DevNonce,
		}
	}
	writeJSON(w, http.StatusOK, dto)
}

func (a *API) deleteTag(w http.ResponseWriter, r *http.Request) {
	id, ok := a.tagID(w, r)
	if !ok {
		return
	}
	if a.orch != nil {
		a.orch.Stop(id)
	}
	if err := a.store.Tags().Delete(r.Context(), id); errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "tag not found")
		return
	} else if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) joinTag(w http.ResponseWriter, r *http.Request) {
	id, ok := a.tagID(w, r)
	if !ok {
		return
	}
	if a.orch == nil {
		writeErr(w, http.StatusServiceUnavailable, "gateway is not running")
		return
	}
	if err := a.orch.Start(r.Context(), id); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "joined"})
}

type uplinkReq struct {
	PayloadHex string `json:"payload_hex"`
}

func (a *API) uplinkTag(w http.ResponseWriter, r *http.Request) {
	id, ok := a.tagID(w, r)
	if !ok {
		return
	}
	if a.orch == nil {
		writeErr(w, http.StatusServiceUnavailable, "gateway is not running")
		return
	}
	var req uplinkReq
	if r.ContentLength > 0 {
		if err := decodeJSON(r, &req); err != nil {
			writeErr(w, http.StatusBadRequest, err.Error())
			return
		}
	}
	var override []byte
	if req.PayloadHex != "" {
		b, err := hex.DecodeString(req.PayloadHex)
		if err != nil {
			writeErr(w, http.StatusBadRequest, "invalid payload_hex")
			return
		}
		override = b
	}
	if err := a.orch.Uplink(r.Context(), id, override); err != nil {
		writeErr(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "sent"})
}
