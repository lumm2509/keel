package dml

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/lumm2509/keel/dal"
)

type txMetrics struct {
	beginCount    atomic.Int64
	commitCount   atomic.Int64
	rollbackCount atomic.Int64
	execCount     atomic.Int64
}

var txDriverSeq atomic.Int64

type txTestDriver struct {
	metrics *txMetrics
}

type txTestConn struct {
	metrics *txMetrics
}

type txTestTx struct {
	metrics *txMetrics
}

type txTestResult struct{}

func (txTestResult) LastInsertId() (int64, error) { return 0, nil }
func (txTestResult) RowsAffected() (int64, error) { return 1, nil }

func (d *txTestDriver) Open(string) (driver.Conn, error) {
	return &txTestConn{metrics: d.metrics}, nil
}

func (c *txTestConn) Prepare(string) (driver.Stmt, error) { return nil, errors.New("not implemented") }
func (c *txTestConn) Close() error                        { return nil }
func (c *txTestConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c *txTestConn) Ping(context.Context) error { return nil }

func (c *txTestConn) BeginTx(context.Context, driver.TxOptions) (driver.Tx, error) {
	c.metrics.beginCount.Add(1)
	return &txTestTx{metrics: c.metrics}, nil
}

func (c *txTestConn) ExecContext(context.Context, string, []driver.NamedValue) (driver.Result, error) {
	c.metrics.execCount.Add(1)
	return txTestResult{}, nil
}

func (c *txTestConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return &txTestRows{}, nil
}

func (tx *txTestTx) Commit() error {
	tx.metrics.commitCount.Add(1)
	return nil
}

func (tx *txTestTx) Rollback() error {
	tx.metrics.rollbackCount.Add(1)
	return nil
}

type txTestRows struct{}

func (r *txTestRows) Columns() []string         { return []string{"noop"} }
func (r *txTestRows) Close() error              { return nil }
func (r *txTestRows) Next([]driver.Value) error { return io.EOF }

func openTxTestDB(metrics *txMetrics) (*sql.DB, error) {
	driverName := fmt.Sprintf("dml_tx_test_%d", txDriverSeq.Add(1))
	sql.Register(driverName, &txTestDriver{metrics: metrics})

	db, err := sql.Open(driverName, "postgres://tx-test")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(64)
	return db, nil
}

func TestRunInTransactionConcurrentMetrics(t *testing.T) {
	t.Parallel()

	metrics := &txMetrics{}
	db, err := openTxTestDB(metrics)
	if err != nil {
		t.Fatalf("openTxTestDB() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	app := NewApp(dal.New(db))

	const workers = 48
	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make(chan error, workers)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			record := &dal.Record{
				ID:         fmt.Sprintf("rec-%d", i),
				Collection: &dal.Collection{Name: "events"},
				Data:       map[string]any{"value": i},
			}
			errs <- app.DML().RunInTransaction(func(txApp App) error {
				return txApp.DML().SaveNoValidate(record)
			})
		}(i)
	}

	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatalf("RunInTransaction() error = %v", err)
		}
	}

	if got := metrics.beginCount.Load(); got != workers {
		t.Fatalf("begin count = %d, want %d", got, workers)
	}
	if got := metrics.commitCount.Load(); got != workers {
		t.Fatalf("commit count = %d, want %d", got, workers)
	}
	if got := metrics.rollbackCount.Load(); got != 0 {
		t.Fatalf("rollback count = %d, want 0", got)
	}
	if got := metrics.execCount.Load(); got != workers {
		t.Fatalf("exec count = %d, want %d", got, workers)
	}
}

