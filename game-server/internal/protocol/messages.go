package protocol

type ClientMessage struct {
	Type         string  `json:"type"`
	Token        string  `json:"token,omitempty"`
	Ready        bool    `json:"ready,omitempty"`
	RequestID    string  `json:"requestId,omitempty"`
	MatchID      string  `json:"matchId,omitempty"`
	TurnNonce    string  `json:"turnNonce,omitempty"`
	AimAngle     float64 `json:"aimAngle,omitempty"`
	Power        float64 `json:"power,omitempty"`
	CueOffsetX   float64 `json:"cueOffsetX,omitempty"`
	CueOffsetY   float64 `json:"cueOffsetY,omitempty"`
	CalledBall   int     `json:"calledBall,omitempty"`
	CalledPocket int     `json:"calledPocket,omitempty"`
	Safety       bool    `json:"safety,omitempty"`
	X            float64 `json:"x,omitempty"`
	Y            float64 `json:"y,omitempty"`
	Text         string  `json:"text,omitempty"`
	ClientTime   int64   `json:"clientTime,omitempty"`
	Choice       string  `json:"choice,omitempty"`
}

type Envelope struct {
	Type       string `json:"type"`
	Seq        uint64 `json:"seq"`
	ServerTime int64  `json:"serverTime"`
	Data       any    `json:"data,omitempty"`
}
