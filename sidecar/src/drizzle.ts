/**
 * Drizzle ORM schema exporter.
 *
 * Drizzle stores table metadata on table objects using symbols.
 * We extract these to build our Schema JSON.
 */

import type { Schema, Table, Column, ColumnType, Index, ForeignKey, PrimaryKey } from "./types.js";

// Drizzle internal symbols — these are how Drizzle stores metadata on table objects.
// We need to access them to read the schema programmatically.
const DrizzleSymbols = {
  Columns: Symbol.for("drizzle:Columns"),
  Name: Symbol.for("drizzle:Name"),
  Schema: Symbol.for("drizzle:Schema"),
  IsDrizzleTable: Symbol.for("drizzle:IsDrizzleTable"),
  BaseName: Symbol.for("drizzle:BaseName"),
};

/**
 * Export a Drizzle schema file to our standard Schema format.
 * @param schemaPath - path to the user's Drizzle schema file (e.g., "./src/schema.ts")
 * @param dialect - "pg" or "mysql"
 */
export async function exportDrizzleSchema(
  schemaPath: string,
  dialect: "pg" | "mysql"
): Promise<Schema> {
  // Dynamically import the user's schema file
  // tsx handles TypeScript imports at runtime
  const schemaModule = await import(schemaPath);

  const tables: Table[] = [];
  const enumSet = new Map<string, string[]>();

  // Iterate all exports looking for Drizzle table objects
  for (const [, value] of Object.entries(schemaModule)) {
    if (!isDrizzleTable(value)) continue;

    const table = extractTable(value, dialect, enumSet);
    if (table) tables.push(table);
  }

  return {
    dialect,
    tables,
    enums: Array.from(enumSet.entries()).map(([name, values]) => ({
      name,
      values,
    })),
  };
}

function isDrizzleTable(obj: unknown): boolean {
  return (
    obj !== null &&
    typeof obj === "object" &&
    (DrizzleSymbols.IsDrizzleTable in (obj as Record<symbol, unknown>) ||
      DrizzleSymbols.Columns in (obj as Record<symbol, unknown>))
  );
}

function extractTable(
  tableObj: any,
  dialect: "pg" | "mysql",
  enumSet: Map<string, string[]>
): Table | null {
  const tableName =
    tableObj[DrizzleSymbols.Name] ||
    tableObj[DrizzleSymbols.BaseName];

  if (!tableName) return null;

  const drizzleColumns = tableObj[DrizzleSymbols.Columns];
  if (!drizzleColumns) return null;

  const columns: Column[] = [];
  const pkColumns: string[] = [];
  const indexes: Index[] = [];
  const foreignKeys: ForeignKey[] = [];

  for (const [colName, colDef] of Object.entries(drizzleColumns) as [string, any][]) {
    const column = extractColumn(colName, colDef, dialect, enumSet);
    columns.push(column);

    if (colDef.primary || colDef.primaryKey) {
      pkColumns.push(colName);
    }
  }

  // Extract indexes if defined
  // Drizzle tables may have a config function that returns indexes
  if (typeof tableObj.getSQL === "function") {
    // Table-level config is handled via the table's config callback
  }

  const table: Table = {
    name: tableName,
    columns,
    indexes,
    foreignKeys,
  };

  if (pkColumns.length > 0) {
    table.primaryKey = { columns: pkColumns };
  }

  return table;
}

function extractColumn(
  name: string,
  colDef: any,
  dialect: "pg" | "mysql",
  enumSet: Map<string, string[]>
): Column {
  const colType = mapDrizzleType(colDef, dialect, enumSet);

  const column: Column = {
    name: colDef.name || name,
    type: colType,
    nullable: colDef.notNull !== true,
  };

  if (colDef.hasDefault && colDef.default !== undefined) {
    if (typeof colDef.default === "function") {
      column.default = { kind: "expression", value: String(colDef.default) };
    } else {
      column.default = { kind: "value", value: String(colDef.default) };
    }
  }

  if (colDef.isUnique) {
    column.unique = true;
  }

  return column;
}

function mapDrizzleType(
  colDef: any,
  dialect: "pg" | "mysql",
  enumSet: Map<string, string[]>
): ColumnType {
  // Drizzle stores the SQL type name in various places
  const dataType: string = (
    colDef.dataType ||
    colDef.columnType ||
    colDef.getSQLType?.() ||
    ""
  ).toLowerCase();

  const sqlName: string = (colDef.sqlName || "").toLowerCase();

  // Serial types
  if (dataType.includes("serial") || sqlName.includes("serial")) {
    if (dataType.includes("bigserial") || sqlName.includes("bigserial")) {
      return { kind: "serial", size: "bigserial" };
    }
    if (dataType.includes("smallserial") || sqlName.includes("smallserial")) {
      return { kind: "serial", size: "smallserial" };
    }
    return { kind: "serial" };
  }

  // Integer types
  if (dataType === "number" || dataType === "integer" || sqlName.includes("int")) {
    if (sqlName.includes("bigint") || dataType.includes("bigint")) {
      return { kind: "int", size: "bigint" };
    }
    if (sqlName.includes("smallint") || dataType.includes("smallint")) {
      return { kind: "int", size: "smallint" };
    }
    return { kind: "int" };
  }

  // Text types
  if (dataType === "string" || sqlName.includes("text") || sqlName.includes("varchar")) {
    const length = colDef.length || colDef.config?.length;
    return { kind: "text", ...(length ? { length } : {}) };
  }

  // Boolean
  if (dataType === "boolean" || sqlName.includes("bool")) {
    return { kind: "boolean" };
  }

  // Timestamp
  if (sqlName.includes("timestamp") || dataType.includes("timestamp")) {
    return {
      kind: "timestamp",
      withTimezone: sqlName.includes("tz") || colDef.withTimezone === true,
    };
  }

  // Date
  if (sqlName === "date" || dataType === "date") {
    return { kind: "date" };
  }

  // JSON
  if (sqlName.includes("json")) {
    return { kind: "json", binary: sqlName === "jsonb" };
  }

  // UUID
  if (sqlName === "uuid" || dataType === "uuid") {
    return { kind: "uuid" };
  }

  // Float/Real/Double
  if (sqlName.includes("real") || sqlName.includes("float")) {
    return { kind: "float" };
  }
  if (sqlName.includes("double")) {
    return { kind: "float", size: "double" };
  }

  // Numeric/Decimal
  if (sqlName.includes("numeric") || sqlName.includes("decimal")) {
    return {
      kind: "decimal",
      precision: colDef.precision,
      scale: colDef.scale,
    };
  }

  // Enum
  if (colDef.enumValues && Array.isArray(colDef.enumValues)) {
    const enumName = colDef.enumName || colDef.name || "unknown_enum";
    enumSet.set(enumName, colDef.enumValues);
    return { kind: "enum", enumName };
  }

  // Fallback
  return { kind: "custom", raw: sqlName || dataType || "unknown" };
}