func TestRunInTransactionConcurrentRollbackMetrics(t *testing.T) {
	t.Parallel()

	metrics := &txMetrics{}
	db, err := openTxTestDB(metrics)
	if err != nil {
		t.Fatalf("openTxTestDB() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	app := NewApp(dal.New(db))

	const workers = 40
	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make(chan error, workers)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			errs <- app.DML().RunInTransaction(func(txApp App) error {
				if i%2 == 0 {
					return errors.New("forced rollback")
				}
				record := &dal.Record{
					ID:         fmt.Sprintf("rec-%d", i),
					Collection: &dal.Collection{Name: "events"},
					Data:       map[string]any{"value": i},
				}
				return txApp.DML().SaveNoValidate(record)
			})
		}(i)
	}

	close(start)
	wg.Wait()
	close(errs)

	var failures int
	for err := range errs {
		if err != nil {
			failures++
		}
	}

	if failures != workers/2 {
		t.Fatalf("rollback failures = %d, want %d", failures, workers/2)
	}
	if got := metrics.beginCount.Load(); got != workers {
		t.Fatalf("begin count = %d, want %d", got, workers)
	}
	if got := metrics.commitCount.Load(); got != workers/2 {
		t.Fatalf("commit count = %d, want %d", got, workers/2)
	}
	if got := metrics.rollbackCount.Load(); got != workers/2 {
		t.Fatalf("rollback count = %d, want %d", got, workers/2)
	}
	if got := metrics.execCount.Load(); got != workers/2 {
		t.Fatalf("exec count = %d, want %d", got, workers/2)
	}
}

func TestValidateRejectsInvalidModels(t *testing.T) {
	t.Parallel()

	app := NewApp(dal.New(nil))

	tests := []struct {
		name  string
		model Model
	}{
		{
			name:  "record missing collection",
			model: &dal.Record{ID: "r1", Data: map[string]any{}},
		},
		{
			name: "collection duplicate fields",
			model: &dal.Collection{
				ID:   "c1",
				Name: "posts",
				Fields: dal.FieldsList{
					{Name: "title"},
					{Name: "title"},
				},
			},
		},
		{
			name: "empty id",
			model: &dal.Record{
				ID:         "",
				Collection: &dal.Collection{Name: "posts"},
				Data:       map[string]any{},
			},
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := app.DML().Validate(tc.model); err == nil {
				t.Fatalf("Validate() error = nil, want error")
			}
		})
	}
}

func TestValidateRejectsPointerToNonStruct(t *testing.T) {
	t.Parallel()

	app := NewApp(dal.New(nil))
	value := "abc"

	err := app.DML().Validate(&value)
	if err == nil {
		t.Fatal("expected error for pointer to non-struct model")
	}
	if !errors.Is(err, ErrInvalidModel) {
		t.Fatalf("expected ErrInvalidModel, got %v", err)
	}
}

