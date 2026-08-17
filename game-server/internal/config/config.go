package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
)

type Table struct {
	Version        string `json:"version"`
	PlayingSurface struct {
		Length float64 `json:"length"`
		Width  float64 `json:"width"`
	} `json:"playingSurface"`
	Ball struct {
		Diameter float64 `json:"diameter"`
		Radius   float64 `json:"radius"`
		Mass     float64 `json:"mass"`
	} `json:"ball"`
	Rails struct {
		VisualWidth            float64 `json:"visualWidth"`
		CushionNoseHeightRatio float64 `json:"cushionNoseHeightRatio"`
	} `json:"rails"`
	Pockets struct {
		Corner PocketConfig `json:"corner"`
		Side   PocketConfig `json:"side"`
	} `json:"pockets"`
	Rack struct {
		FootSpotX   float64 `json:"footSpotX"`
		HeadStringX float64 `json:"headStringX"`
		CueBreakX   float64 `json:"cueBreakX"`
	} `json:"rack"`
}

type PocketConfig struct {
	Mouth            float64 `json:"mouth"`
	HorizontalCutDeg float64 `json:"horizontalCutDeg"`
	Shelf            float64 `json:"shelf"`
	BackDraftDeg     float64 `json:"backDraftDeg"`
	FacingThickness  float64 `json:"facingThickness"`
	ThroatWidth      float64 `json:"throatWidth"`
	DropDepth        float64 `json:"dropDepth"`
	DropRadiusX      float64 `json:"dropRadiusX"`
	DropRadiusY      float64 `json:"dropRadiusY"`
}

type Physics struct {
	Version                         string  `json:"version"`
	Hz                              int     `json:"hz"`
	MaxSubsteps                     int     `json:"maxSubsteps"`
	MaxDisplacementFractionOfRadius float64 `json:"maxDisplacementFractionOfRadius"`
	Gravity                         float64 `json:"gravity"`
	BallRestitution                 float64 `json:"ballRestitution"`
	BallFriction                    float64 `json:"ballFriction"`
	CushionRestitution              float64 `json:"cushionRestitution"`
	CushionFriction                 float64 `json:"cushionFriction"`
	SlidingFriction                 float64 `json:"slidingFriction"`
	RollingResistance               float64 `json:"rollingResistance"`
	SpinDecay                       float64 `json:"spinDecay"`
	SleepLinearSpeed                float64 `json:"sleepLinearSpeed"`
	SleepAngularSpeed               float64 `json:"sleepAngularSpeed"`
	SleepDuration                   float64 `json:"sleepDuration"`
	CueMaxSpeed                     float64 `json:"cueMaxSpeed"`
	CueMinSpeed                     float64 `json:"cueMinSpeed"`
	CueSpinFactor                   float64 `json:"cueSpinFactor"`
	PocketHorizontalDamping         float64 `json:"pocketHorizontalDamping"`
	SolverIterations                int     `json:"solverIterations"`
	PenetrationSlop                 float64 `json:"penetrationSlop"`
	PenetrationPercent              float64 `json:"penetrationPercent"`
}

type Rules struct {
	Version               string `json:"version"`
	ReadyTimeoutSeconds   int    `json:"readyTimeoutSeconds"`
	ReconnectGraceSeconds int    `json:"reconnectGraceSeconds"`
	PostGameSeconds       int    `json:"postGameSeconds"`
	EmptyLobbySeconds     int    `json:"emptyLobbySeconds"`
	CountdownSeconds      int    `json:"countdownSeconds"`
}

type All struct {
	Table   Table
	Physics Physics
	Rules   Rules
}

func Load() (All, error) {
	dirs := []string{}
	if d := os.Getenv("CONFIG_DIR"); d != "" {
		dirs = append(dirs, d)
	}
	dirs = append(dirs, "/app/config", "./config", "../config")
	if cwd, err := os.Getwd(); err == nil {
		for dir, depth := cwd, 0; depth < 8; depth++ {
			dirs = append(dirs, filepath.Join(dir, "config"))
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}
	var root string
	for _, d := range dirs {
		if _, err := os.Stat(filepath.Join(d, "table", "wpa-9ft-v1.json")); err == nil {
			root = d
			break
		}
	}
	if root == "" {
		return All{}, errors.New("config directory not found")
	}
	var out All
	if err := read(filepath.Join(root, "table", "wpa-9ft-v1.json"), &out.Table); err != nil {
		return All{}, err
	}
	if err := read(filepath.Join(root, "physics", "physics-v1.json"), &out.Physics); err != nil {
		return All{}, err
	}
	if err := read(filepath.Join(root, "rules", "wpa-8ball-v1.json"), &out.Rules); err != nil {
		return All{}, err
	}
	if out.Physics.Hz < 30 || out.Table.Ball.Radius <= 0 {
		return All{}, fmt.Errorf("invalid configuration")
	}
	if math.Abs(out.Table.Ball.Diameter-2*out.Table.Ball.Radius) > 1e-9 {
		return All{}, fmt.Errorf("ball diameter/radius mismatch")
	}
	cornerDerived := out.Table.Pockets.Corner.Mouth - 2*out.Table.Pockets.Corner.Shelf*math.Tan((out.Table.Pockets.Corner.HorizontalCutDeg-135)*math.Pi/180)
	sideDerived := out.Table.Pockets.Side.Mouth - 2*out.Table.Pockets.Side.Shelf*math.Tan((out.Table.Pockets.Side.HorizontalCutDeg-90)*math.Pi/180)
	if math.Abs(cornerDerived-out.Table.Pockets.Corner.ThroatWidth) > 1e-6 || math.Abs(sideDerived-out.Table.Pockets.Side.ThroatWidth) > 1e-6 {
		return All{}, fmt.Errorf("pocket throat width does not match mouth/shelf/cut geometry")
	}
	return out, nil
}

func read(path string, target any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, target)
}
