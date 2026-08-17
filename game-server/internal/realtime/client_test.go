package realtime

import (
	"encoding/binary"
	"math"
	"poolarena/game-server/internal/physics"
	"testing"
)

func TestEncodePhysicsSnapshotBinaryV1(t *testing.T) {
	balls := []physics.BallSnapshot{{ID: 8, X: 1.25, Y: -0.5, Z: 0.028575, VX: 2.1, WZ: 4.2, State: physics.BallFalling, PocketID: 5}}
	b := EncodePhysicsSnapshot(42, 123456789, "match-1", 1.5, balls)
	if string(b[:4]) != "PLS1" {
		t.Fatalf("bad magic %q", b[:4])
	}
	if got := binary.LittleEndian.Uint64(b[4:12]); got != 42 {
		t.Fatalf("seq %d", got)
	}
	o := 20
	if got := math.Float32frombits(binary.LittleEndian.Uint32(b[o : o+4])); math.Abs(float64(got)-1.5) > 1e-6 {
		t.Fatalf("sim time %f", got)
	}
	o += 4
	idLen := int(b[o])
	o++
	if string(b[o:o+idLen]) != "match-1" {
		t.Fatal("match id")
	}
	o += idLen
	if b[o] != 1 {
		t.Fatalf("ball count %d", b[o])
	}
	o++
	if b[o] != 8 || b[o+1] != 1 || int8(b[o+2]) != 5 {
		t.Fatalf("ball header %#v", b[o:o+3])
	}
}
