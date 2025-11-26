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
}

// Estado do mundo.
type GameState struct {
	Paddle1Y float64
	Paddle2Y float64
	BallX    float64
	BallY    float64
}

func New() Config {
	return Config{
		ServerDomain: "localhost",
		ServerPort:   "8080",

		ScreenWidth:  640,
		ScreenHeight: 480,
		PaddleWidth:  10,
		PaddleHeight: 50,
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
