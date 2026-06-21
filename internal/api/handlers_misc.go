package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/t0mer/cylon/internal/store"
)

type gatewayDTO struct {
	EUI            string `json:"eui"`
	Region         string `json:"region"`
	SubBand        int    `json:"sub_band"`
	ConnectionMode string `json:"connection_mode"`
	Status         string `json:"status"` // connected | disabled
	TCPAddr        string `json:"tcp_addr,omitempty"`
	TagConns       int    `json:"tag_conns"`
	WSClients      int    `json:"ws_clients"`
}

func (a *API) getGateway(w http.ResponseWriter, r *http.Request) {
	g, err := a.store.Gateway().Get(r.Context())
	if errors.Is(err, store.ErrNotFound) {
		writeErr(w, http.StatusNotFound, "gateway not initialized")
		return
	}
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	dto := gatewayDTO{
		EUI: g.EUI, Region: g.Region, SubBand: g.SubBand, ConnectionMode: g.ConnectionMode,
		Status: "disabled", WSClients: a.hub.Clients(),
	}
	if a.gw != nil {
		dto.Status = "connected"
		dto.TCPAddr = a.gw.Addr()
		dto.TagConns = a.gw.ConnCount()
	}
	writeJSON(w, http.StatusOK, dto)
}

type putGatewayReq struct {
	Region         string `json:"region"`
	SubBand        int    `json:"sub_band"`
	ConnectionMode string `json:"connection_mode"`
}

func (a *API) putGateway(w http.ResponseWriter, r *http.Request) {
	var req putGatewayReq
	if err := decodeJSON(r, &req); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	g, err := a.store.Gateway().Get(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if req.Region == "" {
		req.Region = g.Region
	}
	if req.SubBand == 0 {
		req.SubBand = g.SubBand
	}
	if req.ConnectionMode == "" {
		req.ConnectionMode = g.ConnectionMode
	}
	updated, err := a.store.Gateway().UpdateConfig(r.Context(), req.Region, req.SubBand, req.ConnectionMode)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, gatewayDTO{
		EUI: updated.EUI, Region: updated.Region, SubBand: updated.SubBand,
		ConnectionMode: updated.ConnectionMode, Status: "disabled",
	})
}

func (a *API) listEvents(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var f store.EventFilter
	if v := q.Get("tag"); v != "" {
		if id, err := strconv.ParseInt(v, 10, 64); err == nil {
			f.TagID = &id
		}
	}
	f.Direction = q.Get("dir")
	if v := q.Get("before"); v != "" {
		f.BeforeID, _ = strconv.ParseInt(v, 10, 64)
	}
	if v := q.Get("limit"); v != "" {
		f.Limit, _ = strconv.Atoi(v)
	}
	events, err := a.store.Events().List(r.Context(), f)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	if events == nil {
		events = []store.Event{}
	}
	writeJSON(w, http.StatusOK, events)
}

type burstReq struct {
	Count  int `json:"count"`
	AtOnce int `json:"at_once"`
}

func (a *API) runScenario(w http.ResponseWriter, r *http.Request) {
	if a.orch == nil {
		writeErr(w, http.StatusServiceUnavailable, "gateway is not running")
		return
	}
	switch chi.URLParam(r, "name") {
	case "join_all":
		started, errs := a.orch.JoinAll(r.Context())
		writeJSON(w, http.StatusOK, map[string]int{"started": started, "errors": errs})
	case "burst":
		var req burstReq
		if r.ContentLength > 0 {
			if err := decodeJSON(r, &req); err != nil {
				writeErr(w, http.StatusBadRequest, err.Error())
				return
			}
		}
		if req.Count <= 0 {
			req.Count = len(a.orch.Running())
		}
		if err := a.orch.Burst(r.Context(), req.Count, req.AtOnce); err != nil {
			writeErr(w, http.StatusBadGateway, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	default:
		writeErr(w, http.StatusNotFound, "unknown scenario")
	}
}
