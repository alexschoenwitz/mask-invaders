package main

import (
	"context"
	"fmt"
	"image/color"
	"log"
	"math"
	"time"

	"io"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/alexschoenwitz/mask-invaders/api/server/api"
)

type UIState int

const (
	UIStateIdle UIState = iota
	UIStateCitySelected
	UIStateAttackTargetSelect
	UIStateProduceTypeSelect
)

type HumanUI struct {
	game           *Game
	playerToken    string
	playerName     string
	playerID       string
	state          UIState
	selectedCity   *CityDisplay
	hoveredCity    *CityDisplay
	errorMessage   string
	errorTime      time.Time
	successMessage string
	successTime    time.Time
}

func NewHumanUI(game *Game, playerName string, playerID string, playerToken string) *HumanUI {
	return &HumanUI{
		game:        game,
		playerName:  playerName,
		playerID:    playerID,
		playerToken: playerToken,
		state:       UIStateIdle,
	}
}

func (h *HumanUI) Update() error {
	// Clear old messages
	now := time.Now()
	if now.Sub(h.errorTime) > 3*time.Second {
		h.errorMessage = ""
	}
	if now.Sub(h.successTime) > 3*time.Second {
		h.successMessage = ""
	}

	// Handle keyboard shortcuts
	if inpututil.IsKeyJustPressed(ebiten.KeyS) {
		h.startGame()
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		// Cancel current action
		h.selectedCity = nil
		h.state = UIStateIdle
		log.Println("Action cancelled")
	}

	// Don't process clicks until game state is ready
	if len(h.game.cities) == 0 {
		return nil
	}

	// Get mouse position
	mx, my := ebiten.CursorPosition()
	mouseX := float64(mx)
	mouseY := float64(my)

	// Calculate scale
	scale := float64(h.game.screenWidth) / float64(screenWidth)

	// Find hovered city
	h.hoveredCity = nil
	for _, city := range h.game.cities {
		cityX := city.X * scale
		cityY := city.Y * scale
		distance := math.Sqrt((mouseX-cityX)*(mouseX-cityX) + (mouseY-cityY)*(mouseY-cityY))
		if distance < 40*scale {
			h.hoveredCity = city
			break
		}
	}

	// Handle mouse clicks
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		h.handleClick(mouseX, mouseY, scale)
	}

	return nil
}

func (h *HumanUI) handleClick(mouseX, mouseY, scale float64) {
	log.Printf("handleClick: state=%d, hoveredCity=%v, selectedCity=%v", h.state, h.hoveredCity, h.selectedCity)
	
	switch h.state {
	case UIStateIdle:
		// Select a city if clicked
		if h.hoveredCity != nil {
			log.Printf("Hovered city: %s, Player: %s, MyPlayerID: %s", h.hoveredCity.Name, h.hoveredCity.Player, h.playerID)
			if h.hoveredCity.Player == h.playerID {
				h.selectedCity = h.hoveredCity
				h.state = UIStateCitySelected
				log.Printf("City selected: %s", h.selectedCity.Name)
			} else {
				h.showError(fmt.Sprintf("That castle belongs to another player!"))
			}
		}

	case UIStateCitySelected:
		// Check if clicking action buttons first
		if h.handleActionButtonClick(mouseX, mouseY, scale) {
			log.Println("Action button clicked, staying in selection mode")
			return
		}
		// Clicking same city again keeps it selected
		if h.hoveredCity == h.selectedCity {
			log.Println("Clicked same city, keeping selection")
			return
		}
		// Deselect if clicking elsewhere
		log.Println("Clicked elsewhere, deselecting")
		h.selectedCity = nil
		h.state = UIStateIdle

	case UIStateAttackTargetSelect:
		// Select target city
		if h.hoveredCity != nil && h.hoveredCity != h.selectedCity {
			log.Printf("Attack target selected: %s", h.hoveredCity.Name)
			h.performAttack(h.hoveredCity)
			h.selectedCity = nil
			h.state = UIStateIdle
		} else {
			// Cancel
			log.Println("Attack cancelled")
			h.state = UIStateCitySelected
		}

	case UIStateProduceTypeSelect:
		// Check if clicking troop type buttons
		if h.handleTroopTypeClick(mouseX, mouseY, scale) {
			return
		}
		// Cancel
		log.Println("Produce cancelled")
		h.state = UIStateCitySelected
	}
}

