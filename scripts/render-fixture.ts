import * as fs from "fs";

import { renderLists, type ExportList } from "../dump-todos.js";

type Fixture = {
  lists: ExportList[];
};

const fixturePath = process.argv[2];
if (!fixturePath) {
  console.error("Usage: node dist-ts/scripts/render-fixture.js <fixture-path> [--incomplete]");
  process.exit(1);
}

const fixture = JSON.parse(fs.readFileSync(fixturePath, "utf8")) as Fixture;
const incompleteOnly = process.argv.includes("--incomplete");
const output = renderLists(fixture.lists, incompleteOnly);

process.stdout.write(output);
if (output.length > 0 && !output.endsWith("\n")) {
  process.stdout.write("\n");
}