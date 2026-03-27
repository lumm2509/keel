package search

import "testing"

func BenchmarkProviderExec(b *testing.B) {
	testDB, err := createTestDB()
	if err != nil {
		b.Fatal(err)
	}
	defer testDB.Close()

	query := testDB.Select("*").
		From("test").
		Where(Not(HashExp{"test1": nil})).
		OrderBy("test1 ASC")

	b.Run("current_total", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			testDB.CalledQueries = nil
			p := NewProvider(&testFieldResolver{}).
				Query(query).
				Page(1).
				PerPage(100).
				SkipTotal(false)

			if _, err := p.Exec(&[]testTableStruct{}); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("legacy_total", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			testDB.CalledQueries = nil
			items := []testTableStruct{}
			if _, _, err := legacyProviderExec(query, &items, 1, 100, "id"); err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("skip_total", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			testDB.CalledQueries = nil
			p := NewProvider(&testFieldResolver{}).
				Query(query).
				Page(1).
				PerPage(100).
				SkipTotal(true)

			if _, err := p.Exec(&[]testTableStruct{}); err != nil {
				b.Fatal(err)
			}
		}
	})
}

func legacyProviderExec(query *SelectQuery, items any, page int, perPage int, countCol string) (int, int, error) {
	modelsQuery := *query
	totalCount := -1
	totalPages := -1

	countQuery := modelsQuery
	queryInfo := countQuery.Info()
	if len(queryInfo.From) > 0 {
		countCol = dbutilsAliasOrIdentifier(queryInfo.From[0]) + "." + countCol
	}

	err := countQuery.Distinct(false).
		Select("COUNT(DISTINCT [[" + countCol + "]])").
		GroupBy().
		OrderBy().
		Row(&totalCount)
	if err != nil {
		return 0, 0, err
	}

	if perPage > 0 {
		totalPages = (totalCount + perPage - 1) / perPage
	}

	modelsQuery.Limit(int64(perPage))
	modelsQuery.Offset(int64(perPage * (page - 1)))
	if err := modelsQuery.All(items); err != nil {
		return 0, 0, err
	}

	return totalCount, totalPages, nil
}
