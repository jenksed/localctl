import { stat } from "node:fs/promises";

for (const file of ["app.js", "viewmodel.js"]) {
  await stat(new URL(`./dist/${file}`, import.meta.url));
}
