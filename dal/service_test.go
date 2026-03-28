package dal

import (
	"testing"
)

func TestCanAccessRecordComplexRule(t *testing.T) {
	t.Parallel()

	dao := New(nil)
	record := &Record{
		ID:   "r1",
		Data: map[string]any{"owner": "u1", "active": true, "tenant": "acme"},
	}
	info := &RequestInfo{
		Headers: map[string]string{"authorization": "Bearer x", "x_tenant": "acme"},
		Query:   map[string]string{"mode": "write"},
		Body:    map[string]any{"confirmed": true},
	}
	rule := `(auth && header:x_tenant == record.tenant) && (query:mode == 'write' || body:confirmed == true) && !record.deleted`

	ok, err := dao.CanAccessRecord(record, info, &rule)
	if err != nil {
		t.Fatalf("CanAccessRecord() error = %v", err)
	}
	if !ok {
		t.Fatalf("CanAccessRecord() = false, want true")
	}
}

func TestCanAccessRecordParallelDeterministic(t *testing.T) {
	t.Parallel()

	dao := New(nil)
	record := &Record{
		ID:   "r1",
		Data: map[string]any{"owner": "u1", "active": true, "tenant": "acme"},
	}
	info := &RequestInfo{
		Headers: map[string]string{"authorization": "Bearer x", "x_tenant": "acme"},
		Query:   map[string]string{"mode": "write"},
		Body:    map[string]any{"confirmed": true},
	}

	rules := []struct {
		rule string
		want bool
	}{
		{`auth && header:x_tenant == record.tenant`, true},
		{`auth && header:x_tenant != record.tenant`, false},
		{`(query:mode == "write" && body:confirmed) || header:missing`, true},
		{`!auth || record.missing`, false},
	}

	for _, tc := range rules {
		tc := tc
		t.Run(tc.rule, func(t *testing.T) {
			t.Parallel()
			for i := 0; i < 500; i++ {
				ok, err := dao.CanAccessRecord(record, info, &tc.rule)
				if err != nil {
					t.Fatalf("CanAccessRecord() error = %v", err)
				}
				if ok != tc.want {
					t.Fatalf("CanAccessRecord() = %v, want %v", ok, tc.want)
				}
			}
		})
	}
}

func BenchmarkCanAccessRecordParallel(b *testing.B) {
	dao := New(nil)
	record := &Record{
		ID:   "r1",
		Data: map[string]any{"owner": "u1", "active": true, "tenant": "acme"},
	}
	info := &RequestInfo{
		Headers: map[string]string{"authorization": "Bearer x", "x_tenant": "acme"},
		Query:   map[string]string{"mode": "write"},
		Body:    map[string]any{"confirmed": true},
	}
	rule := `(auth && header:x_tenant == record.tenant) && (query:mode == 'write' || body:confirmed == true) && !record.deleted`

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ok, err := dao.CanAccessRecord(record, info, &rule)
			if err != nil {
				b.Fatalf("CanAccessRecord() error = %v", err)
			}
			if !ok {
				b.Fatalf("CanAccessRecord() = false, want true")
			}
		}
	})
}

func TestValidateViewSelectQueryRejectsUnsafeSQL(t *testing.T) {
	t.Parallel()

	tests := []string{
		"",
		"drop table users",
		"select * from users; delete from users",
		"select * from users -- nope",
		"with x as (select 1) select * from x /* nope */",
	}

	for _, query := range tests {
		query := query
		t.Run(query, func(t *testing.T) {
			t.Parallel()
			if err := ValidateViewSelectQuery(query); err == nil {
				t.Fatalf("ValidateViewSelectQuery() error = nil, want error")
			}
		})
	}

	if err := ValidateViewSelectQuery("select id, name from users"); err != nil {
		t.Fatalf("ValidateViewSelectQuery() unexpected error = %v", err)
	}
}
