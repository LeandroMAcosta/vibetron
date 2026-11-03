package render

import "math"

const maxParticles = 2048

type particle struct {
	x, y, vx, vy float32
	life, ttl    float32
	drag         float32
	c            RGB
}

// pool is a fixed array with the live particles packed at the front, so
// spawning and dying never allocate and the draw loop touches only live ones.
type pool struct {
	p [maxParticles]particle
	n int
}

func (pl *pool) spawn(x, y, vx, vy, ttl, drag float32, c RGB) {
	if pl.n == maxParticles {
		return
	}
	pl.p[pl.n] = particle{x: x, y: y, vx: vx, vy: vy, life: ttl, ttl: ttl, drag: drag, c: c}
	pl.n++
}

func (pl *pool) update(dt float32) {
	for i := 0; i < pl.n; {
		q := &pl.p[i]
		q.life -= dt
		if q.life <= 0 {
			pl.n--
			pl.p[i] = pl.p[pl.n]
			continue
		}
		k := 1 - q.drag*dt
		if k < 0 {
			k = 0
		}
		q.vx *= k
		q.vy *= k
		q.x += q.vx * dt
		q.y += q.vy * dt
		i++
	}
}

// draw adds each particle; young ones burn white before settling into
// their own color, which is what sells a burst as hot.
func (pl *pool) draw(buf []RGB, w, h int) {
	for i := 0; i < pl.n; i++ {
		q := &pl.p[i]
		x, y := int(q.x+0.5), int(q.y+0.5)
		if x < 0 || y < 0 || x >= w || y >= h {
			continue
		}
		l := q.life / q.ttl
		c := q.c
		if l > 0.75 {
			c = mix(c, white, uint8((l-0.75)*4*200))
		}
		b := &buf[y*w+x]
		*b = add(*b, scale(c, int(l*l*256+40)))
	}
}

func (pl *pool) clear() { pl.n = 0 }

// rng is xorshift64*: deterministic, allocation free, plenty for sparks.
type rng uint64

func (r *rng) next() uint64 {
	x := uint64(*r)
	x ^= x >> 12
	x ^= x << 25
	x ^= x >> 27
	*r = rng(x)
	return x * 2685821657736338717
}

// f returns [0, 1).
func (r *rng) f() float32 { return float32(r.next()>>40) / (1 << 24) }

// s returns [-1, 1).
func (r *rng) s() float32 { return r.f()*2 - 1 }

// burst sprays n particles from (x, y) at speeds up to v px/s.
func (a *Arena) burst(x, y float32, n int, v, ttl, drag float32, c RGB) {
	for i := 0; i < n; i++ {
		ang := a.rng.f() * 2 * math.Pi
		sp := v * (0.25 + 0.75*a.rng.f())
		sn, cs := math.Sincos(float64(ang))
		a.parts.spawn(x, y, float32(cs)*sp, float32(sn)*sp, ttl*(0.5+0.5*a.rng.f()), drag, c)
	}
}

// ring sprays n particles evenly around a circle at speed v.
func (a *Arena) ring(x, y float32, n int, v, ttl float32, c RGB) {
	for i := 0; i < n; i++ {
		sn, cs := math.Sincos(float64(i) * 2 * math.Pi / float64(n))
		a.parts.spawn(x, y, float32(cs)*v, float32(sn)*v, ttl, 3, c)
	}
}
