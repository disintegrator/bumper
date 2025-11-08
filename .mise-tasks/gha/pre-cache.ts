#!/usr/bin/env -S bun

//MISE dir="{{ config_root }}"
//MISE hide=true

import fs from "node:fs/promises";
import process from "node:process";

async function goCacheEnv() {
  const ghenv = process.env["GITHUB_ENV"];
  if (!ghenv) {
    throw new Error("GITHUB_ENV is not defined");
  }

  const goBuildCache = (await Bun.$`go env GOCACHE`).text().trim();
  const goModCache = (await Bun.$`go env GOMODCACHE`).text().trim();

  await fs.appendFile(ghenv, `GOCACHE=${goBuildCache}\n`);
  await fs.appendFile(ghenv, `GOMODCACHE=${goModCache}\n`);

  const hasher = new Bun.CryptoHasher("sha256");
  hasher.update(await Bun.file("go.mod").text());
  hasher.update(await Bun.file("go.sum").text());
  const goModHash = hasher.digest("hex");

  const os = process.platform;
  const arch = process.arch;

  const buster = 2; // Increment this if you need to bust the cache
  const cacheKey = `${buster}-${os}-${arch}-${goModHash}`;
  const partialKey = `${buster}-${os}-${arch}-`;
  await fs.appendFile(ghenv, `GH_CACHE_GO_KEY=go-${cacheKey}\n`);
  await fs.appendFile(ghenv, `GH_CACHE_GO_KEY_PARTIAL=go-${partialKey}\n`);

  console.log(`Go cache: ${goBuildCache}`);
  console.log(`Go module cache: ${goModCache}`);
  console.log(`GitHub Go cache key: ${cacheKey}`);
  console.log(`GitHub Go partial cache key: ${partialKey}`);
}

goCacheEnv();
