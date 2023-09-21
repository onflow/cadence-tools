import "FungibleToken"
import "FlowToken"

transaction(amount: UFix64) {
    prepare(account: auth(BorrowValue) &Account) {
        let flowVault = account.storage.borrow<auth(FungibleToken.Withdrawable) &FlowToken.Vault>(
            from: /storage/flowTokenVault
        ) ?? panic("Could not borrow BlpToken.Vault reference)")

        let tokens <- flowVault.withdraw(amount: amount)
        destroy <- tokens
    }
}
