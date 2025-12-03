import {
  createProtocolConnection,
  InitializeRequest,
  ExitNotification,
  StreamMessageReader,
  StreamMessageWriter,
  DidOpenTextDocumentNotification,
  TextDocumentItem,
  ShowMessageNotification,
  ShowMessageParams,
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

test("shows error if init-config flow.json path is invalid or unreadable", async () => {
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), "ws-"));
  const bogusDir = path.join(tmp, "does-not-exist");
  fs.mkdirSync(bogusDir, { recursive: true });
  const bogusCfg = path.join(bogusDir, "flow.json");
  // create invalid json so file exists but is invalid
  fs.writeFileSync(bogusCfg, "{ invalid json ");
  const scriptDir = fs.mkdtempSync(path.join(tmp, "dir-"));
  const scriptPath = path.join(scriptDir, "script.cdc");
  fs.writeFileSync(scriptPath, "access(all) fun main() {}\n");

  const initParams = {
    capabilities: {},
    processId: process.pid,
    rootUri: `file://${tmp}`,
    workspaceFolders: null,
    initializationOptions: {
      configPath: bogusCfg,
      numberOfAccounts: "0",
    },
  };

  await withConnectionParams(initParams, async (connection) => {
    const msgP = new Promise<ShowMessageParams>((resolve, reject) => {
      const timer = setTimeout(
        () => reject(new Error("timeout waiting for ShowMessage")),
        15000
      );
      connection.onNotification(ShowMessageNotification.type, (p) => {
        clearTimeout(timer);
        resolve(p);
      });
    });

    await connection.sendNotification(DidOpenTextDocumentNotification.type, {
      textDocument: TextDocumentItem.create(
        `file://${scriptPath}`,
        "cadence",
        1,
        fs.readFileSync(scriptPath, "utf8")
      ),
    });

    const msg = await msgP;
    expect(msg.type).toBe(1); // Error
    // For init-config invalid JSON, the server reports "Invalid flow.json"
    expect(msg.message).toContain("Invalid flow.json");
  });

  try {
    fs.rmdirSync(tmp, { recursive: true });
  } catch {}
}, 20000);
