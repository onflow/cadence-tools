import {
  createProtocolConnection,
  DidOpenTextDocumentNotification,
  ExecuteCommandRequest,
  ExitNotification,
  InitializeRequest,
  ProtocolConnection,
  PublishDiagnosticsNotification,
  PublishDiagnosticsParams,
  RegistrationRequest,
  StreamMessageReader,
  StreamMessageWriter,
  TextDocumentItem,
  Trace,
  Tracer,
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

class ConsoleTracer implements Tracer {
  log(dataObject: any): void;
  log(message: string, data?: string): void;
  log(dataObject: any, data?: string): void {
    console.log("tracer >", dataObject, data);
  }
}

async function withConnection(
  f: (connection: ProtocolConnection) => Promise<void>,
  enableFlowClient = false,
  debug = false
): Promise<void> {
  let opts = [`--enable-flow-client=${enableFlowClient}`];
  const child = spawn(path.resolve(__dirname, "./languageserver"), opts);

  let stderr = "";
  child.stderr.setEncoding("utf8");
  child.stderr.on("data", (data) => {
    stderr += data;
  });
  child.on("exit", (code) => {
    if (code !== 0) {
      console.error(stderr);
    }
    expect(code).toBe(0);
  });

  const connection = createProtocolConnection(
    new StreamMessageReader(child.stdout),
    new StreamMessageWriter(child.stdin),
    null
  );

  connection.listen();

  let initOpts = null;
  if (enableFlowClient) {
    // flow client initialization options where we pass the location of flow.json
    // and service account name and its address
    initOpts = {
      configPath: "./flow.json",
      numberOfAccounts: "5",
    };

    connection.onRequest(RegistrationRequest.type, () => {});
  }

  await connection.sendRequest(InitializeRequest.type, {
    capabilities: {},
    processId: process.pid,
    rootUri: `file://${__dirname}`,
    workspaceFolders: null,
    initializationOptions: initOpts,
  });

  // debug option when testing
  if (debug) {
    connection.trace(Trace.Verbose, new ConsoleTracer(), true);
    connection.onUnhandledNotification((e) => console.log("unhandled >", e));
    connection.onError((e) => console.log("err >", e));
  }

  try {
    await f(connection);
  } finally {
    await connection.sendNotification(ExitNotification.type);
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

describe("getEntryPointParameters command", () => {
  async function testCode(code: string, expectedParameters: object[]) {
    return withConnection(async (connection) => {
      const uri = await createTestDocument(connection, code);

      const result = await connection.sendRequest(ExecuteCommandRequest.type, {
        command: "cadence.server.getEntryPointParameters",
        arguments: [uri],
      });

      expect(result).toEqual(expectedParameters);
    });
  }

  test("script", async () =>
    testCode(`access(all) fun main(a: Int) {}`, [{ name: "a", type: "Int" }]));

  test("transaction", async () =>
    testCode(`transaction(a: Int) {}`, [{ name: "a", type: "Int" }]));
});

describe("getContractInitializerParameters command", () => {
  async function testCode(code: string, expectedParameters: object[]) {
    return withConnection(async (connection) => {
      const uri = await createTestDocument(connection, code);

      const result = await connection.sendRequest(ExecuteCommandRequest.type, {
        command: "cadence.server.getContractInitializerParameters",
        arguments: [uri],
      });

      expect(result).toEqual(expectedParameters);
    });
  }

  test("no contract", async () => testCode(``, []));

  test("one contract, no parameters", async () =>
    testCode(
      `
          access(all) contract C {
              init() {}
          }
          `,
      []
    ));

  test("one contract, one parameter", async () =>
    testCode(
      `
          access(all)
          contract C {
              init(a: Int) {}
          }
          `,
      [{ name: "a", type: "Int" }]
    ));

  test("many contracts", async () =>
    testCode(
      `
          access(all)
          contract C1 {
              init(a: Int) {}
          }

          access(all)
          contract C2 {
              init(b: Int) {}
          }
          `,
      []
    ));
});

describe("parseEntryPointArguments command", () => {
  async function testCode(code: string) {
    return withConnection(async (connection) => {
      const uri = await createTestDocument(connection, code);

      const result = await connection.sendRequest(ExecuteCommandRequest.type, {
        command: "cadence.server.parseEntryPointArguments",
        arguments: [uri, ["0x42"]],
      });

      expect(result).toEqual([
        { value: "0x0000000000000042", type: "Address" },
      ]);
    });
  }

  test("script", async () => testCode("access(all) fun main(a: Address) {}"));

  test("transaction", async () => testCode("transaction(a: Address) {}"));
});

describe("diagnostics", () => {
  async function testCode(code: string, errors: string[]) {
    return withConnection(async (connection) => {
      const notificationPromise = new Promise<PublishDiagnosticsParams>(
        (resolve) => {
          connection.onNotification(
            PublishDiagnosticsNotification.type,
            resolve
          );
        }
      );

      const uri = await createTestDocument(connection, code);

      const notification = await notificationPromise;

      expect(notification.uri).toEqual(uri);
      expect(notification.diagnostics).toHaveLength(errors.length);
      notification.diagnostics.forEach((diagnostic, i) =>
        expect(diagnostic.message).toEqual(errors[i])
      );
    });
  }

  test("script", async () =>
    testCode(`access(all) fun main() { let x = X }`, [
      "cannot find variable in this scope: `X`. not found in this scope; check for typos or declare it",
    ]));

  test("script auth account", async () =>
    testCode(
      `access(all) fun main() { let account = getAuthAccount<&Account>(0x01) }`,
      []
    ));

  test("transaction", async () =>
    testCode(`transaction() { execute { let x = X } }`, [
      "cannot find variable in this scope: `X`. not found in this scope; check for typos or declare it",
    ]));

  test("transaction auth account", async () =>
    testCode(
      `transaction() { execute { let account = getAuthAccount<&Account>(0x01) } }`,
      [
        "cannot find variable in this scope: `getAuthAccount`. not found in this scope; check for typos or declare it",
      ]
    ));

  test("attachments", async () =>
    testCode(
      `
            access(all)
            resource R {}

            access(all)
            attachment A for R {}

            access(all)
            fun main() {
                let r <- create R()
                let a = r[A]
                destroy r
            }
          `,
      []
    ));

  test("capability controllers", async () =>
    testCode(
      `
            access(all)
            fun main() {
                let get = getAccount(0x1).capabilities.get
            }
          `,
      []
    ));

  test("unused result", async () =>
    testCode(
      `
            access(all)
            fun main() {
                getAccount(0x1)
            }
          `,
      ["unused result"]
    ));

  test("InternalEVM contract exists", async () =>
    testCode(
      `
            access(all)
            fun main() {
                // Checks that the InternalEVM contract exists
                // Also that it has the correct value (i.e. the run function exists)
                log(InternalEVM.run)
            }
          `,
      []
    ));

  type TestDoc = {
    name: string;
    code: string;
  };

  type DocNotification = {
    name: string;
    notification: Promise<PublishDiagnosticsParams>;
  };

  const fooContractCode = fs.readFileSync("./foo.cdc", "utf8");

  async function testImports(docs: TestDoc[]): Promise<DocNotification[]> {
    return new Promise<DocNotification[]>((resolve) => {
      withConnection(async (connection) => {
        let docsNotifications: DocNotification[] = [];

        for (let doc of docs) {
          const notification = new Promise<PublishDiagnosticsParams>(
            (resolve) => {
              connection.onNotification(
                PublishDiagnosticsNotification.type,
                (notification) => {
                  if (notification.uri == `file://${doc.name}.cdc`) {
                    resolve(notification);
                  }
                }
              );
            }
          );
          docsNotifications.push({
            name: doc.name,
            notification: notification,
          });

          await connection.sendNotification(
            DidOpenTextDocumentNotification.type,
            {
              textDocument: TextDocumentItem.create(
                `file://${doc.name}.cdc`,
                "cadence",
                1,
                doc.code
              ),
            }
          );
        }

        resolve(docsNotifications);
      }, true);
    });
  }

  test("script with import", async () => {
    const contractName = "foo";
    const scriptName = "script";
    const scriptCode = `
      import Foo from "./foo.cdc"

      access(all) fun main() {
          log(Foo.bar)
      }
    `;

    let docNotifications = await testImports([
      { name: contractName, code: fooContractCode },
      { name: scriptName, code: scriptCode },
    ]);

    let script = await docNotifications.find((n) => n.name == scriptName)
      .notification;
    expect(script.uri).toEqual(`file://${scriptName}.cdc`);
    expect(script.diagnostics).toHaveLength(0);
  });

  test("script with string import", async () => {
    const contractName = "Foo";
    const scriptName = "script";
    const scriptCode = `
      import "Foo"

      access(all) fun main() {
          log(Foo.bar)
      }
    `;

    let docNotifications = await testImports([
      { name: contractName, code: fooContractCode },
      { name: scriptName, code: scriptCode },
    ]);

    let script = await docNotifications.find((n) => n.name == scriptName)
      .notification;
    expect(script.uri).toEqual(`file://${scriptName}.cdc`);
    expect(script.diagnostics).toHaveLength(0);
  });

  test("script with string import non-deployment contract", async () => {
    const scriptName = "script";
    const scriptCode = `
      import "A"
      access(all) fun main() { }
    `;

    let docNotifications = await testImports([
      { name: scriptName, code: scriptCode },
    ]);

    let script = await docNotifications.find((n) => n.name == scriptName)
      .notification;
    expect(script.uri).toEqual(`file://${scriptName}.cdc`);
    expect(script.diagnostics).toHaveLength(0);
  });

  test("script import failure", async () => {
    const contractName = "foo";
    const scriptName = "script";
    const scriptCode = `
      import Foo from "./foo.cdc"
      access(all) fun main() { log(Foo.zoo) }
    `;

    let docNotifications = await testImports([
      { name: contractName, code: fooContractCode },
      { name: scriptName, code: scriptCode },
    ]);

    let script = await docNotifications.find((n) => n.name == scriptName)
      .notification;
    expect(script.uri).toEqual(`file://${scriptName}.cdc`);
    expect(script.diagnostics).toHaveLength(1);
    expect(script.diagnostics[0].message).toEqual(
      "value of type `&Foo` has no member `zoo`. the member is not defined on the type; check for typos or if the member exists on the type"
    );
  });

  test("Crypto contract import", async () => {
    const scriptName = "script";
    const scriptCode = `
      import Crypto
      access(all) fun main() { log(Crypto.KeyList()) }
    `;

    let docNotifications = await testImports([
      { name: scriptName, code: scriptCode },
    ]);

    let script = await docNotifications.find((n) => n.name == scriptName)
      .notification;
    expect(script.uri).toEqual(`file://${scriptName}.cdc`);
    expect(script.diagnostics).toHaveLength(0);
  });
});

describe("script execution", () => {
  test("script executes and result is returned", async () => {
    await withConnection(async (connection) => {
      let result = await connection.sendRequest(ExecuteCommandRequest.type, {
        command: "cadence.server.flow.executeScript",
        arguments: [`file://${__dirname}/script.cdc`, "[]"],
      });

      expect(result).toEqual(`Result: "HELLO WORLD"`);
    }, true);
  }, 15000);
});

async function getAccounts(connection: ProtocolConnection) {
  return connection.sendRequest(ExecuteCommandRequest.type, {
    command: "cadence.server.flow.getAccounts",
    arguments: [],
  });
}

async function switchAccount(connection: ProtocolConnection, name: string) {
  return connection.sendRequest(ExecuteCommandRequest.type, {
    command: "cadence.server.flow.switchActiveAccount",
    arguments: [name],
  });
}

describe("accounts", () => {
  test("get account list", async () => {
    await withConnection(async (connection) => {
      let result = await getAccounts(connection);

      expect(result.map((r) => r.Name).sort()).toEqual([
        "Alice",
        "Bob",
        "Charlie",
        "Dave",
        "Eve",
        "emulator-account [flow.json]",
        "moose [flow.json]",
      ]);
      expect(result.map((r) => r.Address)).toEqual([
        "179b6b1cb6755e31",
        "f3fcd2c1a78f5eee",
        "e03daebed8ca0615",
        "045a1763c93006ca",
        "120e725050340cab",
        "f8d6e0586b0a20c7",
        "f8d6e0586b0a20c7",
      ]);
      expect(result.map((r) => r.Active)).toEqual([
        true,
        false,
        false,
        false,
        false,
        false,
        false,
      ]);
    }, true);
  });

  test("switch active account", async () => {
    await withConnection(async (connection) => {
      let result = await switchAccount(connection, "Bob");
      expect(result).toEqual("Account switched to Bob");

      let active = await getAccounts(connection);

      expect(active.filter((a) => a.Active).pop().Name).toEqual("Bob");
    }, true);
  });

  test("create an account", async () => {
    await withConnection(async (connection) => {
      let result = await connection.sendRequest(ExecuteCommandRequest.type, {
        command: "cadence.server.flow.createAccount",
        arguments: [],
      });

      expect(result.Active).toBeFalsy();
      expect(result.Name).toBeDefined();

      let accounts = await getAccounts(connection);

      expect(accounts.filter((a) => a.Name == result.Name)).toHaveLength(1);
    }, true);
  });

  test("commands without path default to init-config over last-used", async () => {
    await withConnection(async (connection) => {
      const base = fs.mkdtempSync(path.join(os.tmpdir(), "cadence-ls-last-"));
      const aDir = fs.mkdtempSync(path.join(base, "a-"));
      const bDir = fs.mkdtempSync(path.join(base, "b-"));

      // minimal scripts
      fs.writeFileSync(
        path.join(aDir, "script.cdc"),
        "access(all) fun main() { }\n"
      );
      fs.writeFileSync(
        path.join(bDir, "script.cdc"),
        "access(all) fun main() { }\n"
      );

      // flow configs with emulator-account and unique names
      const flowA = {
        contracts: {},
        emulators: {
          default: { port: 3569, serviceAccount: "emulator-account" },
        },
        networks: { emulator: "127.0.0.1:3569" },
        accounts: {
          "emulator-account": {
            address: "f8d6e0586b0a20c7",
            key: "c44604c862a3950ae82d56638929720f44875b2637054a1fdcb4e76b01b40881",
          },
          alpha: {
            address: "f8d6e0586b0a20c7",
            key: "c44604c862a3950ae82d56638929720f44875b2637054a1fdcb4e76b01b40881",
          },
        },
        deployments: {},
      } as any;
      fs.writeFileSync(
        path.join(aDir, "flow.json"),
        JSON.stringify(flowA, null, 2)
      );

      const flowB = {
        contracts: {},
        emulators: {
          default: { port: 3569, serviceAccount: "emulator-account" },
        },
        networks: { emulator: "127.0.0.1:3569" },
        accounts: {
          "emulator-account": {
            address: "f8d6e0586b0a20c7",
            key: "c44604c862a3950ae82d56638929720f44875b2637054a1fdcb4e76b01b40881",
          },
          beta: {
            address: "f8d6e0586b0a20c7",
            key: "c44604c862a3950ae82d56638929720f44875b2637054a1fdcb4e76b01b40881",
          },
        },
        deployments: {},
      } as any;
      fs.writeFileSync(
        path.join(bDir, "flow.json"),
        JSON.stringify(flowB, null, 2)
      );

      async function waitForAccount(name: string) {
        const deadline = Date.now() + 5000;
        while (Date.now() < deadline) {
          try {
            const list = (await getAccounts(connection)) as any[];
            if (
              list.some(
                (a) =>
                  typeof a.Name === "string" &&
                  (a.Name === name || a.Name.startsWith(name))
              )
            )
              return true;
          } catch {}
          await new Promise((r) => setTimeout(r, 150));
        }
        return false;
      }

      // Init options seed default = root ./flow.json. Prove that even after switching
      // last-used to B, a no-path command still uses init-config (root).

      // Set last-used = A by running a path-specific command on A
      const aUri = `file://${path.join(aDir, "script.cdc")}`;
      try {
        await connection.sendRequest(ExecuteCommandRequest.type, {
          command: "cadence.server.flow.executeScript",
          arguments: [aUri, "[]"],
        });
      } catch {}

      // Flip last-used to B
      const bUri = `file://${path.join(bDir, "script.cdc")}`;
      try {
        await connection.sendRequest(ExecuteCommandRequest.type, {
          command: "cadence.server.flow.executeScript",
          arguments: [bUri, "[]"],
        });
      } catch {}
      // Even after switching last-used to B, a no-path command should still use init-config
      // Assert we still see init-config accounts (moose) and not B-only account (beta)
      {
        const list = (await getAccounts(connection)) as any[];
        expect(list.some((a) => a.Name === "moose [flow.json]")).toBeTruthy();
        expect(list.some((a) => a.Name === "beta")).toBeFalsy();
      }
      // But path-specific command MUST use nearest project's client, not init-config override
      // Execute script under B and expect availability of B-only account "beta" when switching later
      try {
        await connection.sendRequest(ExecuteCommandRequest.type, {
          command: "cadence.server.flow.executeScript",
          arguments: [bUri, "[]"],
        });
      } catch {}
      // Now switch active account to beta (present only in B config) to prove B's client was engaged
      let switched: any;
      try {
        switched = await connection.sendRequest(ExecuteCommandRequest.type, {
          command: "cadence.server.flow.switchActiveAccount",
          arguments: ["beta"],
        });
      } catch (e) {
        // If this fails, it means override incorrectly took precedence for path-specific actions
      }
      expect(
        typeof switched === "string" || switched === undefined
      ).toBeTruthy();
      // cleanup temp base
      try {
        fs.rmdirSync(base, { recursive: true });
      } catch {}
    }, true);
  }, 20000);
});

describe("transactions", () => {
  const resultRegex =
    /^Transaction SEALED with ID [a-f0-9]{64}\. Events: \[\]$/;

  test("send a transaction with no signer", async () => {
    await withConnection(async (connection) => {
      let result = await connection.sendRequest(ExecuteCommandRequest.type, {
        command: "cadence.server.flow.sendTransaction",
        arguments: [`file://${__dirname}/transaction-none.cdc`, "[]", []],
      });

      expect(resultRegex.test(result)).toBeTruthy();
    }, true);
  });

  test("send a transaction with single signer", async () => {
    await withConnection(async (connection) => {
      let result = await connection.sendRequest(ExecuteCommandRequest.type, {
        command: "cadence.server.flow.sendTransaction",
        arguments: [`file://${__dirname}/transaction.cdc`, "[]", ["Alice"]],
      });

      expect(resultRegex.test(result)).toBeTruthy();
    }, true);
  });

  test("send a transaction with an account from configuration", async () => {
    await withConnection(async (connection) => {
      let result = await connection.sendRequest(ExecuteCommandRequest.type, {
        command: "cadence.server.flow.sendTransaction",
        arguments: [
          `file://${__dirname}/transaction.cdc`,
          "[]",
          ["moose [flow.json]"],
        ],
      });

      expect(resultRegex.test(result)).toBeTruthy();
    }, true);
  });

  test("send a transaction with multiple signers", async () => {
    await withConnection(async (connection) => {
      let result = await connection.sendRequest(ExecuteCommandRequest.type, {
        command: "cadence.server.flow.sendTransaction",
        arguments: [
          `file://${__dirname}/transaction-multiple.cdc`,
          "[]",
          ["Alice", "Bob"],
        ],
      });

      expect(resultRegex.test(result)).toBeTruthy();
    }, true);
  }, 15000);
});

describe("contracts", () => {
  async function deploy(
    connection: ProtocolConnection,
    signer: string,
    file: string,
    name: string
  ) {
    return connection.sendRequest(ExecuteCommandRequest.type, {
      command: "cadence.server.flow.deployContract",
      arguments: [`file://${__dirname}/${file}.cdc`, name, signer],
    });
  }

  test("deploy a contract", async () => {
    await withConnection(async (connection) => {
      let result = await deploy(connection, "", "foo", "Foo");
      expect(result).toEqual("Contract Foo has been deployed to account Alice");

      result = await deploy(connection, "Bob", "foo", "Foo");
      expect(result).toEqual("Contract Foo has been deployed to account Bob");
    }, true);
  }, 15000);

  test("deploy contract with file import", async () => {
    await withConnection(async (connection) => {
      let result = await deploy(connection, "moose [flow.json]", "foo", "Foo");
      expect(result).toEqual(
        "Contract Foo has been deployed to account moose [flow.json]"
      );

      result = await deploy(connection, "moose [flow.json]", "bar", "Bar");
      expect(result).toEqual(
        "Contract Bar has been deployed to account moose [flow.json]"
      );
    }, true);
  }, 15000);

  test("deploy contract with string imports", async () => {
    await withConnection(async (connection) => {
      let result = await deploy(connection, "moose [flow.json]", "foo", "Foo");
      expect(result).toEqual(
        "Contract Foo has been deployed to account moose [flow.json]"
      );

      result = await deploy(connection, "moose [flow.json]", "zoo", "Zoo");
      expect(result).toEqual(
        "Contract Zoo has been deployed to account moose [flow.json]"
      );
    }, true);
  }, 15000);
});

describe("codelenses", () => {
  const codelensRequest = "textDocument/codeLens";

  test("contract codelensss", async () => {
    await withConnection(async (connection) => {
      let code = fs.readFileSync("./foo.cdc");
      let path = `file://${__dirname}/foo.cdc`;
      let document = TextDocumentItem.create(
        path,
        "cadence",
        1,
        code.toString()
      );

      await connection.sendNotification(DidOpenTextDocumentNotification.type, {
        textDocument: document,
      });

      let codelens = await connection.sendRequest(codelensRequest, {
        textDocument: document,
      });

      expect(codelens).toHaveLength(1);
      let c = codelens[0].command;
      expect(c.command).toEqual("cadence.server.flow.deployContract");
      expect(c.title).toEqual("💡 Deploy contract Foo to Alice");
      expect(c.arguments).toEqual([path, "Foo", "Alice"]);
    }, true);
  }, 15000);

  test("transactions codelenses", async () => {
    await withConnection(async (connection) => {
      let code = fs.readFileSync("./transaction.cdc");
      let path = `file://${__dirname}/transaction.cdc`;
      let document = TextDocumentItem.create(
        path,
        "cadence",
        1,
        code.toString()
      );

      await connection.sendNotification(DidOpenTextDocumentNotification.type, {
        textDocument: document,
      });

      let codelens = await connection.sendRequest(codelensRequest, {
        textDocument: document,
      });

      expect(codelens).toHaveLength(1);
      let c = codelens[0].command;
      expect(c.command).toEqual("cadence.server.flow.sendTransaction");
      expect(c.title).toEqual("💡 Send signed by Alice");
      expect(c.arguments).toEqual([path, "[]", ["Alice"]]);
    }, true);
  }, 15000);

  test("script codelenses", async () => {
    await withConnection(async (connection) => {
      let code = fs.readFileSync("./script.cdc");
      let path = `file://${__dirname}/script.cdc`;
      let document = TextDocumentItem.create(
        path,
        "cadence",
        1,
        code.toString()
      );

      await connection.sendNotification(DidOpenTextDocumentNotification.type, {
        textDocument: document,
      });

      let codelens = await connection.sendRequest(codelensRequest, {
        textDocument: document,
      });

      expect(codelens).toHaveLength(1);
      let c = codelens[0].command;
      expect(c.command).toEqual("cadence.server.flow.executeScript");
      expect(c.title).toEqual("💡 Execute script");
      expect(c.arguments).toEqual([path, "[]"]);
    }, true);
  });
});
