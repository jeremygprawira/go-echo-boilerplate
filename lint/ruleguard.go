//go:build ruleguard

package lint

import (
	"github.com/quasilyte/go-ruleguard/dsl"
	"github.com/quasilyte/go-ruleguard/dsl/types"
)

func pointsToStruct(ctx *dsl.VarFilterContext) bool {
	p := types.AsPointer(ctx.Type)
	return p != nil && types.AsStruct(p.Elem().Underlying()) != nil
}

// structScan: GORM's Scan and Find never return ErrRecordNotFound, so a
// missing row comes back as a zero-value struct with a nil error.
func structScan(m dsl.Matcher) {
	m.Import(`gorm.io/gorm`)
	m.Match(`$db.Scan($x)`, `$db.Find($x, $*_)`).
		Where(m["db"].Type.Is(`*gorm.DB`) && m["x"].Filter(pointsToStruct) &&
			!(m.File().PkgPath.Matches(`/internal/repository$`) && m.File().Name.Matches(`^store\.go$`))).
		Report(`Scan or Find into a struct returns a zero value when no row matches; read one row through repository.one`)
}

func environmentCompare(m dsl.Matcher) {
	m.Import(`go-echo-boilerplate/internal/config`)

	m.Match(`$a == $b`, `$a != $b`).
		Where((m["a"].Type.Is(`config.Environment`) || m["b"].Type.Is(`config.Environment`) ||
			m["a"].Text.Matches(`\.Environment$`) || m["b"].Text.Matches(`\.Environment$`)) &&
			!m.File().PkgPath.Matches(`/internal/config(_test)?(/|$)`)).
		Report(`compare the environment with config.Environment.IsProduction, not with a string`)

	m.Match(`switch $x { $*_ }`, `switch string($x) { $*_ }`,
		`string($x) == $_`, `$_ == string($x)`, `string($x) != $_`, `$_ != string($x)`,
		`strings.EqualFold(string($x), $_)`, `strings.EqualFold($_, string($x))`,
		`strings.HasPrefix(string($x), $_)`, `strings.HasSuffix(string($x), $_)`,
		`strings.Contains(string($x), $_)`, `strings.Compare(string($x), $_)`, `strings.Compare($_, string($x))`).
		Where(m["x"].Type.Is(`config.Environment`) && !m.File().PkgPath.Matches(`/internal/config(_test)?(/|$)`)).
		Report(`compare the environment with config.Environment.IsProduction, not with a string`)
}
