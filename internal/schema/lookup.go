package schema

// TableMap builds a lookup map from table name to Table.
// In Go, methods are defined outside the struct (unlike TS/Java classes).
// The (s *Schema) part is the "receiver" — it's like "this" in other languages.
func (s *Schema) TableMap() map[string]*Table {
	m := make(map[string]*Table, len(s.Tables))
	for i := range s.Tables {
		m[s.Tables[i].Name] = &s.Tables[i]
	}
	return m
}

// ColumnMap builds a lookup map from column name to Column for a table.
func (t *Table) ColumnMap() map[string]*Column {
	m := make(map[string]*Column, len(t.Columns))
	for i := range t.Columns {
		m[t.Columns[i].Name] = &t.Columns[i]
	}
	return m
}

// IndexMap builds a lookup map from index name to Index.
func (t *Table) IndexMap() map[string]*Index {
	m := make(map[string]*Index, len(t.Indexes))
	for i := range t.Indexes {
		m[t.Indexes[i].Name] = &t.Indexes[i]
	}
	return m
}

// ForeignKeyMap builds a lookup map from FK name to ForeignKey.
func (t *Table) ForeignKeyMap() map[string]*ForeignKey {
	m := make(map[string]*ForeignKey, len(t.ForeignKeys))
	for i := range t.ForeignKeys {
		m[t.ForeignKeys[i].Name] = &t.ForeignKeys[i]
	}
	return m
}

// UniqueConstraintMap builds a lookup map from constraint name to UniqueConstraint.
func (t *Table) UniqueConstraintMap() map[string]*UniqueConstraint {
	m := make(map[string]*UniqueConstraint, len(t.UniqueConstraints))
	for i := range t.UniqueConstraints {
		m[t.UniqueConstraints[i].Name] = &t.UniqueConstraints[i]
	}
	return m
}

// EnumMap builds a lookup map from enum name to Enum.
func (s *Schema) EnumMap() map[string]*Enum {
	m := make(map[string]*Enum, len(s.Enums))
	for i := range s.Enums {
		m[s.Enums[i].Name] = &s.Enums[i]
	}
	return m
}
