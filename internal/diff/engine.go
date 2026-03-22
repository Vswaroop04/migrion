package diff

import "github.com/vswaroop04/migratex/internal/schema"

// DiffSchemas compares the expected schema (from ORM) against the actual schema
// (from database) and returns a list of operations needed to make actual match expected.
//
// Think of it as: "what SQL do I need to run to make the DB look like my ORM schema?"
func DiffSchemas(expected, actual *schema.Schema) []Operation {
	var ops []Operation

	// Build lookup maps for O(1) access by name
	expectedTables := expected.TableMap()
	actualTables := actual.TableMap()

	// 1. Diff enums first (tables may depend on enum types)
	ops = append(ops, diffEnums(expected, actual)...)

	// 2. Find tables to create (in expected but not in actual)
	for _, table := range expected.Tables {
		if _, exists := actualTables[table.Name]; !exists {
			t := table // Go gotcha: loop variables are reused, so we copy
			ops = append(ops, Operation{
				Type:      OpCreateTable,
				TableName: table.Name,
				Table:     &t,
			})
		}
	}

	// 3. Find tables to drop (in actual but not in expected)
	for _, table := range actual.Tables {
		if _, exists := expectedTables[table.Name]; !exists {
			t := table
			ops = append(ops, Operation{
				Type:      OpDropTable,
				TableName: table.Name,
				Table:     &t, // snapshot for reverse migration
			})
		}
	}

	// 4. Diff tables that exist in both
	for _, expectedTable := range expected.Tables {
		actualTable, exists := actualTables[expectedTable.Name]
		if !exists {
			continue // already handled as create_table
		}
		ops = append(ops, diffTable(&expectedTable, actualTable)...)
	}

	return ops
}

// diffTable compares two versions of the same table.
func diffTable(expected, actual *schema.Table) []Operation {
	var ops []Operation

	ops = append(ops, diffColumns(expected, actual)...)
	ops = append(ops, diffIndexes(expected, actual)...)
	ops = append(ops, diffForeignKeys(expected, actual)...)
	ops = append(ops, diffPrimaryKey(expected, actual)...)
	ops = append(ops, diffUniqueConstraints(expected, actual)...)
	ops = append(ops, diffCheckConstraints(expected, actual)...)

	return ops
}

// diffColumns compares columns between expected and actual tables.
func diffColumns(expected, actual *schema.Table) []Operation {
	var ops []Operation
	expectedCols := expected.ColumnMap()
	actualCols := actual.ColumnMap()

	// Columns to add
	for _, col := range expected.Columns {
		if _, exists := actualCols[col.Name]; !exists {
			c := col
			ops = append(ops, Operation{
				Type:      OpAddColumn,
				TableName: expected.Name,
				Column:    &c,
			})
		}
	}

	// Columns to drop
	for _, col := range actual.Columns {
		if _, exists := expectedCols[col.Name]; !exists {
			c := col
			ops = append(ops, Operation{
				Type:      OpDropColumn,
				TableName: expected.Name,
				Column:    &c, // snapshot for reverse
			})
		}
	}

	// Columns to alter
	for _, expectedCol := range expected.Columns {
		actualCol, exists := actualCols[expectedCol.Name]
		if !exists {
			continue
		}
		changes := compareColumn(&expectedCol, actualCol)
		if changes != nil {
			ops = append(ops, Operation{
				Type:          OpAlterColumn,
				TableName:     expected.Name,
				ColumnName:    expectedCol.Name,
				ColumnChanges: changes,
			})
		}
	}

	return ops
}

