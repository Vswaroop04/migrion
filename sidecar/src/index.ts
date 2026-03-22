/**
 * migratex sidecar — ORM schema exporters.
 *
 * This package extracts schema metadata from ORMs (Drizzle, Prisma, TypeORM)
 * and outputs standardized JSON that the Go migratex engine reads.
 */

export { exportDrizzleSchema } from "./drizzle.js";
export { exportPrismaSchema } from "./prisma.js";
export { exportTypeORMSchema } from "./typeorm.js";
export type * from "./types.js";
