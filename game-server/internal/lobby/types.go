package lobby

import (
	"poolarena/game-server/internal/auth"
	"poolarena/game-server/internal/match"
	"poolarena/game-server/internal/realtime"
	"time"
)

const (
	Waiting  = "WAITING"
	Starting = "STARTING"
	Playing  = "PLAYING"
	PostGame = "POST_GAME"
	Rotating = "ROTATING"
	Closing  = "CLOSING"
)

type Participant struct {
	Principal         string
	PublicID          string
	Nickname          string
	CueSkin           string
	Conn              *realtime.Client
	Connected         bool
	Ready             bool
	ActiveSeat        int
	ReconnectDeadline time.Time
	ChatTimes         []time.Time
	ShotTimes         []time.Time
	MessageTimes      []time.Time
	RecentRequests    map[string]time.Time
}

type RuntimeSummary struct {
	ID         string `json:"id"`
	Code       string `json:"code"`
	Name       string `json:"name"`
	State      string `json:"state"`
	Players    int    `json:"players"`
	Spectators int    `json:"spectators"`
	QueueSize  int    `json:"queueSize"`
}
type joinCmd struct {
	claims auth.Claims
	client *realtime.Client
}
type leaveCmd struct{ client *realtime.Client }
type msgCmd struct {
	client *realtime.Client
	msg    any
}
type shutdownCmd struct{ done chan struct{} }
type summaryCmd struct{ reply chan RuntimeSummary }

type Playback struct {
	Start       time.Time
	SimDuration time.Duration
	NextFrame   int
	PauseStart  time.Time
	Paused      bool
}

type PublicParticipant struct {
	ID            string `json:"id"`
	Nickname      string `json:"nickname"`
	CueSkin       string `json:"cueSkin"`
	Role          string `json:"role"`
	Seat          int    `json:"seat"`
	Seats         []int  `json:"seats"`
	Ready         bool   `json:"ready"`
	Reconnecting  bool   `json:"reconnecting"`
	QueuePosition int    `json:"queuePosition"`
}
type PublicState struct {
	ID           string              `json:"id"`
	Code         string              `json:"code"`
	Name         string              `json:"name"`
	State        string              `json:"state"`
	Participants []PublicParticipant `json:"participants"`
	Queue        []string            `json:"queue"`
	Match        *match.PublicState  `json:"match,omitempty"`
	ShotDeadline int64               `json:"shotDeadline,omitempty"`
	Solo         bool                `json:"solo"`
}
