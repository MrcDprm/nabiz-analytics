package store

import (
	"database/sql/driver"

	"modernc.org/sqlite"
)

// uniq(x), COUNT(DISTINCT x) ile aynı sonucu verir ama tekrarları SQLite'ın
// geçici ağacı yerine Go map'inde ayıklar. Yüz binlerce ziyaretçi kimliğinde
// belirgin şekilde daha hızlı (bkz. internal/stats benchmark'ları).
func init() {
	if err := sqlite.RegisterFunction("uniq", &sqlite.FunctionImpl{
		NArgs:         1,
		Deterministic: true,
		MakeAggregate: func(sqlite.FunctionContext) (sqlite.AggregateFunction, error) {
			return &uniqAgg{seen: make(map[int64]struct{})}, nil
		},
	}); err != nil {
		panic(err)
	}
}

type uniqAgg struct{ seen map[int64]struct{} }

func (u *uniqAgg) Step(_ *sqlite.FunctionContext, args []driver.Value) error {
	if v, ok := args[0].(int64); ok {
		u.seen[v] = struct{}{}
	}
	return nil
}

func (u *uniqAgg) WindowInverse(*sqlite.FunctionContext, []driver.Value) error { return nil }

func (u *uniqAgg) WindowValue(*sqlite.FunctionContext) (driver.Value, error) {
	return int64(len(u.seen)), nil
}

func (u *uniqAgg) Final(*sqlite.FunctionContext) {}