func TestSaveNoValidateRejectsMissingTableName(t *testing.T) {
	t.Parallel()

	db, err := openTxTestDB(&txMetrics{})
	if err != nil {
		t.Fatalf("openTxTestDB() error = %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	app := NewApp(dal.New(db))
	model := &struct {
		ID string
	}{
		ID: "abc",
	}

	err = app.DML().SaveNoValidate(model)
	if err == nil {
		t.Fatal("expected error for model without table name")
	}
	if !errors.Is(err, ErrInvalidModel) {
		t.Fatalf("expected ErrInvalidModel, got %v", err)
	}
}

func TestSaveViewRejectsUnsafeSQL(t *testing.T) {
	t.Parallel()

	app := NewApp(dal.New(nil))
	err := app.DML().(*Service).saveView("danger", "select * from users; drop table users")
	if err == nil {
		t.Fatalf("saveView() error = nil, want error")
	}
}

type expandMetrics struct {
	findCalls atomic.Int64
}

type fakeExpandDAL struct {
	*dal.Service
	metrics *expandMetrics
	data    map[string]map[string]*dal.Record
}

func (f *fakeExpandDAL) FindRecordsByIds(collectionModelOrIdentifier any, recordIds []string, optFilters ...dal.QueryFilter) ([]*dal.Record, error) {
	f.metrics.findCalls.Add(1)

	table := fmt.Sprint(collectionModelOrIdentifier)
	collectionData := f.data[table]
	result := make([]*dal.Record, 0, len(recordIds))
	for _, id := range recordIds {
		if record, ok := collectionData[id]; ok {
			clone := *record
			clone.Data = mapsClone(record.Data)
			clone.Expand = map[string]any{}
			result = append(result, &clone)
		}
	}
	return result, nil
}

func TestExpandRecordsNestedConcurrentMetrics(t *testing.T) {
	t.Parallel()

	dao := &fakeExpandDAL{
		Service: dal.New(nil),
		metrics: &expandMetrics{},
		data: map[string]map[string]*dal.Record{
			"users": {
				"u1": {ID: "u1", Collection: &dal.Collection{Name: "users", Fields: dal.FieldsList{{Name: "org", Meta: map[string]any{"collection": "orgs"}}}}, Data: map[string]any{"org": "o1"}},
			},
			"orgs": {
				"o1": {ID: "o1", Collection: &dal.Collection{Name: "orgs"}, Data: map[string]any{"name": "OpenAI"}},
			},
		},
	}
	app := NewApp(dao)

	const workers = 64
	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make(chan error, workers)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			record := &dal.Record{
				ID: "p1",
				Collection: &dal.Collection{
					Name: "posts",
					Fields: dal.FieldsList{
						{Name: "author", Meta: map[string]any{"collection": "users"}},
					},
				},
				Data:   map[string]any{"author": "u1"},
				Expand: map[string]any{},
			}
			if errsMap := app.DML().ExpandRecord(record, []string{"author.org"}, nil); len(errsMap) > 0 {
				for _, err := range errsMap {
					errs <- err
					return
				}
			}
			author, ok := record.Expand["author"].(*dal.Record)
			if !ok || author == nil {
				errs <- errors.New("author expansion missing")
				return
			}
			org, ok := author.Expand["org"].(*dal.Record)
			if !ok || org == nil || org.ID != "o1" {
				errs <- errors.New("nested org expansion missing")
				return
			}
			errs <- nil
		}()
	}

	close(start)
	wg.Wait()
	close(errs)

	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}

	expectedCalls := int64(workers * 2)
	if got := dao.metrics.findCalls.Load(); got != expectedCalls {
		t.Fatalf("FindRecordsByIds calls = %d, want %d", got, expectedCalls)
	}
}

func BenchmarkRunInTransactionParallel(b *testing.B) {
	metrics := &txMetrics{}
	db, err := openTxTestDB(metrics)
	if err != nil {
		b.Fatalf("openTxTestDB() error = %v", err)
	}
	defer db.Close()
	app := NewApp(dal.New(db))

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			record := &dal.Record{
				ID:         fmt.Sprintf("bench-%d", i),
				Collection: &dal.Collection{Name: "events"},
				Data:       map[string]any{"value": i},
			}
			if err := app.DML().RunInTransaction(func(txApp App) error {
				return txApp.DML().SaveNoValidate(record)
			}); err != nil {
				b.Fatalf("RunInTransaction() error = %v", err)
			}
			i++
		}
	})
}

func BenchmarkExpandRecordsParallel(b *testing.B) {
	dao := &fakeExpandDAL{
		Service: dal.New(nil),
		metrics: &expandMetrics{},
		data: map[string]map[string]*dal.Record{
			"users": {
				"u1": {ID: "u1", Collection: &dal.Collection{Name: "users", Fields: dal.FieldsList{{Name: "org", Meta: map[string]any{"collection": "orgs"}}}}, Data: map[string]any{"org": "o1"}},
			},
			"orgs": {
				"o1": {ID: "o1", Collection: &dal.Collection{Name: "orgs"}, Data: map[string]any{"name": "OpenAI"}},
			},
		},
	}
	app := NewApp(dao)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			record := &dal.Record{
				ID: "p1",
				Collection: &dal.Collection{
					Name: "posts",
					Fields: dal.FieldsList{
						{Name: "author", Meta: map[string]any{"collection": "users"}},
					},
				},
				Data:   map[string]any{"author": "u1"},
				Expand: map[string]any{},
			}
			if errs := app.DML().ExpandRecord(record, []string{"author.org"}, nil); len(errs) > 0 {
				b.Fatalf("ExpandRecord() errors = %v", errs)
			}
		}
	})
}

func mapsClone(src map[string]any) map[string]any {
	if src == nil {
		return nil
	}
	dst := make(map[string]any, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
