#!/usr/bin/env node

/**
 * CLI entry point for the sidecar.
 *
 * Usage:
 *   npx migratex-export --orm drizzle --schema ./src/schema.ts --dialect pg
 *   npx migratex-export --orm prisma --schema ./prisma/schema.prisma
 *   npx migratex-export --orm typeorm --schema ./src/entities.ts --dialect pg
 *
 * Outputs JSON to stdout (which the Go binary reads via os/exec).
 */

import { resolve } from "node:path";

async function main() {
  const args = parseArgs(process.argv.slice(2));

  if (!args.orm) {
    console.error("Usage: migratex-export --orm <drizzle|prisma|typeorm> --schema <path> [--dialect <pg|mysql>]");
    process.exit(1);
  }

  if (!args.schema) {
    console.error("Error: --schema is required");
    process.exit(1);
  }

  const schemaPath = resolve(process.cwd(), args.schema);
  const dialect = (args.dialect || "pg") as "pg" | "mysql";

  let schema;

  switch (args.orm) {
    case "drizzle": {
      const { exportDrizzleSchema } = await import("../src/drizzle.js");
      schema = await exportDrizzleSchema(schemaPath, dialect);
      break;
    }
    case "prisma": {
      const { exportPrismaSchema } = await import("../src/prisma.js");
      schema = await exportPrismaSchema(schemaPath);
      break;
    }
    case "typeorm": {
      const { exportTypeORMSchema } = await import("../src/typeorm.js");
      schema = await exportTypeORMSchema(schemaPath, dialect);
      break;
    }
    default:
      console.error(`Unknown ORM: ${args.orm}. Supported: drizzle, prisma, typeorm`);
      process.exit(1);
  }

  // Output JSON to stdout — the Go binary captures this
  console.log(JSON.stringify(schema, null, 2));
}

function parseArgs(argv: string[]): Record<string, string> {
  const args: Record<string, string> = {};
  for (let i = 0; i < argv.length; i++) {
    if (argv[i].startsWith("--") && i + 1 < argv.length) {
      args[argv[i].slice(2)] = argv[i + 1];
      i++;
    }
  }
  return args;
}

main().catch((err) => {
  console.error("Error:", err.message);
  process.exit(1);
});
