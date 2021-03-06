// Copyright (C) 2017 Space Monkey, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sqlcompile

import "gopkg.in/spacemonkeygo/dbx.v1/sqlgen"

// Compile reduces the sql expression to normal form
func Compile(sql sqlgen.SQL) sqlgen.SQL {
	return sqlCompile(sql)
}

func sqlCompile(sql sqlgen.SQL) (out sqlgen.SQL) {
	switch sql := sql.(type) {
	// these cases cannot be compiled further
	case sqlgen.Literal, *sqlgen.Condition, *sqlgen.Hole:
		return sql

	case sqlgen.Literals:
		// if there are no SQLs, we just have an empty string so hoist to a
		// literal type
		if len(sql.SQLs) == 0 {
			return sqlgen.Literal("")
		}

		// if there is one sql, we can just return the compiled form of that.
		if len(sql.SQLs) == 1 {
			return sqlCompile(sql.SQLs[0])
		}

		// keep track of orignal so that we know if we need to recurse
		original := sql

		// recursively compile all of the children
		sql = sqlCompileChildren(sql)

		// intersperse the join in the Literals so that hoisting works better
		sql = sqlIntersperse(sql)

		// hoist any children Literals that have the same join
		sql = sqlHoist(sql)

		// constant fold any Literal children that are next to each other
		sql = sqlConstantFold(sql)

		// filter out any children that are trivial
		sql = sqlFilterTrivial(sql)

		// don't recursive if we haven't changed
		if sqlsEqual(sql.SQLs, original.SQLs) {
			return sql
		}

		// recurse until fixed point. we may have more optimization
		//  opportunities now
		return sqlCompile(sql)

	default:
		panic("unhandled sql type")
	}
}

func sqlCompileChildren(ls sqlgen.Literals) (out sqlgen.Literals) {
	out = ls
	out.SQLs = nil

	for _, sql := range ls.SQLs {
		out.SQLs = append(out.SQLs, sqlCompile(sql))
	}

	return out
}

func sqlIntersperse(ls sqlgen.Literals) (out sqlgen.Literals) {
	if ls.Join == "" {
		return ls
	}

	out = ls
	out.SQLs = nil
	out.Join = ""

	first := true
	for _, sql := range ls.SQLs {
		if !first {
			out.SQLs = append(out.SQLs, sqlgen.Literal(ls.Join))
		}
		first = false
		out.SQLs = append(out.SQLs, sql)
	}

	return out
}

func sqlHoist(ls sqlgen.Literals) (out sqlgen.Literals) {
	out = ls
	out.SQLs = nil

	for _, sql := range ls.SQLs {
		lits, ok := sql.(sqlgen.Literals)
		if !ok || lits.Join != ls.Join {
			out.SQLs = append(out.SQLs, sql)
		}
		out.SQLs = append(out.SQLs, lits.SQLs...)
	}

	return out
}

func sqlConstantFold(ls sqlgen.Literals) (out sqlgen.Literals) {
	out = ls
	out.SQLs = nil

	buf := sqlgen.Literals{Join: ls.Join}
	for _, sql := range ls.SQLs {
		lit, ok := sql.(sqlgen.Literal)
		if ok {
			buf.SQLs = append(buf.SQLs, lit)
			continue
		}

		if len(buf.SQLs) > 0 {
			out.SQLs = append(out.SQLs, sqlgen.Literal(buf.Render()))
			buf.SQLs = buf.SQLs[:0]
		}
		out.SQLs = append(out.SQLs, sql)
	}

	if len(buf.SQLs) > 0 {
		out.SQLs = append(out.SQLs, sqlgen.Literal(buf.Render()))
	}

	return out
}

func sqlFilterTrivial(ls sqlgen.Literals) (out sqlgen.Literals) {
	out = ls
	out.SQLs = nil

	for _, sql := range ls.SQLs {
		lit, ok := sql.(sqlgen.Literal)
		if ok && lit == sqlgen.Literal("") {
			continue
		}
		out.SQLs = append(out.SQLs, sql)
	}

	return out
}
