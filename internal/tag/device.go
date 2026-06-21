package tag

import (
	"context"
	"errors"
	"fmt"

	"github.com/t0mer/cylon/internal/store"
)

// Device is a simulated end-device: it ties a provisioned tag and its persisted
// session to the PHY frame build/parse layer. It is transport-agnostic — methods
// return the bytes to transmit and accept the bytes received, so the gateway/TCP
// layer (Phase 2) can drive it. Frame counters and DevNonce are persisted via
// the store before transmit, never rolled back.
type Device struct {
	tag   store.Tag
	prov  Provisioning
	store *store.Store
	gen   Generator
}

// NewDevice builds a Device from a stored tag, parsing its provisioning and
// constructing its payload generator.
func NewDevice(t store.Tag, st *store.Store) (*Device, error) {
	devEUI, err := parseEUI64(t.DevEUI)
	if err != nil {
		return nil, err
	}
	joinEUI, err := parseEUI64(t.JoinEUI)
	if err != nil {
		return nil, err
	}
	appKey, err := parseAES128(t.AppKey)
	if err != nil {
		return nil, err
	}
	gen, err := NewGenerator(t.PayloadType, t.PayloadConfig)
	if err != nil {
		return nil, err
	}
	return &Device{
		tag:   t,
		prov:  Provisioning{DevEUI: devEUI, JoinEUI: joinEUI, AppKey: appKey},
		store: st,
		gen:   gen,
	}, nil
}

// TagID returns the device's tag id.
func (d *Device) TagID() int64 { return d.tag.ID }

// BuildJoinRequest advances and persists the DevNonce, then builds a signed
// join-request to transmit. It returns the frame and the DevNonce used.
func (d *Device) BuildJoinRequest(ctx context.Context) ([]byte, uint16, error) {
	devNonce, err := d.store.Sessions().NextDevNonce(ctx, d.tag.ID)
	if err != nil {
		return nil, 0, fmt.Errorf("allocating dev nonce: %w", err)
	}
	frame, err := BuildJoinRequest(d.prov, devNonce)
	if err != nil {
		return nil, 0, err
	}
	return frame, devNonce, nil
}

// CompleteJoin parses and validates a join-accept for the given DevNonce, then
// persists the derived session.
func (d *Device) CompleteJoin(ctx context.Context, devNonce uint16, accept []byte) error {
	res, err := ParseJoinAccept(d.prov, devNonce, accept)
	if err != nil {
		return err
	}
	js := store.JoinState{
		DevAddr:     hexBytes(res.DevAddr[:]),
		NwkSKey:     hexBytes(res.NwkSKey[:]),
		AppSKey:     hexBytes(res.AppSKey[:]),
		RxDelay:     int(res.RxDelay),
		RX1DROffset: int(res.RX1DROffset),
		RX2DR:       int(res.RX2DataRate),
	}
	return d.store.Sessions().SaveJoinResult(ctx, d.tag.ID, js)
}

// BuildUplink builds the next data uplink. With override nil the payload comes
// from the device's generator. It advances and persists FCntUp before returning
// the frame and the FCnt used.
func (d *Device) BuildUplink(ctx context.Context, override []byte) ([]byte, uint32, error) {
	sess, err := d.store.Sessions().Get(ctx, d.tag.ID)
	if err != nil {
		return nil, 0, err
	}
	if !sess.Joined {
		return nil, 0, errors.New("device is not joined")
	}

	fcnt, err := d.store.Sessions().TakeFCntUp(ctx, d.tag.ID)
	if err != nil {
		return nil, 0, err
	}

	data := override
	if data == nil {
		if data, err = d.gen.Next(fcnt); err != nil {
			return nil, 0, err
		}
	}

	params, err := d.uplinkParams(sess, fcnt, data)
	if err != nil {
		return nil, 0, err
	}
	frame, err := BuildUplink(params)
	if err != nil {
		return nil, 0, err
	}
	return frame, fcnt, nil
}

func (d *Device) uplinkParams(sess *store.Session, fcnt uint32, data []byte) (UplinkParams, error) {
	devAddr, err := parseDevAddr(sess.DevAddr)
	if err != nil {
		return UplinkParams{}, err
	}
	nwk, err := parseAES128(sess.NwkSKey)
	if err != nil {
		return UplinkParams{}, err
	}
	app, err := parseAES128(sess.AppSKey)
	if err != nil {
		return UplinkParams{}, err
	}
	return UplinkParams{
		DevAddr: devAddr,
		NwkSKey: nwk,
		AppSKey: app,
		FCnt:    fcnt,
		FPort:   uint8(d.tag.FPort),
		Data:    data,
	}, nil
}

// HandleDownlink validates and decodes a downlink, deduplicates by frame
// counter, and persists the highest seen FCntDown.
func (d *Device) HandleDownlink(ctx context.Context, frame []byte) (*Downlink, error) {
	sess, err := d.store.Sessions().Get(ctx, d.tag.ID)
	if err != nil {
		return nil, err
	}
	if !sess.Joined {
		return nil, errors.New("device is not joined")
	}

	devAddr, err := parseDevAddr(sess.DevAddr)
	if err != nil {
		return nil, err
	}
	nwk, err := parseAES128(sess.NwkSKey)
	if err != nil {
		return nil, err
	}
	app, err := parseAES128(sess.AppSKey)
	if err != nil {
		return nil, err
	}

	dl, err := ParseDownlink(DownlinkParams{DevAddr: devAddr, NwkSKey: nwk, AppSKey: app}, frame)
	if err != nil {
		return nil, err
	}

	// Dedup: drop a downlink whose counter we've already passed.
	if sess.FCntDown > 0 && dl.FCnt < sess.FCntDown {
		return nil, fmt.Errorf("stale downlink FCnt %d (have %d)", dl.FCnt, sess.FCntDown)
	}
	if err := d.store.Sessions().SetFCntDown(ctx, d.tag.ID, dl.FCnt); err != nil {
		return nil, err
	}
	return dl, nil
}
