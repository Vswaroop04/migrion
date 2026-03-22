/**
 * Shared JSON schema types — these MUST match the Go structs in internal/schema/schema.go.
 * The sidecar outputs JSON in this format, and Go parses it.
 */

export interface Schema {
  dialect: "pg" | "mysql";
  tables: Table[];
  enums?: Enum[];
}

export interface Table {
  name: string;
  schema?: string;
  columns: Column[];
  primaryKey?: PrimaryKey;
  indexes?: Index[];
  foreignKeys?: ForeignKey[];
  uniqueConstraints?: UniqueConstraint[];
  checkConstraints?: CheckConstraint[];
}

export interface Column {
  name: string;
  type: ColumnType;
  nullable: boolean;
  default?: ColumnDefault;
  primaryKey?: boolean;
  unique?: boolean;
}

export interface ColumnType {
  kind: string;
  size?: string;
  length?: number;
  precision?: number;
  scale?: number;
  withTimezone?: boolean;
  binary?: boolean;
  enumName?: string;
  raw?: string;
}

export interface ColumnDefault {
  kind: "value" | "expression" | "sequence";
  value: string;
}

export interface PrimaryKey {
  name?: string;
  columns: string[];
}

export interface Index {
  name: string;
  columns: string[];
  unique: boolean;
  type?: string;
  where?: string;
}

export type ForeignKeyAction =
  | "CASCADE"
  | "SET NULL"
  | "SET DEFAULT"
  | "RESTRICT"
  | "NO ACTION";

export interface ForeignKey {
  name: string;
  columns: string[];
  referencedTable: string;
  referencedColumns: string[];
  onDelete?: ForeignKeyAction;
  onUpdate?: ForeignKeyAction;
}

export interface UniqueConstraint {
  name: string;
  columns: string[];
}

export interface CheckConstraint {
  name: string;
  expression: string;
}

export interface Enum {
  name: string;
  values: string[];
  schema?: string;
}
