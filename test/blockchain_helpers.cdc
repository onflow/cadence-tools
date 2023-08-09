import Test

pub struct BlockchainHelpers {
    pub let blockchain: Test.Blockchain

    init(blockchain: Test.Blockchain) {
        self.blockchain = blockchain
    }

    /// Returns the current block height of the blockchain.
    ///
    pub fun getCurrentBlockHeight(): UInt64 {
        let script = Test.readFile("\u{0}helper/get_current_block_height.cdc")
        let scriptResult = self.blockchain.executeScript(script, [])

        if scriptResult.status == Test.ResultStatus.failed {
            panic(scriptResult.error!.message)
        }
        return scriptResult.returnValue! as! UInt64
    }

    /// Returns the Flow token balance for the given account.
    ///
    pub fun getFlowBalance(for account: Test.Account): UFix64 {
        let script = Test.readFile("\u{0}helper/get_flow_balance.cdc")
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
    pub fun mintFlow(
        to receiver: Test.Account,
        amount: UFix64
    ): Test.TransactionResult {
        let code = Test.readFile("\u{0}helper/mint_flow.cdc")
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
    pub fun burnFlow(
        from account: Test.Account,
        amount: UFix64
    ): Test.TransactionResult {
        let code = Test.readFile("\u{0}helper/burn_flow.cdc")
        let tx = Test.Transaction(
            code: code,
            authorizers: [account.address],
            signers: [account],
            arguments: [amount]
        )

        return self.blockchain.executeTransaction(tx)
    }
}
