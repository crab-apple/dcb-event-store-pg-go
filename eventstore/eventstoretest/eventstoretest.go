// Package eventstoretest is a behavioral conformance suite for
// eventstore.Store implementations. Run it against any implementation via
// Run.
package eventstoretest

import (
	"testing"

	"github.com/crab-apple/dcb-event-store-pg-go/eventstore"
)

// NewStore constructs a fresh, isolated Store for a single test.
type NewStore func(t *testing.T) eventstore.Store

// Run executes the conformance suite against newStore.
func Run(t *testing.T, newStore NewStore) {
	t.Run("Append", func(t *testing.T) { testAppend(t, newStore) })
}
