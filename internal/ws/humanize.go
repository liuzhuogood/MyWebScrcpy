package ws

import (
	"math"
	"math/rand"
	"time"

	"mywebscrcpy/internal/scrcpy"
)

// humanize.go — 拟人化触摸注入。
//
// 默认的 tap / swipe 是精确单点、直线、无节奏的。启用 Humanize 后：
//   - tap：落点做高斯随机偏移（近似手指按压区域不是精确单点），
//     pressure 随机，down→up 之间加入随机按压时长。
//   - swipe：在起止点之间生成多个中间 move 点，走带随机抖动的曲线路径，
//     并模拟人手"起慢—中快—收慢"的变速节奏，点间加随机延时。
//
// 说明：scrcpy 协议没有"按压面积/contact area"字段，这里只能通过落点
// 偏移 + 压力近似模拟手指按压区域，无法真正上报接触面积。

const (
	// humanizeStdTapPx 点击落点高斯偏移的标准差（像素），按分辨率缩放。
	humanizeStdTapPx = 8.0
	// humanizeMinTapPressure / Max 点击按压压力范围。
	humanizeMinTapPressure = 0.55
	humanizeMaxTapPressure = 1.0
	// 点击按压时长范围（ms）。
	humanizeMinTapHoldMS = 60
	humanizeMaxTapHoldMS = 180
	// 拖动路径点间距（像素）。
	humanizeSwipeStepPx = 14.0
	// 拖动路径法线抖动幅度（像素），按分辨率缩放。
	humanizeSwipeJitterPx = 6.0
	// 拖动每步延时范围（ms），配合变速使用。
	humanizeMinMoveDelayMS = 8.0
	humanizeMaxMoveDelayMS = 35.0
)

// humanizeTap 发送拟人化的点击（down → 按压时长 → up）。
// 调用方已持有 controlMu 锁。
func (e deviceActionExecutor) humanizeTap(x, y int32, w, h uint16) error {
	std := humanizeStdTapPx
	if w < 600 {
		std = humanizeStdTapPx / 2
	}
	fx := clampInt32(int32(math.Round(float64(x)+rand.NormFloat64()*std)), 0, int32(w)-1)
	fy := clampInt32(int32(math.Round(float64(y)+rand.NormFloat64()*std)), 0, int32(h)-1)
	pressure := humanizeMinTapPressure + rand.Float64()*(humanizeMaxTapPressure-humanizeMinTapPressure)

	if err := e.ms.sess.conn.WriteControl(scrcpy.TouchEvent(scrcpy.ActionDown, scrcpy.PointerIDMouse, fx, fy, w, h, pressure, scrcpy.ButtonPrimary, scrcpy.ButtonPrimary)); err != nil {
		return err
	}
	hold := time.Duration(humanizeMinTapHoldMS+rand.Intn(humanizeMaxTapHoldMS-humanizeMinTapHoldMS)) * time.Millisecond
	time.Sleep(hold)
	return e.ms.sess.conn.WriteControl(scrcpy.TouchEvent(scrcpy.ActionUp, scrcpy.PointerIDMouse, fx, fy, w, h, 0, 0, 0))
}

// humanizeSwipe 发送拟人化的拖动（down → 曲线路径 move 序列 → up）。
// 调用方已持有 controlMu 锁。
func (e deviceActionExecutor) humanizeSwipe(x1, y1, x2, y2 int32, w, h uint16) error {
	pressure := humanizeMinTapPressure + rand.Float64()*(humanizeMaxTapPressure-humanizeMinTapPressure)

	if err := e.ms.sess.conn.WriteControl(scrcpy.TouchEvent(scrcpy.ActionDown, scrcpy.PointerIDMouse, x1, y1, w, h, pressure, scrcpy.ButtonPrimary, scrcpy.ButtonPrimary)); err != nil {
		return err
	}
	for _, p := range planHumanizedPath(x1, y1, x2, y2, w, h) {
		time.Sleep(p.delay)
		if err := e.ms.sess.conn.WriteControl(scrcpy.TouchEvent(scrcpy.ActionMove, scrcpy.PointerIDMouse, p.x, p.y, w, h, pressure, scrcpy.ButtonPrimary, scrcpy.ButtonPrimary)); err != nil {
			return err
		}
	}
	// 收尾停顿一下再抬手，更接近人手结束动作。
	time.Sleep(time.Duration(humanizeMinMoveDelayMS) * time.Millisecond)
	return e.ms.sess.conn.WriteControl(scrcpy.TouchEvent(scrcpy.ActionUp, scrcpy.PointerIDMouse, x2, y2, w, h, 0, 0, 0))
}

// pathPoint 一个拟人路径中间点。
type pathPoint struct {
	x, y  int32
	delay time.Duration
}

// planHumanizedPath 在起止点之间生成带法线抖动 + 变速节奏的 move 点序列。
// 抖动幅度用 t*(1-t) 包裹，起止点收敛回直线/端点；变速用 sin(pi*t)，
// 端点慢、中间快。最后一个点收敛到终点 (x2,y2)。
func planHumanizedPath(x1, y1, x2, y2 int32, screenW, screenH uint16) []pathPoint {
	dx := float64(x2 - x1)
	dy := float64(y2 - y1)
	dist := math.Hypot(dx, dy)
	if dist < 1 {
		return nil
	}
	ux, uy := dx/dist, dy/dist
	nx, ny := -uy, ux // 法线方向

	jitter := humanizeSwipeJitterPx
	if screenW < 600 {
		jitter = humanizeSwipeJitterPx / 2
	}

	n := int(math.Ceil(dist / humanizeSwipeStepPx))
	if n < 1 {
		n = 1
	}
	points := make([]pathPoint, 0, n)
	for i := 1; i <= n; i++ {
		t := float64(i) / float64(n)
		bx, by := float64(x1)+dx*t, float64(y1)+dy*t // 直线参考点
		// 抖动：t*(1-t) 使首尾为 0，中段最大；乘以标准正态随机量。
		amp := 4 * jitter * t * (1 - t) * rand.NormFloat64()
		px := clampInt32(int32(math.Round(bx+nx*amp)), 0, int32(screenW)-1)
		py := clampInt32(int32(math.Round(by+ny*amp)), 0, int32(screenH)-1)
		// 变速：sin(pi*t) 两端慢、中间快，delay 与速度成反比，加随机扰动。
		speedWeight := math.Sin(math.Pi * t)
		delay := humanizeMinMoveDelayMS + (humanizeMaxMoveDelayMS-humanizeMinMoveDelayMS)*(1-speedWeight) + rand.Float64()*6
		points = append(points, pathPoint{x: px, y: py, delay: time.Duration(delay) * time.Millisecond})
	}
	return points
}

// clampInt32 将 v 限制在 [lo, hi]。
func clampInt32(v, lo, hi int32) int32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
