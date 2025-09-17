package main

import (
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"
)

type Bullet struct {
	ID            int
	X             float64
	Y             float64
	VelY          float64
	IsAlienBullet bool
}

type Game struct {
	SpaceshipX   float64
	AlienX       float64
	AlienY       float64
	AlienVelX    float64
	Bullets      []Bullet
	NextBulletID int

	MovingLeft  bool
	MovingRight bool

	LastShootTime  time.Time
	LastAlienShoot time.Time

	FrameCount int
	FPSTime    time.Time
	CurrentFPS float64

	GameOver bool
	Score    int

	SpaceshipWidth  float64
	SpaceshipHeight float64
	AlienWidth      float64
	AlienHeight     float64
	BulletWidth     float64
	BulletHeight    float64

	GameWidth  float64
	GameHeight float64

	AlienDead          bool
	AlienDeathTime     time.Time
	SpaceshipDead      bool
	SpaceshipDeathTime time.Time

	// Frame acknowledgment tracking
	FrameID        uint64
	FrameSentTimes map[uint64]time.Time
	RoundTripTimes []float64
	ClientFPS      float64
	AvgLatency     float64
	LastAckTime    time.Time
	AckCount       int

	// Session tracking
	SessionID string

	VisibleBullets      map[int]struct{}
	LastSpaceshipSprite string
	LastAlienSprite     string
	LastGameOver        bool
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
		VisibleBullets:  make(map[int]struct{}),
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
			bullet := Bullet{
				ID:            g.NextBulletID,
				X:             g.SpaceshipX + g.SpaceshipWidth/2 - g.BulletWidth/2,
				Y:             g.GameHeight - 50 - g.SpaceshipHeight,
				VelY:          -400,
				IsAlienBullet: false,
			}
			g.NextBulletID++
			g.Bullets = append(g.Bullets, bullet)
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
		g.AlienY = 50     // Spawn at top
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
			bullet := Bullet{
				ID:            g.NextBulletID,
				X:             g.AlienX + g.AlienWidth/2 - g.BulletWidth/2,
				Y:             g.AlienY + g.AlienHeight,
				VelY:          200,
				IsAlienBullet: true,
			}
			g.NextBulletID++
			g.Bullets = append(g.Bullets, bullet)
			g.LastAlienShoot = time.Now()
		}
	}

	// Update bullets
	newBullets := make([]Bullet, 0, len(g.Bullets))
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

	// HUD counters
	html.WriteString(fmt.Sprintf(`<span id="score-value" hx-swap-oob="innerHTML">%d</span>`, g.Score))
	html.WriteString(fmt.Sprintf(`<span id="server-fps-value" hx-swap-oob="innerHTML">%.0f</span>`, g.CurrentFPS))
	html.WriteString(fmt.Sprintf(`<span id="client-fps-value" hx-swap-oob="innerHTML">%.0f</span>`, g.ClientFPS))
	html.WriteString(fmt.Sprintf(`<span id="latency-value" hx-swap-oob="innerHTML">%.0f</span>`, g.AvgLatency))
	html.WriteString(fmt.Sprintf(`<span id="session-id-value" hx-swap-oob="innerHTML">%s</span>`, g.shortSessionID()))

	// Frame metadata for piggyback acknowledgements
	html.WriteString(fmt.Sprintf(`<div id="frame-info" hx-swap-oob="outerHTML" data-frame-id="%d"></div>`, g.FrameID))

	// Track which bullets are visible so we only re-render when the set changes
	currentBullets := make(map[int]struct{}, len(g.Bullets))
	for _, bullet := range g.Bullets {
		currentBullets[bullet.ID] = struct{}{}
	}

	if !bulletSetEqual(currentBullets, g.VisibleBullets) {
		html.WriteString(`<div id="bullet-layer" hx-swap-oob="innerHTML">`)
		for _, bullet := range g.Bullets {
			html.WriteString(fmt.Sprintf(`<img id="bullet-%d" class="sprite bullet" src="/static/bullet.svg" alt="Bullet">`, bullet.ID))
		}
		html.WriteString(`</div>`)
	}
	g.VisibleBullets = currentBullets

	// Update sprite states with minimal swaps
	spaceshipState := g.determineSpaceshipSprite()
	if spaceshipState != g.LastSpaceshipSprite {
		html.WriteString(renderSpaceshipSwap(spaceshipState))
		g.LastSpaceshipSprite = spaceshipState
	}

	alienState := g.determineAlienSprite()
	if alienState != g.LastAlienSprite {
		html.WriteString(renderAlienSwap(alienState))
		g.LastAlienSprite = alienState
	}

	if g.GameOver != g.LastGameOver {
		html.WriteString(renderOverlaySwap(g.GameOver))
		g.LastGameOver = g.GameOver
	}

	// Position updates via CSS transforms
	spaceshipY := g.GameHeight - 50
	html.WriteString(`<style id="sprite-style" hx-swap-oob="outerHTML">`)
	html.WriteString(fmt.Sprintf(`#spaceship-sprite{transform:translate(%.2fpx, %.2fpx);}`, g.SpaceshipX, spaceshipY))
	html.WriteString(fmt.Sprintf(`#alien-sprite{transform:translate(%.2fpx, %.2fpx);}`, g.AlienX, g.AlienY))
	for _, bullet := range g.Bullets {
		html.WriteString(fmt.Sprintf(`#bullet-%d{transform:translate(%.2fpx, %.2fpx);}`, bullet.ID, bullet.X, bullet.Y))
	}
	html.WriteString(`</style>`)

	return html.String()
}

