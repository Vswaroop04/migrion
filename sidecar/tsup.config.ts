import { defineConfig } from "tsup";

export default defineConfig({
  entry: ["src/index.ts", "bin/migratex-export.ts"],
  format: ["esm"],
  target: "node18",
  dts: false,
  clean: true,
  splitting: false,
});
