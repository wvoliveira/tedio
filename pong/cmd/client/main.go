package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"image/color"
	"log/slog"
	"os"

	"github.com/gorilla/websocket"
	"golang.org/x/image/font/basicfont"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/wvoliveira/pong/configs"
)

type GameState struct {
	Paddle1Y float32
	Paddle2Y float32
	BallX    float32
	BallY    float32
}

type Game struct {
	state GameState
	ws    *websocket.Conn
	cfg   configs.Config
}

func (g *Game) Update() error {
	var (
		cmd    string
		player int
	)
	// Controles player 1 (w/s) e player 2 (setas)
	// Como o cliente não sabe quem ele é, mandamos o input e o servidor decide
	if ebiten.IsKeyPressed(ebiten.KeyW) {
		cmd = "UP"
		player = 1
	}

	if ebiten.IsKeyPressed(ebiten.KeyS) {
		cmd = "DOWN"
		player = 1
	}

	if ebiten.IsKeyPressed(ebiten.KeyArrowUp) {
		cmd = "UP"
		player = 2
	}

	if ebiten.IsKeyPressed(ebiten.KeyArrowDown) {
		cmd = "DOWN"
		player = 2
	}

	// Envia input se houver comando
	if cmd != "" {
		input := configs.ClientInput{Cmd: cmd, Player: player}
		var buf bytes.Buffer
		enc := gob.NewEncoder(&buf)
		if err := enc.Encode(input); err == nil {
			g.ws.WriteMessage(websocket.BinaryMessage, buf.Bytes())
		}
	}
	return nil
}

func (g *Game) Draw(screen *ebiten.Image) {
	// Fundo cinza escuro
	screen.Fill(color.RGBA{0x20, 0x20, 0x30, 0xff})

	// Raquete 1 (branca)
	vector.FillRect(screen, 10, g.state.Paddle1Y, 10, float32(g.cfg.PaddleHeight), color.White, false)

	// Raquete 2 (branca)
	vector.FillRect(screen, float32(g.cfg.ScreenWidth)-20, g.state.Paddle2Y, 10, float32(g.cfg.PaddleHeight), color.White, false)

	// Bola (amarela)
	vector.FillRect(screen, g.state.BallX, g.state.BallY, 10, 10, color.RGBA{0xff, 0xd7, 0x00, 0xff}, false)

	// Texto de ajuda
	msg := "Player 1: W/S  |  Player 2: Arrows"
	text.Draw(screen, msg, text.NewGoXFace(basicfont.Face7x13), &text.DrawOptions{})
}

func (g *Game) Layout(outsideWidth, outsideHeight int) (int, int) {
	return int(g.cfg.ScreenWidth), int(g.cfg.ScreenHeight)
}

func main() {
	var (
		cfg       = configs.New()
		serverURL = fmt.Sprintf("ws://%s:%s/ws", cfg.ServerDomain, cfg.ServerPort)
	)

	// Conecta ao servidor
	ws, _, err := websocket.DefaultDialer.Dial(serverURL, nil)
	if err != nil {
		slog.Error("error to connect to %s: %v", serverURL, err)
		os.Exit(1)
	}

	defer ws.Close()

	game := &Game{ws: ws, cfg: cfg}

	// Goroutine para receber atualizações do servidor
	go func() {
		for {
			msgType, msgData, err := ws.ReadMessage()
			if err != nil {
				slog.Info("desconnect from server.")
				return
			}
			if msgType == websocket.BinaryMessage {
				reader := bytes.NewReader(msgData)
				dec := gob.NewDecoder(reader)
				dec.Decode(&game.state)
			}
		}
	}()

	ebiten.SetWindowSize(int(cfg.ScreenWidth), int(cfg.ScreenHeight))
	ebiten.SetWindowTitle("Pong multiplayer")

	if err := ebiten.RunGame(game); err != nil {
		slog.Error("error to run game", "error", err)
	}
}
