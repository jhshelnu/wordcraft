package game

import (
	"github.com/google/uuid"
)

type Message struct {
	From    uuid.UUID
	Content string
}
