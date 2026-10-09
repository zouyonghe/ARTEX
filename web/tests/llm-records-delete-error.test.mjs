import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import vm from "node:vm";
import ts from "typescript";

const source = readFileSync(new URL("../src/app/(main)/function/llm-records/page.tsx", import.meta.url), "utf8");
const ast = ts.createSourceFile("page.tsx", source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
let handler;
function visit(node) {
  if (ts.isVariableDeclaration(node) && node.name.getText(ast) === "confirmDelete") {
    handler = node.initializer.getText(ast);
  }
  ts.forEachChild(node, visit);
}
visit(ast);
assert.ok(handler, "delete handler must exist");
const code = ts.transpileModule(`(${handler})`, { compilerOptions: { target: ts.ScriptTarget.ES2022 } }).outputText;

test("delete failure reports error without clearing selection, then successful retry refreshes", async () => {
  const state = { busy: false, open: true, selected: "fixture", detail: "fixture", task: "fixture", page: 2, tick: 0 };
  const messages = [];
  let failure = new Error("fixture delete failure");
  const run = vm.runInNewContext(code, {
    api: {
      llmRecordsDeleteTask: async () => {
        if (failure !== undefined) throw failure;
      },
    },
    Error,
    toast: { error: (message) => messages.push(message) },
    pickedTask: "fixture",
    setDeleting: (v) => {
      state.busy = v;
    },
    setDeleteOpen: (v) => {
      state.open = v;
    },
    setSelected: (v) => {
      state.selected = v;
    },
    setDetail: (v) => {
      state.detail = v;
    },
    setPickedTask: (v) => {
      state.task = v;
    },
    setPage: (v) => {
      state.page = v;
    },
    setReloadTick: (fn) => {
      state.tick = fn(state.tick);
    },
  });
  run();
  await new Promise(setImmediate);
  assert.ok(messages[0]?.includes("fixture delete failure"));
  assert.equal(state.busy, false);
  assert.equal(state.open, true);
  assert.equal(state.selected, "fixture");
  assert.equal(state.detail, "fixture");
  assert.equal(state.task, "fixture");
  assert.equal(state.tick, 0);
  failure = "raw fixture failure";
  run();
  await new Promise(setImmediate);
  assert.equal(messages[1], "删除失败：未知错误");
  failure = undefined;
  run();
  await new Promise(setImmediate);
  assert.equal(state.open, false);
  assert.equal(state.busy, false);
  assert.equal(state.selected, null);
  assert.equal(state.detail, null);
  assert.equal(state.task, "");
  assert.equal(state.page, 0);
  assert.equal(state.tick, 1);
});
