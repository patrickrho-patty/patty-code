import { execFileSync } from "node:child_process";
import { cpSync, mkdirSync, readFileSync, rmSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { publishPackages } from "./publish.mjs";

const HERE = dirname(fileURLToPath(import.meta.url));
const ROOT = join(HERE, "..");
const STAGE = join(HERE, ".stage");

const TARGETS = [
  { node: "darwin-arm64", goos: "darwin", goarch: "arm64" },
  { node: "darwin-x64", goos: "darwin", goarch: "amd64" },
  { node: "linux-arm64", goos: "linux", goarch: "arm64" },
  { node: "linux-x64", goos: "linux", goarch: "amd64" },
  { node: "win32-arm64", goos: "windows", goarch: "arm64" },
  { node: "win32-x64", goos: "windows", goarch: "amd64" },
];

const tag = process.argv[2] ?? process.env.GITHUB_REF_NAME;
if (!tag) {
  console.error("usage: node npm/build.mjs <tag>   (e.g. v1.0.0 or npm-v1.0.0)");
  process.exit(1);
}
// npm ships on its own `npm-v*` tag (release-npm.yml); also accept a bare `v*`.
const version = tag.replace(/^(npm-)?v/, "");
const binaryVersion = `v${version}`;
const publish = process.argv.includes("--publish");
const candidateSha = execFileSync("git", ["rev-parse", "HEAD"], {
  cwd: ROOT,
  encoding: "utf8",
}).trim();
const gitCommit = candidateSha.slice(0, 12);
// Real UTC build clock for version --verbose/--json (not VCS commit time).
const buildTimeUTC = new Date().toISOString().replace(/\.\d{3}Z$/, "Z");

rmSync(STAGE, { recursive: true, force: true });
mkdirSync(STAGE, { recursive: true });

const subPackages = [];
for (const t of TARGETS) {
  const name = `@patty-code/cli-${t.node}`;
  const dir = join(STAGE, `cli-${t.node}`);
  const exe = t.goos === "windows" ? "patcode.exe" : "patcode";
  mkdirSync(join(dir, "bin"), { recursive: true });

  console.log(`build ${t.goos}/${t.goarch} -> ${name}`);
  execFileSync(
    "go",
    [
      "build",
      "-trimpath",
      "-ldflags",
      `-s -w -X main.version=${binaryVersion} -X main.gitCommit=${gitCommit} -X main.buildTimeUTC=${buildTimeUTC} -X patty/internal/productdocs.linkedVersion=${binaryVersion} -X patty/internal/productdocs.linkedRevision=${candidateSha}`,
      "-o",
      join(dir, "bin", exe),
      "./cmd/patcode",
    ],
    {
      cwd: ROOT,
      stdio: "inherit",
      env: { ...process.env, CGO_ENABLED: "0", GOOS: t.goos, GOARCH: t.goarch },
    },
  );

  writeFileSync(
    join(dir, "package.json"),
    `${JSON.stringify(
      {
        name,
        version,
        description: `patty-code prebuilt binary for ${t.node}.`,
        os: [t.goos === "windows" ? "win32" : t.goos],
        cpu: [t.goarch === "amd64" ? "x64" : "arm64"],
        files: ["bin/"],
        license: "MIT",
        repository: {
          type: "git",
          url: "git+https://github.com/patty-io/patty-code.git",
        },
        pattyCandidateSha: candidateSha,
      },
      null,
      2,
    )}\n`,
  );
  subPackages.push({ name, dir });
}

const mainDir = join(STAGE, "patty-code");
mkdirSync(mainDir, { recursive: true });
cpSync(join(HERE, "patty-code", "bin"), join(mainDir, "bin"), { recursive: true });
cpSync(join(ROOT, "README.md"), join(mainDir, "README.md"));

const mainPkg = JSON.parse(
  readFileSync(join(HERE, "patty-code", "package.json"), "utf8"),
);
mainPkg.version = version;
mainPkg.pattyCandidateSha = candidateSha;
for (const key of Object.keys(mainPkg.optionalDependencies)) {
  mainPkg.optionalDependencies[key] = version;
}
writeFileSync(
  join(mainDir, "package.json"),
  `${JSON.stringify(mainPkg, null, 2)}\n`,
);

if (!publish) {
  console.log(`\nstaged ${version} in ${STAGE} (dry run; pass --publish to publish)`);
  process.exit(0);
}

// Publish every immutable package before advancing the public channel. Recovery
// reuses packages that already prove the same candidate SHA, fills only missing
// packages, and never moves latest/next/canary back to an older version.
publishPackages({
  packages: [...subPackages, { name: "patty-code", dir: mainDir }],
  version,
  candidateSha,
});
