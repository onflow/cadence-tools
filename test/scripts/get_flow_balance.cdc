import "FungibleToken"
import "FlowToken"

access(all) fun main(address: Address): UFix64 {
    let balanceRef = getAccount(address)
        .capabilities.borrow<&FlowToken.Vault>(/public/flowTokenBalance)
        ?? panic("Could not borrow FungibleToken.Balance reference")

    return balanceRef.balance
}