func (g *Game) shortSessionID() string {
	if len(g.SessionID) <= 8 {
		return g.SessionID
	}
	return g.SessionID[:8]
}

func renderSpaceshipSwap(state string) string {
	switch state {
	case "exploding":
		return `<img id="spaceship-sprite" class="sprite" src="/static/explosion-spaceship.svg" alt="Spaceship explosion" hx-swap-oob="outerHTML">`
	case "hidden":
		return `<img id="spaceship-sprite" class="sprite sprite-hidden" src="/static/spaceship.svg" alt="Spaceship" hx-swap-oob="outerHTML">`
	default:
		return `<img id="spaceship-sprite" class="sprite" src="/static/spaceship.svg" alt="Spaceship" hx-swap-oob="outerHTML">`
	}
}

func renderAlienSwap(state string) string {
	switch state {
	case "exploding":
		return `<img id="alien-sprite" class="sprite" src="/static/explosion-alien.svg" alt="Alien explosion" hx-swap-oob="outerHTML">`
	case "hidden":
		return `<img id="alien-sprite" class="sprite sprite-hidden" src="/static/alien.svg" alt="Alien" hx-swap-oob="outerHTML">`
	default:
		return `<img id="alien-sprite" class="sprite" src="/static/alien.svg" alt="Alien" hx-swap-oob="outerHTML">`
	}
}

func renderOverlaySwap(visible bool) string {
	className := "game-overlay"
	if visible {
		className += " is-visible"
	}
	return fmt.Sprintf(`<div id="game-overlay" class="%s" hx-swap-oob="outerHTML"><div class="game-over-text">GAME OVER</div></div>`, className)
}

func (g *Game) determineSpaceshipSprite() string {
	if g.SpaceshipDead {
		if time.Since(g.SpaceshipDeathTime) < 500*time.Millisecond {
			return "exploding"
		}
		return "hidden"
	}
	return "alive"
}

func (g *Game) determineAlienSprite() string {
	if g.AlienDead {
		if time.Since(g.AlienDeathTime) < 500*time.Millisecond {
			return "exploding"
		}
		return "hidden"
	}
	return "alive"
}

func bulletSetEqual(a, b map[int]struct{}) bool {
	if len(a) != len(b) {
		return false
	}
	for id := range a {
		if _, ok := b[id]; !ok {
			return false
		}
	}
	return true
}
