#!/usr/bin/env -S bun

//MISE dir="{{ config_root }}"

async function generateCLIReferenceDocs() {
  const commands: Array<string[]> = [
    ["init"],
    ["create"],
    ["bump"],
    ["commit"],
    ["latest"],
    ["cat"],
    ["builtins"],
    ["builtins", "current:default"],
    ["builtins", "current:file"],
    ["builtins", "current:npm"],
    ["builtins", "next:default"],
    ["builtins", "next:file"],
    ["builtins", "next:npm"],
    ["builtins", "amendlog:default"],
    ["builtins", "cat:default"],
  ];

  await Bun.$`rm -rf ./site/src/content/docs/cli/*`;

  for (const command of commands) {
    const helpText = await Bun.$`mise run bumper ${command} --help`.text();
    const title = command.join(" ");

    const docContent = `---
title: "bumper ${title}"
---

\`\`\`
${helpText.trim()}
\`\`\`
`;

    const filename = command.join("-").replaceAll(":", "-");

    await Bun.write(`./site/src/content/docs/cli/${filename}.md`, docContent);
  }

  const sidebarEntries = commands.map((command) => ({
    label: command.join(" "),
    slug: "cli/" + command.join("-").replaceAll(":", "-"),
  }));

  await Bun.write(
    "./site/cli-sidebar.json",
    JSON.stringify(sidebarEntries, null, 2),
  );
}

generateCLIReferenceDocs();
