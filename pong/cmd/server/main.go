package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"github.com/wvoliveira/pong-multiplayer/configs"
)

// Input do Cliente
type ClientInput struct {
	Cmd    string // "UP" ou "DOWN"
	Player int    // 1 ou 2
}

var (
	cfg      configs.Config
	state    configs.GameState
	ballVelX = 4.0
	ballVelY = 4.0
	mutex    sync.Mutex
	clients  = make(map[*websocket.Conn]int) // Mapa de Conexão -> PlayerID
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}
)

func init() {
	cfg = configs.New()
	state = cfg.GameState
}

func main() {
	// Seria legal um graceful shutdown aqui, vamos ver.
	go gameLoop(cfg)

	// Configura servidor HTTP/WebSocket
	http.HandleFunc("/ws", handleConnections(cfg))

	slog.Info("Server running at :" + cfg.ServerPort)
	http.ListenAndServe(":"+cfg.ServerPort, nil)
}

func handleConnections(cfg configs.Config) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			slog.Error("error to upgrade to websocket", "error", err)
			return
		}

		defer ws.Close()

		// Define quem é o player (1 ou 2) baseado na ordem de chegada
		mutex.Lock()
		playerID := len(clients) + 1
		clients[ws] = playerID
		mutex.Unlock()

		slog.Info(fmt.Sprintf("Player %d connected!", playerID))

		for {
			// Lê mensagem do WebSocket
			messageType, messageData, err := ws.ReadMessage()
			if err != nil {
				slog.Info(fmt.Sprintf("Player %d desconnected", playerID))

				// Deleta o player na lista dae clientes.
				mutex.Lock()
				delete(clients, ws)
				mutex.Unlock()
				break
			}

			// Ignora se não for binário
			if messageType != websocket.BinaryMessage {
				continue
			}

			// Decodifica Gob
			var input ClientInput
			reader := bytes.NewReader(messageData)
			dec := gob.NewDecoder(reader)
			if err := dec.Decode(&input); err != nil {
				slog.Info("error to decode input", "error", err)
				continue
			}

			// Aplica input no estado (Thread-safe)
			// Talvez seja interessante adicionar fila inves do mutex lock e unlock.
			mutex.Lock()

			// Caso ja tenha dois players online, ignora o segundo player local.
			if len(clients) >= 2 {
				changePlayerState(cfg, playerID, input.Cmd, &state)
			} else {
				changePlayerState(cfg, input.Player, input.Cmd, &state)
			}

			mutex.Unlock()
		}
	}
}

func changePlayerState(cfg configs.Config, player int, direction string, state *configs.GameState) {
	switch player {
	case 1:
		if direction == "UP" && state.Paddle1Y > 0 {
			state.Paddle1Y -= cfg.Speed
		}
		if direction == "DOWN" && state.Paddle1Y < cfg.ScreenHeight-cfg.PaddleHeight {
			state.Paddle1Y += cfg.Speed
		}
	case 2:
		if direction == "UP" && state.Paddle2Y > 0 {
			state.Paddle2Y -= cfg.Speed
		}
		if direction == "DOWN" && state.Paddle2Y < cfg.ScreenHeight-cfg.PaddleHeight {
			state.Paddle2Y += cfg.Speed
		}
	}
}

func gameLoop(cfg configs.Config) {
	ticker := time.NewTicker(15 * time.Millisecond) // ~64 FPS
	defer ticker.Stop()

	for range ticker.C {
		mutex.Lock()

		// 1. Atualiza a posição da bola
		state.BallX += ballVelX
		state.BallY += ballVelY

		// 2. Colisão com teto e chão
		if state.BallY <= 0 || state.BallY >= cfg.ScreenHeight-cfg.BallSize {
			ballVelY = -ballVelY
		}

		// 3. Colisão com raquete 1 (esquerda)
		// X da raquete é 10
		if state.BallX <= 10+cfg.PaddleWidth && state.BallX >= 10 {
			// Verifica altura
			if state.BallY+cfg.BallSize >= state.Paddle1Y && state.BallY <= state.Paddle1Y+cfg.PaddleHeight {
				ballVelX = -ballVelX                   // Inverte
				ballVelX *= 1.05                       // Acelera um pouco
				state.BallX = 10 + cfg.PaddleWidth + 2 // Para não grudar a bolinha
			}
		}

		// 4. Colisão com raquete 2 (direita)
		// X da raquete é 620
		if state.BallX+cfg.BallSize >= 620 && state.BallX <= 620+cfg.PaddleWidth {
			// Verifica altura
			if state.BallY+cfg.BallSize >= state.Paddle2Y && state.BallY <= state.Paddle2Y+cfg.PaddleHeight {
				ballVelX = -ballVelX
				ballVelX *= 1.05
				state.BallX = 620 - cfg.BallSize - 2 // Para não grudar a bolinha
			}
		}

		// 5. Saiu da tela (ponto)
		if state.BallX < 0 || state.BallX > cfg.ScreenWidth {
			// Reset
			state.BallX = 320
			state.BallY = 240
			ballVelX = 4.0 // Reseta velocidade
			if state.BallX < 0 {
				ballVelX = 4.0 // Ponto do player 2, bola vai pra direita
			} else {
				ballVelX = -4.0 // Ponto do player 1, bola vai pra esquerda
			}
			ballVelY = 4.0
		}

		// 6. Broadcast via Gob
		var buf bytes.Buffer
		enc := gob.NewEncoder(&buf)
		if err := enc.Encode(state); err != nil {
			slog.Error("error to encode state:", "error", err)
		} else {
			stateBytes := buf.Bytes()
			for client := range clients {
				client.WriteMessage(websocket.BinaryMessage, stateBytes)
			}
		}
		mutex.Unlock()
	}
}
