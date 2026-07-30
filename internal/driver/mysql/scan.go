package mysql

import "database/sql"

// scanRows reads every column as text so one grid renders any type. A maxRows of
// zero or less reads to the end; otherwise it stops at the cap and reports that
// the server had more to give.
func scanRows(rows *sql.Rows, width, maxRows int) ([][]*string, bool, error) {
	holders := make([]sql.NullString, width)
	targets := make([]any, width)
	for i := range holders {
		targets[i] = &holders[i]
	}

	out := [][]*string{}
	for rows.Next() {
		if maxRows > 0 && len(out) == maxRows {
			return out, true, nil
		}

		if err := rows.Scan(targets...); err != nil {
			return nil, false, translateError(err)
		}

		row := make([]*string, width)
		for i, holder := range holders {
			row[i] = toPointer(holder)
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, false, translateError(err)
	}

	return out, false, nil
}
