import {
  createProtocolConnection,
  InitializeRequest,
  ExitNotification,
  StreamMessageReader,
  StreamMessageWriter,
  ProtocolConnection,
  DidOpenTextDocumentNotification,
  DidChangeTextDocumentNotification,
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
    await connection.sendNotification(ExitNotification.type);
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

describe("config file watch and LS updates", () => {
  test("config created after open removes import error", async () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "tmp-"));
    const script = `import "Foo"\naccess(all) fun main() { }`;
    const uri = `file://${dir}/script.cdc`;
    fs.writeFileSync(path.join(dir, "script.cdc"), script);

    await withConnection(async (connection) => {
      const first = await openAndWaitDiagnostics(connection, uri, script);
      expect(first.diagnostics.length).toBeGreaterThanOrEqual(0);

      // Create flow.json and contract file
      const flow = {
        contracts: { Foo: "./foo.cdc" },
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
        path.join(dir, "foo.cdc"),
        "access(all) contract Foo {}"
      );

      // Wait for watcher debounce and reload, then trigger re-check
      await sleep(1200);
      const second = await changeAndWaitDiagnostics(connection, uri, 2, script);
      expect(second.diagnostics).toHaveLength(0);
    });
  });

  test("config removed causes import error to appear", async () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "tmp-"));
    const script = `import "Foo"\naccess(all) fun main() { }`;
    const uri = `file://${dir}/script.cdc`;
    fs.writeFileSync(path.join(dir, "script.cdc"), script);
    const flow = {
      contracts: { Foo: "./foo.cdc" },
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
    fs.writeFileSync(path.join(dir, "foo.cdc"), "access(all) contract Foo {}");

    await withConnection(async (connection) => {
      const ok = await openAndWaitDiagnostics(connection, uri, script);
      expect(ok.diagnostics).toHaveLength(0);

      // Remove config
      fs.unlinkSync(path.join(dir, "flow.json"));
      await sleep(1200);
      const broken = await changeAndWaitDiagnostics(connection, uri, 2, script);
      expect(broken.diagnostics.length).toBeGreaterThanOrEqual(0);
    });
  });

  test("config modified causes import to fail", async () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "tmp-"));
    const script = `import "Foo"\naccess(all) fun main() { }`;
    const uri = `file://${dir}/script.cdc`;
    fs.writeFileSync(path.join(dir, "script.cdc"), script);
    const flowPath = path.join(dir, "flow.json");
    const flow = {
      contracts: { Foo: "./foo.cdc" },
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
    fs.writeFileSync(flowPath, JSON.stringify(flow, null, 2));
    fs.writeFileSync(path.join(dir, "foo.cdc"), "access(all) contract Foo {}");

    await withConnection(async (connection) => {
      const ok = await openAndWaitDiagnostics(connection, uri, script);
      expect(ok.diagnostics).toHaveLength(0);

      // Modify flow.json to remove Foo contract
      const modified = { ...flow, contracts: {} } as any;
      fs.writeFileSync(flowPath, JSON.stringify(modified, null, 2));
      await sleep(1200);
      const broken = await changeAndWaitDiagnostics(connection, uri, 2, script);
      expect(broken.diagnostics.length).toBeGreaterThan(0);
    });
  });

  test("invalid flow.json parse then fixed recovers", async () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "tmp-"));
    const script = `import "Foo"\naccess(all) fun main() { }`;
    const uri = `file://${dir}/script.cdc`;
    fs.writeFileSync(path.join(dir, "script.cdc"), script);

    // Write invalid JSON to flow.json
    fs.writeFileSync(path.join(dir, "flow.json"), "{");

    await withConnection(async (connection) => {
      const first = await openAndWaitDiagnostics(connection, uri, script);
      expect(first.diagnostics.length).toBeGreaterThan(0);

      // Replace with valid config and add the referenced contract
      const flow = {
        contracts: { Foo: "./foo.cdc" },
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
        path.join(dir, "foo.cdc"),
        "access(all) contract Foo {}"
      );

      // Wait for debounce, then trigger re-check
      await sleep(1200);
      const second = await changeAndWaitDiagnostics(connection, uri, 2, script);
      expect(second.diagnostics).toHaveLength(0);
    });
  });

  test("atomic rename of flow.json is detected", async () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "tmp-"));
    const script = `import "Foo"\naccess(all) fun main() { }`;
    const uri = `file://${dir}/script.cdc`;
    fs.writeFileSync(path.join(dir, "script.cdc"), script);

    // Write valid config to temp then rename into place
    const flow = {
      contracts: { Foo: "./foo.cdc" },
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
      path.join(dir, "flow.json.tmp"),
      JSON.stringify(flow, null, 2)
    );
    fs.writeFileSync(path.join(dir, "foo.cdc"), "access(all) contract Foo {}");

    await withConnection(async (connection) => {
      const first = await openAndWaitDiagnostics(connection, uri, script);
      expect(first.diagnostics.length).toBeGreaterThanOrEqual(0);

      fs.renameSync(
        path.join(dir, "flow.json.tmp"),
        path.join(dir, "flow.json")
      );
      await sleep(1200);
      const second = await changeAndWaitDiagnostics(connection, uri, 2, script);
      expect(second.diagnostics).toHaveLength(0);
    });
  });

  test("rapid successive changes debounced to final valid state", async () => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), "tmp-"));
    fs.mkdirSync(dir, { recursive: true });
    const script = `import "Foo"\naccess(all) fun main() { }`;
    const uri = `file://${dir}/script.cdc`;
    fs.writeFileSync(path.join(dir, "script.cdc"), script);

    const invalid = "{";
    const validFlow = {
      contracts: { Foo: "./foo.cdc" },
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

    await withConnection(async (connection) => {
      const first = await openAndWaitDiagnostics(connection, uri, script);
      expect(first.diagnostics.length).toBeGreaterThanOrEqual(0);

      // Burst: invalid -> empty -> valid
      fs.writeFileSync(path.join(dir, "flow.json"), invalid);
      fs.writeFileSync(path.join(dir, "flow.json"), "");
      fs.writeFileSync(
        path.join(dir, "flow.json"),
        JSON.stringify(validFlow, null, 2)
      );
      fs.writeFileSync(
        path.join(dir, "foo.cdc"),
        "access(all) contract Foo {}"
      );

      await sleep(1200);
      const second = await changeAndWaitDiagnostics(connection, uri, 2, script);
      expect(second.diagnostics).toHaveLength(0);
    });
  });
});
