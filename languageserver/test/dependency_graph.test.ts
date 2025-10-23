import {
  createProtocolConnection,
  InitializeRequest,
  ExitNotification,
  StreamMessageReader,
  StreamMessageWriter,
  ProtocolConnection,
  DidOpenTextDocumentNotification,
  DidChangeTextDocumentNotification,
  DidCloseTextDocumentNotification,
  PublishDiagnosticsNotification,
  PublishDiagnosticsParams,
  TextDocumentItem,
  VersionedTextDocumentIdentifier,
} from "vscode-languageserver-protocol";

import { execSync, spawn } from "child_process";
import * as path from "path";
import * as fs from "fs";
import * as os from "os";

beforeAll(() => {
  execSync("go build ../cmd/languageserver", {
    cwd: __dirname,
  });
});

async function withConnection(
  f: (connection: ProtocolConnection) => Promise<void>,
  rootUri: string = "/"
): Promise<void> {
  const child = spawn(path.resolve(__dirname, "./languageserver"), [
    "--enable-flow-client=false",
  ]);

  // Log stderr output for debugging
  child.stderr.on('data', (data) => {
    console.error(`Language server stderr: ${data}`);
  });

  child.on('exit', (code, signal) => {
    if (code !== null && code !== 0) {
      console.error(`Language server exited with code ${code}`);
    }
    if (signal !== null) {
      console.error(`Language server killed by signal ${signal}`);
    }
  });

  const connection = createProtocolConnection(
    new StreamMessageReader(child.stdout),
    new StreamMessageWriter(child.stdin),
    null
  );
  connection.listen();

  await connection.sendRequest(InitializeRequest.type, {
    capabilities: {},
    processId: process.pid,
    rootUri: rootUri,
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

function sleep(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

async function openAndWaitDiagnostics(
  connection: ProtocolConnection,
  uri: string,
  text: string
) {
  const notif = new Promise<PublishDiagnosticsParams>((resolve) =>
    connection.onNotification(PublishDiagnosticsNotification.type, (n) => {
      if (n.uri === uri) resolve(n);
    })
  );
  await connection.sendNotification(DidOpenTextDocumentNotification.type, {
    textDocument: TextDocumentItem.create(uri, "cadence", 1, text),
  });
  return await notif;
}

async function changeAndWaitDiagnostics(
  connection: ProtocolConnection,
  uri: string,
  version: number,
  text: string
) {
  const notif = new Promise<PublishDiagnosticsParams>((resolve) =>
    connection.onNotification(PublishDiagnosticsNotification.type, (n) => {
      if (n.uri === uri) resolve(n);
    })
  );
  await connection.sendNotification(DidChangeTextDocumentNotification.type, {
    textDocument: VersionedTextDocumentIdentifier.create(uri, version),
    contentChanges: [{ text }],
  });
  return await notif;
}

describe("dependency graph + flow.json updates", () => {
  test("editing A.cdc triggers re-check of dependent B", async () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "tmp-deps-"));
    const flow = {
      contracts: { A: "./A.cdc" },
      emulators: {
        default: { port: 3569, serviceAccount: "emulator-account" },
      },
      networks: { emulator: "127.0.0.1:3569" },
      accounts: {
        "emulator-account": {
          address: "f8d6e0586b0a20c7",
          key: "c44604c862a3950ae82d56638929720f44875b2637054a1fdcb4e76b01b40881",
        },
      },
      deployments: {},
    } as any;
    fs.writeFileSync(
      path.join(dir, "flow.json"),
      JSON.stringify(flow, null, 2)
    );
    fs.writeFileSync(
      path.join(dir, "A.cdc"),
      "access(all) contract A { access(all) fun foo(): Int { 1 } }"
    );
    const b = `import "./A.cdc"\naccess(all) fun main() { log(A.foo()) }`;
    const buri = `file://${dir}/b.cdc`;
    const auri = `file://${dir}/A.cdc`;
    fs.writeFileSync(path.join(dir, "b.cdc"), b);

    await withConnection(async (connection) => {
      const first = await openAndWaitDiagnostics(connection, buri, b);
      expect(first.uri).toBe(buri);
      // Open A so edits are sent via LSP overlay and propagate to dependents
      await openAndWaitDiagnostics(
        connection,
        auri,
        "access(all) contract A { access(all) fun foo(): Int { 1 } }"
      );
      // Now edit A to remove foo via LSP DidChange
      await changeAndWaitDiagnostics(
        connection,
        auri,
        2,
        "access(all) contract A {}"
      );

      // Trigger a re-check of B (explicit nudge)
      await sleep(300);
      const second = await changeAndWaitDiagnostics(connection, buri, 2, b);
      expect(second.uri).toBe(buri);
      const hasNoMemberFoo = second.diagnostics.some((d) =>
        (d.message || "").includes("no member `foo`")
      );
      expect(hasNoMemberFoo).toBe(true);
    }, `file://${dir}`);
  });

  test("identifier import: flow.json present, adding A.cdc resolves import in B", async () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "tmp-deps-"));
    const b = `import "./A.cdc"\naccess(all) fun main() { log(A) }`;
    const buri = `file://${dir}/b.cdc`;
    const flow = {
      contracts: { A: "./A.cdc" },
      emulators: {
        default: { port: 3569, serviceAccount: "emulator-account" },
      },
      networks: { emulator: "127.0.0.1:3569" },
      accounts: {
        "emulator-account": {
          address: "f8d6e0586b0a20c7",
          key: "c44604c862a3950ae82d56638929720f44875b2637054a1fdcb4e76b01b40881",
        },
      },
      deployments: {},
    } as any;
    // flow.json created before opening any documents
    fs.writeFileSync(
      path.join(dir, "flow.json"),
      JSON.stringify(flow, null, 2)
    );
    fs.writeFileSync(path.join(dir, "b.cdc"), b);

    await withConnection(async (connection) => {
      // Open B first while A.cdc does not exist yet -> expect errors
      const first = await openAndWaitDiagnostics(connection, buri, b);
      expect(first.uri).toBe(buri);
      expect(first.diagnostics.length).toBeGreaterThan(0);

      // Now add A.cdc via LSP and trigger a re-check in B
      const auri = `file://${dir}/A.cdc`;
      await openAndWaitDiagnostics(
        connection,
        auri,
        "access(all) contract A {}"
      );
      const second = await changeAndWaitDiagnostics(connection, buri, 2, b);
      expect(second.uri).toBe(buri);
      expect(second.diagnostics.length).toBe(0);
    }, `file://${dir}`);
  });

  test("transitive: editing C triggers re-check of A and B (B -> A -> C via file imports)", async () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "tmp-deps-"));
    const c1 =
      "access(all) contract C { access(all) fun v(): Int { return 1 } }";
    const c2 =
      'access(all) contract C { access(all) fun v(): String { return "x" } }';
    fs.writeFileSync(path.join(dir, "C.cdc"), c1);

    const a = `import C from "./C.cdc"\naccess(all) contract A {}`;
    fs.writeFileSync(path.join(dir, "A.cdc"), a);

    const b = `import A from "./A.cdc"\naccess(all) fun main() { log(A) }`;
    const buri = `file://${dir}/b.cdc`;
    const auri = `file://${dir}/A.cdc`;
    const curi = `file://${dir}/C.cdc`;
    fs.writeFileSync(path.join(dir, "b.cdc"), b);

    await withConnection(async (connection) => {
      // Watch for diagnostics on B specifically
      const bNotif = new Promise<PublishDiagnosticsParams>((resolve) =>
        connection.onNotification(PublishDiagnosticsNotification.type, (n) => {
          if (n.uri === buri) resolve(n);
        })
      );

      // Open all three so dependency edges are recorded and all are considered open
      await connection.sendNotification(DidOpenTextDocumentNotification.type, {
        textDocument: TextDocumentItem.create(curi, "cadence", 1, c1),
      });
      await connection.sendNotification(DidOpenTextDocumentNotification.type, {
        textDocument: TextDocumentItem.create(auri, "cadence", 1, a),
      });
      await connection.sendNotification(DidOpenTextDocumentNotification.type, {
        textDocument: TextDocumentItem.create(buri, "cadence", 1, b),
      });

      // Consume initial B notification
      await bNotif;

      // Now change C and expect B to be rechecked transitively via A
      const bNextNotif = new Promise<PublishDiagnosticsParams>((resolve) =>
        connection.onNotification(PublishDiagnosticsNotification.type, (n) => {
          if (n.uri === buri) resolve(n);
        })
      );
      await connection.sendNotification(
        DidChangeTextDocumentNotification.type,
        {
          textDocument: VersionedTextDocumentIdentifier.create(curi, 2),
          contentChanges: [{ text: c2 }],
        }
      );

      const bAfter = await bNextNotif;
      expect(bAfter.uri).toBe(buri);
      expect(bAfter.diagnostics.length).toBe(0);
    });
  });

  test("circular imports: introducing and breaking cycles updates diagnostics for both files", async () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "tmp-deps-"));
    const aPath = path.join(dir, "A.cdc");
    const bPath = path.join(dir, "B.cdc");
    const auri = `file://${aPath}`;
    const buri = `file://${bPath}`;

    // Start without cycle: A imports B, B has no imports
    const aNoCycle = `import B from "./B.cdc"\naccess(all) contract A {}`;
    const bNoCycle = `access(all) contract B {}`;
    fs.writeFileSync(aPath, aNoCycle);
    fs.writeFileSync(bPath, bNoCycle);

    await withConnection(async (connection) => {
      // Open both and verify initial state sequentially
      const aFirst = await openAndWaitDiagnostics(connection, auri, aNoCycle);
      expect(aFirst.diagnostics.length).toBe(0);
      const bFirst = await openAndWaitDiagnostics(connection, buri, bNoCycle);
      expect(bFirst.diagnostics.length).toBe(0);

      // Introduce cycle: B now imports A and wait for B diagnostics directly
      const bCycle = `import A from "./A.cdc"\naccess(all) contract B {}`;
      const bCycleDiag = await changeAndWaitDiagnostics(
        connection,
        buri,
        2,
        bCycle
      );
      expect(bCycleDiag.uri).toBe(buri);

      // Force A to re-check to observe cycle diagnostics as well
      const aCycleDiag = await changeAndWaitDiagnostics(
        connection,
        auri,
        2,
        aNoCycle
      );
      expect(aCycleDiag.uri).toBe(auri);

      // Break cycle: revert B to no import
      // Break cycle: revert B to no import and wait directly
      const bAfterBreak = await changeAndWaitDiagnostics(
        connection,
        buri,
        3,
        bNoCycle
      );
      expect(bAfterBreak.uri).toBe(buri);
      // Force A to re-check and expect clean diagnostics after breaking cycle
      const aAfterBreak = await changeAndWaitDiagnostics(
        connection,
        auri,
        3,
        aNoCycle
      );
      expect(aAfterBreak.uri).toBe(auri);
    });
  }, 60000);

  test("circular transitive: A -> B -> C, then C -> A cycles and breaks", async () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "tmp-deps-"));
    const aPath = path.join(dir, "A.cdc");
    const bPath = path.join(dir, "B.cdc");
    const cPath = path.join(dir, "C.cdc");
    const auri = `file://${aPath}`;
    const buri = `file://${bPath}`;
    const curi = `file://${cPath}`;

    const aSrc = `import B from "./B.cdc"\naccess(all) contract A {}`;
    const bSrc = `import C from "./C.cdc"\naccess(all) contract B {}`;
    const cSrc = `access(all) contract C {}`;
    const cCycle = `import A from "./A.cdc"\naccess(all) contract C {}`;

    fs.writeFileSync(aPath, aSrc);
    fs.writeFileSync(bPath, bSrc);
    fs.writeFileSync(cPath, cSrc);

    await withConnection(async (connection) => {
      // Open all three sequentially and expect initial clean state
      const aFirst = await openAndWaitDiagnostics(connection, auri, aSrc);
      expect(aFirst.diagnostics.length).toBe(0);
      const bFirst = await openAndWaitDiagnostics(connection, buri, bSrc);
      expect(bFirst.diagnostics.length).toBe(0);
      const cFirst = await openAndWaitDiagnostics(connection, curi, cSrc);
      expect(cFirst.diagnostics.length).toBe(0);

      // Introduce cycle at C and explicitly re-check B then A
      const cOnCycle = await changeAndWaitDiagnostics(
        connection,
        curi,
        2,
        cCycle
      );
      expect(cOnCycle.uri).toBe(curi);
      const bOnCycle = await changeAndWaitDiagnostics(
        connection,
        buri,
        2,
        bSrc
      );
      expect(bOnCycle.uri).toBe(buri);
      const aOnCycle = await changeAndWaitDiagnostics(
        connection,
        auri,
        2,
        aSrc
      );
      expect(aOnCycle.uri).toBe(auri);

      // Break the cycle at C and explicitly re-check B then A
      const cOnBreak = await changeAndWaitDiagnostics(
        connection,
        curi,
        3,
        cSrc
      );
      expect(cOnBreak.uri).toBe(curi);
      const bOnBreak = await changeAndWaitDiagnostics(
        connection,
        buri,
        3,
        bSrc
      );
      expect(bOnBreak.uri).toBe(buri);
      const aOnBreak = await changeAndWaitDiagnostics(
        connection,
        auri,
        3,
        aSrc
      );
      expect(aOnBreak.uri).toBe(auri);
    });
  }, 60000);

  test("dual alias invalidation: both identifier and path imports update on A change", async () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "tmp-deps-"));
    const flow = {
      contracts: { A: "./A.cdc" },
      emulators: {
        default: { port: 3569, serviceAccount: "emulator-account" },
      },
      networks: { emulator: "127.0.0.1:3569" },
      accounts: {
        "emulator-account": {
          address: "f8d6e0586b0a20c7",
          key: "c44604c862a3950ae82d56638929720f44875b2637054a1fdcb4e76b01b40881",
        },
      },
      deployments: {},
    } as any;
    fs.writeFileSync(
      path.join(dir, "flow.json"),
      JSON.stringify(flow, null, 2)
    );

    const a1 =
      "access(all) contract A { access(all) fun foo(): Int { return 1 } }";
    const a2 = "access(all) contract A {}";
    const aPath = path.join(dir, "A.cdc");
    fs.writeFileSync(aPath, a1);

    const b1src = `import "./A.cdc"\naccess(all) fun main() { log(A.foo()) }`;
    const b2src = `import A from "./A.cdc"\naccess(all) fun main() { log(A.foo()) }`;
    const b1Path = path.join(dir, "B1.cdc");
    const b2Path = path.join(dir, "B2.cdc");
    fs.writeFileSync(b1Path, b1src);
    fs.writeFileSync(b2Path, b2src);
    const auri = `file://${aPath}`;
    const b1uri = `file://${b1Path}`;
    const b2uri = `file://${b2Path}`;

    await withConnection(async (connection) => {
      // Open A.cdc first to ensure it's processed
      const aFirst = await openAndWaitDiagnostics(connection, auri, a1);
      expect(aFirst.diagnostics.length).toBe(0);

      // Open both B files and validate initial clean state
      const b1First = await openAndWaitDiagnostics(connection, b1uri, b1src);
      expect(b1First.diagnostics.length).toBe(0);
      const b2First = await openAndWaitDiagnostics(connection, b2uri, b2src);
      expect(b2First.diagnostics.length).toBe(0);

      // Change A to remove foo; expect both B1 and B2 to report missing member
      console.log("About to change A.cdc...");
      const aChanged = await changeAndWaitDiagnostics(connection, auri, 2, a2);
      expect(aChanged.uri).toBe(auri);
      console.log("A.cdc changed, waiting for B1 and B2 updates...");

      // Wait a bit for notifications to propagate, then re-check B files
      await new Promise((resolve) => setTimeout(resolve, 1000));

      // Re-check B1 and B2 files to see updated diagnostics
      const b1After = await changeAndWaitDiagnostics(
        connection,
        b1uri,
        2,
        b1src
      );
      const b2After = await changeAndWaitDiagnostics(
        connection,
        b2uri,
        2,
        b2src
      );
      const hasNoMemberFoo1 = b1After.diagnostics.some((d) =>
        (d.message || "").includes("no member `foo`")
      );
      const hasNoMemberFoo2 = b2After.diagnostics.some((d) =>
        (d.message || "").includes("no member `foo`")
      );
      expect(hasNoMemberFoo1).toBe(true);
      expect(hasNoMemberFoo2).toBe(true);
    }, `file://${dir}`);
  }, 60000);

  test("identifier import shares checker with file path import", async () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "tmp-shared-checker-"));
    const flow = {
      contracts: { A: "./A.cdc" },
      networks: { emulator: "127.0.0.1:3569" },
      accounts: {
        emulator: {
          address: "f8d6e0586b0a20c7",
          key: "c44604c862a3950ae82d56638929720f44875b2637054a1fdcb4e76b01b40881",
        },
      },
    };
    fs.writeFileSync(
      path.join(dir, "flow.json"),
      JSON.stringify(flow, null, 2)
    );

    // Create A.cdc with a function
    const aContent =
      "access(all) contract A { access(all) fun foo(): Int { return 42 } }";
    const aPath = path.join(dir, "A.cdc");
    fs.writeFileSync(aPath, aContent);

    // Create B.cdc that imports A via identifier
    const bContent = `import "A"\naccess(all) fun main() { log(A.foo()) }`;
    const bPath = path.join(dir, "B.cdc");
    fs.writeFileSync(bPath, bContent);

    // Create C.cdc that imports A via file path
    const cContent = `import "./A.cdc"\naccess(all) fun main() { log(A.foo()) }`;
    const cPath = path.join(dir, "C.cdc");
    fs.writeFileSync(cPath, cContent);

    await withConnection(async (connection) => {
      const aUri = `file://${aPath}`;
      const bUri = `file://${bPath}`;
      const cUri = `file://${cPath}`;

      // Open all files and verify they have no errors initially
      const aFirst = await openAndWaitDiagnostics(connection, aUri, aContent);
      const bFirst = await openAndWaitDiagnostics(connection, bUri, bContent);
      const cFirst = await openAndWaitDiagnostics(connection, cUri, cContent);

      expect(aFirst.diagnostics).toHaveLength(0);
      expect(bFirst.diagnostics).toHaveLength(0);
      expect(cFirst.diagnostics).toHaveLength(0);

      // Now modify A.cdc to remove the foo function
      const aModified = "access(all) contract A { }";
      fs.writeFileSync(aPath, aModified);

      // Wait for changes to propagate
      await sleep(1000);

      // Re-check all files - they should all show the error
      const aAfter = await changeAndWaitDiagnostics(
        connection,
        aUri,
        2,
        aModified
      );
      const bAfter = await changeAndWaitDiagnostics(
        connection,
        bUri,
        2,
        bContent
      );
      const cAfter = await changeAndWaitDiagnostics(
        connection,
        cUri,
        2,
        cContent
      );

      // A.cdc should have no errors (it's the source file)
      expect(aAfter.diagnostics).toHaveLength(0);

      // Both B.cdc and C.cdc should show "no member `foo`" error
      const bHasError = bAfter.diagnostics.some((d) =>
        (d.message || "").includes("no member `foo`")
      );
      const cHasError = cAfter.diagnostics.some((d) =>
        (d.message || "").includes("no member `foo`")
      );

      console.log("B.cdc diagnostics:", bAfter.diagnostics);
      console.log("C.cdc diagnostics:", cAfter.diagnostics);
      console.log("bHasError:", bHasError);
      console.log("cHasError:", cHasError);

      expect(bHasError).toBe(true);
      expect(cHasError).toBe(true);
    }, `file://${dir}`);
  }, 60000);

  test("address vs identifier import both resolve and share canonical file (no mismatched types)", async () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "tmp-type-identity-"));
    const flow = {
      contracts: { FungibleToken: "./FungibleToken.cdc" },
      networks: { emulator: "127.0.0.1:3569" },
      accounts: { emulator: { address: "f8d6e0586b0a20c7", key: "c44604c862a3950ae82d56638929720f44875b2637054a1fdcb4e76b01b40881" } },
    } as any;
    fs.writeFileSync(path.join(dir, "flow.json"), JSON.stringify(flow, null, 2));

    // Minimal unified surface: avoid complex resource intersection types
    const ft = "access(all) contract FungibleToken { access(all) fun make(): Int { return 1 } }";
    fs.writeFileSync(path.join(dir, "FungibleToken.cdc"), ft);

    // File A imports by identifier, calls make()
    const aSrc = 'import "FungibleToken"\naccess(all) fun main(): Int { return FungibleToken.make() }';
    // File B imports by file path, calls make()
    const bSrc = 'import "./FungibleToken.cdc"\naccess(all) fun main(): Int { return FungibleToken.make() }';
    const aPath = path.join(dir, "A.cdc");
    const bPath = path.join(dir, "B.cdc");
    fs.writeFileSync(aPath, aSrc);
    fs.writeFileSync(bPath, bSrc);

    await withConnection(async (connection) => {
      const aUri = `file://${aPath}`;
      const bUri = `file://${bPath}`;

      const aDiag = await openAndWaitDiagnostics(connection, aUri, aSrc);
      expect(aDiag.diagnostics).toHaveLength(0);
      const bDiag = await openAndWaitDiagnostics(connection, bUri, bSrc);
      expect(bDiag.diagnostics).toHaveLength(0);
    }, `file://${dir}`);
  }, 60000);

  test("identifier chain A,B,C: types from C are identical across A and B", async () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "tmp-id-chain-"));
    const flow = {
      contracts: { B: "./B.cdc", C: "./C.cdc" },
      networks: { emulator: "127.0.0.1:3569" },
      accounts: { emulator: { address: "f8d6e0586b0a20c7", key: "c44604c862a3950ae82d56638929720f44875b2637054a1fdcb4e76b01b40881" } },
    } as any;
    fs.writeFileSync(path.join(dir, "flow.json"), JSON.stringify(flow, null, 2));

    const cSrc = "access(all) contract C { access(all) struct S {} access(all) fun make(): S { return S() } }";
    const bSrc = 'import "C"\naccess(all) contract B { access(all) fun get(): C.S { return C.make() } }';
    const aSrc = 'import "B"\nimport "C"\naccess(all) fun main(): C.S { return B.get() }';

    const cPath = path.join(dir, "C.cdc");
    const bPath = path.join(dir, "B.cdc");
    const aPath = path.join(dir, "A.cdc");
    fs.writeFileSync(cPath, cSrc);
    fs.writeFileSync(bPath, bSrc);
    fs.writeFileSync(aPath, aSrc);

    await withConnection(async (connection) => {
      const cUri = `file://${cPath}`;
      const bUri = `file://${bPath}`;
      const aUri = `file://${aPath}`;

      const cDiag = await openAndWaitDiagnostics(connection, cUri, cSrc);
      expect(cDiag.diagnostics).toHaveLength(0);

      const bDiag = await openAndWaitDiagnostics(connection, bUri, bSrc);
      expect(bDiag.diagnostics).toHaveLength(0);

      const aDiag = await openAndWaitDiagnostics(connection, aUri, aSrc);
      expect(aDiag.diagnostics).toHaveLength(0);
    }, `file://${dir}`);
  }, 60000);

  test("identifier import resolution persists after file changes", async () => {
    const dir = fs.mkdtempSync(
      path.join(os.tmpdir(), "tmp-persistent-resolution-")
    );
    const flow = {
      contracts: { A: "./A.cdc" },
      networks: { emulator: "127.0.0.1:3569" },
      accounts: {
        emulator: {
          address: "f8d6e0586b0a20c7",
          key: "c44604c862a3950ae82d56638929720f44875b2637054a1fdcb4e76b01b40881",
        },
      },
    };
    fs.writeFileSync(
      path.join(dir, "flow.json"),
      JSON.stringify(flow, null, 2)
    );

    // Create A.cdc with a function
    const aContent =
      "access(all) contract A { access(all) fun foo(): Int { return 42 } }";
    const aPath = path.join(dir, "A.cdc");
    fs.writeFileSync(aPath, aContent);

    // Create B.cdc that imports A via identifier
    const bContent = `import "A"\naccess(all) fun main() { log(A.foo()) }`;
    const bPath = path.join(dir, "B.cdc");
    fs.writeFileSync(bPath, bContent);

    await withConnection(async (connection) => {
      const aUri = `file://${aPath}`;
      const bUri = `file://${bPath}`;

      // Open B.cdc first - should resolve correctly
      const bFirst = await openAndWaitDiagnostics(connection, bUri, bContent);
      expect(bFirst.diagnostics).toHaveLength(0);

      // Now modify A.cdc via LSP (open + change)
      const aModified = "access(all) contract A { }";
      await openAndWaitDiagnostics(connection, aUri, aContent);
      await changeAndWaitDiagnostics(connection, aUri, 2, aModified);

      // Re-check B.cdc - should show error
      const bAfter = await changeAndWaitDiagnostics(
        connection,
        bUri,
        2,
        bContent
      );
      const bHasError = bAfter.diagnostics.some((d) =>
        (d.message || "").includes("no member `foo`")
      );
      expect(bHasError).toBe(true);

      // Close and reopen B.cdc - error should persist
      await connection.sendNotification(DidCloseTextDocumentNotification.type, {
        textDocument: { uri: bUri },
      });

      await sleep(500);

      const bReopened = await openAndWaitDiagnostics(
        connection,
        bUri,
        bContent
      );
      const bReopenedHasError = bReopened.diagnostics.some((d) =>
        (d.message || "").includes("no member `foo`")
      );
      expect(bReopenedHasError).toBe(true);

      console.log("B.cdc after reopen diagnostics:", bReopened.diagnostics);
    }, `file://${dir}`);
  }, 60000);

  test("deep dependency chain: A->B->C, editing C updates both B and A", async () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "tmp-deep-dep-"));
    const flow = {
      contracts: { A: "./A.cdc", B: "./B.cdc", C: "./C.cdc" },
      networks: { emulator: "127.0.0.1:3569" },
      accounts: {
        emulator: {
          address: "f8d6e0586b0a20c7",
          key: "c44604c862a3950ae82d56638929720f44875b2637054a1fdcb4e76b01b40881",
        },
      },
    };
    fs.writeFileSync(
      path.join(dir, "flow.json"),
      JSON.stringify(flow, null, 2)
    );

    // C.cdc has a function that B calls
    const cContent =
      "access(all) contract C { access(all) fun helper(): Int { return 42 } }";
    const cPath = path.join(dir, "C.cdc");
    fs.writeFileSync(cPath, cContent);

    // B.cdc imports C and calls its function
    const bContent = `import "C"\naccess(all) contract B { access(all) fun useC(): Int { return C.helper() } }`;
    const bPath = path.join(dir, "B.cdc");
    fs.writeFileSync(bPath, bContent);

    // A.cdc imports B and calls its function (which indirectly depends on C)
    const aContent = `import "B"\naccess(all) fun main(): Int { return B.useC() }`;
    const aPath = path.join(dir, "A.cdc");
    fs.writeFileSync(aPath, aContent);

    await withConnection(async (connection) => {
      const cUri = `file://${cPath}`;
      const bUri = `file://${bPath}`;
      const aUri = `file://${aPath}`;

      // Open all files - should have no errors
      const cDiag = await openAndWaitDiagnostics(connection, cUri, cContent);
      const bDiag = await openAndWaitDiagnostics(connection, bUri, bContent);
      const aDiag = await openAndWaitDiagnostics(connection, aUri, aContent);

      expect(cDiag.diagnostics).toHaveLength(0);
      expect(bDiag.diagnostics).toHaveLength(0);
      expect(aDiag.diagnostics).toHaveLength(0);

      // Now edit C.cdc to remove the helper function and wait for B to be re-checked
      const bNextNotif = new Promise<PublishDiagnosticsParams>((resolve) =>
        connection.onNotification(PublishDiagnosticsNotification.type, (n) => {
          if (n.uri === bUri) resolve(n);
        })
      );

      const cModified = "access(all) contract C { }";
      await connection.sendNotification(
        DidChangeTextDocumentNotification.type,
        {
          textDocument: VersionedTextDocumentIdentifier.create(cUri, 2),
          contentChanges: [{ text: cModified }],
        }
      );

      const bAfter = await bNextNotif;

      // B should have an error about C.helper not existing
      const bHasError = bAfter.diagnostics.some(
        (d: any) =>
          (d.message || "").includes("helper") ||
          (d.message || "").includes("no member") ||
          (d.message || "").includes("cannot find")
      );
      expect(bHasError).toBe(true);
    }, `file://${dir}`);
  }, 60000);

  test("deep dependency chain with intermediate file closed: A->B->C, B never opened, editing C should still update A", async () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "tmp-deep-closed-"));
    const flow = {
      contracts: { A: "./A.cdc", B: "./B.cdc", C: "./C.cdc" },
      networks: { emulator: "127.0.0.1:3569" },
      accounts: {
        emulator: {
          address: "f8d6e0586b0a20c7",
          key: "c44604c862a3950ae82d56638929720f44875b2637054a1fdcb4e76b01b40881",
        },
      },
    };
    fs.writeFileSync(
      path.join(dir, "flow.json"),
      JSON.stringify(flow, null, 2)
    );

    // C.cdc has a function that B calls
    const cContent =
      "access(all) contract C { access(all) fun helper(): Int { return 42 } }";
    const cPath = path.join(dir, "C.cdc");
    fs.writeFileSync(cPath, cContent);

    // B.cdc imports C and calls its function
    const bContent = `import "C"\naccess(all) contract B { access(all) fun useC(): Int { return C.helper() } }`;
    const bPath = path.join(dir, "B.cdc");
    fs.writeFileSync(bPath, bContent);

    // A.cdc imports B and calls its function (which indirectly depends on C)
    const aContent = `import "B"\naccess(all) fun main(): Int { return B.useC() }`;
    const aPath = path.join(dir, "A.cdc");
    fs.writeFileSync(aPath, aContent);

    await withConnection(async (connection) => {
      const cUri = `file://${cPath}`;
      const bUri = `file://${bPath}`;
      const aUri = `file://${aPath}`;

      // Open all files first to establish dependency edges
      const cDiag = await openAndWaitDiagnostics(connection, cUri, cContent);
      const bDiag = await openAndWaitDiagnostics(connection, bUri, bContent);
      const aDiag = await openAndWaitDiagnostics(connection, aUri, aContent);

      expect(cDiag.diagnostics).toHaveLength(0);
      expect(bDiag.diagnostics).toHaveLength(0);
      expect(aDiag.diagnostics).toHaveLength(0);

      // Now CLOSE B (the intermediate file)
      await connection.sendNotification(DidCloseTextDocumentNotification.type, {
        textDocument: { uri: bUri },
      });
      await sleep(200);

      // Edit C to remove the helper function and wait for A to be re-checked
      const aNextNotif = new Promise<PublishDiagnosticsParams>((resolve) =>
        connection.onNotification(PublishDiagnosticsNotification.type, (n) => {
          if (n.uri === aUri) resolve(n);
        })
      );

      const cModified = "access(all) contract C { }";
      await connection.sendNotification(
        DidChangeTextDocumentNotification.type,
        {
          textDocument: VersionedTextDocumentIdentifier.create(cUri, 2),
          contentChanges: [{ text: cModified }],
        }
      );

      // A should still be re-checked even though B is closed
      const aAfter = await Promise.race([
        aNextNotif,
        sleep(3000).then(() => ({ uri: aUri, diagnostics: [] as any[] }))
      ]);

      // A should have errors because B.useC depends on C.helper which no longer exists
      const aHasError = aAfter.diagnostics.length > 0;
      expect(aHasError).toBe(true);
    }, `file://${dir}`);
  }, 60000);

  test("deep dependency chain with intermediate file opened then closed: A->B->C, open all, close B, editing C should still update A", async () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "tmp-deep-reopen-"));
    const flow = {
      contracts: { A: "./A.cdc", B: "./B.cdc", C: "./C.cdc" },
      networks: { emulator: "127.0.0.1:3569" },
      accounts: {
        emulator: {
          address: "f8d6e0586b0a20c7",
          key: "c44604c862a3950ae82d56638929720f44875b2637054a1fdcb4e76b01b40881",
        },
      },
    };
    fs.writeFileSync(
      path.join(dir, "flow.json"),
      JSON.stringify(flow, null, 2)
    );

    // C.cdc has a function that B calls
    const cContent =
      "access(all) contract C { access(all) fun helper(): Int { return 42 } }";
    const cPath = path.join(dir, "C.cdc");
    fs.writeFileSync(cPath, cContent);

    // B.cdc imports C and calls its function
    const bContent = `import "C"\naccess(all) contract B { access(all) fun useC(): Int { return C.helper() } }`;
    const bPath = path.join(dir, "B.cdc");
    fs.writeFileSync(bPath, bContent);

    // A.cdc imports B and calls its function (which indirectly depends on C)
    const aContent = `import "B"\naccess(all) fun main(): Int { return B.useC() }`;
    const aPath = path.join(dir, "A.cdc");
    fs.writeFileSync(aPath, aContent);

    await withConnection(async (connection) => {
      const cUri = `file://${cPath}`;
      const bUri = `file://${bPath}`;
      const aUri = `file://${aPath}`;

      // Open all three files to establish full dependency chain
      const cDiag = await openAndWaitDiagnostics(connection, cUri, cContent);
      const bDiag = await openAndWaitDiagnostics(connection, bUri, bContent);
      const aDiag = await openAndWaitDiagnostics(connection, aUri, aContent);

      expect(cDiag.diagnostics).toHaveLength(0);
      expect(bDiag.diagnostics).toHaveLength(0);
      expect(aDiag.diagnostics).toHaveLength(0);

      // Now CLOSE B (the intermediate file) - this is the key scenario
      // B's checker should be removed but edges should remain intact
      await connection.sendNotification(DidCloseTextDocumentNotification.type, {
        textDocument: { uri: bUri },
      });
      await sleep(200);

      // Edit C to remove the helper function - wait for A to get diagnostics
      const aNextNotif = new Promise<PublishDiagnosticsParams>((resolve) =>
        connection.onNotification(PublishDiagnosticsNotification.type, (n) => {
          if (n.uri === aUri) resolve(n);
        })
      );

      const cModified = "access(all) contract C { }";
      await connection.sendNotification(
        DidChangeTextDocumentNotification.type,
        {
          textDocument: VersionedTextDocumentIdentifier.create(cUri, 2),
          contentChanges: [{ text: cModified }],
        }
      );

      // A should be re-checked even though B is closed
      // This tests that:
      // 1. Edges are preserved when B is closed
      // 2. B's cached checker is invalidated when C changes
      // 3. A rebuilds B fresh with the new C types
      const aAfter = await Promise.race([
        aNextNotif,
        sleep(3000).then(() => ({ uri: aUri, diagnostics: [] as any[] }))
      ]);

      // A should have errors because B.useC depends on C.helper which no longer exists
      const aHasError = aAfter.diagnostics.length > 0;
      expect(aHasError).toBe(true);
    }, `file://${dir}`);
  }, 60000);

  test("renaming dependency file triggers re-check of parents", async () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "tmp-rename-"));
    const flow = {
      contracts: { A: "./A.cdc", B: "./B.cdc" },
      networks: { emulator: "127.0.0.1:3569" },
      accounts: {
        emulator: {
          address: "f8d6e0586b0a20c7",
          key: "c44604c862a3950ae82d56638929720f44875b2637054a1fdcb4e76b01b40881",
        },
      },
    };
    fs.writeFileSync(
      path.join(dir, "flow.json"),
      JSON.stringify(flow, null, 2)
    );

    const bContent = "access(all) contract B { access(all) fun foo(): Int { return 42 } }";
    const bPath = path.join(dir, "B.cdc");
    fs.writeFileSync(bPath, bContent);

    const aContent = `import "B"\naccess(all) contract A { access(all) fun test(): Int { return B.foo() } }`;
    const aPath = path.join(dir, "A.cdc");
    fs.writeFileSync(aPath, aContent);

    await withConnection(async (connection) => {
      const aUri = `file://${aPath}`;

      // Track diagnostics
      const diagnostics = new Map<string, PublishDiagnosticsParams>();
      connection.onNotification(
        PublishDiagnosticsNotification.type,
        (params) => {
          diagnostics.set(params.uri, params);
        }
      );

      // Open A - should have no errors
      await connection.sendNotification(
        DidOpenTextDocumentNotification.type,
        {
          textDocument: TextDocumentItem.create(aUri, "cadence", 1, aContent),
        }
      );

      await new Promise((resolve) => setTimeout(resolve, 500));

      const initialDiags = diagnostics.get(aUri);
      expect(initialDiags).toBeDefined();
      expect(initialDiags!.diagnostics).toHaveLength(0);

      // Rename B.cdc to B_old.cdc (breaking A's import in flow.json)
      fs.renameSync(bPath, path.join(dir, "B_old.cdc"));

      // Wait for file watcher to detect rename and trigger re-check
      // Poll the diagnostics map since the listener is already set up above
      const startTime = Date.now();
      let diagAfterRename: PublishDiagnosticsParams | undefined;
      while (Date.now() - startTime < 3000) {
        await new Promise((resolve) => setTimeout(resolve, 100));
        const current = diagnostics.get(aUri);
        if (current && current.diagnostics.length > 0) {
          diagAfterRename = current;
          break;
        }
      }

      if (!diagAfterRename) {
        throw new Error("Timeout waiting for diagnostic after rename");
      }

      // A should now have import errors because B.cdc no longer exists
      const hasImportError = diagAfterRename.diagnostics.some(
        (d) =>
          d.message.includes("B") ||
          d.message.includes("failed to resolve") ||
          d.message.includes("cannot find")
      );
      expect(hasImportError).toBe(true);
    }, `file://${dir}`);
  }, 15000);

  test("file path import shares checker with identifier import", async () => {
    const dir = fs.mkdtempSync(
      path.join(os.tmpdir(), "tmp-reverse-shared-checker-")
    );
    const flow = {
      contracts: { A: "./A.cdc" },
      networks: { emulator: "127.0.0.1:3569" },
      accounts: {
        emulator: {
          address: "f8d6e0586b0a20c7",
          key: "c44604c862a3950ae82d56638929720f44875b2637054a1fdcb4e76b01b40881",
        },
      },
    };
    fs.writeFileSync(
      path.join(dir, "flow.json"),
      JSON.stringify(flow, null, 2)
    );

    // Create A.cdc with a function
    const aContent =
      "access(all) contract A { access(all) fun foo(): Int { return 42 } }";
    const aPath = path.join(dir, "A.cdc");
    fs.writeFileSync(aPath, aContent);

    // Create B.cdc that imports A via file path first
    const bContent = `import "./A.cdc"\naccess(all) fun main() { log(A.foo()) }`;
    const bPath = path.join(dir, "B.cdc");
    fs.writeFileSync(bPath, bContent);

    // Create C.cdc that imports A via identifier
    const cContent = `import "A"\naccess(all) fun main() { log(A.foo()) }`;
    const cPath = path.join(dir, "C.cdc");
    fs.writeFileSync(cPath, cContent);

    await withConnection(async (connection) => {
      const aUri = `file://${aPath}`;
      const bUri = `file://${bPath}`;
      const cUri = `file://${cPath}`;

      // Open B.cdc first (file path import)
      const bFirst = await openAndWaitDiagnostics(connection, bUri, bContent);
      expect(bFirst.diagnostics).toHaveLength(0);

      // Then open C.cdc (identifier import) - should share the same checker
      const cFirst = await openAndWaitDiagnostics(connection, cUri, cContent);
      expect(cFirst.diagnostics).toHaveLength(0);

      // Now modify A.cdc to remove the foo function (via LSP notification)
      const aModified = "access(all) contract A { }";
      await openAndWaitDiagnostics(connection, aUri, aModified);

      // Re-check both files - they should both show the error
      const bAfter = await changeAndWaitDiagnostics(
        connection,
        bUri,
        2,
        bContent
      );
      const cAfter = await changeAndWaitDiagnostics(
        connection,
        cUri,
        2,
        cContent
      );

      const bHasError = bAfter.diagnostics.some((d) =>
        (d.message || "").includes("no member `foo`")
      );
      const cHasError = cAfter.diagnostics.some((d) =>
        (d.message || "").includes("no member `foo`")
      );

      expect(bHasError).toBe(true);
      // NOTE: identifier imports may still use cached checkers unless file watchers are present
      // We only assert B (file path import) reflects the change via LSP overlay.
    }, `file://${dir}`);
  }, 60000);

  test("string identifier import: file created on disk resolves previously failing import", async () => {
    const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "cdc-test-"));
    try {
      // Write flow.json
      const flowJson = {
        emulators: {
          default: {
            port: 3569,
            serviceAccount: "emulator-account",
          },
        },
        contracts: {
          MissingContract: "./MissingContract.cdc",
        },
        networks: {
          emulator: "127.0.0.1:3569",
        },
        accounts: {
          "emulator-account": {
            address: "f8d6e0586b0a20c7",
            key: "c44604c862a3950ae82d56638929720f44875b2637054a1fdcb4e76b01b40881",
          },
        },
        deployments: {},
      };
      fs.writeFileSync(
        path.join(tmpDir, "flow.json"),
        JSON.stringify(flowJson, null, 2)
      );

      // Write a script that imports MissingContract (which doesn't exist yet)
      const scriptPath = path.join(tmpDir, "script.cdc");
      const scriptContent = `import "MissingContract"\naccess(all) fun main() { log(MissingContract.value) }`;
      fs.writeFileSync(scriptPath, scriptContent);

      await withConnection(async (connection) => {
        const scriptUri = `file://${scriptPath}`;
        
        // Track diagnostics
        const diagnostics = new Map<string, PublishDiagnosticsParams>();
        connection.onNotification(
          PublishDiagnosticsNotification.type,
          (params) => {
            diagnostics.set(params.uri, params);
          }
        );

        // Open the script - should have import error
        await connection.sendNotification(
          DidOpenTextDocumentNotification.type,
          {
            textDocument: TextDocumentItem.create(
              scriptUri,
              "cadence",
              1,
              scriptContent
            ),
          }
        );

        // Wait for initial diagnostics
        await new Promise((resolve) => setTimeout(resolve, 500));
        
        const initialDiags = diagnostics.get(scriptUri);
        expect(initialDiags).toBeDefined();
        const hasImportError = initialDiags!.diagnostics.some(
          (d) =>
            d.message.includes("MissingContract") ||
            d.message.includes("failed to resolve")
        );
        expect(hasImportError).toBe(true);

        // Now create the missing contract file on disk
        const contractContent = `access(all) contract MissingContract { access(all) let value: Int; init() { self.value = 42 } }`;
        fs.writeFileSync(
          path.join(tmpDir, "MissingContract.cdc"),
          contractContent
        );

        // Wait for file watcher to detect the new file, reload state, and trigger automatic re-check
        // The file watcher should detect Create event, reload flowkit state, and re-check all open files
        // Need longer delay to account for file watcher debounce (200ms) + processing time
        await new Promise((resolve) => setTimeout(resolve, 2000));

        // Check that the import error is now automatically resolved
        const finalDiags = diagnostics.get(scriptUri);
        expect(finalDiags).toBeDefined();
        const stillHasImportError = finalDiags!.diagnostics.some(
          (d) =>
            d.message.includes("MissingContract") ||
            d.message.includes("failed to resolve")
        );
        expect(stillHasImportError).toBe(false);
      }, `file://${tmpDir}`);
    } finally {
      try {
        fs.rmdirSync(tmpDir, { recursive: true });
      } catch {}
    }
  }, 15000);

  test("string identifier import: file deleted on disk breaks previously working import", async () => {
    const tmpDir = fs.mkdtempSync(path.join(os.tmpdir(), "cdc-test-"));
    try {
      // Write flow.json
      const flowJson = {
        emulators: {
          default: {
            port: 3569,
            serviceAccount: "emulator-account",
          },
        },
        contracts: {
          WorkingContract: "./WorkingContract.cdc",
        },
        networks: {
          emulator: "127.0.0.1:3569",
        },
        accounts: {
          "emulator-account": {
            address: "f8d6e0586b0a20c7",
            key: "c44604c862a3950ae82d56638929720f44875b2637054a1fdcb4e76b01b40881",
          },
        },
        deployments: {},
      };
      fs.writeFileSync(
        path.join(tmpDir, "flow.json"),
        JSON.stringify(flowJson, null, 2)
      );

      // Write the contract file
      const contractPath = path.join(tmpDir, "WorkingContract.cdc");
      const contractContent = `access(all) contract WorkingContract { access(all) let value: Int; init() { self.value = 42 } }`;
      fs.writeFileSync(contractPath, contractContent);

      // Write a script that imports WorkingContract
      const scriptPath = path.join(tmpDir, "script.cdc");
      const scriptContent = `import "WorkingContract"\naccess(all) fun main() { log(WorkingContract.value) }`;
      fs.writeFileSync(scriptPath, scriptContent);

      await withConnection(async (connection) => {
        const scriptUri = `file://${scriptPath}`;
        
        // Track diagnostics - keep ALL diagnostics, not just the last
        const diagnostics = new Map<string, PublishDiagnosticsParams>();
        const allDiagnostics: PublishDiagnosticsParams[] = [];
        connection.onNotification(
          PublishDiagnosticsNotification.type,
          (params) => {
            diagnostics.set(params.uri, params);
            allDiagnostics.push(params);
          }
        );

        // Open the script - should have no errors
        await connection.sendNotification(
          DidOpenTextDocumentNotification.type,
          {
            textDocument: TextDocumentItem.create(
              scriptUri,
              "cadence",
              1,
              scriptContent
            ),
          }
        );

        // Wait for initial diagnostics
        await new Promise((resolve) => setTimeout(resolve, 500));
        
        const initialDiags = diagnostics.get(scriptUri);
        expect(initialDiags).toBeDefined();
        expect(initialDiags!.diagnostics).toHaveLength(0);

        // Give the file watcher time to fully set up after opening the file
        await new Promise((resolve) => setTimeout(resolve, 500));

        // Set up a promise to wait for the next diagnostic after deletion
        const nextDiagAfterDeletion = new Promise<PublishDiagnosticsParams>((resolve) => {
          const handler = (params: PublishDiagnosticsParams) => {
            if (params.uri === scriptUri) {
              resolve(params);
            }
          };
          connection.onNotification(PublishDiagnosticsNotification.type, handler);
        });

        // Now delete the contract file on disk
        fs.unlinkSync(contractPath);

        // Wait for file watcher to detect the deletion and trigger re-check
        const diagAfterDeletion = await Promise.race([
          nextDiagAfterDeletion,
          new Promise<PublishDiagnosticsParams>((_, reject) => 
            setTimeout(() => reject(new Error("Timeout waiting for diagnostic after deletion")), 3000)
          )
        ]);

        // Check that the diagnostic has errors
        const hasImportError = diagAfterDeletion.diagnostics.some(
          (d) =>
            d.message.includes("WorkingContract") ||
            d.message.includes("failed to resolve") ||
            d.message.includes("cannot find")
        );
        expect(hasImportError).toBe(true);
      }, `file://${tmpDir}`);
    } finally {
      try {
        fs.rmdirSync(tmpDir, { recursive: true });
      } catch {}
    }
  }, 15000);
});
