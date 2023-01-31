import Foo from "./foo.cdc"

pub contract Bar {
     pub let x: String
     init() {
       self.x = Foo.bar
     }
}