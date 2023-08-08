import "FungibleToken"
import "FlowToken"

pub fun main(address: Address): UFix64 {
    let balanceRef = getAccount(address)
        .getCapability(/public/flowTokenBalance)
        .borrow<&FlowToken.Vault{FungibleToken.Balance}>()
        ?? panic("Could not borrow FungibleToken.Balance reference")

    return balanceRef.balance
}
