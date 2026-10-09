import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import vm from "node:vm";
import ts from "typescript";

// Exercise the page's actual list effect without adding a DOM/test dependency.
const source = readFileSync(new URL("../src/app/(main)/function/llm-records/page.tsx", import.meta.url), "utf8");
const ast = ts.createSourceFile("page.tsx", source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
let effect;
let pagination;
function visit(node) {
  if (
    ts.isCallExpression(node) &&
    node.expression.getText(ast) === "React.useEffect" &&
    node.arguments[0]?.getText(ast).includes(".llmRecords(")
  ) {
    effect = node.arguments[0].getText(ast);
  }
  if (ts.isJsxExpression(node) && node.expression?.getText(ast).includes("`${page + 1}")) {
    pagination = node.expression.getText(ast);
  }
  ts.forEachChild(node, visit);
}
visit(ast);
assert.ok(effect, "list effect must exist");
const code = ts.transpileModule(`(${effect})`, { compilerOptions: { target: ts.ScriptTarget.ES2022 } }).outputText;

test("failed filter clears stale rows, reports failure, and allows successful retry", async () => {
  const state = { records: ["stale"], total: 1, error: "old", loading: false, selected: "stale", detail: "stale" };
  let fail = true;
  const run = vm.runInNewContext(code, {
    api: {
      llmRecords: async () => {
        if (fail) throw new Error("fixture failure");
        return { records: ["fresh"], total: 1 };
      },
      llmTasks: async () => ({ tasks: [] }),
    },
    Error,
    model: "fixture",
    sessionQ: "",
    pickedTask: "",
    page: 0,
    size: 50,
    setRecords: (value) => {
      state.records = value;
    },
    setSelected: (value) => {
      state.selected = value;
    },
    setDetail: (value) => {
      state.detail = value;
    },
    setTotal: (value) => {
      state.total = value;
    },
    setListError: (value) => {
      state.error = value;
    },
    setLoading: (value) => {
      state.loading = value;
    },
    setTasks: () => {},
  });
  run();
  assert.equal(state.records.length, 0);
  assert.equal(state.total, 0);
  assert.equal(state.selected, null);
  assert.equal(state.detail, null);
  await new Promise(setImmediate);
  assert.equal(state.records.length, 0);
  assert.equal(state.total, 0);
  assert.equal(state.error, "fixture failure");
  assert.equal(state.loading, false);
  fail = false;
  run();
  await new Promise(setImmediate);
  assert.equal(state.records[0], "fresh");
  assert.equal(state.total, 1);
  assert.equal(state.error, "");
  const cleanup = run();
  cleanup();
  state.records = ["current request"];
  state.total = 9;
  state.error = "current error";
  state.loading = true;
  await new Promise(setImmediate);
  assert.equal(state.records[0], "current request");
  assert.equal(state.total, 9);
  assert.equal(state.error, "current error");
  assert.equal(state.loading, true);
});

test("pagination does not advertise page three of one on loading or failure", () => {
  assert.ok(pagination, "pagination expression must exist");
  const render = (loading, listError, totalPages) =>
    vm.runInNewContext(pagination, { loading, listError, totalPages, page: 2 });
  assert.equal(render(true, "", 1), "—");
  assert.equal(render(false, "fixture failure", 1), "—");
  assert.equal(render(false, "", 4), "3 / 4");
});
