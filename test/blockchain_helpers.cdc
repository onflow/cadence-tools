import Test

access(all)
struct BlockchainHelpers {

    access(all)
    let blockchain: Test.Blockchain

    init(blockchain: Test.Blockchain) {
        self.blockchain = blockchain
    }

    /// Returns the current block height of the blockchain.
    ///
    access(all)
    fun getCurrentBlockHeight(): UInt64 {
        let script = self.readFile("get_current_block_height.cdc")
        let scriptResult = self.blockchain.executeScript(script, [])

        if scriptResult.status == Test.ResultStatus.failed {
            panic(scriptResult.error!.message)
        }
        return scriptResult.returnValue! as! UInt64
    }

    /// Returns the Flow token balance for the given account.
    ///
    access(all)
    fun getFlowBalance(account: Test.TestAccount): UFix64 {
        let script = self.readFile("get_flow_balance.cdc")
        let scriptResult = self.blockchain.executeScript(script, [account.address])

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
        let code = self.readFile("mint_flow.cdc")
        let tx = Test.Transaction(
            code: code,
            authorizers: [self.blockchain.serviceAccount().address],
            signers: [],
            arguments: [receiver.address, amount]
        )

        return self.blockchain.executeTransaction(tx)
    }

    /// Burns the specified amount of Flow tokens for the given
    /// test account. Returns the result of the transaction.
    ///
    access(all)
    fun burnFlow(
        from account: Test.TestAccount,
        amount: UFix64
    ): Test.TransactionResult {
        let code = self.readFile("burn_flow.cdc")
        let tx = Test.Transaction(
            code: code,
            authorizers: [account.address],
            signers: [account],
            arguments: [amount]
        )

        return self.blockchain.executeTransaction(tx)
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
}
