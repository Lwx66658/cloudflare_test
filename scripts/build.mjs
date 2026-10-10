import { execFileSync } from "node:child_process";

const run = (args, env = {}) =>
  execFileSync("go", args, { stdio: "inherit", env: { ...process.env, ...env } });

run(["run", "github.com/syumai/workers-go/cmd/workers-assets-gen", "-mode=go"]);
run(["build", "-o", "./build/app.wasm", "."], { GOOS: "js", GOARCH: "wasm" });
