package postgres

import (
	"testing"

	"github.com/crab-apple/dcb-event-store-pg-go/eventstore"
	"github.com/crab-apple/dcb-event-store-pg-go/eventstore/eventstoretest"
)

// The COPY path has to be behaviourally identical to the stored-procedure
// one, so it gets the same suite. A zero threshold routes every append
// through COPY, including the single-event appends that dominate the suite.
func TestConformance_CopyPath(t *testing.T) {
	eventstoretest.Run(t, func(t *testing.T) eventstore.Store {
		store, _ := NewInstalledStore(t)
		store.copyThreshold = 0
		return store
	})
}
