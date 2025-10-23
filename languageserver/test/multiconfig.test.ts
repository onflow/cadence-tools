import {
  createProtocolConnection,
  InitializeRequest,
  ExitNotification,
  StreamMessageReader,
  StreamMessageWriter,
  ProtocolConnection,
  DidOpenTextDocumentNotification,
  PublishDiagnosticsNotification,
  PublishDiagnosticsParams,
  TextDocumentItem,
} from "vscode-languageserver-protocol";

import { execSync, spawn } from "child_process";
import * as path from "path";
import * as fs from "fs";
import * as os from "os";

const FLOW_JSON = "flow.json";

beforeAll(() => {
  execSync("go build ../cmd/languageserver", {
    cwd: __dirname,
  });
});

async function withConnection(
  f: (connection: ProtocolConnection) => Promise<void>
): Promise<void> {
  const child = spawn(path.resolve(__dirname, "./languageserver"), [
    "--enable-flow-client=false",
  ]);

  const connection = createProtocolConnection(
    new StreamMessageReader(child.stdout),
    new StreamMessageWriter(child.stdin),
    null
  );
  connection.listen();

  await connection.sendRequest(InitializeRequest.type, {
    capabilities: {},
    processId: process.pid,
    rootUri: "/",
    workspaceFolders: null,
    initializationOptions: null,
  });

  try {
    await f(connection);
  } finally {
    try {
      await connection.sendNotification(ExitNotification.type);
    } catch (e) {
      // Connection may already be closed
    }
    child.kill();
  }
}

