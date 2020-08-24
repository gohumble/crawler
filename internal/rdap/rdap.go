package rdap

import (
	"github.com/openrdap/rdap"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Result struct {
	Result    rdap.Domain
	Referer   string
	Seed      string
	Inserted  bool
	Timestamp primitive.Timestamp
}
