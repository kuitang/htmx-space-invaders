package main

import (
	"fmt"
	"log"
	"math"
	"math/rand"
	"strings"
	"time"
)

type Bullet struct {
	X     float64
	Y     float64
	VelY  float64
	IsAlienBullet bool
}

type Game struct {
	SpaceshipX      float64
	AlienX          float64
	AlienY          float64
	AlienVelX       float64
	Bullets         []Bullet

	MovingLeft      bool
	MovingRight     bool

	LastShootTime   time.Time
	LastAlienShoot  time.Time

	FrameCount      int
	FPSTime         time.Time
	CurrentFPS      float64

	GameOver        bool
	Score           int

	SpaceshipWidth  float64
	SpaceshipHeight float64
	AlienWidth      float64
	AlienHeight     float64
	BulletWidth     float64
	BulletHeight    float64

	GameWidth       float64
	GameHeight      float64

	AlienDead       bool
	AlienDeathTime  time.Time
	SpaceshipDead   bool
	SpaceshipDeathTime time.Time

	// Frame acknowledgment tracking
	FrameID         uint64
	FrameSentTimes  map[uint64]time.Time
	RoundTripTimes  []float64
	ClientFPS       float64
	AvgLatency      float64
	LastAckTime     time.Time
	AckCount        int

	// Session tracking
	SessionID       string
}

func NewGame() *Game {
	return &Game{
		SpaceshipX:      164, // Adjusted for 360px width
		AlienX:          164,
		AlienY:          50,
		AlienVelX:       100, // Reduced for smaller canvas
		Bullets:         make([]Bullet, 0),
		SpaceshipWidth:  32,
		SpaceshipHeight: 32,
		AlienWidth:      32,
		AlienHeight:     32,
		BulletWidth:     4,
		BulletHeight:    12,
		GameWidth:       360, // Mobile width
		GameHeight:      480, // Mobile height
		FPSTime:         time.Now(),
		FrameSentTimes:  make(map[uint64]time.Time),
		RoundTripTimes:  make([]float64, 0, 30), // Keep last 30 RTT samples
		LastAckTime:     time.Now(),
	}
}

func (g *Game) ProcessInput(action string, inputType string) {
	if g.GameOver {
		return
	}

	log.Printf("ProcessInput: action=%s, type=%s, SpaceshipX=%.1f", action, inputType, g.SpaceshipX)

	switch action {
	case "left":
		if inputType == "keydown" {
			g.MovingLeft = true
			log.Printf("Set MovingLeft=true")
		} else if inputType == "keyup" {
			g.MovingLeft = false
			log.Printf("Set MovingLeft=false")
		}
	case "right":
		if inputType == "keydown" {
			g.MovingRight = true
			log.Printf("Set MovingRight=true")
		} else if inputType == "keyup" {
			g.MovingRight = false
			log.Printf("Set MovingRight=false")
		}
	case "shoot":
		if inputType == "keydown" && time.Since(g.LastShootTime) > 500*time.Millisecond {
			g.Bullets = append(g.Bullets, Bullet{
				X:     g.SpaceshipX + g.SpaceshipWidth/2 - g.BulletWidth/2,
				Y:     g.GameHeight - 50 - g.SpaceshipHeight,
				VelY:  -400,
				IsAlienBullet: false,
			})
			g.LastShootTime = time.Now()
		}
	}
}

func (g *Game) ProcessFrameAck(frameID uint64) {
	if sentTime, ok := g.FrameSentTimes[frameID]; ok {
		now := time.Now()
		rtt := float64(now.Sub(sentTime).Milliseconds())

		// Add to rolling buffer
		g.RoundTripTimes = append(g.RoundTripTimes, rtt)
		if len(g.RoundTripTimes) > 30 {
			g.RoundTripTimes = g.RoundTripTimes[1:] // Keep last 30
		}

		// Calculate average latency
		if len(g.RoundTripTimes) > 0 {
			sum := 0.0
			for _, t := range g.RoundTripTimes {
				sum += t
			}
			g.AvgLatency = sum / float64(len(g.RoundTripTimes))
		}

		// Calculate client FPS based on acknowledgment rate
		g.AckCount++
		if time.Since(g.LastAckTime) >= time.Second {
			g.ClientFPS = float64(g.AckCount) / time.Since(g.LastAckTime).Seconds()
			g.AckCount = 0
			g.LastAckTime = now
		}

		// Clean up old frame times to prevent memory leak
		delete(g.FrameSentTimes, frameID)
		// Also clean up frames older than 5 seconds
		for id, t := range g.FrameSentTimes {
			if now.Sub(t) > 5*time.Second {
				delete(g.FrameSentTimes, id)
			}
		}
	}
}

