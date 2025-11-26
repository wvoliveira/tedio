package main

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"time"

	"github.com/gorilla/websocket"

	"github.com/wvoliveira/pong/configs"
)

// Eventos que o Game Loop aceita
type EventType int

const (
	EventInput EventType = iota
	EventJoin
	EventLeave
)

type GameEvent struct {
	Type     EventType
	PlayerID int
	Cmd      string
	Client   *websocket.Conn
}

var (
	cfg   configs.Config
	state configs.GameState

	// Trocando o mutex para canal de eventos.
	events   = make(chan GameEvent, 100)
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
	http.HandleFunc("/ws", handleConnections)

	slog.Info("Server running at :" + cfg.ServerPort)
	http.ListenAndServe(":"+cfg.ServerPort, nil)
}

func handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Error("error to upgrade to websocket", "error", err)
		return
	}

	defer ws.Close()

	// Envia evento de "Join" para a fila
	events <- GameEvent{Type: EventJoin, Client: ws}

	for {
		messageType, messageData, err := ws.ReadMessage()
		if err != nil {
			// Envia evento de "Leave" para a fila
			events <- GameEvent{Type: EventLeave, Client: ws}
			break
		}

		if messageType != websocket.BinaryMessage {
			continue
		}

		var input configs.ClientInput
		reader := bytes.NewReader(messageData)
		if err := gob.NewDecoder(reader).Decode(&input); err == nil {
			// Envia o Input para a fila. Note que não processamos nada aqui!
			// Apenas repassamos. A conexão não sabe quem é o PlayerID,
			// mas o websocket serve como identificador único.
			events <- GameEvent{Type: EventInput, Client: ws, Cmd: input.Cmd, PlayerID: input.Player}
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
	state := cfg.GameState

	ballSpeedX := cfg.BallSpeedX
	ballSpeedY := cfg.BallSpeedY

	// Mapa de clientes local
	clients := make(map[*websocket.Conn]int)

	ticker := time.NewTicker(15 * time.Millisecond) // ~64 FPS
	defer ticker.Stop()

	for {
		select {
		// 1. Processa Eventos da Fila (Inputs, Conexões)
		case evt := <-events:
			switch evt.Type {
			case EventJoin:
				// Lógica de Join
				id := len(clients) + 1
				clients[evt.Client] = id
				log.Printf("Player %d entrou (Total: %d)", id, len(clients))

			case EventLeave:
				// Lógica de Leave
				if id, ok := clients[evt.Client]; ok {
					delete(clients, evt.Client)
					evt.Client.Close() // Fecha conexão de verdade aqui
					log.Printf("Player %d saiu", id)
				}

			case EventInput:
				// Lógica de Input
				id, ok := clients[evt.Client]
				if !ok {
					continue
				}

				fmt.Println(evt.PlayerID)
				fmt.Println(evt.Cmd)

				// Caso ja tenha dois players online, ignora o segundo player local.
				if len(clients) >= 2 {
					changePlayerState(cfg, id, evt.Cmd, &state)
				} else {
					changePlayerState(cfg, evt.PlayerID, evt.Cmd, &state)
				}
			}

		// 2. Passagem do Tempo (Física)
		case <-ticker.C:
			state.BallX += ballSpeedX
			state.BallY += ballSpeedY

			// Teto/Chão
			if state.BallY <= 0 || state.BallY >= cfg.ScreenHeight-cfg.BallSize {
				ballSpeedY = -ballSpeedY
			}

			// Colisão Raquete 1
			if state.BallX <= 10+cfg.PaddleWidth && state.BallX >= 10 {
				if state.BallY+cfg.BallSize >= state.Paddle1Y && state.BallY <= state.Paddle1Y+cfg.PaddleHeight {
					ballSpeedX = -ballSpeedX * 1.05
					state.BallX = 10 + cfg.PaddleWidth + 2
				}
			}

			// Colisão Raquete 2
			if state.BallX+cfg.BallSize >= (cfg.ScreenWidth-20) && state.BallX <= (cfg.ScreenWidth-20)+cfg.PaddleWidth {
				if state.BallY+cfg.BallSize >= state.Paddle2Y && state.BallY <= state.Paddle2Y+cfg.PaddleHeight {
					ballSpeedX = -ballSpeedX * 1.05
					state.BallX = (cfg.ScreenWidth - 20) - cfg.BallSize - 2
				}
			}

			// Ponto / Reset
			if state.BallX < 0 || state.BallX > cfg.ScreenWidth {
				state.BallX = cfg.BallX
				state.BallY = cfg.BallY
				if state.BallX < 0 {
					ballSpeedX = 4.0
				} else {
					ballSpeedX = -4.0
				}
				ballSpeedY = 4.0
			}

			// Broadcast
			var buf bytes.Buffer
			if err := gob.NewEncoder(&buf).Encode(state); err == nil {
				msg := buf.Bytes()
				for client := range clients {
					// Aqui tem um pequeno risco: se o cliente estiver lento,
					// o WriteMessage pode bloquear o GameLoop.
					// Em produção, usaríamos um canal de saída por cliente.
					if err := client.WriteMessage(websocket.BinaryMessage, msg); err != nil {
						// Se falhar envio, força desconexão no próximo loop
						go func(c *websocket.Conn) { events <- GameEvent{Type: EventLeave, Client: c} }(client)
					}
				}
			}
		}
	}
}
