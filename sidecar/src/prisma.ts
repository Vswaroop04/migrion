/**
 * Prisma schema exporter.
 *
 * Parses schema.prisma using @prisma/internals (getDMMF) and converts
 * the Prisma Data Model Meta Format to our standard Schema JSON.
 */

import type {
  Schema,
  Table,
  Column,
  ColumnType,
  Index,
  ForeignKey,
  PrimaryKey,
  Enum,
} from "./types.js";

/**
 * Export a Prisma schema file to our standard Schema format.
 * @param schemaPath - path to schema.prisma
 */
export async function exportPrismaSchema(schemaPath: string): Promise<Schema> {
  // Dynamic import — @prisma/internals is an optional peer dependency
  let getDMMF: any;
  try {
    const internals = await import("@prisma/internals");
    getDMMF = internals.getDMMF;
  } catch {
    throw new Error(
      "Cannot find @prisma/internals. Install it: npm install @prisma/internals"
    );
  }

  const { readFileSync } = await import("node:fs");
  const datamodel = readFileSync(schemaPath, "utf-8");

  // Detect dialect from datasource block
  const dialect = detectDialect(datamodel);

  const dmmf = await getDMMF({ datamodel });

  const tables: Table[] = [];
  const enums: Enum[] = [];

  // Convert models to tables
  for (const model of dmmf.datamodel.models) {
    tables.push(convertModel(model, dmmf.datamodel.models, dialect));
  }

  // Convert enums
  for (const enumDef of dmmf.datamodel.enums) {
    enums.push({
      name: enumDef.name,
      values: enumDef.values.map((v: any) => v.name),
    });
  }

  return { dialect, tables, enums };
}

function detectDialect(datamodel: string): "pg" | "mysql" {
  const match = datamodel.match(/provider\s*=\s*"(\w+)"/);
  if (match) {
    const provider = match[1].toLowerCase();
    if (provider === "mysql") return "mysql";
  }
  return "pg"; // default to PostgreSQL
}

function convertModel(model: any, allModels: any[], dialect: "pg" | "mysql"): Table {
  const columns: Column[] = [];
  const foreignKeys: ForeignKey[] = [];
  const indexes: Index[] = [];
  const pkColumns: string[] = [];

  for (const field of model.fields) {
    // Skip relation fields (they don't map to columns)
    if (field.kind === "object") continue;

    const column = convertField(field, dialect);
    columns.push(column);

    if (field.isId) {
      pkColumns.push(field.name);
    }
  }

  // Extract @relation foreign keys
  for (const field of model.fields) {
    if (field.kind === "object" && field.relationFromFields?.length > 0) {
      const fk: ForeignKey = {
        name: `fk_${model.name.toLowerCase()}_${field.name}`,
        columns: field.relationFromFields,
        referencedTable: field.type.toLowerCase(),
        referencedColumns: field.relationToFields || ["id"],
        onDelete: mapPrismaAction(field.relationOnDelete),
        onUpdate: mapPrismaAction(field.relationOnUpdate),
      };
      foreignKeys.push(fk);
    }
  }

  // Extract @@index and @@unique from model attributes
  if (model.uniqueFields) {
    for (const fields of model.uniqueFields) {
      indexes.push({
        name: `${model.name.toLowerCase()}_${fields.join("_")}_unique`,
        columns: fields,
        unique: true,
      });
    }
  }

  const table: Table = {
    // Prisma uses PascalCase model names; DB uses snake_case
    // The @@map attribute overrides this, but dbName captures it
    name: model.dbName || model.name.toLowerCase(),
    columns,
    indexes,
    foreignKeys,
  };

  if (pkColumns.length > 0) {
    table.primaryKey = { columns: pkColumns };
  }

  return table;
}

function convertField(field: any, dialect: "pg" | "mysql"): Column {
  return {
    name: field.dbName || field.name,
    type: mapPrismaType(field, dialect),
    nullable: !field.isRequired,
    ...(field.hasDefaultValue && field.default !== undefined
      ? { default: parsePrismaDefault(field.default) }
      : {}),
    ...(field.isId ? { primaryKey: true } : {}),
    ...(field.isUnique ? { unique: true } : {}),
  };
}

function mapPrismaType(field: any, dialect: "pg" | "mysql"): ColumnType {
  // Check for @db.* native type overrides first
  const nativeType = field.nativeType;

  switch (field.type) {
    case "Int":
      if (nativeType) {
        if (nativeType[0] === "SmallInt") return { kind: "int", size: "smallint" };
        if (nativeType[0] === "BigInt") return { kind: "int", size: "bigint" };
      }
      return { kind: "int" };

    case "BigInt":
      return { kind: "int", size: "bigint" };

    case "Float":
      return { kind: "float", size: "double" };

    case "Decimal":
      return {
        kind: "decimal",
        precision: nativeType?.[1]?.precision || 65,
        scale: nativeType?.[1]?.scale || 30,
      };

    case "String":
      if (nativeType?.[0] === "VarChar") {
        return { kind: "text", length: nativeType[1]?.length || 255 };
      }
      return { kind: "text" };

    case "Boolean":
      return { kind: "boolean" };

    case "DateTime":
      return { kind: "timestamp", withTimezone: dialect === "pg" };

    case "Json":
      return { kind: "json", binary: dialect === "pg" };

    case "Bytes":
      return { kind: "bytea" };

    default:
      // Could be an enum
      if (field.kind === "enum") {
        return { kind: "enum", enumName: field.type };
      }
      return { kind: "custom", raw: field.type };
  }
}

function parsePrismaDefault(def: any): { kind: "value" | "expression"; value: string } {
  if (typeof def === "object" && def.name) {
    // Prisma function defaults like autoincrement(), now(), uuid()
    return { kind: "expression", value: `${def.name}()` };
  }
  return { kind: "value", value: String(def) };
}

function mapPrismaAction(action?: string): "CASCADE" | "SET NULL" | "SET DEFAULT" | "RESTRICT" | "NO ACTION" | undefined {
  if (!action) return undefined;
  const map: Record<string, any> = {
    Cascade: "CASCADE",
    SetNull: "SET NULL",
    SetDefault: "SET DEFAULT",
    Restrict: "RESTRICT",
    NoAction: "NO ACTION",
  };
  return map[action];
}
