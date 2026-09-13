package ws

import "testing"

// TestPlanHumanizedPathDeterministicEndpoints 验证拟人路径：首个 move 点在
// 起终点附近、最后一个点收敛回终点、所有点都在屏幕范围内。
func TestPlanHumanizedPathDeterministicEndpoints(t *testing.T) {
	const (
		w, h = 1080, 2400
		x1   = int32(100)
		y1   = int32(200)
		x2   = int32(900)
		y2   = int32(2000)
	)
	pts := planHumanizedPath(x1, y1, x2, y2, w, h)
	if len(pts) < 3 {
		t.Fatalf("expected >=3 path points for a long swipe, got %d", len(pts))
	}
	last := pts[len(pts)-1]
	if last.x != x2 || last.y != y2 {
		t.Fatalf("last move point must converge to endpoint (%d,%d), got (%d,%d)", x2, y2, last.x, last.y)
	}
	for _, p := range pts {
		if p.x < 0 || int(p.x) >= int(w) || p.y < 0 || int(p.y) >= int(h) {
			t.Fatalf("path point (%d,%d) out of screen %dx%d", p.x, p.y, w, h)
		}
		if p.delay <= 0 {
			t.Fatalf("expected positive delay, got %v", p.delay)
		}
	}
}

// TestPlanHumanizedPathShortDistance 验证起终点重合时退化为无中间点。
func TestPlanHumanizedPathShortDistance(t *testing.T) {
	pts := planHumanizedPath(10, 10, 10, 10, 1080, 2400)
	if pts != nil {
		t.Fatalf("expected nil path for zero distance, got %v", pts)
	}
}

// TestPlanHumanizedPathBoundsWithJitter 验证抖动不会把点推出屏幕边界。
func TestPlanHumanizedPathBoundsWithJitter(t *testing.T) {
	for i := 0; i < 200; i++ {
		pts := planHumanizedPath(0, 0, 1079, 2399, 1080, 2400)
		for _, p := range pts {
			if p.x < 0 || int(p.x) >= 1080 || p.y < 0 || int(p.y) >= 2400 {
				t.Fatalf("jitter pushed point (%d,%d) out of bounds", p.x, p.y)
			}
		}
	}
}
