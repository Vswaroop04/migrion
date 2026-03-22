/**
 * TypeORM schema exporter.
 *
 * TypeORM uses decorators (@Entity, @Column, etc.) that store metadata
 * via reflect-metadata. We read that metadata to build our Schema JSON.
 */

import type {
  Schema,
  Table,
  Column,
  ColumnType,
  ForeignKey,
} from "./types.js";

/**
 * Export TypeORM entities to our standard Schema format.
 * @param schemaPath - path to a file that exports TypeORM entities
 * @param dialect - "pg" or "mysql"
 */
export async function exportTypeORMSchema(
  schemaPath: string,
  dialect: "pg" | "mysql"
): Promise<Schema> {
  // TypeORM stores metadata globally via reflect-metadata
  // We need to import the entities to trigger decorator evaluation
  let getMetadataArgsStorage: any;
  try {
    const typeorm = await import("typeorm");
    getMetadataArgsStorage = typeorm.getMetadataArgsStorage;
  } catch {
    throw new Error("Cannot find typeorm. Install it: npm install typeorm reflect-metadata");
  }

  // Import user's entities to trigger decorator registration
  await import(schemaPath);

  const storage = getMetadataArgsStorage();
  const tables: Table[] = [];

  // Process each registered table/entity
  for (const tableArgs of storage.tables) {
    const table = convertTypeORMTable(tableArgs, storage, dialect);
    tables.push(table);
  }

  return { dialect, tables };
}

function convertTypeORMTable(tableArgs: any, storage: any, dialect: "pg" | "mysql"): Table {
  const entityTarget = tableArgs.target;
  const tableName = tableArgs.name || entityTarget.name?.toLowerCase() || "unknown";

  // Get columns for this entity
  const columnArgs = storage.columns.filter(
    (c: any) => c.target === entityTarget || c.target?.prototype instanceof entityTarget
  );

  const columns: Column[] = [];
  const foreignKeys: ForeignKey[] = [];

  for (const colArg of columnArgs) {
    columns.push(convertTypeORMColumn(colArg, dialect));
  }

  // Get relations for FK detection
  const relations = storage.relations.filter(
    (r: any) => r.target === entityTarget
  );

  for (const rel of relations) {
    const joinColumns = storage.joinColumns.filter(
      (jc: any) => jc.target === entityTarget && jc.propertyName === rel.propertyName
    );

    for (const jc of joinColumns) {
      foreignKeys.push({
        name: `fk_${tableName}_${jc.name || rel.propertyName}`,
        columns: [jc.name || `${rel.propertyName}Id`],
        referencedTable: typeof rel.type === "function" ? rel.type().name?.toLowerCase() : String(rel.type).toLowerCase(),
        referencedColumns: [jc.referencedColumnName || "id"],
      });
    }
  }

  // Detect primary key columns
  const pkColumns = columns
    .filter((c) => c.primaryKey)
    .map((c) => c.name);

  const table: Table = {
    name: tableName,
    columns,
    foreignKeys,
  };

  if (pkColumns.length > 0) {
    table.primaryKey = { columns: pkColumns };
  }

  return table;
}

function convertTypeORMColumn(colArg: any, dialect: "pg" | "mysql"): Column {
  const options = colArg.options || {};
  const isPrimary = colArg.mode === "regular" ? options.primary : colArg.mode === "objectId";

  return {
    name: options.name || colArg.propertyName,
    type: mapTypeORMType(colArg, options, dialect),
    nullable: options.nullable === true,
    ...(isPrimary ? { primaryKey: true } : {}),
    ...(options.unique ? { unique: true } : {}),
    ...(options.default !== undefined
      ? {
          default: {
            kind: typeof options.default === "string" && options.default.includes("(")
              ? "expression" as const
              : "value" as const,
            value: String(options.default),
          },
        }
      : {}),
  };
}

function mapTypeORMType(colArg: any, options: any, dialect: "pg" | "mysql"): ColumnType {
  // TypeORM can specify type as a string or via the TypeScript type
  const type = (options.type || colArg.mode === "createDate" || colArg.mode === "updateDate"
    ? "timestamp"
    : "unknown"
  ) as string;

  switch (type.toLowerCase()) {
    case "int":
    case "integer":
    case "int4":
      return { kind: "int" };
    case "smallint":
    case "int2":
      return { kind: "int", size: "smallint" };
    case "bigint":
    case "int8":
      return { kind: "int", size: "bigint" };
    case "varchar":
    case "character varying":
      return { kind: "text", length: options.length || 255 };
    case "text":
      return { kind: "text" };
    case "boolean":
    case "bool":
      return { kind: "boolean" };
    case "timestamp":
    case "timestamp with time zone":
    case "timestamptz":
      return { kind: "timestamp", withTimezone: dialect === "pg" };
    case "timestamp without time zone":
      return { kind: "timestamp" };
    case "date":
      return { kind: "date" };
    case "json":
      return { kind: "json" };
    case "jsonb":
      return { kind: "json", binary: true };
    case "uuid":
      return { kind: "uuid" };
    case "float":
    case "real":
    case "float4":
      return { kind: "float" };
    case "double precision":
    case "float8":
      return { kind: "float", size: "double" };
    case "decimal":
    case "numeric":
      return {
        kind: "decimal",
        precision: options.precision,
        scale: options.scale,
      };
    case "bytea":
    case "blob":
      return { kind: "bytea" };
    case "enum":
      return { kind: "enum", enumName: options.enum?.name || "unknown" };
    default:
      return { kind: "custom", raw: type };
  }
}
