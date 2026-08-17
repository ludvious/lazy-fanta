package parsing

import (
	"fmt"
	"lazy-fanta/internal/costants"
	"strings"
)

// ParseRole maps CLI shorthands to a Role.
func ParseRole(s string) (costants.Role, error) {
	switch strings.ToLower(s) {
	case "gk", "por":
		return costants.Goalkeeper, nil
	case "df", "dif":
		return costants.Defender, nil
	case "md", "cen":
		return costants.Midfielder, nil
	case "fw", "att":
		return costants.Forward, nil
	}
	return "", fmt.Errorf("unknown role %q (Use `gk`, `df`, `md`, `fw`, or italian aliases like `por`, `dif`, `cen`, `att`)", s)
}