describe("multi-config routing (no flow client)", () => {
  const baseFlow = JSON.parse(
    fs.readFileSync(path.join(__dirname, FLOW_JSON), "utf8")
  );

  function writeFlow(dir: string, accountName: string) {
    const flow = JSON.parse(JSON.stringify(baseFlow));
    // Rename the configured account from "moose" to provided name
    if (flow.accounts && flow.accounts.moose) {
      flow.accounts[accountName] = flow.accounts.moose;
      delete flow.accounts.moose;
    }
    if (
      flow.deployments &&
      flow.deployments.emulator &&
      flow.deployments.emulator.moose
    ) {
      flow.deployments.emulator[accountName] = flow.deployments.emulator.moose;
      delete flow.deployments.emulator.moose;
    }
    fs.mkdirSync(dir, { recursive: true });
    fs.writeFileSync(path.join(dir, FLOW_JSON), JSON.stringify(flow, null, 2));
  }

  test("string imports resolve against closest flow.json per file", async () => {
    const root = fs.mkdtempSync(path.join(os.tmpdir(), "tmp-"));
    const aDir = fs.mkdtempSync(path.join(root, "dir-"));
    const bDir = fs.mkdtempSync(path.join(root, "dir-"));
    writeFlow(aDir, "moose");
    writeFlow(bDir, "elk");
    // Ensure contracts exist per flow.json mapping
    fs.writeFileSync(
      path.join(aDir, "foo.cdc"),
      "access(all) contract Foo { access(all) let bar: Int; init(){ self.bar = 1 } }"
    );
    fs.writeFileSync(
      path.join(bDir, "bar.cdc"),
      "access(all) contract Bar { access(all) let baz: Int; init(){ self.baz = 2 } }"
    );
    // Scripts importing by relative file path
    const aScript = `import Foo from "./foo.cdc"\naccess(all) fun main() { log(Foo.bar) }`;
    const bScript = `import Bar from "./bar.cdc"\naccess(all) fun main() { log(Bar.baz) }`;
    fs.writeFileSync(path.join(aDir, "script.cdc"), aScript);
    fs.writeFileSync(path.join(bDir, "script.cdc"), bScript);

    try {
      await withConnection(async (connection) => {
        // Open a script under aDir and expect no diagnostics
        const aUri = `file://${aDir}/script.cdc`;
        const aDoc = TextDocumentItem.create(aUri, "cadence", 1, aScript);
        const aNotif = new Promise<PublishDiagnosticsParams>((resolve) =>
          connection.onNotification(
            PublishDiagnosticsNotification.type,
            (n) => {
              if (n.uri === aUri) resolve(n);
            }
          )
        );
        await connection.sendNotification(
          DidOpenTextDocumentNotification.type,
          {
            textDocument: aDoc,
          }
        );
        const aDiag = await aNotif;
        expect(aDiag.diagnostics).toHaveLength(0);

        // Open a script under bDir and expect no diagnostics
        const bUri = `file://${bDir}/script.cdc`;
        const bDoc = TextDocumentItem.create(bUri, "cadence", 1, bScript);
        const bNotif = new Promise<PublishDiagnosticsParams>((resolve) =>
          connection.onNotification(
            PublishDiagnosticsNotification.type,
            (n) => {
              if (n.uri === bUri) resolve(n);
            }
          )
        );
        await connection.sendNotification(
          DidOpenTextDocumentNotification.type,
          {
            textDocument: bDoc,
          }
        );
        const bDiag = await bNotif;
        expect(bDiag.diagnostics).toHaveLength(0);
      });
    } finally {
      try {
        fs.rmdirSync(root, { recursive: true });
      } catch {}
    }
  });

  test("import crossing into another config's coverage errors as different project", async () => {
    const root = fs.mkdtempSync(path.join(os.tmpdir(), "tmp-"));
    const aDir = fs.mkdtempSync(path.join(root, "dir-"));
    const bDir = fs.mkdtempSync(path.join(root, "dir-"));
    writeFlow(aDir, "moose");
    writeFlow(bDir, "elk");

    // Create a contract only in bDir
    const bContract = path.join(bDir, "foo.cdc");
    fs.mkdirSync(bDir, { recursive: true });
    fs.writeFileSync(bContract, "access(all) contract Foo { }\n");

    // In aDir, attempt to import bDir/foo.cdc using a relative path that escapes into bDir
    // For the test, we compute the relative path from aDir to bDir/foo.cdc
    const relToB = path.relative(aDir, bContract).replace(/\\/g, "/");
    const aScript = `import Foo from "${relToB}"\naccess(all) fun main() { }`;
    const aScriptPath = path.join(aDir, "script.cdc");
    fs.writeFileSync(aScriptPath, aScript);

    try {
      await withConnection(async (connection) => {
        const aUri = `file://${aScriptPath}`;
        const aNotif = new Promise<PublishDiagnosticsParams>((resolve) =>
          connection.onNotification(
            PublishDiagnosticsNotification.type,
            (n) => {
              if (n.uri === aUri) resolve(n);
            }
          )
        );
        await connection.sendNotification(
          DidOpenTextDocumentNotification.type,
          {
            textDocument: TextDocumentItem.create(aUri, "cadence", 1, aScript),
          }
        );
        const diag = await aNotif;
        // Expect an error because the import path points into a different config coverage
        expect(diag.diagnostics.length).toBeGreaterThan(0);
      });
    } finally {
      try {
        fs.rmdirSync(root, { recursive: true });
      } catch {}
    }
  });

  test("relative traversal outside project root errors", async () => {
    // Create random nested project root and a separate random outside dir
    const root = fs.mkdtempSync(path.join(os.tmpdir(), "tmp-"));
    const p = fs.mkdtempSync(path.join(root, "p-"));
    const q = fs.mkdtempSync(path.join(p, "q-"));
    const aDir = fs.mkdtempSync(path.join(q, "r-"));
    const outsideBase = fs.mkdtempSync(path.join(root, "out-"));
    const contractsDir = path.join(outsideBase, "cadence", "contracts");
    writeFlow(aDir, "moose");
    fs.mkdirSync(contractsDir, { recursive: true });
    const counterPath = path.join(contractsDir, "Counter.cdc");
    fs.writeFileSync(counterPath, "access(all) contract Counter {}\n");

    // Import using path that traverses to outsideBase
    const rel = path.relative(aDir, counterPath).replace(/\\/g, "/");
    const script = `import Counter from "${rel}"\naccess(all) fun main() { }`;
    const scriptPath = path.join(aDir, "script.cdc");
    fs.mkdirSync(aDir, { recursive: true });
    fs.writeFileSync(scriptPath, script);

    try {
      await withConnection(async (connection) => {
        const uri = `file://${scriptPath}`;
        const notif = new Promise<PublishDiagnosticsParams>((resolve) =>
          connection.onNotification(
            PublishDiagnosticsNotification.type,
            (n) => {
              if (n.uri === uri) resolve(n);
            }
          )
        );
        await connection.sendNotification(
          DidOpenTextDocumentNotification.type,
          {
            textDocument: TextDocumentItem.create(uri, "cadence", 1, script),
          }
        );
        const diag = await notif;
        expect(diag.diagnostics.length).toBeGreaterThan(0);
      });
    } finally {
      try {
        fs.rmdirSync(root, { recursive: true });
      } catch {}
    }
  });

  test("identifier imports resolve independently across projects", async () => {
    const root = fs.mkdtempSync(path.join(os.tmpdir(), "tmp-"));
    const aDir = fs.mkdtempSync(path.join(root, "dir-"));
    const bDir = fs.mkdtempSync(path.join(root, "dir-"));

    // Write contract files first
    fs.writeFileSync(
      path.join(aDir, "fooA.cdc"),
      "access(all) contract Foo { access(all) let bar: Int; init(){ self.bar = 1 } }"
    );
    fs.writeFileSync(
      path.join(bDir, "fooB.cdc"),
      "access(all) contract Foo { access(all) let bar: Int; init(){ self.bar = 2 } }"
    );

    // Write flow.json files with correct contracts mapping - only write ONCE
    // For aDir, keep account as "moose"
    const aFlow = JSON.parse(JSON.stringify(baseFlow));
    aFlow.contracts = { Foo: "./fooA.cdc" };
    // Update deployments to only include Foo (remove Bar which doesn't exist)
    if (aFlow.deployments && aFlow.deployments.emulator && aFlow.deployments.emulator.moose) {
      aFlow.deployments.emulator.moose = ["Foo"];
    }
    fs.mkdirSync(aDir, { recursive: true });
    fs.writeFileSync(
      path.join(aDir, FLOW_JSON),
      JSON.stringify(aFlow, null, 2)
    );

    // For bDir, rename account from "moose" to "elk"
    const bFlow = JSON.parse(JSON.stringify(baseFlow));
    if (bFlow.accounts && bFlow.accounts.moose) {
      bFlow.accounts["elk"] = bFlow.accounts.moose;
      delete bFlow.accounts.moose;
    }
    bFlow.contracts = { Foo: "./fooB.cdc" };
    // Update deployments: rename moose to elk and only deploy Foo
    if (bFlow.deployments && bFlow.deployments.emulator && bFlow.deployments.emulator.moose) {
      bFlow.deployments.emulator["elk"] = ["Foo"];
      delete bFlow.deployments.emulator.moose;
    }
    fs.mkdirSync(bDir, { recursive: true });
    fs.writeFileSync(
      path.join(bDir, FLOW_JSON),
      JSON.stringify(bFlow, null, 2)
    );

    const script = `import "Foo"\naccess(all) fun main() { log(Foo.bar) }`;
    fs.writeFileSync(path.join(aDir, "script.cdc"), script);
    fs.writeFileSync(path.join(bDir, "script.cdc"), script);

    // Give filesystem time to flush writes before starting language server
    await new Promise((resolve) => setTimeout(resolve, 200));

    try {
      await withConnection(async (connection) => {
        const aUri = `file://${aDir}/script.cdc`;
        const bUri = `file://${bDir}/script.cdc`;

        // Give language server time to initialize and discover flow.json files
        await new Promise((resolve) => setTimeout(resolve, 500));

        const got = new Map<string, PublishDiagnosticsParams>();
        const both = new Promise<void>((resolve) => {
          connection.onNotification(
            PublishDiagnosticsNotification.type,
            (n) => {
              if (n.uri === aUri || n.uri === bUri) {
                if (!got.has(n.uri)) {
                  got.set(n.uri, n);
                }
                if (got.has(aUri) && got.has(bUri)) resolve();
              }
            }
          );
        });

        await connection.sendNotification(
          DidOpenTextDocumentNotification.type,
          {
            textDocument: TextDocumentItem.create(aUri, "cadence", 1, script),
          }
        );
        await connection.sendNotification(
          DidOpenTextDocumentNotification.type,
          {
            textDocument: TextDocumentItem.create(bUri, "cadence", 1, script),
          }
        );

        await both;
        expect(got.get(aUri)!.diagnostics).toHaveLength(0);
        expect(got.get(bUri)!.diagnostics).toHaveLength(0);
      });
    } finally {
      try {
        fs.rmdirSync(root, { recursive: true });
      } catch {}
    }
  }, 15000);

  test("circular string import does not crash and reports diagnostics", async () => {
    const root = fs.mkdtempSync(path.join(os.tmpdir(), "tmp-"));
    const dir = fs.mkdtempSync(path.join(root, "dir-"));
    writeFlow(dir, "moose");
    // Create A importing B and B importing A
    const aPath = path.join(dir, "A.cdc");
    const bPath = path.join(dir, "B.cdc");
    fs.writeFileSync(
      aPath,
      [
        'import "B"',
        "access(all) contract A { access(all) fun x(): Int { return 1 } }",
        "",
      ].join("\n")
    );
    fs.writeFileSync(
      bPath,
      [
        'import "A"',
        "access(all) contract B { access(all) fun y(): Int { return 1 } }",
        "",
      ].join("\n")
    );
    const script = ['import "A"', "access(all) fun main() { log(1) }", ""].join(
      "\n"
    );
    const scriptPath = path.join(dir, "script.cdc");
    fs.writeFileSync(scriptPath, script);
    try {
      await withConnection(async (connection) => {
        const uri = `file://${scriptPath}`;
        const notif = new Promise<PublishDiagnosticsParams>((resolve) =>
          connection.onNotification(
            PublishDiagnosticsNotification.type,
            (n) => {
              if (n.uri === uri) resolve(n);
            }
          )
        );
        await connection.sendNotification(
          DidOpenTextDocumentNotification.type,
          {
            textDocument: TextDocumentItem.create(uri, "cadence", 1, script),
          }
        );
        const diag = await notif;
        // Expect at least one diagnostic due to circular dependency
        expect(diag.diagnostics.length).toBeGreaterThan(0);
      });
    } finally {
      try {
        fs.rmdirSync(root, { recursive: true });
      } catch {}
    }
  }, 15000);
});
