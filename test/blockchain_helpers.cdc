import Test

/// Returns the Flow token balance for the given account.
///
pub fun getFlowBalance(
    for account: Test.Account,
    blockchain: Test.Blockchain
): UFix64 {
    let script = Test.readFile("get_flow_balance.cdc")
    let scriptResult = blockchain.executeScript(script, [account.address])

    if scriptResult.status == Test.ResultStatus.failed {
        panic(scriptResult.error!.message)
    }
    return scriptResult.returnValue! as! UFix64
}

/// Returns the current block height of the blockchain.
///
pub fun getCurrentBlockHeight(blockchain: Test.Blockchain): UInt64 {
    let script = Test.readFile("get_current_block_height.cdc")
    let scriptResult = blockchain.executeScript(script, [])

    if scriptResult.status == Test.ResultStatus.failed {
        panic(scriptResult.error!.message)
    }
    return scriptResult.returnValue! as! UInt64
}

/// Mints the given amount of Flow tokens to a specified test account.
/// The transaction is authorized and signed by the service account.
/// Returns the result of the transaction.
///
pub fun mintFlow(
    to receiver: Test.Account,
    amount: UFix64,
    blockchain: Test.Blockchain
): Test.TransactionResult {
    let code = Test.readFile("mint_flow.cdc")
    let tx = Test.Transaction(
        code: code,
        authorizers: [blockchain.serviceAccount().address],
        signers: [],
        arguments: [receiver.address, amount]
    )

    return blockchain.executeTransaction(tx)
}

/// Burns the specified amount of Flow tokens for the given
/// test account. Returns the result of the transaction.
///
pub fun burnFlow(
    from account: Test.Account,
    amount: UFix64,
    blockchain: Test.Blockchain
): Test.TransactionResult {
    let code = Test.readFile("burn_flow.cdc")
    let tx = Test.Transaction(
        code: code,
        authorizers: [account.address],
        signers: [account],
        arguments: [amount]
    )

    return blockchain.executeTransaction(tx)
}