func (h *HumanUI) handleActionButtonClick(mouseX, mouseY, scale float64) bool {
	if h.selectedCity == nil {
		return false
	}

	cityX := h.selectedCity.X * scale
	cityY := h.selectedCity.Y * scale

	// Button positions
	attackButtonX := cityX - 80*scale
	attackButtonY := cityY + 50*scale
	attackButtonW := 70 * scale
	attackButtonH := 30 * scale

	produceButtonX := cityX + 10*scale
	produceButtonY := cityY + 50*scale
	produceButtonW := 70 * scale
	produceButtonH := 30 * scale

	// Check Attack button
	if mouseX >= attackButtonX && mouseX <= attackButtonX+attackButtonW &&
		mouseY >= attackButtonY && mouseY <= attackButtonY+attackButtonH {
		if h.hasTroops(h.selectedCity) {
			h.state = UIStateAttackTargetSelect
			return true
		} else {
			h.showError("No troops to attack with!")
			return true
		}
	}

	// Check Produce button
	if mouseX >= produceButtonX && mouseX <= produceButtonX+produceButtonW &&
		mouseY >= produceButtonY && mouseY <= produceButtonY+produceButtonH {
		h.state = UIStateProduceTypeSelect
		return true
	}

	return false
}

func (h *HumanUI) handleTroopTypeClick(mouseX, mouseY, scale float64) bool {
	if h.selectedCity == nil {
		return false
	}

	cityX := h.selectedCity.X * scale
	cityY := h.selectedCity.Y * scale

	// Troop type button positions
	troopTypes := []string{"A", "B", "C"}
	buttonY := cityY + 90*scale
	buttonW := 40 * scale
	buttonH := 30 * scale
	buttonSpacing := 45 * scale
	startX := cityX - 60*scale

	for i, troopType := range troopTypes {
		buttonX := startX + float64(i)*buttonSpacing
		if mouseX >= buttonX && mouseX <= buttonX+buttonW &&
			mouseY >= buttonY && mouseY <= buttonY+buttonH {
			h.performProduce(troopType)
			h.selectedCity = nil
			h.state = UIStateIdle
			return true
		}
	}

	return false
}

func (h *HumanUI) hasTroops(city *CityDisplay) bool {
	return city.Troops["A"] > 0 || city.Troops["B"] > 0 || city.Troops["C"] > 0
}

func (h *HumanUI) performAttack(target *CityDisplay) {
	if h.selectedCity == nil {
		return
	}

	// Create attack action with all troops
	action := &api.Action{
		Player: h.playerID,
		Action: &api.Action_Attack{
			Attack: &api.Attack{
				From: h.selectedCity.Name,
				To: &api.Attack_City{
					City: target.Name,
				},
				Troops: map[string]int64{
					"A": h.selectedCity.Troops["A"],
					"B": h.selectedCity.Troops["B"],
					"C": h.selectedCity.Troops["C"],
				},
			},
		},
	}

	if err := h.sendAction(action); err != nil {
		h.showError(fmt.Sprintf("Attack failed: %v", err))
	} else {
		h.showSuccess(fmt.Sprintf("Attacking %s!", target.Name))
	}
}

func (h *HumanUI) performProduce(troopType string) {
	if h.selectedCity == nil {
		return
	}

	// Create produce action
	action := &api.Action{
		Player: h.playerID,
		Action: &api.Action_CreateTroop{
			CreateTroop: &api.CreateTroop{
				In:   h.selectedCity.Name,
				Type: troopType,
			},
		},
	}

	if err := h.sendAction(action); err != nil {
		h.showError(fmt.Sprintf("Production failed: %v", err))
	} else {
		h.showSuccess(fmt.Sprintf("Producing troop %s!", troopType))
	}
}

func (h *HumanUI) startGame() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	log.Printf("Starting game with token: %s", h.playerToken)

	// Create HTTP request
	req, err := newJSONRequest(ctx, "POST", h.game.apiURL+"/v1/start", []byte("{}"))
	if err != nil {
		h.showError(fmt.Sprintf("Failed to create start request: %v", err))
		return
	}

	// Add authorization header
	req.Header.Set("Authorization", h.playerToken)
	log.Printf("Request headers: %v", req.Header)

	// Send request
	resp, err := h.game.httpClient.Do(req)
	if err != nil {
		h.showError(fmt.Sprintf("Failed to start game: %v", err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Start game failed: status %d, body: %s", resp.StatusCode, string(body))
		h.showError(fmt.Sprintf("Failed to start game: status %d", resp.StatusCode))
		return
	}

	h.showSuccess("Game started!")
	log.Println("Game started successfully")
}

func (h *HumanUI) sendAction(action *api.Action) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	log.Printf("Sending action with token: %s", h.playerToken)

	// Serialize action to JSON
	jsonData, err := protojson.Marshal(&api.PostActionRequest{Action: action})
	if err != nil {
		return fmt.Errorf("failed to marshal action: %v", err)
	}

	log.Printf("Action JSON: %s", string(jsonData))

	// Create HTTP request
	req, err := newJSONRequest(ctx, "POST", h.game.apiURL+"/v1/action", jsonData)
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}

	// Add authorization header
	req.Header.Set("Authorization", h.playerToken)
	log.Printf("Request headers: %v", req.Header)

	// Send request
	resp, err := h.game.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send action: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("Action failed: status %d, body: %s", resp.StatusCode, string(body))
		return fmt.Errorf("server returned status %d: %s", resp.StatusCode, string(body))
	}

	log.Println("Action sent successfully")
	return nil
}

