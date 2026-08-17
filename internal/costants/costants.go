package costants

type Role string

const (
	Goalkeeper Role = "Goalkeeper"
	Defender   Role = "Defender"
	Midfielder Role = "Midfielder"
	Forward    Role = "Forward"
)

const (
	DataDir           = "data"
	SessionDir        = "data/session"
	SessionTimeFormat = "2006-01-02 15:04:05"
	DefaultBudget     = 500
	SquadSize         = 25
)

// SquadSlots is the per-role roster size of a full squad (3+8+8+6 = 25).
var SquadSlots = map[Role]int{
	Goalkeeper: 3,
	Defender:   8,
	Midfielder: 8,
	Forward:    6,
}