func (g *Game) Update(deltaTime float64) {
	if g.GameOver {
		return
	}

	// Check if alien should respawn after explosion
	if g.AlienDead && time.Since(g.AlienDeathTime) > 2*time.Second {
		g.AlienDead = false
		g.AlienX = rand.Float64() * (g.GameWidth - g.AlienWidth)
		g.AlienY = 50 // Spawn at top
		g.AlienVelX = 100 // Reduced for smaller canvas
		if rand.Float64() > 0.5 {
			g.AlienVelX = -g.AlienVelX
		}
	}

	// Update spaceship position (reduced speed for smaller canvas)
	if g.MovingLeft {
		g.SpaceshipX -= 150 * deltaTime
		if g.SpaceshipX < 0 {
			g.SpaceshipX = 0
		}
	}
	if g.MovingRight {
		g.SpaceshipX += 150 * deltaTime
		if g.SpaceshipX > g.GameWidth-g.SpaceshipWidth {
			g.SpaceshipX = g.GameWidth - g.SpaceshipWidth
		}
	}

	// Update alien position only if alive
	if !g.AlienDead {
		g.AlienX += g.AlienVelX * deltaTime
		if g.AlienX <= 0 || g.AlienX >= g.GameWidth-g.AlienWidth {
			g.AlienVelX = -g.AlienVelX
			g.AlienY += 20

			// Clamp alien to boundaries
			if g.AlienX < 0 {
				g.AlienX = 0
			} else if g.AlienX > g.GameWidth-g.AlienWidth {
				g.AlienX = g.GameWidth - g.AlienWidth
			}
		}

		// Alien shoots periodically
		if time.Since(g.LastAlienShoot) > 2*time.Second {
			g.Bullets = append(g.Bullets, Bullet{
				X:     g.AlienX + g.AlienWidth/2 - g.BulletWidth/2,
				Y:     g.AlienY + g.AlienHeight,
				VelY:  200,
				IsAlienBullet: true,
			})
			g.LastAlienShoot = time.Now()
		}
	}

	// Update bullets
	newBullets := make([]Bullet, 0)
	for i := range g.Bullets {
		g.Bullets[i].Y += g.Bullets[i].VelY * deltaTime

		// Keep bullets that are still on screen
		if g.Bullets[i].Y > -g.BulletHeight && g.Bullets[i].Y < g.GameHeight {
			newBullets = append(newBullets, g.Bullets[i])
		}
	}
	g.Bullets = newBullets

	// Check collisions
	g.checkCollisions()

	// Update FPS
	g.FrameCount++
	if time.Since(g.FPSTime) >= time.Second {
		g.CurrentFPS = float64(g.FrameCount) / time.Since(g.FPSTime).Seconds()
		g.FrameCount = 0
		g.FPSTime = time.Now()
	}
}

func (g *Game) checkCollisions() {
	newBullets := make([]Bullet, 0)

	spaceshipY := g.GameHeight - 50

	for _, bullet := range g.Bullets {
		bulletHit := false

		if !bullet.IsAlienBullet && !g.AlienDead {
			// Check collision with alien
			if g.checkAABB(bullet.X, bullet.Y, g.BulletWidth, g.BulletHeight,
				g.AlienX, g.AlienY, g.AlienWidth, g.AlienHeight) {
				g.AlienDead = true
				g.AlienDeathTime = time.Now()
				bulletHit = true
				g.Score += 100
			}
		} else if bullet.IsAlienBullet && !g.SpaceshipDead {
			// Check collision with spaceship
			if g.checkAABB(bullet.X, bullet.Y, g.BulletWidth, g.BulletHeight,
				g.SpaceshipX, spaceshipY, g.SpaceshipWidth, g.SpaceshipHeight) {
				g.SpaceshipDead = true
				g.SpaceshipDeathTime = time.Now()
				g.GameOver = true
				bulletHit = true
			}
		}

		if !bulletHit {
			newBullets = append(newBullets, bullet)
		}
	}

	g.Bullets = newBullets
}

