package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
)

const (
	TableConfigFile   = "pool-7ft-v2.json"
	PhysicsConfigFile = "physics-v3.json"
	RulesConfigFile   = "red-yellow-8ball-v1.json"
	TableVersion      = "pool-7ft-v2"
	PhysicsVersion    = "physics-v3"
	RulesVersion      = "red-yellow-8ball-v1"
)

type Table struct {
	Version        string `json:"version"`
	Units          string `json:"units"`
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
	StationarySpinDecay             float64 `json:"stationarySpinDecay"`
	SleepLinearSpeed                float64 `json:"sleepLinearSpeed"`
	SleepAngularSpeed               float64 `json:"sleepAngularSpeed"`
	SleepSideSpinSpeed              float64 `json:"sleepSideSpinSpeed"`
	SleepDuration                   float64 `json:"sleepDuration"`
	CueMaxSpeed                     float64 `json:"cueMaxSpeed"`
	CueMinSpeed                     float64 `json:"cueMinSpeed"`
	CuePowerExponent                float64 `json:"cuePowerExponent"`
	CueSpinFactor                   float64 `json:"cueSpinFactor"`
	PocketHorizontalDamping         float64 `json:"pocketHorizontalDamping"`
	SolverIterations                int     `json:"solverIterations"`
	PenetrationSlop                 float64 `json:"penetrationSlop"`
	PenetrationPercent              float64 `json:"penetrationPercent"`
	RestitutionVelocityThreshold    float64 `json:"restitutionVelocityThreshold"`
	SlipSpeedEpsilon                float64 `json:"slipSpeedEpsilon"`
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
		if _, err := os.Stat(filepath.Join(d, "table", TableConfigFile)); err == nil {
			root = d
			break
		}
	}
	if root == "" {
		return All{}, errors.New("config directory not found")
	}
	var out All
	if err := read(filepath.Join(root, "table", TableConfigFile), &out.Table); err != nil {
		return All{}, err
	}
	if err := read(filepath.Join(root, "physics", PhysicsConfigFile), &out.Physics); err != nil {
		return All{}, err
	}
	if err := read(filepath.Join(root, "rules", RulesConfigFile), &out.Rules); err != nil {
		return All{}, err
	}
	if out.Table.Version != TableVersion || out.Physics.Version != PhysicsVersion || out.Rules.Version != RulesVersion || out.Table.Units != "meters" {
		return All{}, fmt.Errorf("unexpected table, physics, or rules version")
	}
	if out.Physics.Hz < 30 || out.Physics.MaxSubsteps < 1 || out.Physics.MaxDisplacementFractionOfRadius <= 0 || out.Physics.MaxDisplacementFractionOfRadius > 1 || out.Table.PlayingSurface.Length <= 0 || out.Table.PlayingSurface.Width <= 0 || out.Table.Ball.Radius <= 0 || out.Table.Ball.Mass <= 0 {
		return All{}, fmt.Errorf("invalid configuration")
	}
	if math.Abs(out.Table.PlayingSurface.Length-2*out.Table.PlayingSurface.Width) > 1e-9 {
		return All{}, fmt.Errorf("playing surface must have a 2:1 aspect ratio")
	}
	if math.Abs(out.Table.Ball.Diameter-2*out.Table.Ball.Radius) > 1e-9 {
		return All{}, fmt.Errorf("ball diameter/radius mismatch")
	}
	if out.Physics.BallRestitution < 0 || out.Physics.BallRestitution > 1 || out.Physics.CushionRestitution < 0 || out.Physics.CushionFriction < 0 || out.Physics.CushionRestitution > 1 || out.Physics.BallFriction < 0 || out.Physics.SlidingFriction <= 0 || out.Physics.RollingResistance <= 0 || out.Physics.SpinDecay <= 0 || out.Physics.StationarySpinDecay < out.Physics.SpinDecay || out.Physics.SleepSideSpinSpeed <= 0 || out.Physics.SlipSpeedEpsilon <= 0 || out.Physics.RestitutionVelocityThreshold < 0 || out.Physics.CueMinSpeed < 0 || out.Physics.CueMaxSpeed <= out.Physics.CueMinSpeed || out.Physics.CuePowerExponent < 1 {
		return All{}, fmt.Errorf("invalid physics coefficients")
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
