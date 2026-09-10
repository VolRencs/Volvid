package tui

import (
	"context"

	"volvid/internal/adapters"
)

func newTestModel() Model {
	return newModelWithAPI(context.Background(), newAppAPI(adapters.NewEnv()))
}
