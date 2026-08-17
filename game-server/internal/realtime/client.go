package realtime

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"github.com/gorilla/websocket"
	"poolarena/game-server/internal/physics"
	"poolarena/game-server/internal/protocol"
	"sync/atomic"
	"time"
)

type Incoming struct {
	Client  *Client
	Message protocol.ClientMessage
}

type Client struct {
	Principal string
	LobbyID   string
	Conn      *websocket.Conn
	Events    chan []byte
	Snapshots chan []byte
	Closed    chan struct{}
	seq       atomic.Uint64
}

func NewClient(principal, lobbyID string, conn *websocket.Conn) *Client {
	return &Client{
		Principal: principal,
		LobbyID:   lobbyID,
		Conn:      conn,
		Events:    make(chan []byte, 128),
		Snapshots: make(chan []byte, 1),
		Closed:    make(chan struct{}),
	}
}

func (c *Client) SendEvent(kind string, data any) bool {
	env := protocol.Envelope{Type: kind, Seq: c.seq.Add(1), ServerTime: time.Now().UnixMilli(), Data: data}
	b, err := json.Marshal(env)
	if err != nil {
		return false
	}
	select {
	case c.Events <- b:
		return true
	default:
		// Reliable control events must not be silently accumulated forever. A full
		// queue marks the peer as too slow and lets the lobby disconnect it.
		return false
	}
}

// Physics binary frame v1 (little endian):
// magic[4]="PLS1", seq u64, serverTime i64, simTime f32,
// matchIDLen u8 + UTF-8 match ID, ballCount u8, then per ball:
// id u8, state u8, pocket i8, x/y/z/vx/vy/wx/wy/wz float32.
// Simulation remains float64; float32 is only a rendering/network representation.
func EncodePhysicsSnapshot(seq uint64, serverTime int64, matchID string, simTime float64, balls []physics.BallSnapshot) []byte {
	if len(matchID) > 255 {
		matchID = matchID[:255]
	}
	if len(balls) > 255 {
		balls = balls[:255]
	}
	buf := bytes.NewBuffer(make([]byte, 0, 64+len(balls)*35))
	buf.WriteString("PLS1")
	_ = binary.Write(buf, binary.LittleEndian, seq)
	_ = binary.Write(buf, binary.LittleEndian, serverTime)
	_ = binary.Write(buf, binary.LittleEndian, float32(simTime))
	buf.WriteByte(byte(len(matchID)))
	buf.WriteString(matchID)
	buf.WriteByte(byte(len(balls)))
	for _, b := range balls {
		buf.WriteByte(byte(b.ID))
		buf.WriteByte(encodeBallState(b.State))
		buf.WriteByte(byte(int8(b.PocketID)))
		for _, v := range []float64{b.X, b.Y, b.Z, b.VX, b.VY, b.WX, b.WY, b.WZ} {
			_ = binary.Write(buf, binary.LittleEndian, float32(v))
		}
	}
	return buf.Bytes()
}

func encodeBallState(state physics.BallState) byte {
	switch state {
	case physics.BallFalling:
		return 1
	case physics.BallPocketed:
		return 2
	case physics.BallOffTable:
		return 3
	default:
		return 0
	}
}

func (c *Client) SendPhysicsSnapshot(matchID string, simTime float64, balls []physics.BallSnapshot) bool {
	seq := c.seq.Add(1)
	b := EncodePhysicsSnapshot(seq, time.Now().UnixMilli(), matchID, simTime, balls)
	// Physics snapshots are state samples, not a history stream. If a client is
	// behind, keeping only the newest frame prevents old animation data from
	// blocking turn/foul/match events.
	select {
	case c.Snapshots <- b:
		return true
	default:
	}
	select {
	case <-c.Snapshots:
	default:
	}
	select {
	case c.Snapshots <- b:
		return true
	default:
		return false
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	defer close(c.Closed)

	write := func(messageType int, payload []byte) bool {
		_ = c.Conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		return c.Conn.WriteMessage(messageType, payload) == nil
	}

	for {
		// Prefer queued control events over snapshots whenever possible.
		select {
		case b, ok := <-c.Events:
			if !ok || !write(websocket.TextMessage, b) {
				return
			}
			continue
		default:
		}

		select {
		case b, ok := <-c.Events:
			if !ok || !write(websocket.TextMessage, b) {
				return
			}
		case b, ok := <-c.Snapshots:
			if !ok || !write(websocket.BinaryMessage, b) {
				return
			}
		case <-ticker.C:
			if !write(websocket.PingMessage, nil) {
				return
			}
		}
	}
}
