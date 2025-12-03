import Test

/// Returns the current block height of the blockchain.
///
access(all)
fun getCurrentBlockHeight(): UInt64 {
    let script = readFile("get_current_block_height.cdc")
    let scriptResult = Test.executeScript(script, [])

    if scriptResult.status == Test.ResultStatus.failed {
        panic(scriptResult.error!.message)
    }
    return scriptResult.returnValue! as! UInt64
}

/// Returns the Flow token balance for the given account.
///
access(all)
fun getFlowBalance(account: Test.TestAccount): UFix64 {
    let script = readFile("get_flow_balance.cdc")
    let scriptResult = Test.executeScript(script, [account.address])

    if scriptResult.status == Test.ResultStatus.failed {
        panic(scriptResult.error!.message)
    }
    return scriptResult.returnValue! as! UFix64
}

/// Mints the given amount of Flow tokens to a specified test account.
/// The transaction is authorized and signed by the service account.
/// Returns the result of the transaction.
///
access(all)
fun mintFlow(
    to receiver: Test.TestAccount,
    amount: UFix64
): Test.TransactionResult {
    let code = readFile("mint_flow.cdc")
    let tx = Test.Transaction(
        code: code,
        authorizers: [Test.serviceAccount().address],
        signers: [],
        arguments: [receiver.address, amount]
    )

    return Test.executeTransaction(tx)
}

/// Burns the specified amount of Flow tokens for the given
/// test account. Returns the result of the transaction.
///
access(all)
fun burnFlow(
    from account: Test.TestAccount,
    amount: UFix64
): Test.TransactionResult {
    let code = readFile("burn_flow.cdc")
    let tx = Test.Transaction(
        code: code,
        authorizers: [account.address],
        signers: [account],
        arguments: [amount]
    )

    return Test.executeTransaction(tx)
}

/// Executes a script and returns the script return value and the status.
/// `returnValue` field of the result will be `nil` if the script failed.
///
access(all)
fun executeScript(
    _ path: String,
    _ arguments: [AnyStruct]
): Test.ScriptResult {
    let script = Test.readFile(path)
    return Test.executeScript(script, arguments)
}

/// Executes a given transaction, commits the current block
/// and returns the result of transaction execution.
///
access(all)
fun executeTransaction(
    _ path: String,
    _ arguments: [AnyStruct],
    _ account: Test.TestAccount
): Test.TransactionResult {
    let code = Test.readFile(path)
    let tx = Test.Transaction(
        code: code,
        authorizers: [account.address],
        signers: [account],
        arguments: arguments
    )

    return Test.executeTransaction(tx)
}

/// Reads the code for the script/transaction with the given
/// file name and returns its content as a String.
///
access(self)
fun readFile(_ name: String): String {
    // The "\u{0}helper/" prefix is used in order to prevent
    // conflicts with user-defined scripts/transactions.
    return Test.readFile("\u{0}helper/".concat(name))
}
