package crud

import (
	"webtyp.com/model"
	"webtyp.com/storage"
)

// Query is what a read asks a Store for. Without Order, rows come ordered by primary key, ascending.
// Rule R1: every condition passed to a Store has Logic() == "AND".
type Query struct {
	Conditions []storage.Condition // AND only (rule R1)
	Order      []storage.Order     // empty = primary key ascending; a hook reads e.g. by expiry date (storage.Asc("expires_at"))
	After      string              // rows with primary key > After; only valid with an empty Order (ErrAfterWithOrder) and a single-column key (ErrAfterCompositeKey)
	Limit      int64               // already capped
}

// Tx reads and writes one table's rows. A Store scoped to a tenant applies the tenant to every call.
type Tx interface {
	ReadOne(rec model.Model, id string) error                                          // an error of kind model.ErrorNotFound when absent
	ReadAll(newRow func() model.Model, q Query, onRow func(model.Model)) error
	Create(rec model.Model) error
	UpdateFields(rec model.Model, id string, fields []string) error
	Delete(rec model.Model, id string) error
}

// Store is the repository of one table or resource (Repository pattern): orm.TenantStore for a table of the module,
// or a Store the module writes over another service's API (D21).
type Store interface {
	Tx
	NewID() string
}

// Transactional is an optional Store capability: fn runs in one transaction and an error rolls everything back.
// A Store without it gets batches of one record only (ErrBatchNotAtomic): crud never promises atomicity that does not exist.
type Transactional interface {
	Atomic(fn func(tx Tx) error) error
}

// ModelChecker is an optional Store capability: Table calls it at mount so a wrong Store for a model stops the boot.
type ModelChecker interface {
	CheckModel(m model.Model) error
}