func (h *HumanUI) showError(msg string) {
	h.errorMessage = msg
	h.errorTime = time.Now()
	log.Printf("Error: %s", msg)
}

func (h *HumanUI) showSuccess(msg string) {
	h.successMessage = msg
	h.successTime = time.Now()
	log.Printf("Success: %s", msg)
}

func (h *HumanUI) Draw(screen *ebiten.Image) {
	scale := float64(h.game.screenWidth) / float64(screenWidth)

	// Highlight hovered city
	if h.hoveredCity != nil {
		h.drawCityHighlight(screen, h.hoveredCity, scale, color.RGBA{255, 255, 255, 100})
	}

	// Draw selection highlight and buttons
	if h.selectedCity != nil {
		h.drawCityHighlight(screen, h.selectedCity, scale, color.RGBA{255, 255, 0, 150})
		h.drawActionButtons(screen, scale)
	}

	// Draw instructions
	h.drawInstructions(screen)

	// Draw messages
	h.drawMessages(screen)
}

func (h *HumanUI) drawCityHighlight(screen *ebiten.Image, city *CityDisplay, scale float64, col color.RGBA) {
	cityX := float32(city.X * scale)
	cityY := float32(city.Y * scale)
	radius := float32(45 * scale)

	vector.StrokeCircle(screen, cityX, cityY, radius, 3, col, false)
}

func (h *HumanUI) drawActionButtons(screen *ebiten.Image, scale float64) {
	if h.selectedCity == nil {
		return
	}

	cityX := h.selectedCity.X * scale
	cityY := h.selectedCity.Y * scale

	if h.state == UIStateCitySelected {
		// Draw Attack button
		h.drawButton(screen, "Attack", cityX-80*scale, cityY+50*scale, 70*scale, 30*scale, color.RGBA{200, 50, 50, 255})

		// Draw Produce button
		h.drawButton(screen, "Produce", cityX+10*scale, cityY+50*scale, 70*scale, 30*scale, color.RGBA{50, 200, 50, 255})

	} else if h.state == UIStateAttackTargetSelect {
		// Draw cancel instruction
		ebitenutil.DebugPrintAt(screen, "Click target castle or click elsewhere to cancel", int(cityX-100*scale), int(cityY+50*scale))

	} else if h.state == UIStateProduceTypeSelect {
		// Draw troop type buttons
		troopTypes := []string{"A", "B", "C"}
		troopNames := []string{"Archer", "Knight", "Infantry"}
		buttonY := cityY + 90*scale
		startX := cityX - 60*scale

		for i, troopType := range troopTypes {
			buttonX := startX + float64(i)*45*scale
			h.drawButton(screen, troopType, buttonX, buttonY, 40*scale, 30*scale, color.RGBA{100, 100, 200, 255})
			// Draw troop name below
			ebitenutil.DebugPrintAt(screen, troopNames[i], int(buttonX), int(buttonY+35*scale))
		}
	}
}

func (h *HumanUI) drawButton(screen *ebiten.Image, text string, x, y, w, height float64, col color.RGBA) {
	// Draw button background
	vector.DrawFilledRect(screen, float32(x), float32(y), float32(w), float32(height), col, false)

	// Draw button border
	vector.StrokeRect(screen, float32(x), float32(y), float32(w), float32(height), 2, color.RGBA{255, 255, 255, 255}, false)

	// Draw text
	textX := int(x + w/2 - float64(len(text)*3))
	textY := int(y + height/2 - 4)
	ebitenutil.DebugPrintAt(screen, text, textX, textY)
}

func (h *HumanUI) drawInstructions(screen *ebiten.Image) {
	var instructions string
	if len(h.game.cities) == 0 {
		instructions = fmt.Sprintf("Player: %s | Waiting for game state... | Press 'S' to start game", h.playerName)
	} else {
		instructions = fmt.Sprintf("Player: %s | Click your castle to select | Press 'S' to start | ESC to cancel", h.playerName)
	}
	ebitenutil.DebugPrintAt(screen, instructions, 10, 30)
}

func (h *HumanUI) drawMessages(screen *ebiten.Image) {
	y := 50

	if h.errorMessage != "" {
		ebitenutil.DebugPrintAt(screen, "ERROR: "+h.errorMessage, 10, y)
		y += 15
	}

	if h.successMessage != "" {
		ebitenutil.DebugPrintAt(screen, "SUCCESS: "+h.successMessage, 10, y)
	}
}
