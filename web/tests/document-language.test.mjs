import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import ts from "typescript";

test("root document declares the Chinese interface language", () => {
  const source = readFileSync(new URL("../src/app/layout.tsx", import.meta.url), "utf8");
  const ast = ts.createSourceFile("layout.tsx", source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
  const languages = [];
  function visit(node) {
    if (ts.isJsxOpeningElement(node) && node.tagName.getText(ast) === "html") {
      const lang = node.attributes.properties.find(
        (attribute) => ts.isJsxAttribute(attribute) && attribute.name.getText(ast) === "lang",
      );
      languages.push(lang?.initializer && ts.isStringLiteral(lang.initializer) ? lang.initializer.text : undefined);
    }
    ts.forEachChild(node, visit);
  }
  visit(ast);
  assert.deepEqual(languages, ["zh-CN"]);
});
