import {
  createProtocolConnection,
  InitializeRequest,
  ExitNotification,
  StreamMessageReader,
  StreamMessageWriter,
  DidOpenTextDocumentNotification,
  TextDocumentItem,
} from "vscode-languageserver-protocol";

import { execSync, spawn } from "child_process";
const FLOW_JSON = "flow.json";

import * as path from "path";
import * as fs from "fs";
import * as os from "os";

beforeAll(() => {
  execSync("go build ../cmd/languageserver", {
    cwd: __dirname,
  });
});

async function withConnection(f: (connection: any) => Promise<void>) {
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

// Helper that allows passing initialize params (e.g., custom rootUri)
async function withConnectionParams(
  initParams: any,
  f: (connection: any) => Promise<void>
) {
  const child = spawn(path.resolve(__dirname, "./languageserver"), [
    "--enable-flow-client=false",
  ]);

  const connection = createProtocolConnection(
    new StreamMessageReader(child.stdout),
    new StreamMessageWriter(child.stdin),
    null
  );
  connection.listen();

  await connection.sendRequest(InitializeRequest.type, initParams);

  try {
    await f(connection);
  } finally {
    await connection.sendNotification(ExitNotification.type);
  }
}

test("prompts and creates flow.json on accept", async () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "tmp-"));
  const scriptPath = path.join(dir, "script.cdc");
  const uri = `file://${scriptPath}`;
  fs.writeFileSync(scriptPath, 'import "Foo"\naccess(all) fun main() {}');

  await withConnection(async (connection) => {
    connection.onRequest("window/showMessageRequest", async () => {
      return { title: "Create flow.json" };
    });

    await connection.sendNotification(DidOpenTextDocumentNotification.type, {
      textDocument: TextDocumentItem.create(
        uri,
        "cadence",
        1,
        fs.readFileSync(scriptPath, "utf8")
      ),
    });

    // Wait up to 10s for flow.json to appear
    const target = path.join(dir, FLOW_JSON);
    const deadline = Date.now() + 10000;
    while (!fs.existsSync(target) && Date.now() < deadline) {
      await new Promise((r) => setTimeout(r, 150));
    }
    expect(fs.existsSync(target)).toBeTruthy();
  });

  try {
    fs.rmdirSync(dir, { recursive: true });
  } catch {}
}, 20000);

// Workspace-root init behavior (merged from workspace_init.test.ts)
test("creates flow.json at workspace folder root when file is nested", async () => {
  const root = fs.mkdtempSync(path.join(os.tmpdir(), "wsroot-"));
  const nested = path.join(root, "src", "contracts");
  fs.mkdirSync(nested, { recursive: true });
  const scriptPath = path.join(nested, "script.cdc");
  const uri = `file://${scriptPath}`;
  fs.writeFileSync(scriptPath, 'import "Foo"\naccess(all) fun main() {}');

  const initParams = {
    capabilities: {},
    processId: process.pid,
    rootUri: `file://${root}`,
    workspaceFolders: null,
    initializationOptions: null,
  };

  await withConnectionParams(initParams, async (connection) => {
    connection.onRequest("window/showMessageRequest", async () => {
      return { title: "Create flow.json" };
    });

    await connection.sendNotification(DidOpenTextDocumentNotification.type, {
      textDocument: TextDocumentItem.create(
        uri,
        "cadence",
        1,
        fs.readFileSync(scriptPath, "utf8")
      ),
    });

    const targetAtRoot = path.join(root, FLOW_JSON);
    const deadline = Date.now() + 10000;
    while (!fs.existsSync(targetAtRoot) && Date.now() < deadline) {
      await new Promise((r) => setTimeout(r, 150));
    }
    expect(fs.existsSync(targetAtRoot)).toBeTruthy();
    expect(fs.existsSync(path.join(nested, "flow.json"))).toBeFalsy();
  });

  try {
    fs.rmdirSync(root, { recursive: true });
  } catch {}
}, 20000);

test("fallback: creates in file directory when not under any workspace folder", async () => {
  const wsRoot = fs.mkdtempSync(path.join(os.tmpdir(), "wsX-"));
  const otherRoot = fs.mkdtempSync(path.join(os.tmpdir(), "other-"));
  const scriptPath = path.join(otherRoot, "script.cdc");
  const uri = `file://${scriptPath}`;
  fs.writeFileSync(scriptPath, 'import "Foo"\naccess(all) fun main() {}');

  const initParams = {
    capabilities: {},
    processId: process.pid,
    rootUri: `file://${wsRoot}`,
    workspaceFolders: null,
    initializationOptions: null,
  };

  await withConnectionParams(initParams, async (connection) => {
    connection.onRequest("window/showMessageRequest", async () => {
      return { title: "Create flow.json" };
    });

    await connection.sendNotification(DidOpenTextDocumentNotification.type, {
      textDocument: TextDocumentItem.create(
        uri,
        "cadence",
        1,
        fs.readFileSync(scriptPath, "utf8")
      ),
    });

    const targetInFileDir = path.join(otherRoot, FLOW_JSON);
    const deadline = Date.now() + 10000;
    while (!fs.existsSync(targetInFileDir) && Date.now() < deadline) {
      await new Promise((r) => setTimeout(r, 150));
    }
    expect(fs.existsSync(targetInFileDir)).toBeTruthy();
    expect(fs.existsSync(path.join(wsRoot, "flow.json"))).toBeFalsy();
  });

  try {
    fs.rmdirSync(wsRoot, { recursive: true });
  } catch {}
  try {
    fs.rmdirSync(otherRoot, { recursive: true });
  } catch {}
}, 20000);

test("prompts and does not create flow.json on ignore", async () => {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), "tmp-"));
  const scriptPath = path.join(dir, "script.cdc");
  const uri = `file://${scriptPath}`;
  fs.writeFileSync(scriptPath, 'import "Foo"\naccess(all) fun main() {}');

  await withConnection(async (connection) => {
    connection.onRequest("window/showMessageRequest", async () => {
      return { title: "Ignore" };
    });

    await connection.sendNotification(DidOpenTextDocumentNotification.type, {
      textDocument: TextDocumentItem.create(
        uri,
        "cadence",
        1,
        fs.readFileSync(scriptPath, "utf8")
      ),
    });

    // Give time in case any background action would run
    await new Promise((r) => setTimeout(r, 400));
    expect(fs.existsSync(path.join(dir, FLOW_JSON))).toBeFalsy();
  });

  try {
    fs.rmdirSync(dir, { recursive: true });
  } catch {}
}, 20000);
