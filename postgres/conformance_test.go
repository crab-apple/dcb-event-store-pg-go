package postgres_test

import (
	"testing"

	"github.com/crab-apple/dcb-event-store-pg-go/eventstore"
	"github.com/crab-apple/dcb-event-store-pg-go/eventstore/eventstoretest"
)

func TestConformance(t *testing.T) {
	eventstoretest.Run(t, func(t *testing.T) eventstore.Store {
		store, _ := newInstalledStore(t)
		return store
	})
}
