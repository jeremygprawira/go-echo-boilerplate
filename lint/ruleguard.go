//go:build ruleguard

package lint

import "github.com/quasilyte/go-ruleguard/dsl"

// structScan: GORM's Scan never returns ErrRecordNotFound, so a missing row
// comes back as a zero-value struct with a nil error.
func structScan(m dsl.Matcher) {
	m.Import(`gorm.io/gorm`)
	m.Match(`$db.Scan(&$x)`).
		Where(m["db"].Type.Is(`*gorm.DB`) && m["x"].Type.Underlying().Is(`struct{$*_}`) &&
			!(m.File().PkgPath.Matches(`/internal/repository$`) && m.File().Name.Matches(`^store\.go$`))).
		Report(`Scan into a struct returns a zero value when no row matches; read one row through repository.one`)
}