func (g *Game) checkAABB(x1, y1, w1, h1, x2, y2, w2, h2 float64) bool {
	return x1 < x2+w2 && x1+w1 > x2 && y1 < y2+h2 && y1+h1 > y2
}

func (g *Game) RenderHTML() string {
	var html strings.Builder

	if g.GameOver {
		html.WriteString(`<div id="game-container" style="position:relative; width:360px; height:480px; border:2px solid black; display:flex; align-items:center; justify-content:center; font-family:monospace; font-size:32px;">`)

		// Show explosion for spaceship if it just died
		if g.SpaceshipDead && time.Since(g.SpaceshipDeathTime) < 500*time.Millisecond {
			spaceshipY := int(g.GameHeight - 50)
			html.WriteString(fmt.Sprintf(
				`<img src="/static/explosion-spaceship.svg" style="position:absolute; left:%dpx; top:%dpx; width:32px; height:32px;">`,
				int(math.Round(g.SpaceshipX)), spaceshipY))
		}

		html.WriteString(`GAME OVER`)
		html.WriteString(`</div>`)
	} else {
		html.WriteString(`<div id="game-container" style="position:relative; width:360px; height:480px; border:2px solid black; background:white;">`)

		// FPS and latency display inside canvas
		html.WriteString(fmt.Sprintf(
			`<div style="position:absolute; top:5px; right:5px; font-family:monospace; font-size:10px; text-align:right; z-index:10;">
				S:%.0f<br>
				C:%.0f<br>
				L:%.0fms
			</div>`,
			g.CurrentFPS, g.ClientFPS, g.AvgLatency))

		// Score and session display inside canvas
		html.WriteString(fmt.Sprintf(
			`<div style="position:absolute; top:5px; left:5px; font-family:monospace; font-size:12px; z-index:10;">
				Score: %d<br>
				<span style="font-size:8px; color:#666;">Session: %s</span>
			</div>`,
			g.Score, g.SessionID[:8]))

		// Render spaceship or its explosion
		spaceshipY := int(g.GameHeight - 50)
		if g.SpaceshipDead && time.Since(g.SpaceshipDeathTime) < 500*time.Millisecond {
			html.WriteString(fmt.Sprintf(
				`<img src="/static/explosion-spaceship.svg" style="position:absolute; left:%dpx; top:%dpx; width:32px; height:32px;">`,
				int(math.Round(g.SpaceshipX)), spaceshipY))
		} else if !g.SpaceshipDead {
			html.WriteString(fmt.Sprintf(
				`<img src="/static/spaceship.svg" style="position:absolute; left:%dpx; top:%dpx; width:32px; height:32px;">`,
				int(math.Round(g.SpaceshipX)), spaceshipY))
		}

		// Render alien or its explosion
		if g.AlienDead && time.Since(g.AlienDeathTime) < 500*time.Millisecond {
			// Show explosion for 500ms
			html.WriteString(fmt.Sprintf(
				`<img src="/static/explosion-alien.svg" style="position:absolute; left:%dpx; top:%dpx; width:32px; height:32px;">`,
				int(math.Round(g.AlienX)), int(math.Round(g.AlienY))))
		} else if !g.AlienDead {
			// Show normal alien
			html.WriteString(fmt.Sprintf(
				`<img src="/static/alien.svg" style="position:absolute; left:%dpx; top:%dpx; width:32px; height:32px;">`,
				int(math.Round(g.AlienX)), int(math.Round(g.AlienY))))
		}

		// Render bullets
		for _, bullet := range g.Bullets {
			html.WriteString(fmt.Sprintf(
				`<img src="/static/bullet.svg" style="position:absolute; left:%dpx; top:%dpx; width:4px; height:12px;">`,
				int(math.Round(bullet.X)), int(math.Round(bullet.Y))))
		}

		// Add frame acknowledgment form that will be handled by the global event listener
		html.WriteString(fmt.Sprintf(
			`<form id="frame-ack-%d" ws-send hx-trigger="load" style="display:none;"></form>`,
			g.FrameID))

		html.WriteString(`</div>`)
	}

	return html.String()
}