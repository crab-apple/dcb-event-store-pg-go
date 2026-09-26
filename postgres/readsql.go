package postgres

import (
	"fmt"
	"strings"

	"github.com/crab-apple/dcb-event-store-pg-go/eventstore"
)

type paramManager struct {
	args []any
}

func (p *paramManager) add(v any) string {
	p.args = append(p.args, v)
	return fmt.Sprintf("$%d", len(p.args))
}

const readCursorName = "event_cursor"

func buildReadSQL(tableName string, query eventstore.Query, opts eventstore.ReadOptions, upperBound *int64) (string, []any) {
	pm := &paramManager{}
	var filters []string

	if opts.After != nil {
		op := ">"
		if opts.Backwards {
			op = "<"
		}
		filters = append(filters, fmt.Sprintf("e.sequence_position %s %s", op, pm.add(positionToInt64(*opts.After))))
	}
	if upperBound != nil {
		filters = append(filters, fmt.Sprintf("e.sequence_position <= %s", pm.add(*upperBound)))
	}
	if !query.IsAll() {
		if clause := criteriaClause(query, pm); clause != "" {
			filters = append(filters, clause)
		}
	}

	where := ""
	if len(filters) > 0 {
		where = "WHERE " + strings.Join(filters, " AND ")
	}

	order := "ASC"
	if opts.Backwards {
		order = "DESC"
	}

	limit := ""
	if opts.Limit > 0 {
		limit = "LIMIT " + pm.add(opts.Limit)
	}

	sql := fmt.Sprintf(
		`DECLARE %q CURSOR FOR SELECT e.sequence_position, e.type, e.payload, e.tags FROM %s e %s ORDER BY e.sequence_position %s %s`,
		readCursorName, tableName, where, order, limit,
	)
	return sql, pm.args
}

func criteriaClause(query eventstore.Query, pm *paramManager) string {
	var clauses []string
	for _, item := range query.Items() {
		if clause := itemFilterClause(item, pm); clause != "" {
			clauses = append(clauses, clause)
		}
	}
	switch len(clauses) {
	case 0:
		return ""
	case 1:
		return clauses[0]
	default:
		return "(" + strings.Join(clauses, " OR ") + ")"
	}
}

func itemFilterClause(item eventstore.QueryItem, pm *paramManager) string {
	var parts []string
	if len(item.Types) > 0 {
		placeholders := make([]string, len(item.Types))
		for i, t := range item.Types {
			placeholders[i] = pm.add(t)
		}
		parts = append(parts, fmt.Sprintf("type IN (%s)", strings.Join(placeholders, ", ")))
	}
	if item.Tags.Len() > 0 {
		parts = append(parts, fmt.Sprintf("tags && %s::text[]", pm.add(item.Tags.Values())))
	}
	if len(parts) == 0 {
		return ""
	}
	return "(" + strings.Join(parts, " AND ") + ")"
}
