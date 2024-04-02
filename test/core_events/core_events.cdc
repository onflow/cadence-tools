access(all)
contract CoreEvents {

    access(all)
    event AccountCreated(address: Address)

    access(all)
    event BlockExecuted(
        height: UInt64,
        hash: String,
        totalSupply: Int,
        parentHash: String,
        receiptRoot: String,
        transactionHashes: [String]
    )

    access(all)
    event TransactionExecuted(
        blockHeight: UInt64,
        blockHash: String,
        transactionHash: String,
        encodedTransaction: String,
        failed: Bool,
        vmError: String,
        transactionType: UInt8,
        gasConsumed: UInt64,
        deployedContractAddress: String,
        returnedValue: String,
        logs: String
    )
}
