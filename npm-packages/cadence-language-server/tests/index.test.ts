import { CadenceLanguageServer } from "../src";
import * as fs from "fs";
import {
  createProtocolConnection,
  DidOpenTextDocumentNotification,
  InitializeRequest,
  ProtocolConnection,
  PublishDiagnosticsNotification,
  PublishDiagnosticsParams,
  DataCallback,
  Disposable,
  Logger,
  Message,
  MessageReader,
  MessageWriter,
  PartialMessageInfo,
  TextDocumentItem,
  ExitNotification,
} from "vscode-languageserver-protocol";
import { Callbacks } from "../dist";

const binary = fs.readFileSync(
  require.resolve("../dist/cadence-language-server.wasm")
);
async function withConnection(
  callbacks: Callbacks = {},
  callback: (connection: ProtocolConnection) => Promise<void>
) {
  // Start the language server
  await CadenceLanguageServer.create(binary, callbacks);

  const logger: Logger = {
    error(message: string) {
      console.error(message);
    },
    warn(message: string) {
      console.warn(message);
    },
    info(message: string) {
      console.info(message);
    },
    log(message: string) {
      console.log(message);
    },
  };

  const writer: MessageWriter = {
    onClose(_: (_: void) => void): Disposable {
      return Disposable.create(() => {});
    },
    onError(_: (error: [Error, Message, number]) => void): Disposable {
      return Disposable.create(() => {});
    },
    async write(msg: Message): Promise<void> {
      callbacks.toServer?.(null, msg);
    },
    end() {},
    dispose() {
      callbacks.onClientClose?.();
    },
  };

  const reader: MessageReader = {
    onError(_: (error: Error) => void): Disposable {
      return Disposable.create(() => {});
    },
    onClose(_: (_: void) => void): Disposable {
      return Disposable.create(() => {});
    },
    onPartialMessage(_: (m: PartialMessageInfo) => void): Disposable {
      return Disposable.create(() => {});
    },
    listen(dataCallback: DataCallback): Disposable {
      callbacks.toClient = (message) => dataCallback(message);
      return Disposable.create(() => {});
    },
    dispose() {
      callbacks.onClientClose?.();
    },
  };

  const connection = createProtocolConnection(reader, writer, logger);
  connection.listen();

  await connection.sendRequest(InitializeRequest.type, {
    capabilities: {},
    processId: process.pid,
    rootUri: "/",
    workspaceFolders: null,
    initializationOptions: {},
  });

  try {
    await callback(connection);
  } finally {
    try {
      connection.dispose();
    } catch {}
  }
}

async function createTestDocument(
  connection: ProtocolConnection,
  code: string
): Promise<string> {
  const uri = "file:///test.cdc";

  await connection.sendNotification(DidOpenTextDocumentNotification.type, {
    textDocument: TextDocumentItem.create(uri, "cadence", 1, code),
  });

  return uri;
}

describe("import", () => {
  test("string import", async () => {
    await withConnection(
      {
        getStringCode(location: string) {
          if (location === "Test") {
            return "access(all) contract Test {}";
          }
          return undefined;
        },
      },
      async (connection) => {
        const uri = await createTestDocument(connection, 'import "Test"');

        const notificationPromise = new Promise<PublishDiagnosticsParams>(
          (resolve) => {
            connection.onNotification(
              PublishDiagnosticsNotification.type,
              resolve
            );
          }
        );

        const notification = await notificationPromise;

        expect(notification.uri).toEqual(uri);
        expect(notification.diagnostics).toEqual([]);
      }
    );
  });

  test("string import not found", async () => {
    await withConnection(
      {
        getStringCode(_: string) {
          return undefined;
        },
      },
      async (connection) => {
        const uri = await createTestDocument(connection, 'import "Test"');

        const notificationPromise = new Promise<PublishDiagnosticsParams>(
          (resolve) => {
            connection.onNotification(
              PublishDiagnosticsNotification.type,
              resolve
            );
          }
        );

        const notification = await notificationPromise;

        expect(notification.uri).toEqual(uri);
        expect(notification.diagnostics).toEqual([
          {
            severity: 1,
            range: {
              start: {
                line: 0,
                character: 7,
              },
              end: {
                line: 0,
                character: 8,
              },
            },
            message: "checking of imported program `Test` failed",
          },
        ]);
      }
    );
  });

  test("address import", async () => {
    await withConnection(
      {
        getAddressCode(address: string) {
          if (address === "0000000000000001.Test") {
            return "access(all) contract Test {}";
          }
          return undefined;
        },
      },
      async (connection) => {
        const uri = await createTestDocument(
          connection,
          "import Test from 0x01"
        );

        const notificationPromise = new Promise<PublishDiagnosticsParams>(
          (resolve) => {
            connection.onNotification(
              PublishDiagnosticsNotification.type,
              resolve
            );
          }
        );

        const notification = await notificationPromise;

        expect(notification.uri).toEqual(uri);
        expect(notification.diagnostics).toEqual([]);
      }
    );
  });

  test("address import not found", async () => {
    await withConnection(
      {
        getAddressCode(_: string) {
          return undefined;
        },
      },
      async (connection) => {
        const uri = await createTestDocument(
          connection,
          "import Test from 0x01"
        );

        const notificationPromise = new Promise<PublishDiagnosticsParams>(
          (resolve) => {
            connection.onNotification(
              PublishDiagnosticsNotification.type,
              resolve
            );
          }
        );

        const notification = await notificationPromise;

        expect(notification.uri).toEqual(uri);
        expect(notification.diagnostics).toEqual([
          {
            severity: 1,
            range: {
              start: {
                line: 0,
                character: 17,
              },
              end: {
                line: 0,
                character: 18,
              },
            },
            message:
              "checking of imported program `0000000000000001.Test` failed",
          },
        ]);
      }
    );
  });

  test("Crypto import", async () => {
    await withConnection({}, async (connection) => {
      const uri = await createTestDocument(
        connection,
        `
        import Crypto
        access(all) contract Test {
          init() {
            log(Crypto.KeyList())
          }
        }
        `
      );

      const notificationPromise = new Promise<PublishDiagnosticsParams>(
        (resolve) => {
          connection.onNotification(
            PublishDiagnosticsNotification.type,
            resolve
          );
        }
      );

      const notification = await notificationPromise;

      expect(notification.uri).toEqual(uri);
      expect(notification.diagnostics).toEqual([]);
    });
  });
});

afterAll(() => {
  // Kill the WASM instance when the tests are done
  withConnection({}, async (connection) => {
    await connection.sendNotification(ExitNotification.type);
  });
});