// diffIndexes compares indexes between expected and actual tables.
func diffIndexes(expected, actual *schema.Table) []Operation {
	var ops []Operation
	expectedIdxs := expected.IndexMap()
	actualIdxs := actual.IndexMap()

	for name, idx := range expectedIdxs {
		actualIdx, exists := actualIdxs[name]
		if !exists {
			i := *idx
			ops = append(ops, Operation{
				Type:      OpCreateIndex,
				TableName: expected.Name,
				Index:     &i,
			})
		} else if !indexesEqual(idx, actualIdx) {
			// Index changed: drop old, create new
			ops = append(ops, Operation{
				Type:      OpDropIndex,
				TableName: expected.Name,
				IndexName: name,
				Index:     actualIdx,
			})
			i := *idx
			ops = append(ops, Operation{
				Type:      OpCreateIndex,
				TableName: expected.Name,
				Index:     &i,
			})
		}
	}

	for name, idx := range actualIdxs {
		if _, exists := expectedIdxs[name]; !exists {
			i := *idx
			ops = append(ops, Operation{
				Type:      OpDropIndex,
				TableName: expected.Name,
				IndexName: name,
				Index:     &i,
			})
		}
	}

	return ops
}

// diffForeignKeys compares foreign keys between expected and actual tables.
func diffForeignKeys(expected, actual *schema.Table) []Operation {
	var ops []Operation
	expectedFKs := expected.ForeignKeyMap()
	actualFKs := actual.ForeignKeyMap()

	for name, fk := range expectedFKs {
		actualFK, exists := actualFKs[name]
		if !exists {
			f := *fk
			ops = append(ops, Operation{
				Type:       OpAddForeignKey,
				TableName:  expected.Name,
				ForeignKey: &f,
			})
		} else if !foreignKeysEqual(fk, actualFK) {
			ops = append(ops, Operation{
				Type:       OpDropForeignKey,
				TableName:  expected.Name,
				FKName:     name,
				ForeignKey: actualFK,
			})
			f := *fk
			ops = append(ops, Operation{
				Type:       OpAddForeignKey,
				TableName:  expected.Name,
				ForeignKey: &f,
			})
		}
	}

	for name, fk := range actualFKs {
		if _, exists := expectedFKs[name]; !exists {
			f := *fk
			ops = append(ops, Operation{
				Type:       OpDropForeignKey,
				TableName:  expected.Name,
				FKName:     name,
				ForeignKey: &f,
			})
		}
	}

	return ops
}

// diffPrimaryKey compares primary keys between expected and actual tables.
func diffPrimaryKey(expected, actual *schema.Table) []Operation {
	var ops []Operation

	ePK := expected.PrimaryKey
	aPK := actual.PrimaryKey

	if ePK == nil && aPK == nil {
		return ops
	}

	if ePK != nil && aPK == nil {
		ops = append(ops, Operation{
			Type:       OpAddPrimaryKey,
			TableName:  expected.Name,
			PrimaryKey: ePK,
		})
	} else if ePK == nil && aPK != nil {
		ops = append(ops, Operation{
			Type:       OpDropPrimaryKey,
			TableName:  expected.Name,
			PKName:     aPK.Name,
			PrimaryKey: aPK,
		})
	} else {
		// Both exist — check if they differ
		if !columnsListEqual(ePK.Columns, aPK.Columns) {
			ops = append(ops, Operation{
				Type:       OpDropPrimaryKey,
				TableName:  expected.Name,
				PKName:     aPK.Name,
				PrimaryKey: aPK,
			})
			ops = append(ops, Operation{
				Type:       OpAddPrimaryKey,
				TableName:  expected.Name,
				PrimaryKey: ePK,
			})
		}
	}

	return ops
}

