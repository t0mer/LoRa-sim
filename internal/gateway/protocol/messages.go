// Package protocol defines the LoRa Basics Station LNS message types exchanged
// between the gateway and the network server (AWS IoT Core for LoRaWAN, or the
// offline mock-lns) over a WebSocket. Every message carries a "msgtype" field.
//
// Phase 2 covers the LNS-direct subset: version, router_config, jreq, updf,
// dnmsg, and dntxed. DevAddr and MIC are signed int32 on the wire.
package protocol

// Message type discriminators (the "msgtype" field).
const (
	TypeVersion      = "version"
	TypeRouterConfig = "router_config"
	TypeJreq         = "jreq"
	TypeUpdf         = "updf"
	TypeDnmsg        = "dnmsg"
	TypeDntxed       = "dntxed"
)

// Envelope is used to peek at a message's type before full decoding.
type Envelope struct {
	MsgType string `json:"msgtype"`
}

// UpInfo carries the synthesized radio metadata attached to every uplink.
type UpInfo struct {
	RCtx    int64   `json:"rctx"`
	XTime   int64   `json:"xtime"`
	GpsTime int64   `json:"gpstime"`
	RSSI    float64 `json:"rssi"`
	SNR     float64 `json:"snr"`
}

// Version is sent by the gateway right after connecting to the LNS.
type Version struct {
	MsgType  string `json:"msgtype"`
	Station  string `json:"station"`
	Firmware string `json:"firmware"`
	Package  string `json:"package,omitempty"`
	Model    string `json:"model"`
	Protocol int    `json:"protocol"`
	Features string `json:"features,omitempty"`
}

// RouterConfig is the channel plan the LNS pushes after the version handshake.
type RouterConfig struct {
	MsgType    string           `json:"msgtype"`
	NetID      []uint32         `json:"NetID"`
	JoinEui    [][]uint64       `json:"JoinEui"` // inclusive [beg,end] ranges
	Region     string           `json:"region"`
	HwSpec     string           `json:"hwspec"`
	FreqRange  []uint32         `json:"freq_range"`
	DRs        [][]int          `json:"DRs"`
	Sx1301Conf []map[string]any `json:"sx1301_conf"`
	NoCCA      bool             `json:"nocca"`
	NoDC       bool             `json:"nodc"`
	NoDwell    bool             `json:"nodwell"`
	MaxEIRP    float64          `json:"max_eirp"`
}

// Jreq is a join-request uplink forwarded to the LNS.
type Jreq struct {
	MsgType  string `json:"msgtype"`
	MHdr     uint8  `json:"MHdr"`
	JoinEui  string `json:"JoinEui"`
	DevEui   string `json:"DevEui"`
	DevNonce uint16 `json:"DevNonce"`
	MIC      int32  `json:"MIC"`
	DR       uint8  `json:"DR"`
	Freq     uint32 `json:"Freq"`
	UpInfo   UpInfo `json:"upinfo"`
}

// Updf is a data uplink forwarded to the LNS. FPort is -1 when absent.
type Updf struct {
	MsgType    string `json:"msgtype"`
	MHdr       uint8  `json:"MHdr"`
	DevAddr    int32  `json:"DevAddr"`
	FCtrl      uint8  `json:"FCtrl"`
	FCnt       uint32 `json:"FCnt"`
	FOpts      string `json:"FOpts"`
	FPort      int    `json:"FPort"`
	FRMPayload string `json:"FRMPayload"`
	MIC        int32  `json:"MIC"`
	DR         uint8  `json:"DR"`
	Freq       uint32 `json:"Freq"`
	UpInfo     UpInfo `json:"upinfo"`
}

// Dnmsg is a downlink the LNS asks the gateway to transmit. dC selects the
// class/window (0=A RX1/RX2, 2=C). pdu is the raw PHYPayload in hex.
type Dnmsg struct {
	MsgType  string `json:"msgtype"`
	DevEui   string `json:"DevEui"`
	DC       int    `json:"dC"`
	DIID     int64  `json:"diid"`
	Pdu      string `json:"pdu"`
	RxDelay  int    `json:"RxDelay"`
	RX1DR    uint8  `json:"RX1DR"`
	RX1Freq  uint32 `json:"RX1Freq"`
	RX2DR    uint8  `json:"RX2DR"`
	RX2Freq  uint32 `json:"RX2Freq"`
	Priority int    `json:"priority"`
	XTime    int64  `json:"xtime"`
	GpsTime  int64  `json:"gpstime,omitempty"` // Class B ping-slot time
	RCtx     int64  `json:"rctx"`
}

// Dntxed confirms a downlink was transmitted on air.
type Dntxed struct {
	MsgType string  `json:"msgtype"`
	DIID    int64   `json:"diid"`
	DevEui  string  `json:"DevEui"`
	RCtx    int64   `json:"rctx"`
	XTime   int64   `json:"xtime"`
	TxTime  float64 `json:"txtime"`
	GpsTime int64   `json:"gpstime"`
}
