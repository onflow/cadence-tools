import "FungibleToken"
import "FlowToken"

transaction(amount: UFix64) {
    prepare(account: AuthAccount) {
        let flowVault = account.borrow<&FlowToken.Vault>(
            from: /storage/flowTokenVault
        ) ?? panic("Could not borrow BlpToken.Vault reference)")

        let tokens <- flowVault.withdraw(amount: amount)
        destroy <- tokens
    }
}
