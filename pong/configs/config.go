package configs

// Constantes do jogo (devem ser iguais no cliente).
type Config struct {
	ServerDomain string
	ServerPort   string

	ScreenWidth  float64
	ScreenHeight float64
	PaddleWidth  float64
	PaddleHeight float64
	BallSize     float64

	BallSpeedX float64
	BallSpeedY float64
	Speed      float64

	GameState
	ClientInput
}

// Estado do mundo.
type GameState struct {
	Paddle1Y float64
	Paddle2Y float64
	BallX    float64
	BallY    float64
}

type ClientInput struct {
	Cmd    string
	Player int
}

func New() Config {
	return Config{
		ServerDomain: "localhost",
		ServerPort:   "8080",

		ScreenWidth:  960,
		ScreenHeight: 540,
		PaddleWidth:  10,
		PaddleHeight: 100,
		BallSize:     10,

		BallSpeedX: 4.0,
		BallSpeedY: 4.0,
		Speed:      8.0,

		GameState: GameState{
			Paddle1Y: 200,
			Paddle2Y: 200,
			BallX:    320,
			BallY:    240,
		},
	}
}