// diffUniqueConstraints compares unique constraints.
func diffUniqueConstraints(expected, actual *schema.Table) []Operation {
	var ops []Operation
	expectedUCs := expected.UniqueConstraintMap()
	actualUCs := actual.UniqueConstraintMap()

	for name, uc := range expectedUCs {
		actualUC, exists := actualUCs[name]
		if !exists {
			u := *uc
			ops = append(ops, Operation{
				Type:             OpAddUniqueConstraint,
				TableName:        expected.Name,
				UniqueConstraint: &u,
			})
		} else if !columnsListEqual(uc.Columns, actualUC.Columns) {
			ops = append(ops, Operation{
				Type:             OpDropUniqueConstraint,
				TableName:        expected.Name,
				ConstraintName:   name,
				UniqueConstraint: actualUC,
			})
			u := *uc
			ops = append(ops, Operation{
				Type:             OpAddUniqueConstraint,
				TableName:        expected.Name,
				UniqueConstraint: &u,
			})
		}
	}

	for name, uc := range actualUCs {
		if _, exists := expectedUCs[name]; !exists {
			u := *uc
			ops = append(ops, Operation{
				Type:             OpDropUniqueConstraint,
				TableName:        expected.Name,
				ConstraintName:   name,
				UniqueConstraint: &u,
			})
		}
	}

	return ops
}

// diffCheckConstraints compares check constraints.
func diffCheckConstraints(expected, actual *schema.Table) []Operation {
	var ops []Operation
	expectedCCs := make(map[string]*schema.CheckConstraint)
	for i := range expected.CheckConstraints {
		expectedCCs[expected.CheckConstraints[i].Name] = &expected.CheckConstraints[i]
	}
	actualCCs := make(map[string]*schema.CheckConstraint)
	for i := range actual.CheckConstraints {
		actualCCs[actual.CheckConstraints[i].Name] = &actual.CheckConstraints[i]
	}

	for name, cc := range expectedCCs {
		actualCC, exists := actualCCs[name]
		if !exists {
			c := *cc
			ops = append(ops, Operation{
				Type:            OpAddCheckConstraint,
				TableName:       expected.Name,
				CheckConstraint: &c,
			})
		} else if cc.Expression != actualCC.Expression {
			ops = append(ops, Operation{
				Type:            OpDropCheckConstraint,
				TableName:       expected.Name,
				ConstraintName:  name,
				CheckConstraint: actualCC,
			})
			c := *cc
			ops = append(ops, Operation{
				Type:            OpAddCheckConstraint,
				TableName:       expected.Name,
				CheckConstraint: &c,
			})
		}
	}

	for name, cc := range actualCCs {
		if _, exists := expectedCCs[name]; !exists {
			c := *cc
			ops = append(ops, Operation{
				Type:            OpDropCheckConstraint,
				TableName:       expected.Name,
				ConstraintName:  name,
				CheckConstraint: &c,
			})
		}
	}

	return ops
}

// diffEnums compares enum types between schemas.
func diffEnums(expected, actual *schema.Schema) []Operation {
	var ops []Operation
	expectedEnums := expected.EnumMap()
	actualEnums := actual.EnumMap()

	for name, enum := range expectedEnums {
		actualEnum, exists := actualEnums[name]
		if !exists {
			e := *enum
			ops = append(ops, Operation{
				Type: OpCreateEnum,
				Enum: &e,
			})
		} else if !enumsEqual(enum, actualEnum) {
			// Find added and removed values
			var addVals, dropVals []string
			actualSet := make(map[string]bool)
			for _, v := range actualEnum.Values {
				actualSet[v] = true
			}
			expectedSet := make(map[string]bool)
			for _, v := range enum.Values {
				expectedSet[v] = true
			}
			for _, v := range enum.Values {
				if !actualSet[v] {
					addVals = append(addVals, v)
				}
			}
			for _, v := range actualEnum.Values {
				if !expectedSet[v] {
					dropVals = append(dropVals, v)
				}
			}
			ops = append(ops, Operation{
				Type:       OpAlterEnum,
				EnumName:   name,
				AddValues:  addVals,
				DropValues: dropVals,
			})
		}
	}

	for name, enum := range actualEnums {
		if _, exists := expectedEnums[name]; !exists {
			e := *enum
			ops = append(ops, Operation{
				Type:     OpDropEnum,
				EnumName: name,
				Enum:     &e,
			})
		}
	}

	return ops
}

// columnsListEqual compares two string slices for equality.
func columnsListEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
