package memory_test

import (
	"testing"

	"github.com/crab-apple/dcb-event-store-pg-go/eventstore"
	"github.com/crab-apple/dcb-event-store-pg-go/eventstore/eventstoretest"
	"github.com/crab-apple/dcb-event-store-pg-go/memory"
)

func TestConformance(t *testing.T) {
	eventstoretest.Run(t, func(t *testing.T) eventstore.Store {
		return memory.New()
	})
}
